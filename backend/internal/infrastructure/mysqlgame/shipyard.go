package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type ShipyardRepository struct {
	queryer Queryer
	prefix  string
}

func NewShipyardRepository(db *sql.DB, prefix string) ShipyardRepository {
	return ShipyardRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewShipyardRepositoryWithQueryer(queryer Queryer, prefix string) ShipyardRepository {
	return ShipyardRepository{queryer: queryer, prefix: prefix}
}

func (r ShipyardRepository) GetShipyard(ctx context.Context, query appgame.ShipyardQuery) (domaingame.Shipyard, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return domaingame.Shipyard{}, err
	}

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Shipyard{}, err
	}

	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	fleet, err := r.loadFleetCounts(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	speed, orderCap, err := r.loadShipyardUniverseConfig(ctx)
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	busy, err := r.loadShipyardBusy(ctx, buildQueueTable, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Shipyard{}, err
	}

	return domaingame.BuildShipyard(overview, levels, research, fleet, speed, busy, orderCap), nil
}

func (r ShipyardRepository) loadFleetCounts(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.FleetCounts, error) {
	ids := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", numericColumns(ids), planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("fleet counts not found")
	}
	counts, err := scanFleetMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func (r ShipyardRepository) loadShipyardUniverseConfig(ctx context.Context) (float64, int, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return 0, 0, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT speed, max_werf FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 1, 1000, nil
	}
	var speed float64
	var orderCap int
	if err := rows.Scan(&speed, &orderCap); err != nil {
		return 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if speed <= 0 {
		speed = 1
	}
	if orderCap <= 0 {
		orderCap = 1000
	}
	return speed, orderCap, nil
}

func (r ShipyardRepository) loadShipyardBusy(ctx context.Context, buildQueueTable string, planetID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT tech_id FROM %s WHERE planet_id = ? ORDER BY list_id ASC LIMIT 1", buildQueueTable), planetID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var techID int
	if err := rows.Scan(&techID); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return techID == domaingame.BuildingShipyard || techID == domaingame.BuildingNaniteFactory, nil
}

func scanFleetMap(rows Rows, ids []int) (domaingame.FleetCounts, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	counts := make(domaingame.FleetCounts, len(ids))
	for index, id := range ids {
		counts[id] = values[index]
	}
	return counts, nil
}

package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type DefenseRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

func NewDefenseRepository(db *sql.DB, prefix string) DefenseRepository {
	runner := SQLQueryer{DB: db}
	return DefenseRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewDefenseRepositoryWithQueryer(queryer Queryer, prefix string) DefenseRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewDefenseRepositoryWithRunner(queryer, execer, prefix, time.Now)
}

func NewDefenseRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) DefenseRepository {
	if now == nil {
		now = time.Now
	}
	return DefenseRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r DefenseRepository) GetDefense(ctx context.Context, query appgame.DefenseQuery) (domaingame.Defense, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Defense{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Defense{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return domaingame.Defense{}, err
	}
	if r.execer != nil {
		shipyard := ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
		if err := shipyard.FinishDueShipyardQueues(ctx, int(shipyard.currentTime().Unix())); err != nil {
			return domaingame.Defense{}, err
		}
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Defense{}, err
	}

	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Defense{}, err
	}
	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Defense{}, err
	}
	defense, err := r.loadDefenseCounts(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Defense{}, err
	}
	shipyard := ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
	speed, orderCap, err := shipyard.loadShipyardUniverseConfig(ctx)
	if err != nil {
		return domaingame.Defense{}, err
	}
	busy, err := shipyard.loadShipyardBusy(ctx, buildQueueTable, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Defense{}, err
	}

	return domaingame.BuildDefense(overview, levels, research, defense, speed, busy, orderCap), nil
}

func (r DefenseRepository) loadDefenseCounts(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.DefenseCounts, error) {
	ids := domaingame.DefenseIDs()
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
		return nil, errors.New("defense counts not found")
	}
	counts, err := scanDefenseMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func scanDefenseMap(rows Rows, ids []int) (domaingame.DefenseCounts, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	counts := make(domaingame.DefenseCounts, len(ids))
	for index, id := range ids {
		counts[id] = values[index]
	}
	return counts, nil
}

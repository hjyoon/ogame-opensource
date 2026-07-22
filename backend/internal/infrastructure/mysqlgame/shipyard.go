package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type ShipyardRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

func NewShipyardRepository(db *sql.DB, prefix string) ShipyardRepository {
	runner := SQLQueryer{DB: db}
	return ShipyardRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewShipyardReadRepository(db *sql.DB, prefix string) ShipyardRepository {
	return NewShipyardRepositoryWithRunner(SQLQueryer{DB: db}, nil, prefix, time.Now)
}

func NewShipyardRepositoryWithQueryer(queryer Queryer, prefix string) ShipyardRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewShipyardRepositoryWithRunner(queryer, execer, prefix, time.Now)
}

func NewShipyardRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) ShipyardRepository {
	if now == nil {
		now = time.Now
	}
	return ShipyardRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
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
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	now := int(r.currentTime().Unix())
	if r.execer != nil {
		if err := r.FinishDueShipyardQueues(ctx, now); err != nil {
			return domaingame.Shipyard{}, err
		}
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
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
	commanderActive, err := (BuildingsRepository{queryer: r.queryer}).loadBuildingCommanderActive(ctx, usersTable, query.PlayerID, now)
	if err != nil {
		return domaingame.Shipyard{}, err
	}
	queue, err := r.loadShipyardQueueEntries(ctx, queueTable, overview.CurrentPlanet.ID, now)
	if err != nil {
		return domaingame.Shipyard{}, err
	}

	return domaingame.BuildShipyardWithQueue(overview, levels, research, fleet, speed, busy, orderCap, commanderActive, queue), nil
}

func (r ShipyardRepository) GetMCPShipyardOptions(ctx context.Context, playerID int, planetID int) (domainmcp.ShipyardOptions, error) {
	if r.queryer == nil {
		return domainmcp.ShipyardOptions{}, errors.New("shipyard reader unavailable")
	}
	shipyard, err := r.GetShipyard(ctx, appgame.ShipyardQuery{PlayerID: playerID, PlanetID: planetID})
	if err != nil {
		return domainmcp.ShipyardOptions{}, err
	}
	queue := make([]domainmcp.ShipyardQueueEntry, 0, len(shipyard.Queue))
	for index, entry := range shipyard.Queue {
		queue = append(queue, domainmcp.ShipyardQueueEntry{
			TaskID:           entry.TaskID,
			UnitID:           entry.UnitID,
			Name:             entry.Name,
			Count:            entry.Count,
			Start:            entry.Start,
			End:              entry.End,
			RemainingSeconds: entry.RemainingSeconds,
			Status:           mcpQueueStatus(entry.RemainingSeconds, index > 0),
		})
	}
	items := make([]domainmcp.ShipyardOption, 0, len(shipyard.Items))
	for _, item := range shipyard.Items {
		items = append(items, domainmcp.ShipyardOption{
			ID:               item.ID,
			Name:             item.Name,
			Description:      item.Description,
			Count:            item.Count,
			Cost:             mcpTechnologyCost(item.Cost),
			DurationSeconds:  item.DurationSeconds,
			CanBuild:         item.CanBuild,
			MeetsRequirement: item.MeetsRequirement,
			MaxBuild:         item.MaxBuild,
			BlockedReason:    item.BlockedReason,
		})
	}
	return domainmcp.ShipyardOptions{
		PlayerID: playerID,
		Planet: domainmcp.Planet{
			ID:       shipyard.CurrentPlanet.ID,
			Name:     shipyard.CurrentPlanet.Name,
			Type:     shipyard.CurrentPlanet.Type,
			TypeName: mcpPlanetTypeName(shipyard.CurrentPlanet.Type),
			Coordinates: domainmcp.Coordinates{
				Galaxy:   shipyard.CurrentPlanet.Coordinates.Galaxy,
				System:   shipyard.CurrentPlanet.Coordinates.System,
				Position: shipyard.CurrentPlanet.Coordinates.Position,
			},
			Current: true,
		},
		CommanderActive: shipyard.CommanderActive,
		HasShipyard:     shipyard.HasShipyard,
		Busy:            shipyard.Busy,
		Queue:           queue,
		Items:           items,
	}, nil
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

func (r ShipyardRepository) loadShipyardQueueEntries(ctx context.Context, queueTable string, planetID int, now int) ([]domaingame.ShipyardQueueEntry, error) {
	rows, err := r.loadShipyardQueueTasks(ctx, queueTable, planetID)
	if err != nil {
		return nil, err
	}
	entries := make([]domaingame.ShipyardQueueEntry, 0, len(rows))
	for _, row := range rows {
		name := domaingame.TechnologyName(row.ObjID)
		if name == "" {
			continue
		}
		remaining := row.End - now
		if remaining < 0 {
			remaining = 0
		}
		entries = append(entries, domaingame.ShipyardQueueEntry{
			TaskID:           row.TaskID,
			UnitID:           row.ObjID,
			Name:             name,
			Count:            row.Level,
			Start:            row.Start,
			End:              row.End,
			RemainingSeconds: remaining,
		})
	}
	return entries, nil
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

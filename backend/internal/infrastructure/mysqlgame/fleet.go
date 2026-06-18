package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	queueTypeFleet            = "Fleet"
	legacyPlanetTypeAbandoned = 10004
)

type FleetRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewFleetRepository(db *sql.DB, prefix string) FleetRepository {
	return FleetRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewFleetRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) FleetRepository {
	if now == nil {
		now = time.Now
	}
	return FleetRepository{queryer: queryer, prefix: prefix, now: now}
}

func (r FleetRepository) GetFleet(ctx context.Context, query appgame.FleetQuery) (domaingame.Fleet, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.Fleet{}, err
	}

	overviewRepository := OverviewRepository{queryer: r.queryer, prefix: r.prefix}
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Fleet{}, err
	}

	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	counts, err := ShipyardRepository{queryer: r.queryer, prefix: r.prefix}.loadFleetCounts(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	admiral, err := r.loadAdmiral(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	acsEnabled, err := r.loadACSEnabled(ctx, uniTable)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	missions, err := r.loadActiveMissions(ctx, queueTable, fleetTable, planetsTable, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}

	return domaingame.BuildFleet(overview, counts, research, missions, admiral, acsEnabled), nil
}

func (r FleetRepository) loadAdmiral(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT adm_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("fleet premium state not found")
	}
	var admiralUntil int64
	if err := rows.Scan(&admiralUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return admiralUntil > r.now().Unix(), nil
}

func (r FleetRepository) loadACSEnabled(ctx context.Context, uniTable string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT acs FROM %s LIMIT 1", uniTable))
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
	var acs int
	if err := rows.Scan(&acs); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return acs > 0, nil
}

func (r FleetRepository) loadActiveMissions(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, playerID int) ([]domaingame.FleetMission, error) {
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, f.mission, f.start_planet, f.target_planet, %s, COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(u.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s u ON u.player_id = t.owner_id WHERE q.type = ? AND f.mission <> ? AND f.owner_id = ? ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			queueTable,
			fleetTable,
			planetsTable,
			planetsTable,
			usersTable,
		),
		legacyPlanetTypeAbandoned,
		queueTypeFleet,
		domaingame.FleetMissionMissile,
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	missions := make([]domaingame.FleetMission, 0)
	for rows.Next() {
		mission, err := scanFleetMissionRow(rows, fleetIDs)
		if err != nil {
			return nil, err
		}
		missions = append(missions, mission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return missions, nil
}

func scanFleetMissionRow(rows Rows, fleetIDs []int) (domaingame.FleetMission, error) {
	var id int
	var departureAt int64
	var arrivalAt int64
	var mission int
	var startPlanetID int
	var targetPlanetID int
	shipValues := make([]int, len(fleetIDs))
	var origin domaingame.Coordinates
	var target domaingame.Coordinates
	var targetType int
	var targetOwner string

	dest := []any{&id, &departureAt, &arrivalAt, &mission, &startPlanetID, &targetPlanetID}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	dest = append(dest,
		&origin.Galaxy,
		&origin.System,
		&origin.Position,
		&target.Galaxy,
		&target.System,
		&target.Position,
		&targetType,
		&targetOwner,
	)
	if err := rows.Scan(dest...); err != nil {
		return domaingame.FleetMission{}, err
	}

	ships := make(domaingame.FleetCounts, len(fleetIDs))
	for index, fleetID := range fleetIDs {
		ships[fleetID] = shipValues[index]
	}
	return domaingame.BuildFleetMission(id, mission, ships, origin, target, targetType, targetOwner, departureAt, arrivalAt), nil
}

func prefixedNumericColumns(prefix string, ids []int) string {
	columns := make([]string, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, fmt.Sprintf("%s.`%d`", prefix, id))
	}
	return strings.Join(columns, ", ")
}

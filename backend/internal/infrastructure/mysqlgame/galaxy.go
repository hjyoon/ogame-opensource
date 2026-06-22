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

type GalaxyRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewGalaxyRepository(db *sql.DB, prefix string) GalaxyRepository {
	runner := SQLQueryer{DB: db}
	return GalaxyRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewGalaxyRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) GalaxyRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewGalaxyRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewGalaxyRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) GalaxyRepository {
	if now == nil {
		now = time.Now
	}
	return GalaxyRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r GalaxyRepository) GetGalaxy(ctx context.Context, query appgame.GalaxyQuery) (domaingame.Galaxy, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return domaingame.Galaxy{}, err
	}

	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Galaxy{}, err
	}

	viewer, err := r.loadGalaxyViewer(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	viewer.PlayerID = query.PlayerID
	viewer.SpyProbes, viewer.Recyclers, viewer.Missiles, err = r.loadGalaxyUnits(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Galaxy{}, err
	}

	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	admiral, err := FleetRepository{queryer: r.queryer, prefix: r.prefix, now: r.now}.loadAdmiral(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	activeFleets, err := r.loadActiveFleetCount(ctx, queueTable, fleetTable, query.PlayerID)
	if err != nil {
		return domaingame.Galaxy{}, err
	}
	bounds, err := r.loadGalaxyBounds(ctx, uniTable)
	if err != nil {
		return domaingame.Galaxy{}, err
	}

	coordinates := query.Coordinates
	if coordinates.Galaxy == 0 {
		coordinates.Galaxy = overview.CurrentPlanet.Coordinates.Galaxy
	}
	if coordinates.System == 0 {
		coordinates.System = overview.CurrentPlanet.Coordinates.System
	}
	coordinates = clampCoordinatesForRepository(coordinates, bounds)
	objects, err := r.loadGalaxyObjects(ctx, planetsTable, usersTable, allyTable, coordinates)
	if err != nil {
		return domaingame.Galaxy{}, err
	}

	baseMax := research[domaingame.ResearchComputer] + 1
	maxFleet := baseMax
	if admiral {
		maxFleet += 2
	}

	return domaingame.BuildGalaxy(overview, domaingame.GalaxyInput{
		Coordinates: coordinates,
		Bounds:      bounds,
		Viewer:      viewer,
		FleetSlots: domaingame.FleetSlots{
			Used:    activeFleets,
			Max:     maxFleet,
			BaseMax: baseMax,
			Admiral: admiral,
		},
		Objects: objects,
		Now:     r.now().Unix(),
	}), nil
}

type galaxyMissilePlanet struct {
	ID             int
	OwnerID        int
	Type           int
	Coordinates    domaingame.Coordinates
	Missiles       int
	OwnerScore     int64
	OwnerAdmin     int
	OwnerVacation  bool
	OwnerBanned    bool
	OwnerLastClick int64
	ImpulseDrive   int
}

func (r GalaxyRepository) LaunchMissiles(ctx context.Context, query appgame.GalaxyMissileLaunchQuery) (*domaingame.GalaxyActionIssue, error) {
	if r.execer == nil {
		return nil, errors.New("galaxy missile mutation unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return nil, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return nil, err
	}
	fleetLogsTable, err := tableName(r.prefix, "fleetlogs")
	if err != nil {
		return nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}

	fleetRepository := FleetRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now}
	frozen, err := fleetRepository.loadUniverseFrozen(ctx, uniTable)
	if err != nil {
		return nil, err
	}
	if frozen {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketFrozen), nil
	}

	origin, found, err := r.loadGalaxyMissileOrigin(ctx, planetsTable, usersTable, query.PlayerID, query.PlanetID)
	if err != nil || !found {
		return nil, err
	}
	target, found, err := r.loadGalaxyMissileTarget(ctx, planetsTable, usersTable, query.TargetPlanetID)
	if err != nil {
		return nil, err
	}
	if !found {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketNoTarget), nil
	}

	amount := absInt(query.Amount)
	targetDefenseID := absInt(query.TargetDefenseID)
	if !domaingame.GalaxyMissileTargetAllowed(targetDefenseID) {
		targetDefenseID = 0
	}
	distance := absInt(origin.Coordinates.System - target.Coordinates.System)
	ipmRadius := max(0, 5*origin.ImpulseDrive-1)

	var parameterIssue *domaingame.GalaxyActionIssue
	if amount == 0 {
		parameterIssue = domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketNoRockets)
	}
	if amount > origin.Missiles {
		parameterIssue = domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketNotEnough)
	}
	if distance > ipmRadius {
		parameterIssue = domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketWeakDrive)
	}
	if parameterIssue != nil {
		return parameterIssue, nil
	}

	if origin.OwnerVacation {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketVacationSelf), nil
	}
	if target.OwnerVacation {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketVacationOther), nil
	}
	if target.OwnerID == query.PlayerID {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketSelf), nil
	}
	if target.OwnerAdmin > 0 && target.OwnerID != userSpace {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketAdmin), nil
	}
	viewer := domaingame.GalaxyViewer{PlayerID: query.PlayerID, Score: origin.OwnerScore}
	owner := domaingame.GalaxyObjectPlayer{
		ID:        target.OwnerID,
		Score:     target.OwnerScore,
		LastClick: target.OwnerLastClick,
		Vacation:  target.OwnerVacation,
		Banned:    target.OwnerBanned,
	}
	if domaingame.GalaxyPlayerProtectedFromMissiles(owner, viewer, r.now().Unix()) {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketNoob), nil
	}

	now := r.now().Unix()
	seconds := int64(30 + 60*distance)
	if err := fleetRepository.deleteOldFleetLogs(ctx, fleetLogsTable, now); err != nil {
		return nil, err
	}
	reserved, err := r.reserveGalaxyMissiles(ctx, planetsTable, query.PlayerID, origin.ID, amount, int(now))
	if err != nil {
		return nil, err
	}
	if !reserved {
		return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketLaunchRace), nil
	}
	fleetID, err := r.insertGalaxyMissileFleet(ctx, fleetTable, origin, target, amount, targetDefenseID, int(seconds))
	if err != nil {
		return nil, err
	}
	if err := r.insertGalaxyMissileLog(ctx, fleetLogsTable, origin, target, amount, targetDefenseID, now, seconds); err != nil {
		return nil, err
	}
	if err := fleetRepository.insertRecallQueue(ctx, queueTable, query.PlayerID, fleetID, domaingame.FleetMissionMissile, now, seconds); err != nil {
		return nil, err
	}
	return domaingame.GalaxyMissileLaunchedIssue(amount), nil
}

func (r GalaxyRepository) loadGalaxyViewer(ctx context.Context, usersTable string, playerID int) (domaingame.GalaxyViewer, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT score1, admin, flags, maxspy, com_until FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.GalaxyViewer{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.GalaxyViewer{}, err
		}
		return domaingame.GalaxyViewer{}, errors.New("galaxy viewer not found")
	}
	var viewer domaingame.GalaxyViewer
	var commanderUntil int64
	if err := rows.Scan(&viewer.Score, &viewer.Admin, &viewer.Flags, &viewer.MaxSpy, &commanderUntil); err != nil {
		return domaingame.GalaxyViewer{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.GalaxyViewer{}, err
	}
	viewer.Commander = commanderUntil > r.now().Unix()
	return viewer, nil
}

func (r GalaxyRepository) loadGalaxyUnits(ctx context.Context, planetsTable string, playerID int, planetID int) (int, int, int, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT `%d`, `%d`, `%d` FROM %s WHERE planet_id = ? AND owner_id = ? LIMIT 1", domaingame.FleetEspionageProbe, domaingame.FleetRecycler, domaingame.DefenseInterplanetaryMissile, planetsTable),
		planetID,
		playerID,
	)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, 0, err
		}
		return 0, 0, 0, errors.New("galaxy current planet units not found")
	}
	var spyProbes int
	var recyclers int
	var missiles int
	if err := rows.Scan(&spyProbes, &recyclers, &missiles); err != nil {
		return 0, 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	return spyProbes, recyclers, missiles, nil
}

func (r GalaxyRepository) loadGalaxyMissileOrigin(ctx context.Context, planetsTable string, usersTable string, playerID int, planetID int) (galaxyMissilePlanet, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT p.planet_id, p.owner_id, p.type, p.g, p.s, p.p, p.`%d`, COALESCE(u.score1, 0), COALESCE(u.admin, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.lastclick, 0), COALESCE(u.`%d`, 0) FROM %s p JOIN %s u ON u.player_id = p.owner_id WHERE p.owner_id = ? AND (p.planet_id = ? OR (? = 0 AND p.planet_id = u.aktplanet)) LIMIT 1",
			domaingame.DefenseInterplanetaryMissile,
			domaingame.ResearchImpulseDrive,
			planetsTable,
			usersTable,
		),
		playerID,
		planetID,
		planetID,
	)
	if err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return galaxyMissilePlanet{}, false, err
		}
		return galaxyMissilePlanet{}, false, nil
	}
	planet, err := scanGalaxyMissilePlanet(rows, true)
	if err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	return planet, true, nil
}

func (r GalaxyRepository) loadGalaxyMissileTarget(ctx context.Context, planetsTable string, usersTable string, planetID int) (galaxyMissilePlanet, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT p.planet_id, p.owner_id, p.type, p.g, p.s, p.p, COALESCE(u.score1, 0), COALESCE(u.admin, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.lastclick, 0) FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id WHERE p.planet_id = ? LIMIT 1",
			planetsTable,
			usersTable,
		),
		planetID,
	)
	if err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return galaxyMissilePlanet{}, false, err
		}
		return galaxyMissilePlanet{}, false, nil
	}
	planet, err := scanGalaxyMissilePlanet(rows, false)
	if err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return galaxyMissilePlanet{}, false, err
	}
	return planet, true, nil
}

func scanGalaxyMissilePlanet(rows Rows, includeOriginFields bool) (galaxyMissilePlanet, error) {
	var planet galaxyMissilePlanet
	var vacation int
	var banned int
	dest := []any{
		&planet.ID,
		&planet.OwnerID,
		&planet.Type,
		&planet.Coordinates.Galaxy,
		&planet.Coordinates.System,
		&planet.Coordinates.Position,
	}
	if includeOriginFields {
		dest = append(dest, &planet.Missiles)
	}
	dest = append(dest,
		&planet.OwnerScore,
		&planet.OwnerAdmin,
		&vacation,
		&banned,
		&planet.OwnerLastClick,
	)
	if includeOriginFields {
		dest = append(dest, &planet.ImpulseDrive)
	}
	if err := rows.Scan(dest...); err != nil {
		return galaxyMissilePlanet{}, err
	}
	planet.OwnerVacation = vacation != 0
	planet.OwnerBanned = banned != 0
	return planet, nil
}

func (r GalaxyRepository) reserveGalaxyMissiles(ctx context.Context, planetsTable string, playerID int, planetID int, amount int, now int) (bool, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = `%d` - ?, lastpeek = ? WHERE planet_id = ? AND owner_id = ? AND `%d` >= ?", planetsTable, domaingame.DefenseInterplanetaryMissile, domaingame.DefenseInterplanetaryMissile, domaingame.DefenseInterplanetaryMissile),
		amount,
		now,
		planetID,
		playerID,
		amount,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r GalaxyRepository) insertGalaxyMissileFleet(ctx context.Context, fleetTable string, origin galaxyMissilePlanet, target galaxyMissilePlanet, amount int, targetDefenseID int, seconds int) (int, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, union_id, fuel, mission, start_planet, target_planet, flight_time, deploy_time, ipm_amount, ipm_target) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", fleetTable),
		origin.OwnerID,
		0,
		0,
		domaingame.FleetMissionMissile,
		origin.ID,
		target.ID,
		seconds,
		0,
		amount,
		targetDefenseID,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("galaxy missile fleet id unavailable")
	}
	return int(id), nil
}

func (r GalaxyRepository) insertGalaxyMissileLog(ctx context.Context, fleetLogsTable string, origin galaxyMissilePlanet, target galaxyMissilePlanet, amount int, targetDefenseID int, now int64, seconds int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, target_id, union_id, fuel, mission, flight_time, deploy_time, start, end, origin_g, origin_s, origin_p, origin_type, target_g, target_s, target_p, target_type, ipm_amount, ipm_target) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", fleetLogsTable),
		origin.OwnerID,
		target.OwnerID,
		0,
		0,
		domaingame.FleetMissionMissile,
		seconds,
		0,
		now,
		now+seconds,
		origin.Coordinates.Galaxy,
		origin.Coordinates.System,
		origin.Coordinates.Position,
		origin.Type,
		target.Coordinates.Galaxy,
		target.Coordinates.System,
		target.Coordinates.Position,
		target.Type,
		amount,
		targetDefenseID,
	)
	return err
}

func (r GalaxyRepository) loadGalaxyBounds(ctx context.Context, uniTable string) (domaingame.GalaxyBounds, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT galaxies, systems FROM %s LIMIT 1", uniTable))
	if err != nil {
		return domaingame.GalaxyBounds{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.GalaxyBounds{}, err
		}
		return domaingame.GalaxyBounds{Galaxies: 9, Systems: 499}, nil
	}
	var bounds domaingame.GalaxyBounds
	if err := rows.Scan(&bounds.Galaxies, &bounds.Systems); err != nil {
		return domaingame.GalaxyBounds{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.GalaxyBounds{}, err
	}
	if bounds.Galaxies < 1 {
		bounds.Galaxies = 9
	}
	if bounds.Systems < 1 {
		bounds.Systems = 499
	}
	return bounds, nil
}

func (r GalaxyRepository) loadActiveFleetCount(ctx context.Context, queueTable string, fleetTable string, playerID int) (int, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s q JOIN %s f ON f.fleet_id = q.sub_id WHERE q.type = ? AND f.mission <> ? AND f.owner_id = ?", queueTable, fleetTable),
		queueTypeFleet,
		domaingame.FleetMissionMissile,
		playerID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r GalaxyRepository) loadGalaxyObjects(ctx context.Context, planetsTable string, usersTable string, allyTable string, coordinates domaingame.Coordinates) ([]domaingame.GalaxyObject, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT p.planet_id, p.name, p.type, p.g, p.s, p.p, p.diameter, p.temp, p.lastakt, p.`%d`, p.`%d`, p.owner_id, COALESCE(u.oname, ''), COALESCE(u.score1, 0), COALESCE(u.place1, 0), COALESCE(u.ally_id, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.admin, 0), COALESCE(a.ally_id, 0), COALESCE(a.tag, '') FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id LEFT JOIN %s a ON a.ally_id = u.ally_id WHERE p.g = ? AND p.s = ? AND p.p BETWEEN 1 AND ? AND p.type IN (?, ?, ?, ?, ?, ?) ORDER BY p.p ASC, p.type ASC",
			domaingame.ResourceMetal,
			domaingame.ResourceCrystal,
			planetsTable,
			usersTable,
			allyTable,
		),
		coordinates.Galaxy,
		coordinates.System,
		domaingame.GalaxyPositions,
		domaingame.PlanetTypePlanet,
		domaingame.PlanetTypeDestroyedPlanet,
		domaingame.PlanetTypeAbandoned,
		domaingame.PlanetTypeMoon,
		domaingame.PlanetTypeDestroyedMoon,
		domaingame.PlanetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	objects := make([]domaingame.GalaxyObject, 0)
	for rows.Next() {
		object, err := scanGalaxyObject(rows)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return objects, nil
}

func scanGalaxyObject(rows Rows) (domaingame.GalaxyObject, error) {
	var object domaingame.GalaxyObject
	var ownerID int
	var ownerVacation int
	var ownerBanned int
	err := rows.Scan(
		&object.ID,
		&object.Name,
		&object.Type,
		&object.Coordinates.Galaxy,
		&object.Coordinates.System,
		&object.Coordinates.Position,
		&object.Diameter,
		&object.Temperature,
		&object.LastActivity,
		&object.DebrisMetal,
		&object.DebrisCrystal,
		&ownerID,
		&object.Owner.Name,
		&object.Owner.Score,
		&object.Owner.Rank,
		&object.Owner.Alliance,
		&object.Owner.LastClick,
		&ownerVacation,
		&ownerBanned,
		&object.Owner.Admin,
		&object.Alliance.ID,
		&object.Alliance.Tag,
	)
	if err != nil {
		return domaingame.GalaxyObject{}, err
	}
	object.Owner.ID = ownerID
	object.Owner.Vacation = ownerVacation != 0
	object.Owner.Banned = ownerBanned != 0
	return object, nil
}

func clampCoordinatesForRepository(coordinates domaingame.Coordinates, bounds domaingame.GalaxyBounds) domaingame.Coordinates {
	if coordinates.Galaxy < 1 {
		coordinates.Galaxy = 1
	}
	if coordinates.System < 1 {
		coordinates.System = 1
	}
	if coordinates.Galaxy > bounds.Galaxies {
		coordinates.Galaxy = bounds.Galaxies
	}
	if coordinates.System > bounds.Systems {
		coordinates.System = bounds.Systems
	}
	return coordinates
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

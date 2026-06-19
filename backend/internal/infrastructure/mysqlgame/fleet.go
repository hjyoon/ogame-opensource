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
	queuePriorityFleet        = 200
)

type recallFleetRow struct {
	ID             int
	OwnerID        int
	UnionID        int
	Metal          float64
	Crystal        float64
	Deuterium      float64
	Fuel           int
	Mission        int
	StartPlanetID  int
	TargetPlanetID int
	FlightTime     int
	DeployTime     int
	Ships          domaingame.FleetCounts
}

type recallQueueRow struct {
	TaskID int
	Start  int64
	End    int64
}

type fleetLaunchTarget struct {
	ID      int
	OwnerID int
	Type    int
}

type FleetRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewFleetRepository(db *sql.DB, prefix string) FleetRepository {
	runner := SQLQueryer{DB: db}
	return FleetRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewFleetRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) FleetRepository {
	if now == nil {
		now = time.Now
	}
	execer, _ := queryer.(Execer)
	return FleetRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func NewFleetRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) FleetRepository {
	if now == nil {
		now = time.Now
	}
	return FleetRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
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
	templateTable, err := tableName(r.prefix, "template")
	if err != nil {
		return domaingame.Fleet{}, err
	}

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
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
	commanderActive, err := r.loadCommanderActive(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	acsEnabled, err := r.loadACSEnabled(ctx, uniTable)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	speedFactor, err := r.loadFleetSpeedFactor(ctx, uniTable)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	missions, err := r.loadActiveMissions(ctx, queueTable, fleetTable, planetsTable, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}

	fleet := domaingame.BuildFleet(overview, counts, research, missions, admiral, acsEnabled)
	fleet.SpeedFactor = speedFactor
	fleet.CommanderActive = commanderActive
	fleet.TemplateLimit = research[domaingame.ResearchComputer] + 1
	if commanderActive {
		templates, err := r.loadFleetTemplates(ctx, templateTable, query.PlayerID, fleet.TemplateLimit)
		if err != nil {
			return domaingame.Fleet{}, err
		}
		fleet.Templates = templates
	}
	return fleet, nil
}

func (r FleetRepository) MutateFleetTemplate(ctx context.Context, query appgame.FleetTemplateMutationQuery) error {
	if r.execer == nil {
		return errors.New("fleet template writer unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	templateTable, err := tableName(r.prefix, "template")
	if err != nil {
		return err
	}

	commanderActive, maxTemplates, err := r.loadFleetTemplateAccess(ctx, usersTable, query.PlayerID)
	if err != nil {
		return err
	}
	if !commanderActive {
		return nil
	}

	switch query.Action {
	case "save":
		return r.saveFleetTemplate(ctx, templateTable, query.PlayerID, maxTemplates, query.TemplateID, query.Name, query.Ships)
	case "delete":
		return r.deleteFleetTemplate(ctx, templateTable, query.PlayerID, query.TemplateID)
	default:
		return nil
	}
}

func (r FleetRepository) LaunchFleetDispatch(ctx context.Context, query appgame.FleetLaunchQuery) (*domaingame.FleetActionIssue, error) {
	if r.execer == nil {
		return nil, errors.New("fleet launch writer unavailable")
	}
	if !query.Draft.Ready || query.PlayerID <= 0 || query.PlanetID <= 0 {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidOrder), nil
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
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return nil, err
	}

	frozen, err := r.loadUniverseFrozen(ctx, uniTable)
	if err != nil {
		return nil, err
	}
	if frozen {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueFrozen), nil
	}

	target, found, err := r.loadFleetLaunchTarget(ctx, planetsTable, query.Draft.Target, query.Draft.TargetType)
	if err != nil {
		return nil, err
	}
	if !found {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}

	now := r.now().Unix()
	resources := fleetLaunchResources(query.Draft.Resources)
	ships := fleetLaunchShips(query.Draft.Ships)
	if err := r.deleteOldFleetLogs(ctx, fleetLogsTable, now); err != nil {
		return nil, err
	}
	reserved, err := r.reserveFleetLaunchOrigin(ctx, planetsTable, query.PlayerID, query.PlanetID, ships, resources, query.Draft.FuelConsumption, int(now))
	if err != nil {
		return nil, err
	}
	if !reserved {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueLaunchRace), nil
	}

	fleetID, err := r.insertFleetLaunchFleet(ctx, fleetTable, query, target, ships, resources)
	if err != nil {
		return nil, err
	}
	if err := r.insertFleetLaunchLog(ctx, fleetLogsTable, query, target, ships, resources, now); err != nil {
		return nil, err
	}
	if err := r.insertRecallQueue(ctx, queueTable, query.PlayerID, fleetID, query.Draft.Mission, now, int64(query.Draft.DurationSeconds)); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r FleetRepository) RecallFleet(ctx context.Context, query appgame.FleetRecallQuery) error {
	if r.execer == nil {
		return errors.New("fleet writer unavailable")
	}
	if query.FleetID <= 0 {
		return nil
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return err
	}
	unionTable, err := tableName(r.prefix, "union")
	if err != nil {
		return err
	}

	frozen, err := r.loadUniverseFrozen(ctx, uniTable)
	if err != nil {
		return err
	}
	if frozen {
		return nil
	}

	fleet, found, err := r.loadRecallFleet(ctx, fleetTable, query.PlayerID, query.FleetID)
	if err != nil || !found {
		return err
	}
	if !fleetRecallable(fleet.Mission) {
		return nil
	}
	queue, found, err := r.loadRecallQueue(ctx, queueTable, fleet.ID)
	if err != nil || !found {
		return err
	}
	originOwner, found, err := r.loadRecallOriginOwner(ctx, planetsTable, fleet.StartPlanetID)
	if err != nil || !found {
		return err
	}
	if exists, err := r.recallPlanetExists(ctx, planetsTable, fleet.TargetPlanetID); err != nil || !exists {
		return err
	}

	now := r.now().Unix()
	newMission, seconds := recallMissionAndDuration(fleet, queue, now)
	newFleetID, err := r.insertRecallFleet(ctx, fleetTable, originOwner, fleet, newMission, seconds)
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, originOwner, newFleetID, newMission, now, seconds); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE fleet_id = ? AND owner_id = ? LIMIT 1", fleetTable), fleet.ID, query.PlayerID); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ? AND type = ? LIMIT 1", queueTable), queue.TaskID, queueTypeFleet); err != nil {
		return err
	}
	if fleet.UnionID > 0 && (fleet.Mission == domaingame.FleetMissionACSAttack || fleet.Mission == domaingame.FleetMissionACSAttackHead) {
		return r.removeEmptyRecallUnion(ctx, fleetTable, unionTable, fleet.UnionID)
	}
	return nil
}

func (r FleetRepository) loadFleetLaunchTarget(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, targetType int) (fleetLaunchTarget, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, type FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		fleetLaunchPlanetType(targetType),
	)
	if err != nil {
		return fleetLaunchTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetLaunchTarget{}, false, err
		}
		return fleetLaunchTarget{}, false, nil
	}
	var target fleetLaunchTarget
	if err := rows.Scan(&target.ID, &target.OwnerID, &target.Type); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	if err := rows.Err(); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) deleteOldFleetLogs(ctx context.Context, fleetLogsTable string, now int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE start < ?", fleetLogsTable), now-4*7*24*60*60)
	return err
}

func (r FleetRepository) reserveFleetLaunchOrigin(ctx context.Context, planetsTable string, playerID int, planetID int, ships domaingame.FleetCounts, resources map[int]int, fuel int, now int) (bool, error) {
	metal := max(0, resources[resourceMetal])
	crystal := max(0, resources[resourceCrystal])
	deuterium := max(0, resources[resourceDeuterium]+fuel)
	setParts := []string{
		fmt.Sprintf("`%d` = `%d` - ?", resourceMetal, resourceMetal),
		fmt.Sprintf("`%d` = `%d` - ?", resourceCrystal, resourceCrystal),
		fmt.Sprintf("`%d` = `%d` - ?", resourceDeuterium, resourceDeuterium),
	}
	args := []any{metal, crystal, deuterium}
	whereParts := []string{
		"planet_id = ?",
		"owner_id = ?",
		fmt.Sprintf("`%d` >= ?", resourceMetal),
		fmt.Sprintf("`%d` >= ?", resourceCrystal),
		fmt.Sprintf("`%d` >= ?", resourceDeuterium),
	}
	whereArgs := []any{planetID, playerID, metal, crystal, deuterium}
	for _, id := range domaingame.FleetIDs() {
		count := ships[id]
		if count <= 0 {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("`%d` = `%d` - ?", id, id))
		args = append(args, count)
		whereParts = append(whereParts, fmt.Sprintf("`%d` >= ?", id))
		whereArgs = append(whereArgs, count)
	}
	setParts = append(setParts, "lastpeek = ?")
	args = append(args, now)
	args = append(args, whereArgs...)
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE %s", planetsTable, strings.Join(setParts, ", "), strings.Join(whereParts, " AND ")), args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return true, nil
	}
	return affected > 0, nil
}

func (r FleetRepository) insertFleetLaunchFleet(ctx context.Context, fleetTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget, ships domaingame.FleetCounts, resources map[int]int) (int, error) {
	ids := domaingame.FleetIDs()
	args := []any{
		query.PlayerID,
		query.UnionID,
		resources[resourceMetal],
		resources[resourceCrystal],
		resources[resourceDeuterium],
		query.Draft.FuelConsumption,
		query.Draft.Mission,
		query.PlanetID,
		target.ID,
		query.Draft.DurationSeconds,
		query.HoldSeconds,
	}
	args = append(args, fleetCountValues(ids, ships)...)
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetTable, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("fleet launch id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) insertFleetLaunchLog(ctx context.Context, fleetLogsTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget, ships domaingame.FleetCounts, resources map[int]int, now int64) error {
	ids := domaingame.FleetIDs()
	args := []any{
		query.PlayerID,
		target.OwnerID,
		query.UnionID,
		query.Draft.FuelConsumption,
		query.Draft.Mission,
		query.Draft.DurationSeconds,
		query.HoldSeconds,
		now,
		now + int64(query.Draft.DurationSeconds),
		query.Origin.Coordinates.Galaxy,
		query.Origin.Coordinates.System,
		query.Origin.Coordinates.Position,
		query.Origin.Type,
		query.Draft.Target.Galaxy,
		query.Draft.Target.System,
		query.Draft.Target.Position,
		target.Type,
		int(query.Origin.Resources.Metal),
		int(query.Origin.Resources.Crystal),
		int(query.Origin.Resources.Deuterium),
		resources[resourceMetal],
		resources[resourceCrystal],
		resources[resourceDeuterium],
	}
	args = append(args, fleetCountValues(ids, ships)...)
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, target_id, union_id, fuel, mission, flight_time, deploy_time, start, end, origin_g, origin_s, origin_p, origin_type, target_g, target_s, target_p, target_type, `p%d`, `p%d`, `p%d`, `%d`, `%d`, `%d`, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetLogsTable, resourceMetal, resourceCrystal, resourceDeuterium, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	return err
}

func (r FleetRepository) loadUniverseFrozen(ctx context.Context, uniTable string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT freeze FROM %s LIMIT 1", uniTable))
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
	var frozen int
	if err := rows.Scan(&frozen); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return frozen != 0, nil
}

func (r FleetRepository) loadRecallFleet(ctx context.Context, fleetTable string, playerID int, fleetID int) (recallFleetRow, bool, error) {
	ids := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT fleet_id, owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s FROM %s WHERE fleet_id = ? AND owner_id = ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), fleetTable),
		fleetID,
		playerID,
	)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return recallFleetRow{}, false, err
		}
		return recallFleetRow{}, false, nil
	}
	fleet, err := scanRecallFleetRow(rows, ids)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	if err := rows.Err(); err != nil {
		return recallFleetRow{}, false, err
	}
	return fleet, true, nil
}

func scanRecallFleetRow(rows Rows, ids []int) (recallFleetRow, error) {
	fleet := recallFleetRow{Ships: make(domaingame.FleetCounts, len(ids))}
	shipValues := make([]int, len(ids))
	dest := []any{
		&fleet.ID,
		&fleet.OwnerID,
		&fleet.UnionID,
		&fleet.Metal,
		&fleet.Crystal,
		&fleet.Deuterium,
		&fleet.Fuel,
		&fleet.Mission,
		&fleet.StartPlanetID,
		&fleet.TargetPlanetID,
		&fleet.FlightTime,
		&fleet.DeployTime,
	}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return recallFleetRow{}, err
	}
	for index, id := range ids {
		fleet.Ships[id] = shipValues[index]
	}
	return fleet, nil
}

func (r FleetRepository) loadRecallQueue(ctx context.Context, queueTable string, fleetID int) (recallQueueRow, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id, start, end FROM %s WHERE type = ? AND sub_id = ? LIMIT 1", queueTable), queueTypeFleet, fleetID)
	if err != nil {
		return recallQueueRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return recallQueueRow{}, false, err
		}
		return recallQueueRow{}, false, nil
	}
	var queue recallQueueRow
	if err := rows.Scan(&queue.TaskID, &queue.Start, &queue.End); err != nil {
		return recallQueueRow{}, false, err
	}
	if err := rows.Err(); err != nil {
		return recallQueueRow{}, false, err
	}
	return queue, true, nil
}

func (r FleetRepository) loadRecallOriginOwner(ctx context.Context, planetsTable string, planetID int) (int, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT owner_id FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	var ownerID int
	if err := rows.Scan(&ownerID); err != nil {
		return 0, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	return ownerID, true, nil
}

func (r FleetRepository) recallPlanetExists(ctx context.Context, planetsTable string, planetID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT planet_id FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
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
	var id int
	if err := rows.Scan(&id); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return id > 0, nil
}

func (r FleetRepository) insertRecallFleet(ctx context.Context, fleetTable string, ownerID int, fleet recallFleetRow, mission int, seconds int64) (int, error) {
	ids := domaingame.FleetIDs()
	args := []any{
		ownerID,
		0,
		fleet.Metal,
		fleet.Crystal,
		fleet.Deuterium,
		fleet.Fuel / 2,
		mission,
		fleet.StartPlanetID,
		fleet.TargetPlanetID,
		seconds,
		0,
	}
	args = append(args, fleetCountValues(ids, fleet.Ships)...)
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetTable, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("recall return fleet id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) insertRecallQueue(ctx context.Context, queueTable string, ownerID int, fleetID int, mission int, now int64, seconds int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		ownerID,
		queueTypeFleet,
		fleetID,
		0,
		0,
		now,
		now+seconds,
		fleetQueuePriority(mission),
	)
	return err
}

func (r FleetRepository) removeEmptyRecallUnion(ctx context.Context, fleetTable string, unionTable string, unionID int) error {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE union_id = ?", fleetTable), unionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}
	var remaining int
	if err := rows.Scan(&remaining); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE union_id = ? LIMIT 1", unionTable), unionID)
	return err
}

func fleetRecallable(mission int) bool {
	return mission < domaingame.FleetMissionReturnOffset || mission > domaingame.FleetMissionOrbitingOffset
}

func recallMissionAndDuration(fleet recallFleetRow, queue recallQueueRow, now int64) (int, int64) {
	if fleet.Mission < domaingame.FleetMissionReturnOffset {
		seconds := now - queue.Start
		if seconds < 0 {
			seconds = 0
		}
		return fleet.Mission + domaingame.FleetMissionReturnOffset, seconds
	}
	seconds := int64(fleet.DeployTime)
	if seconds < 0 {
		seconds = 0
	}
	return fleet.Mission - domaingame.FleetMissionReturnOffset, seconds
}

func fleetQueuePriority(mission int) int {
	if mission == domaingame.FleetMissionMissile {
		return queuePriorityFleet + 1300
	}
	switch mission {
	case domaingame.FleetMissionAttack, domaingame.FleetMissionACSAttack, domaingame.FleetMissionACSAttackHead, domaingame.FleetMissionDestroy:
		return queuePriorityFleet + 1000 + mission
	case domaingame.FleetMissionRecycle:
		return queuePriorityFleet + 900
	default:
		return queuePriorityFleet + mission
	}
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

func (r FleetRepository) loadCommanderActive(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("fleet commander state not found")
	}
	var commanderUntil int64
	if err := rows.Scan(&commanderUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return commanderUntil > r.now().Unix(), nil
}

func (r FleetRepository) loadFleetTemplateAccess(ctx context.Context, usersTable string, playerID int) (bool, int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until, `%d` FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchComputer, usersTable), playerID)
	if err != nil {
		return false, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, 0, err
		}
		return false, 0, errors.New("fleet template access not found")
	}
	var commanderUntil int64
	var computer int
	if err := rows.Scan(&commanderUntil, &computer); err != nil {
		return false, 0, err
	}
	if err := rows.Err(); err != nil {
		return false, 0, err
	}
	return commanderUntil > r.now().Unix(), computer + 1, nil
}

func (r FleetRepository) loadFleetTemplates(ctx context.Context, templateTable string, playerID int, limit int) ([]domaingame.FleetTemplate, error) {
	if limit <= 0 {
		return nil, nil
	}
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT id, name, date, %s FROM %s WHERE owner_id = ? ORDER BY date DESC, id DESC LIMIT ?", numericColumns(fleetIDs), templateTable),
		playerID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]domaingame.FleetTemplate, 0)
	for rows.Next() {
		template, err := scanFleetTemplateRow(rows, fleetIDs)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return templates, nil
}

func scanFleetTemplateRow(rows Rows, fleetIDs []int) (domaingame.FleetTemplate, error) {
	var id int
	var name string
	var updatedAt int64
	shipValues := make([]int, len(fleetIDs))
	dest := []any{&id, &name, &updatedAt}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return domaingame.FleetTemplate{}, err
	}

	ships := make(domaingame.FleetCounts, len(fleetIDs))
	for index, fleetID := range fleetIDs {
		ships[fleetID] = shipValues[index]
	}
	return domaingame.BuildFleetTemplate(id, name, updatedAt, ships), nil
}

func (r FleetRepository) saveFleetTemplate(ctx context.Context, templateTable string, playerID int, maxTemplates int, templateID int, name string, ships map[int]int) error {
	fleetIDs := domaingame.FleetIDs()
	if templateID > 0 {
		args := []any{domaingame.NormalizeFleetTemplateName(name), r.now().Unix()}
		args = append(args, fleetTemplateValues(fleetIDs, ships)...)
		args = append(args, templateID, playerID)
		_, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET name = ?, date = ?, %s WHERE id = ? AND owner_id = ? LIMIT 1", templateTable, numericAssignments(fleetIDs)),
			args...,
		)
		return err
	}

	count, err := r.countFleetTemplates(ctx, templateTable, playerID)
	if err != nil {
		return err
	}
	if count >= maxTemplates {
		return nil
	}
	args := []any{playerID, domaingame.NormalizeFleetTemplateName(name), r.now().Unix()}
	args = append(args, fleetTemplateValues(fleetIDs, ships)...)
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, name, date, %s) VALUES (?, ?, ?, %s)", templateTable, numericColumns(fleetIDs), placeholders(len(fleetIDs))),
		args...,
	)
	return err
}

func (r FleetRepository) countFleetTemplates(ctx context.Context, templateTable string, playerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ?", templateTable), playerID)
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

func (r FleetRepository) deleteFleetTemplate(ctx context.Context, templateTable string, playerID int, templateID int) error {
	if templateID <= 0 {
		return nil
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ? AND owner_id = ? LIMIT 1", templateTable), templateID, playerID)
	return err
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

func (r FleetRepository) loadFleetSpeedFactor(ctx context.Context, uniTable string) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fspeed FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 1, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 1, err
		}
		return 1, nil
	}
	var speedFactor int
	if err := rows.Scan(&speedFactor); err != nil {
		return 1, err
	}
	if err := rows.Err(); err != nil {
		return 1, err
	}
	if speedFactor < 1 {
		return 1, nil
	}
	return speedFactor, nil
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

func numericAssignments(ids []int) string {
	assignments := make([]string, 0, len(ids))
	for _, id := range ids {
		assignments = append(assignments, fmt.Sprintf("`%d` = ?", id))
	}
	return strings.Join(assignments, ", ")
}

func placeholders(count int) string {
	values := make([]string, 0, count)
	for range count {
		values = append(values, "?")
	}
	return strings.Join(values, ", ")
}

func fleetTemplateValues(ids []int, ships map[int]int) []any {
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		count := ships[id]
		if id == domaingame.FleetSolarSatellite || count < 0 {
			count = 0
		}
		values = append(values, count)
	}
	return values
}

func fleetCountValues(ids []int, ships map[int]int) []any {
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		count := ships[id]
		if count < 0 {
			count = 0
		}
		values = append(values, count)
	}
	return values
}

func fleetLaunchPlanetType(targetType int) int {
	switch targetType {
	case domaingame.GamePlanetTypeMoon:
		return domaingame.PlanetTypeMoon
	case domaingame.GamePlanetTypeDebris:
		return domaingame.PlanetTypeDebris
	default:
		return domaingame.PlanetTypePlanet
	}
}

func fleetLaunchShips(rows []domaingame.FleetShipCount) domaingame.FleetCounts {
	ships := make(domaingame.FleetCounts, len(rows))
	for _, row := range rows {
		if row.Count > 0 {
			ships[row.ID] = row.Count
		}
	}
	return ships
}

func fleetLaunchResources(rows []domaingame.FleetResourceLoad) map[int]int {
	resources := map[int]int{
		resourceMetal:     0,
		resourceCrystal:   0,
		resourceDeuterium: 0,
	}
	for _, row := range rows {
		if row.Loaded > 0 {
			resources[row.ID] = row.Loaded
		}
	}
	return resources
}

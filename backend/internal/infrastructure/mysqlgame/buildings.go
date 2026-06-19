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

type BuildingsRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewBuildingsRepository(db *sql.DB, prefix string) BuildingsRepository {
	runner := SQLQueryer{DB: db}
	return BuildingsRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewBuildingsRepositoryWithQueryer(queryer Queryer, prefix string) BuildingsRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewBuildingsRepositoryWithRunner(queryer, execer, prefix, time.Now)
}

func NewBuildingsRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) BuildingsRepository {
	if now == nil {
		now = time.Now
	}
	return BuildingsRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r BuildingsRepository) GetBuildings(ctx context.Context, query appgame.BuildingsQuery) (domaingame.Buildings, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Buildings{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Buildings{}, err
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Buildings{}, err
	}

	levels, err := r.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Buildings{}, err
	}
	research, err := r.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Buildings{}, err
	}
	speed, err := r.loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.Buildings{}, err
	}

	return domaingame.BuildBuildings(overview, levels, research, speed), nil
}

func (r BuildingsRepository) MutateBuildings(ctx context.Context, query appgame.BuildingsMutationQuery) (appgame.BuildingsMutationOutcome, error) {
	if r.execer == nil {
		return appgame.BuildingsMutationOutcome{}, errors.New("buildings updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return appgame.BuildingsMutationOutcome{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return appgame.BuildingsMutationOutcome{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return appgame.BuildingsMutationOutcome{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return appgame.BuildingsMutationOutcome{}, err
	}

	overview, err := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return appgame.BuildingsMutationOutcome{}, err
	}
	planetID := overview.CurrentPlanet.ID
	now := r.now().Unix()

	switch domaingame.NormalizeBuildingsMutationAction(query.Action) {
	case domaingame.BuildingsMutationAdd:
		issue, err := r.enqueueBuilding(ctx, usersTable, planetsTable, buildQueueTable, queueTable, query.PlayerID, planetID, query.TechID, false, int(now))
		return appgame.BuildingsMutationOutcome{ActionIssue: issue}, err
	case domaingame.BuildingsMutationDestroy:
		issue, err := r.enqueueBuilding(ctx, usersTable, planetsTable, buildQueueTable, queueTable, query.PlayerID, planetID, query.TechID, true, int(now))
		return appgame.BuildingsMutationOutcome{ActionIssue: issue}, err
	case domaingame.BuildingsMutationRemove:
		if err := r.dequeueBuilding(ctx, planetsTable, buildQueueTable, queueTable, query.PlayerID, planetID, query.ListID, int(now)); err != nil {
			return appgame.BuildingsMutationOutcome{}, err
		}
	}
	return appgame.BuildingsMutationOutcome{}, nil
}

func (r BuildingsRepository) loadBuildingLevels(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.BuildingLevels, error) {
	ids := domaingame.BuildingIDs()
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
		return nil, errors.New("building levels not found")
	}
	levels, err := scanLevelMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return levels, nil
}

func (r BuildingsRepository) loadResearchLevels(ctx context.Context, usersTable string, playerID int) (domaingame.ResearchLevels, error) {
	ids := domaingame.BuildingResearchIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE player_id = ? LIMIT 1", numericColumns(ids), usersTable),
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("research levels not found")
	}
	levels, err := scanResearchMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return levels, nil
}

func (r BuildingsRepository) loadUniverseSpeed(ctx context.Context) (float64, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return 0, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT speed FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	var speed float64
	if err := rows.Scan(&speed); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if speed <= 0 {
		return 1, nil
	}
	return speed, nil
}

func numericColumns(ids []int) string {
	columns := make([]string, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, fmt.Sprintf("`%d`", id))
	}
	return strings.Join(columns, ", ")
}

func scanLevelMap(rows Rows, ids []int) (domaingame.BuildingLevels, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	levels := make(domaingame.BuildingLevels, len(ids))
	for index, id := range ids {
		levels[id] = values[index]
	}
	return levels, nil
}

func scanResearchMap(rows Rows, ids []int) (domaingame.ResearchLevels, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	levels := make(domaingame.ResearchLevels, len(ids))
	for index, id := range ids {
		levels[id] = values[index]
	}
	return levels, nil
}

type buildingMutationUser struct {
	Vacation       bool
	CommanderUntil int64
	Research       domaingame.ResearchLevels
}

type buildingMutationPlanet struct {
	ID        int
	OwnerID   int
	Type      int
	Fields    int
	MaxFields int
	Resources domaingame.Resources
	Levels    domaingame.BuildingLevels
}

type buildQueueRow struct {
	ID       int
	OwnerID  int
	PlanetID int
	ListID   int
	TechID   int
	Level    int
	Destroy  int
	Start    int
	End      int
}

type buildingUniverseConfig struct {
	Speed  float64
	Frozen bool
}

func (r BuildingsRepository) enqueueBuilding(ctx context.Context, usersTable string, planetsTable string, buildQueueTable string, queueTable string, playerID int, planetID int, techID int, destroy bool, now int) (*domaingame.BuildingsActionIssue, error) {
	user, err := r.loadBuildingMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return nil, err
	}
	if user.Vacation {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueVacation), nil
	}
	config, err := r.loadBuildingUniverseConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config.Frozen {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueUniversePause), nil
	}
	planet, err := r.loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return nil, err
	}
	if planet.ID == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), nil
	}
	queueRows, err := r.loadBuildQueueRows(ctx, buildQueueTable, planetID)
	if err != nil {
		return nil, err
	}
	maxQueue := 1
	if user.CommanderUntil > int64(now) {
		maxQueue = 5
	}
	if len(queueRows) >= maxQueue {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueQueueFull), nil
	}
	for _, row := range queueRows {
		if row.Start == now {
			return domaingame.BuildingActionIssue(domaingame.BuildingsIssueSameSecond), nil
		}
	}

	nowLevel := planet.Levels[techID]
	listID := 0
	for _, row := range queueRows {
		if row.TechID == techID {
			nowLevel = row.Level
		}
		if row.ListID > listID {
			listID = row.ListID
		}
	}
	listID++
	level := nowLevel + 1
	destroyFlag := 0
	queueType := queueTypeBuild
	if destroy {
		level = nowLevel - 1
		destroyFlag = 1
		queueType = queueTypeDemolish
		if level < 0 {
			return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoSuchBuilding), nil
		}
	}

	issue, cost, duration, err := r.validateBuildingOrder(ctx, queueTable, user, planet, techID, level, destroy, listID == 1, config.Speed)
	if err != nil || issue != nil {
		return issue, err
	}
	if listID == 1 {
		spent, err := r.spendBuildingResources(ctx, planetsTable, playerID, planetID, cost, now)
		if err != nil {
			return nil, err
		}
		if !spent {
			return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), nil
		}
	}

	buildQueueID, err := r.insertBuildQueue(ctx, buildQueueTable, playerID, planetID, listID, techID, level, destroyFlag, now, now+duration)
	if err != nil {
		return nil, err
	}
	if listID == 1 {
		if _, err := r.insertGlobalQueue(ctx, queueTable, playerID, queueType, buildQueueID, techID, level, now, duration); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r BuildingsRepository) dequeueBuilding(ctx context.Context, planetsTable string, buildQueueTable string, queueTable string, playerID int, planetID int, listID int, now int) error {
	if listID <= 0 {
		return nil
	}
	row, err := r.loadBuildQueueRow(ctx, buildQueueTable, playerID, planetID, listID)
	if err != nil || row == nil {
		return err
	}
	queueID, err := r.loadActiveBuildQueueTask(ctx, queueTable, row.ID)
	if err != nil {
		return err
	}
	if queueID > 0 {
		cost, ok := domaingame.BuildingCostForLevel(row.TechID, row.Level)
		if ok {
			if err := r.refundBuildingResources(ctx, planetsTable, planetID, cost, now); err != nil {
				return err
			}
		}
		if err := r.removeGlobalQueue(ctx, queueTable, queueID); err != nil {
			return err
		}
	}
	if err := r.shiftQueuedBuildingLevels(ctx, buildQueueTable, planetID, row.TechID, row.ListID); err != nil {
		return err
	}
	if err := r.removeBuildQueueRow(ctx, buildQueueTable, row.ID); err != nil {
		return err
	}
	if queueID > 0 {
		return r.startNextBuildQueue(ctx, planetsTable, buildQueueTable, queueTable, playerID, planetID, now)
	}
	return nil
}

func (r BuildingsRepository) startNextBuildQueue(ctx context.Context, planetsTable string, buildQueueTable string, queueTable string, playerID int, planetID int, now int) error {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	user, err := r.loadBuildingMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return err
	}
	config, err := r.loadBuildingUniverseConfig(ctx)
	if err != nil {
		return err
	}
	if user.Vacation || config.Frozen {
		return nil
	}
	for {
		rows, err := r.loadBuildQueueRows(ctx, buildQueueTable, planetID)
		if err != nil || len(rows) == 0 {
			return err
		}
		row := rows[0]
		planet, err := r.loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
		if err != nil {
			return err
		}
		destroy := row.Destroy != 0
		issue, cost, duration, err := r.validateBuildingOrder(ctx, queueTable, user, planet, row.TechID, row.Level, destroy, true, config.Speed)
		if err != nil {
			return err
		}
		if issue != nil {
			if err := r.shiftQueuedBuildingLevels(ctx, buildQueueTable, planetID, row.TechID, row.ListID); err != nil {
				return err
			}
			if err := r.removeBuildQueueRow(ctx, buildQueueTable, row.ID); err != nil {
				return err
			}
			continue
		}
		spent, err := r.spendBuildingResources(ctx, planetsTable, playerID, planetID, cost, now)
		if err != nil || !spent {
			return err
		}
		queueType := queueTypeBuild
		if destroy {
			queueType = queueTypeDemolish
		}
		if _, err := r.insertGlobalQueue(ctx, queueTable, playerID, queueType, row.ID, row.TechID, row.Level, now, duration); err != nil {
			return err
		}
		return r.updateBuildQueueTiming(ctx, buildQueueTable, row.ID, now, now+duration)
	}
}

func (r BuildingsRepository) validateBuildingOrder(ctx context.Context, queueTable string, user buildingMutationUser, planet buildingMutationPlanet, techID int, level int, destroy bool, requireResources bool, speed float64) (*domaingame.BuildingsActionIssue, domaingame.BuildingCost, int, error) {
	if destroy && level < 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoSuchBuilding), domaingame.BuildingCost{}, 0, nil
	}
	cost, ok := domaingame.BuildingCostForLevel(techID, level)
	if !ok || !domaingame.BuildingAllowedOnPlanet(techID, planet.Type) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), domaingame.BuildingCost{}, 0, nil
	}
	if destroy && !domaingame.BuildingCanDemolish(techID) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueCannotDemolish), domaingame.BuildingCost{}, 0, nil
	}
	if destroy && planet.Levels[techID] <= 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoSuchBuilding), domaingame.BuildingCost{}, 0, nil
	}
	if !destroy && planet.Fields >= planet.MaxFields {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoSpace), domaingame.BuildingCost{}, 0, nil
	}
	if !domaingame.BuildingRequirementsMet(techID, planet.Levels, user.Research) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueRequirements), domaingame.BuildingCost{}, 0, nil
	}
	busy, err := r.buildingBlocksOnBusyQueue(ctx, queueTable, techID, planet.ID, planet.OwnerID)
	if err != nil || busy {
		if err != nil {
			return nil, domaingame.BuildingCost{}, 0, err
		}
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy), domaingame.BuildingCost{}, 0, nil
	}
	if requireResources && !buildingResourcesEnough(planet.Resources, cost) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), domaingame.BuildingCost{}, 0, nil
	}
	duration := domaingame.BuildingDurationForCost(cost, planet.Levels[domaingame.BuildingRoboticsFactory], planet.Levels[domaingame.BuildingNaniteFactory], speed)
	return nil, cost, duration, nil
}

func (r BuildingsRepository) loadBuildingMutationUser(ctx context.Context, usersTable string, playerID int) (buildingMutationUser, error) {
	ids := domaingame.BuildingResearchIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT vacation, com_until, %s FROM %s WHERE player_id = ? LIMIT 1", numericColumns(ids), usersTable), playerID)
	if err != nil {
		return buildingMutationUser{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return buildingMutationUser{}, err
		}
		return buildingMutationUser{}, errors.New("building user not found")
	}
	var vacation int
	var commanderUntil int64
	values := make([]int, len(ids))
	dest := []any{&vacation, &commanderUntil}
	for index := range values {
		dest = append(dest, &values[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return buildingMutationUser{}, err
	}
	if err := rows.Err(); err != nil {
		return buildingMutationUser{}, err
	}
	research := make(domaingame.ResearchLevels, len(ids))
	for index, id := range ids {
		research[id] = values[index]
	}
	return buildingMutationUser{Vacation: vacation != 0, CommanderUntil: commanderUntil, Research: research}, nil
}

func (r BuildingsRepository) loadBuildingMutationPlanet(ctx context.Context, planetsTable string, playerID int, planetID int) (buildingMutationPlanet, error) {
	ids := domaingame.BuildingIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, type, fields, maxfields, `%d`, `%d`, `%d`, %s FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return buildingMutationPlanet{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return buildingMutationPlanet{}, err
		}
		return buildingMutationPlanet{}, nil
	}
	levels := make([]int, len(ids))
	planet := buildingMutationPlanet{}
	dest := []any{&planet.ID, &planet.OwnerID, &planet.Type, &planet.Fields, &planet.MaxFields, &planet.Resources.Metal, &planet.Resources.Crystal, &planet.Resources.Deuterium}
	for index := range levels {
		dest = append(dest, &levels[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return buildingMutationPlanet{}, err
	}
	if err := rows.Err(); err != nil {
		return buildingMutationPlanet{}, err
	}
	planet.Levels = make(domaingame.BuildingLevels, len(ids))
	for index, id := range ids {
		planet.Levels[id] = levels[index]
	}
	return planet, nil
}

func (r BuildingsRepository) loadBuildingUniverseConfig(ctx context.Context) (buildingUniverseConfig, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return buildingUniverseConfig{}, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT speed, freeze FROM %s LIMIT 1", uniTable))
	if err != nil {
		return buildingUniverseConfig{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return buildingUniverseConfig{}, err
		}
		return buildingUniverseConfig{Speed: 1}, nil
	}
	var config buildingUniverseConfig
	var freeze int
	if err := rows.Scan(&config.Speed, &freeze); err != nil {
		return buildingUniverseConfig{}, err
	}
	if err := rows.Err(); err != nil {
		return buildingUniverseConfig{}, err
	}
	if config.Speed <= 0 {
		config.Speed = 1
	}
	config.Frozen = freeze != 0
	return config, nil
}

func (r BuildingsRepository) loadBuildQueueRows(ctx context.Context, buildQueueTable string, planetID int) ([]buildQueueRow, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id, owner_id, planet_id, list_id, tech_id, level, destroy, start, end FROM %s WHERE planet_id = ? ORDER BY list_id ASC", buildQueueTable), planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []buildQueueRow{}
	for rows.Next() {
		var row buildQueueRow
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.PlanetID, &row.ListID, &row.TechID, &row.Level, &row.Destroy, &row.Start, &row.End); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r BuildingsRepository) loadBuildQueueRow(ctx context.Context, buildQueueTable string, playerID int, planetID int, listID int) (*buildQueueRow, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id, owner_id, planet_id, list_id, tech_id, level, destroy, start, end FROM %s WHERE owner_id = ? AND planet_id = ? AND list_id = ? LIMIT 1", buildQueueTable), playerID, planetID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var row buildQueueRow
	if err := rows.Scan(&row.ID, &row.OwnerID, &row.PlanetID, &row.ListID, &row.TechID, &row.Level, &row.Destroy, &row.Start, &row.End); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r BuildingsRepository) buildingBlocksOnBusyQueue(ctx context.Context, queueTable string, techID int, planetID int, playerID int) (bool, error) {
	switch techID {
	case domaingame.BuildingResearchLab:
		return r.queueExists(ctx, queueTable, "owner_id = ? AND type = ?", playerID, queueTypeResearch)
	case domaingame.BuildingNaniteFactory, domaingame.BuildingShipyard:
		return r.queueExists(ctx, queueTable, "sub_id = ? AND type = ?", planetID, queueTypeShipyard)
	default:
		return false, nil
	}
}

func (r BuildingsRepository) queueExists(ctx context.Context, queueTable string, where string, args ...any) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id FROM %s WHERE %s LIMIT 1", queueTable, where), args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (r BuildingsRepository) loadActiveBuildQueueTask(ctx context.Context, queueTable string, buildQueueID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id FROM %s WHERE (type = ? OR type = ?) AND sub_id = ? LIMIT 1", queueTable), queueTypeBuild, queueTypeDemolish, buildQueueID)
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
	var id int
	if err := rows.Scan(&id); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r BuildingsRepository) spendBuildingResources(ctx context.Context, planetsTable string, playerID int, planetID int, cost domaingame.BuildingCost, now int) (bool, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = `%d` - ?, `%d` = `%d` - ?, `%d` = `%d` - ?, lastpeek = ? WHERE planet_id = ? AND owner_id = ? AND `%d` >= ? AND `%d` >= ? AND `%d` >= ?", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal, resourceDeuterium, resourceDeuterium, resourceMetal, resourceCrystal, resourceDeuterium),
		cost.Metal,
		cost.Crystal,
		cost.Deuterium,
		now,
		planetID,
		playerID,
		cost.Metal,
		cost.Crystal,
		cost.Deuterium,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return true, nil
	}
	return affected > 0, nil
}

func (r BuildingsRepository) refundBuildingResources(ctx context.Context, planetsTable string, planetID int, cost domaingame.BuildingCost, now int) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = `%d` + ?, `%d` = `%d` + ?, `%d` = `%d` + ?, lastpeek = ? WHERE planet_id = ?", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal, resourceDeuterium, resourceDeuterium),
		cost.Metal,
		cost.Crystal,
		cost.Deuterium,
		now,
		planetID,
	)
	return err
}

func (r BuildingsRepository) insertBuildQueue(ctx context.Context, buildQueueTable string, playerID int, planetID int, listID int, techID int, level int, destroy int, start int, end int) (int, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, planet_id, list_id, tech_id, level, destroy, start, end) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", buildQueueTable),
		playerID,
		planetID,
		listID,
		techID,
		level,
		destroy,
		start,
		end,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("database returned empty build queue id")
	}
	return int(id), nil
}

func (r BuildingsRepository) insertGlobalQueue(ctx context.Context, queueTable string, playerID int, queueType string, subID int, objID int, level int, start int, duration int) (int, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		playerID,
		queueType,
		subID,
		objID,
		level,
		start,
		start+duration,
		20,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r BuildingsRepository) removeGlobalQueue(ctx context.Context, queueTable string, queueID int) error {
	if queueID <= 0 {
		return nil
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ?", queueTable), queueID)
	return err
}

func (r BuildingsRepository) shiftQueuedBuildingLevels(ctx context.Context, buildQueueTable string, planetID int, techID int, afterListID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET level = level - 1 WHERE tech_id = ? AND planet_id = ? AND list_id > ?", buildQueueTable), techID, planetID, afterListID)
	return err
}

func (r BuildingsRepository) removeBuildQueueRow(ctx context.Context, buildQueueTable string, buildQueueID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", buildQueueTable), buildQueueID)
	return err
}

func (r BuildingsRepository) updateBuildQueueTiming(ctx context.Context, buildQueueTable string, buildQueueID int, start int, end int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET start = ?, end = ? WHERE id = ?", buildQueueTable), start, end, buildQueueID)
	return err
}

func buildingResourcesEnough(resources domaingame.Resources, cost domaingame.BuildingCost) bool {
	return resources.Metal >= cost.Metal && resources.Crystal >= cost.Crystal && resources.Deuterium >= cost.Deuterium && cost.Energy <= 0
}

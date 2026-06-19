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

type ResearchRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

func NewResearchRepository(db *sql.DB, prefix string) ResearchRepository {
	runner := SQLQueryer{DB: db}
	return ResearchRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewResearchRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) ResearchRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewResearchRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewResearchRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) ResearchRepository {
	if now == nil {
		now = time.Now
	}
	return ResearchRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r ResearchRepository) GetResearch(ctx context.Context, query appgame.ResearchQuery) (domaingame.Research, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Research{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Research{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Research{}, err
	}
	if r.execer != nil {
		if err := r.FinishDueResearchQueues(ctx, int(r.now().Unix())); err != nil {
			return domaingame.Research{}, err
		}
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Research{}, err
	}

	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Research{}, err
	}
	research, err := r.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Research{}, err
	}
	otherLabs, err := r.loadOtherResearchLabs(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Research{}, err
	}
	speed, err := buildings.loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.Research{}, err
	}
	technocrat, err := r.loadTechnocrat(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Research{}, err
	}
	active, err := r.loadActiveResearchQueue(ctx, queueTable, query.PlayerID, overview.CurrentPlanet.ID, int(r.now().Unix()))
	if err != nil {
		return domaingame.Research{}, err
	}

	labLevels := domaingame.BuildResearchLabLevels(levels[domaingame.BuildingResearchLab], otherLabs, research)
	return domaingame.BuildResearch(overview, levels, research, labLevels, speed, technocrat, active), nil
}

func (r ResearchRepository) MutateResearch(ctx context.Context, query appgame.ResearchMutationQuery) (appgame.ResearchMutationOutcome, error) {
	if r.execer == nil {
		return appgame.ResearchMutationOutcome{}, errors.New("research updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return appgame.ResearchMutationOutcome{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return appgame.ResearchMutationOutcome{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return appgame.ResearchMutationOutcome{}, err
	}
	now := int(r.now().Unix())
	if err := r.FinishDueResearchQueues(ctx, now); err != nil {
		return appgame.ResearchMutationOutcome{}, err
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return appgame.ResearchMutationOutcome{}, err
	}

	switch query.Action {
	case "start":
		issue, err := r.startResearch(ctx, usersTable, planetsTable, queueTable, query.PlayerID, overview.CurrentPlanet.ID, query.TechID, now)
		return appgame.ResearchMutationOutcome{ActionIssue: issue}, err
	case "cancel":
		issue, err := r.cancelResearch(ctx, usersTable, planetsTable, queueTable, query.PlayerID, now)
		return appgame.ResearchMutationOutcome{ActionIssue: issue}, err
	default:
		return appgame.ResearchMutationOutcome{ActionIssue: domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid)}, nil
	}
}

type researchMutationUser struct {
	Vacation        bool
	TechnocratUntil int64
	Research        domaingame.ResearchLevels
}

func (r ResearchRepository) FinishDueResearchQueues(ctx context.Context, until int) error {
	if r.execer == nil {
		return errors.New("research updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	config, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingUniverseConfig(ctx)
	if err != nil {
		return err
	}
	if config.Frozen {
		return nil
	}
	tasks, err := r.loadDueResearchQueueTasks(ctx, queueTable, until, buildQueueBatch)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.finishResearchQueueTask(ctx, usersTable, planetsTable, queueTable, task); err != nil {
			return err
		}
	}
	return nil
}

func (r ResearchRepository) startResearch(ctx context.Context, usersTable string, planetsTable string, queueTable string, playerID int, planetID int, techID int, now int) (*domaingame.BuildingsActionIssue, error) {
	user, err := r.loadResearchMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return nil, err
	}
	if user.Vacation {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueVacation), nil
	}
	config, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingUniverseConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config.Frozen {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueUniversePause), nil
	}
	if active, err := r.loadActiveResearchQueue(ctx, queueTable, playerID, planetID, now); err != nil {
		return nil, err
	} else if active != nil {
		return researchIssue("research_already", "Research is already in progress."), nil
	}
	if busy, err := r.researchLabBusy(ctx, queueTable, playerID); err != nil {
		return nil, err
	} else if busy {
		return researchIssue("research_lab_building", "A research lab is being upgraded."), nil
	}
	planet, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return nil, err
	}
	if planet.ID == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), nil
	}
	level := user.Research[techID] + 1
	issue, cost, duration, err := r.validateResearchOrder(ctx, planetsTable, user, planet, techID, level, config.Speed)
	if err != nil || issue != nil {
		return issue, err
	}
	spent, err := (BuildingsRepository{execer: r.execer}).spendBuildingResources(ctx, planetsTable, playerID, planetID, cost, now)
	if err != nil {
		return nil, err
	}
	if !spent {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), nil
	}
	if _, err := (BuildingsRepository{execer: r.execer}).insertGlobalQueue(ctx, queueTable, playerID, queueTypeResearch, planetID, techID, level, now, duration); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r ResearchRepository) cancelResearch(ctx context.Context, usersTable string, planetsTable string, queueTable string, playerID int, now int) (*domaingame.BuildingsActionIssue, error) {
	config, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingUniverseConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config.Frozen {
		return nil, nil
	}
	queue, err := r.loadActiveResearchQueue(ctx, queueTable, playerID, 0, now)
	if err != nil || queue == nil {
		return nil, err
	}
	planet, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingMutationPlanet(ctx, planetsTable, playerID, queue.PlanetID)
	if err != nil {
		return nil, err
	}
	if planet.ID == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), nil
	}
	if r.updateResources {
		if err := r.updatePlanetResources(ctx, usersTable, planetsTable, playerID, queue.PlanetID, now); err != nil {
			return nil, err
		}
	}
	cost, ok := domaingame.ResearchCostForLevel(queue.TechID, queue.Level)
	if ok {
		if err := (BuildingsRepository{execer: r.execer}).refundBuildingResources(ctx, planetsTable, queue.PlanetID, cost, now); err != nil {
			return nil, err
		}
	}
	return nil, (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, queue.TaskID)
}

func (r ResearchRepository) validateResearchOrder(ctx context.Context, planetsTable string, user researchMutationUser, planet buildingMutationPlanet, techID int, level int, speed float64) (*domaingame.BuildingsActionIssue, domaingame.BuildingCost, int, error) {
	cost, ok := domaingame.ResearchCostForLevel(techID, level)
	if !ok || !domaingame.ResearchRequirementsMet(techID, planet.Levels, user.Research) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueRequirements), domaingame.BuildingCost{}, 0, nil
	}
	if level > 99 {
		return researchIssue("max_level", "Maximum level reached."), domaingame.BuildingCost{}, 0, nil
	}
	if !buildingResourcesEnough(planet.Resources, cost) {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), domaingame.BuildingCost{}, 0, nil
	}
	otherLabs, err := r.loadOtherResearchLabs(ctx, planetsTable, planet.OwnerID, planet.ID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	labLevels := domaingame.BuildResearchLabLevels(planet.Levels[domaingame.BuildingResearchLab], otherLabs, user.Research)
	factor := speed
	if user.TechnocratUntil > r.now().Unix() {
		factor *= 1.1
	}
	duration, ok := domaingame.ResearchDurationForLevel(techID, level, labLevels[techID], factor)
	if !ok {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), domaingame.BuildingCost{}, 0, nil
	}
	return nil, cost, duration, nil
}

func (r ResearchRepository) finishResearchQueueTask(ctx context.Context, usersTable string, planetsTable string, queueTable string, task buildingQueueTask) error {
	if r.updateResources {
		if err := r.updatePlanetResources(ctx, usersTable, planetsTable, task.OwnerID, task.SubID, task.End); err != nil {
			return err
		}
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET `%d` = ? WHERE player_id = ?", usersTable, task.ObjID), task.Level, task.OwnerID); err != nil {
		return err
	}
	if err := (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID); err != nil {
		return err
	}
	points, ok := domaingame.ResearchScoreForLevel(task.ObjID, task.Level)
	if ok {
		if err := r.adjustResearchStats(ctx, usersTable, task.OwnerID, points); err != nil {
			return err
		}
	}
	return (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable)
}

func (r ResearchRepository) loadResearchMutationUser(ctx context.Context, usersTable string, playerID int) (researchMutationUser, error) {
	ids := domaingame.ResearchIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT vacation, tec_until, %s FROM %s WHERE player_id = ? LIMIT 1", numericColumns(ids), usersTable), playerID)
	if err != nil {
		return researchMutationUser{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return researchMutationUser{}, err
		}
		return researchMutationUser{}, errors.New("research user not found")
	}
	var vacation int
	var technocratUntil int64
	values := make([]int, len(ids))
	dest := []any{&vacation, &technocratUntil}
	for index := range values {
		dest = append(dest, &values[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return researchMutationUser{}, err
	}
	if err := rows.Err(); err != nil {
		return researchMutationUser{}, err
	}
	research := make(domaingame.ResearchLevels, len(ids))
	for index, id := range ids {
		research[id] = values[index]
	}
	return researchMutationUser{Vacation: vacation != 0, TechnocratUntil: technocratUntil, Research: research}, nil
}

func (r ResearchRepository) loadActiveResearchQueue(ctx context.Context, queueTable string, playerID int, currentPlanetID int, now int) (*domaingame.ResearchQueue, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id, sub_id, obj_id, level, start, end, freeze, frozen FROM %s WHERE type = ? AND owner_id = ? ORDER BY start ASC LIMIT 1", queueTable), queueTypeResearch, playerID)
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
	var queue domaingame.ResearchQueue
	var freeze int
	var frozen int
	if err := rows.Scan(&queue.TaskID, &queue.PlanetID, &queue.TechID, &queue.Level, &queue.Start, &queue.End, &freeze, &frozen); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	queue.RemainingSeconds = queue.End - now
	if freeze != 0 {
		queue.RemainingSeconds = queue.End - frozen
	}
	if queue.RemainingSeconds < 0 {
		queue.RemainingSeconds = 0
	}
	queue.Cancelable = currentPlanetID == 0 || currentPlanetID == queue.PlanetID
	return &queue, nil
}

func (r ResearchRepository) loadDueResearchQueueTasks(ctx context.Context, queueTable string, until int, limit int) ([]buildingQueueTask, error) {
	if limit <= 0 {
		limit = buildQueueBatch
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen FROM %s WHERE end <= ? AND freeze = 0 AND type = ? ORDER BY end ASC, prio DESC LIMIT ?", queueTable),
		until,
		queueTypeResearch,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []buildingQueueTask{}
	for rows.Next() {
		var task buildingQueueTask
		if err := rows.Scan(&task.TaskID, &task.OwnerID, &task.Type, &task.SubID, &task.ObjID, &task.Level, &task.Start, &task.End, &task.Prio, &task.Freeze, &task.Frozen); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r ResearchRepository) researchLabBusy(ctx context.Context, queueTable string, playerID int) (bool, error) {
	return (BuildingsRepository{queryer: r.queryer}).queueExists(ctx, queueTable, "obj_id = ? AND (type = ? OR type = ?) AND owner_id = ?", domaingame.BuildingResearchLab, queueTypeBuild, queueTypeDemolish, playerID)
}

func (r ResearchRepository) adjustResearchStats(ctx context.Context, usersTable string, playerID int, points int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET score1 = score1 + ?, score2 = score2 + ?, score3 = score3 + ? WHERE player_id = ? AND banned = 0 AND admin = 0", usersTable),
		points,
		0,
		1,
		playerID,
	)
	return err
}

func researchIssue(code string, message string) *domaingame.BuildingsActionIssue {
	return &domaingame.BuildingsActionIssue{Code: code, Message: message}
}

func (r ResearchRepository) loadResearchLevels(ctx context.Context, usersTable string, playerID int) (domaingame.ResearchLevels, error) {
	ids := domaingame.ResearchIDs()
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

func (r ResearchRepository) loadOtherResearchLabs(ctx context.Context, planetsTable string, playerID int, currentPlanetID int) ([]int, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, `%d` FROM %s WHERE owner_id = ? AND type = ? AND `%d` > 0", domaingame.BuildingResearchLab, planetsTable, domaingame.BuildingResearchLab),
		playerID,
		domaingame.PlanetTypePlanet,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labs := []int{}
	for rows.Next() {
		var planetID int
		var lab int
		if err := rows.Scan(&planetID, &lab); err != nil {
			return nil, err
		}
		if planetID != currentPlanetID {
			labs = append(labs, lab)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return labs, nil
}

func (r ResearchRepository) loadTechnocrat(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT tec_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("research premium state not found")
	}
	var technocratUntil int64
	if err := rows.Scan(&technocratUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return technocratUntil > r.now().Unix(), nil
}

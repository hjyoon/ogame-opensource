package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const maxShipyardOrders = 99

type shipyardOrderKind int

const (
	shipyardOrderFleet shipyardOrderKind = iota
	shipyardOrderDefense
)

type shipyardMutationConfig struct {
	Speed    float64
	OrderCap int
	Frozen   bool
}

type shipyardMutationState struct {
	user      buildingMutationUser
	playerID  int
	planetID  int
	levels    domaingame.BuildingLevels
	defense   domaingame.DefenseCounts
	items     []domaingame.ShipyardItem
	config    shipyardMutationConfig
	queueRows []buildingQueueTask
}

func (r ShipyardRepository) MutateShipyard(ctx context.Context, query appgame.ShipyardMutationQuery) (appgame.ShipyardMutationOutcome, error) {
	issue, err := r.mutateShipyardOrders(ctx, query.PlayerID, query.PlanetID, query.Orders, shipyardOrderFleet)
	return appgame.ShipyardMutationOutcome{ActionIssue: issue}, err
}

func (r DefenseRepository) MutateDefense(ctx context.Context, query appgame.DefenseMutationQuery) (appgame.DefenseMutationOutcome, error) {
	shipyard := ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
	issue, err := shipyard.mutateShipyardOrders(ctx, query.PlayerID, query.PlanetID, query.Orders, shipyardOrderDefense)
	return appgame.DefenseMutationOutcome{ActionIssue: issue}, err
}

func (r ShipyardRepository) FinishDueShipyardQueues(ctx context.Context, until int) error {
	if r.execer == nil {
		return errors.New("shipyard updater unavailable")
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
	config, err := r.loadShipyardMutationConfig(ctx)
	if err != nil {
		return err
	}
	if config.Frozen {
		return nil
	}
	tasks, err := r.loadDueShipyardQueueTasks(ctx, queueTable, until, buildQueueBatch)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.finishShipyardQueueTask(ctx, usersTable, planetsTable, queueTable, task, until); err != nil {
			return err
		}
	}
	return nil
}

func (r ShipyardRepository) mutateShipyardOrders(ctx context.Context, playerID int, planetID int, orders map[int]int, kind shipyardOrderKind) (*domaingame.BuildingsActionIssue, error) {
	if r.execer == nil {
		return nil, errors.New("shipyard updater unavailable")
	}
	if len(orders) == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), nil
	}
	now := int(r.currentTime().Unix())

	pending := map[int]int{}
	for id, amount := range orders {
		if amount > 0 {
			pending[id] = amount
		}
	}
	if len(pending) == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), nil
	}

	unlock, err := r.acquireShipyardMutationLock(ctx, playerID, planetID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if err := r.FinishDueShipyardQueues(ctx, now); err != nil {
		return nil, err
	}

	enqueued := 0
	var firstIssue *domaingame.BuildingsActionIssue
	for len(pending) > 0 {
		state, err := r.loadShipyardMutationState(ctx, playerID, planetID, kind)
		if err != nil {
			return nil, err
		}
		handled := false
		for _, item := range state.items {
			amount, exists := pending[item.ID]
			if !exists {
				continue
			}
			delete(pending, item.ID)
			handled = true
			issue, ok, err := r.enqueueShipyardItem(ctx, state, item, amount, now)
			if err != nil {
				return nil, err
			}
			if ok {
				enqueued++
			} else if firstIssue == nil {
				firstIssue = issue
			}
			break
		}
		if !handled {
			if firstIssue == nil {
				firstIssue = domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid)
			}
			break
		}
	}
	if enqueued > 0 {
		return nil, nil
	}
	return firstIssue, nil
}

func (r ShipyardRepository) acquireShipyardMutationLock(ctx context.Context, playerID int, planetID int) (func(), error) {
	db := (BuildingsRepository{queryer: r.queryer}).sqlDB()
	if db == nil || playerID <= 0 {
		return func() {}, nil
	}
	lockName := fmt.Sprintf("%sshipyard:%d:%d", r.prefix, playerID, planetID)
	if planetID <= 0 {
		lockName = fmt.Sprintf("%sshipyard:%d", r.prefix, playerID)
	}
	return acquireMySQLNamedLock(ctx, db, lockName, "shipyard mutation lock timeout")
}

func (r ShipyardRepository) enqueueShipyardItem(ctx context.Context, state shipyardMutationState, item domaingame.ShipyardItem, requested int, now int) (*domaingame.BuildingsActionIssue, bool, error) {
	if state.user.Vacation {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueVacation), false, nil
	}
	if state.config.Frozen {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueUniversePause), false, nil
	}
	if issue := shipyardItemIssue(item); issue != nil {
		return issue, false, nil
	}
	queueCount := len(state.queueRows)
	if queueCount >= maxShipyardOrders {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueQueueFull), false, nil
	}

	amount := requested
	if amount > state.config.OrderCap {
		amount = state.config.OrderCap
	}
	if amount > item.MaxBuild {
		amount = item.MaxBuild
	}
	amount = clampDefenseShipyardAmount(item.ID, amount, state.levels, state.defense, state.queueRows)
	if amount <= 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), false, nil
	}

	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, false, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, false, err
	}
	cost := multiplyCost(item.Cost, amount)
	spent, err := (BuildingsRepository{execer: r.execer}).spendBuildingResources(ctx, planetsTable, state.playerID, state.planetID, cost, now)
	if err != nil {
		return nil, false, err
	}
	if !spent {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources), false, nil
	}
	start, err := r.loadShipyardLatestTime(ctx, queueTable, state.planetID, now)
	if err != nil {
		return nil, false, err
	}
	if _, err := (BuildingsRepository{execer: r.execer}).insertGlobalQueue(ctx, queueTable, state.playerID, queueTypeShipyard, state.planetID, item.ID, amount, start, item.DurationSeconds); err != nil {
		return nil, false, err
	}
	return nil, true, nil
}

func (r ShipyardRepository) loadShipyardMutationState(ctx context.Context, playerID int, planetID int, kind shipyardOrderKind) (shipyardMutationState, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return shipyardMutationState{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return shipyardMutationState{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return shipyardMutationState{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return shipyardMutationState{}, err
	}

	user, err := r.loadShipyardMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	config, err := r.loadShipyardMutationConfig(ctx)
	if err != nil {
		return shipyardMutationState{}, err
	}
	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: playerID,
		PlanetID: planetID,
	})
	if err != nil {
		return shipyardMutationState{}, err
	}
	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, playerID, overview.CurrentPlanet.ID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	busy, err := r.loadShipyardBusy(ctx, buildQueueTable, overview.CurrentPlanet.ID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	defense, err := DefenseRepository{queryer: r.queryer, prefix: r.prefix}.loadDefenseCounts(ctx, planetsTable, playerID, overview.CurrentPlanet.ID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	queueRows, err := r.loadShipyardQueueTasks(ctx, queueTable, overview.CurrentPlanet.ID)
	if err != nil {
		return shipyardMutationState{}, err
	}

	var items []domaingame.ShipyardItem
	if kind == shipyardOrderDefense {
		items = domaingame.BuildDefense(overview, levels, user.Research, defense, config.Speed, busy, config.OrderCap).Items
	} else {
		fleet, err := r.loadFleetCounts(ctx, planetsTable, playerID, overview.CurrentPlanet.ID)
		if err != nil {
			return shipyardMutationState{}, err
		}
		items = domaingame.BuildShipyard(overview, levels, user.Research, fleet, config.Speed, busy, config.OrderCap).Items
	}

	return shipyardMutationState{
		user:      user,
		playerID:  playerID,
		planetID:  overview.CurrentPlanet.ID,
		levels:    levels,
		defense:   defense,
		items:     items,
		config:    config,
		queueRows: queueRows,
	}, nil
}

func (r ShipyardRepository) loadShipyardMutationUser(ctx context.Context, usersTable string, playerID int) (buildingMutationUser, error) {
	ids := domaingame.ResearchIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT vacation, com_until, %s FROM %s WHERE player_id = ? LIMIT 1", numericColumns(ids), usersTable), playerID)
	if err != nil {
		return buildingMutationUser{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return buildingMutationUser{}, err
		}
		return buildingMutationUser{}, errors.New("shipyard user not found")
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

func (r ShipyardRepository) currentTime() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

func shipyardItemIssue(item domaingame.ShipyardItem) *domaingame.BuildingsActionIssue {
	if !item.MeetsRequirement || item.BlockedReason == "impossibly" {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueRequirements)
	}
	if item.BlockedReason == "busy" {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy)
	}
	if item.BlockedReason != "" {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid)
	}
	if item.MaxBuild <= 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)
	}
	return nil
}

func clampDefenseShipyardAmount(id int, amount int, levels domaingame.BuildingLevels, defense domaingame.DefenseCounts, queueRows []buildingQueueTask) int {
	if amount <= 0 {
		return 0
	}
	if id == domaingame.DefenseSmallShieldDome || id == domaingame.DefenseLargeShieldDome {
		if defense[id] > 0 {
			return 0
		}
		for _, row := range queueRows {
			if row.ObjID == id {
				return 0
			}
		}
		if amount > 1 {
			return 1
		}
		return amount
	}
	if id != domaingame.DefenseAntiBallisticMissile && id != domaingame.DefenseInterplanetaryMissile {
		return amount
	}
	free := levels[domaingame.BuildingMissileSilo]*10 - (defense[domaingame.DefenseAntiBallisticMissile] + 2*defense[domaingame.DefenseInterplanetaryMissile])
	for _, row := range queueRows {
		switch row.ObjID {
		case domaingame.DefenseAntiBallisticMissile:
			free -= row.Level
		case domaingame.DefenseInterplanetaryMissile:
			free -= 2 * row.Level
		}
	}
	if id == domaingame.DefenseInterplanetaryMissile {
		free /= 2
	}
	if free <= 0 {
		return 0
	}
	if amount > free {
		return free
	}
	return amount
}

func multiplyCost(cost domaingame.BuildingCost, amount int) domaingame.BuildingCost {
	factor := float64(amount)
	return domaingame.BuildingCost{
		Metal:     cost.Metal * factor,
		Crystal:   cost.Crystal * factor,
		Deuterium: cost.Deuterium * factor,
		Energy:    cost.Energy * factor,
	}
}

func (r ShipyardRepository) loadShipyardMutationConfig(ctx context.Context) (shipyardMutationConfig, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return shipyardMutationConfig{}, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT speed, max_werf, freeze FROM %s LIMIT 1", uniTable))
	if err != nil {
		return shipyardMutationConfig{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return shipyardMutationConfig{}, err
		}
		return shipyardMutationConfig{Speed: 1, OrderCap: 1000}, nil
	}
	var config shipyardMutationConfig
	var freeze int
	if err := rows.Scan(&config.Speed, &config.OrderCap, &freeze); err != nil {
		return shipyardMutationConfig{}, err
	}
	if err := rows.Err(); err != nil {
		return shipyardMutationConfig{}, err
	}
	if config.Speed <= 0 {
		config.Speed = 1
	}
	if config.OrderCap <= 0 {
		config.OrderCap = 1000
	}
	config.Frozen = freeze != 0
	return config, nil
}

func (r ShipyardRepository) loadShipyardQueueTasks(ctx context.Context, queueTable string, planetID int) ([]buildingQueueTask, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen FROM %s WHERE type = ? AND sub_id = ? ORDER BY start ASC, task_id ASC", queueTable),
		queueTypeShipyard,
		planetID,
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

func (r ShipyardRepository) loadShipyardLatestTime(ctx context.Context, queueTable string, planetID int, now int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT start, end, level FROM %s WHERE type = ? AND sub_id = ? ORDER BY end DESC LIMIT 1", queueTable), queueTypeShipyard, planetID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return now, nil
	}
	var start int
	var end int
	var level int
	if err := rows.Scan(&start, &end, &level); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	unitDuration := end - start
	if unitDuration < 0 {
		unitDuration = 0
	}
	if level < 1 {
		level = 1
	}
	return end + unitDuration*(level-1), nil
}

func (r ShipyardRepository) loadDueShipyardQueueTasks(ctx context.Context, queueTable string, until int, limit int) ([]buildingQueueTask, error) {
	if limit <= 0 {
		limit = buildQueueBatch
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen FROM %s WHERE end <= ? AND freeze = 0 AND type = ? ORDER BY end ASC, prio DESC LIMIT ?", queueTable),
		until,
		queueTypeShipyard,
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

func (r ShipyardRepository) finishShipyardQueueTask(ctx context.Context, usersTable string, planetsTable string, queueTable string, task buildingQueueTask, until int) error {
	if task.Level <= 0 {
		return (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID)
	}
	unitDuration := task.End - task.Start
	done := task.Level
	if unitDuration > 0 {
		done = (until - task.Start) / unitDuration
		if done < 1 {
			done = 1
		}
		if done > task.Level {
			done = task.Level
		}
	}
	if done <= 0 {
		return nil
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET `%d` = `%d` + ? WHERE planet_id = ? AND owner_id = ? LIMIT 1", planetsTable, task.ObjID, task.ObjID), done, task.SubID, task.OwnerID); err != nil {
		return err
	}
	points, fleetPoints, ok := domaingame.UnitScoreForCount(task.ObjID, done)
	if ok {
		if err := r.adjustShipyardStats(ctx, usersTable, task.OwnerID, points, fleetPoints); err != nil {
			return err
		}
	}
	if done < task.Level {
		newStart := task.Start + done*unitDuration
		newEnd := task.End + done*unitDuration
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET start = ?, end = ?, level = level - ? WHERE task_id = ?", queueTable), newStart, newEnd, done, task.TaskID); err != nil {
			return err
		}
		if unitDuration > 60 {
			return (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable)
		}
		return nil
	}
	if err := (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID); err != nil {
		return err
	}
	return (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable)
}

func (r ShipyardRepository) adjustShipyardStats(ctx context.Context, usersTable string, playerID int, points int64, fleetPoints int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET score1 = score1 + ?, score2 = score2 + ?, score3 = score3 + ? WHERE player_id = ? AND banned = 0 AND admin = 0", usersTable),
		points,
		fleetPoints,
		int64(0),
		playerID,
	)
	return err
}

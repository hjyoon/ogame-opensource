package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type fleetQueueTask struct {
	TaskID  int
	OwnerID int
	FleetID int
	End     int64
}

type expeditionSettings struct {
	ChanceSuccess int
	ChanceAlien   int
	ChancePirates int
	ChanceDM      int
	ChanceLost    int
	ChanceDelay   int
	ChanceAccel   int
	ChanceRes     int
	ChanceFleet   int
	DMFactor      int
}

type expeditionTargetState struct {
	Galaxy       int
	System       int
	Position     int
	VisitCounter int
}

type expeditionResult int

const (
	expeditionResultNothing expeditionResult = iota
	expeditionResultAliens
	expeditionResultPirates
	expeditionResultDarkMatter
	expeditionResultBlackHole
	expeditionResultDelay
	expeditionResultAccel
	expeditionResultResources
	expeditionResultFleet
	expeditionResultTrader
)

func (r FleetRepository) FinishDueFleetQueues(ctx context.Context, until int) error {
	if r.execer == nil {
		return errors.New("fleet queue updater unavailable")
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return err
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
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	expeditionTable, err := tableName(r.prefix, "exptab")
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

	tasks, err := r.loadDueFleetQueueTasks(ctx, queueTable, until, buildQueueBatch)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.finishFleetQueueTask(ctx, fleetTable, queueTable, planetsTable, messagesTable, usersTable, expeditionTable, task); err != nil {
			return err
		}
	}
	return nil
}

func (r FleetRepository) loadDueFleetQueueTasks(ctx context.Context, queueTable string, until int, limit int) ([]fleetQueueTask, error) {
	if limit <= 0 {
		limit = buildQueueBatch
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, owner_id, sub_id, end FROM %s WHERE type = ? AND end <= ? AND freeze = 0 ORDER BY end ASC, prio DESC LIMIT ?", queueTable),
		queueTypeFleet,
		until,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []fleetQueueTask{}
	for rows.Next() {
		var task fleetQueueTask
		if err := rows.Scan(&task.TaskID, &task.OwnerID, &task.FleetID, &task.End); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r FleetRepository) finishFleetQueueTask(ctx context.Context, fleetTable string, queueTable string, planetsTable string, messagesTable string, usersTable string, expeditionTable string, task fleetQueueTask) error {
	fleet, found, err := r.loadRecallFleetAnyOwner(ctx, fleetTable, task.FleetID)
	if err != nil {
		return err
	}
	if !found {
		return (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID)
	}

	switch fleet.Mission {
	case domaingame.FleetMissionTransport:
		return r.finishTransportFleetArrival(ctx, fleetTable, queueTable, planetsTable, task, fleet)
	case domaingame.FleetMissionDeploy:
		return r.finishDeployFleetArrival(ctx, fleetTable, queueTable, planetsTable, task, fleet)
	case domaingame.FleetMissionExpedition:
		return r.finishExpeditionArrival(ctx, fleetTable, queueTable, task, fleet)
	case domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset:
		return r.finishExpeditionHold(ctx, fleetTable, queueTable, planetsTable, messagesTable, usersTable, expeditionTable, task, fleet)
	default:
		if fleet.Mission >= domaingame.FleetMissionReturnOffset && fleet.Mission < domaingame.FleetMissionOrbitingOffset {
			return r.finishReturningFleetArrival(ctx, fleetTable, queueTable, planetsTable, task, fleet)
		}
	}
	return nil
}

func (r FleetRepository) finishTransportFleetArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, task fleetQueueTask, fleet recallFleetRow) error {
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, fleet.Metal, fleet.Crystal, fleet.Deuterium, task.End); err != nil {
		return err
	}
	returning := fleet
	returning.Metal = 0
	returning.Crystal = 0
	returning.Deuterium = 0
	returnFleetID, err := r.insertRecallFleet(ctx, fleetTable, fleet.OwnerID, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, fleet.Mission+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.FlightTime)); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishDeployFleetArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, task fleetQueueTask, fleet recallFleetRow) error {
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, fleet.Metal, fleet.Crystal, fleet.Deuterium+float64(fleet.Fuel/2), task.End); err != nil {
		return err
	}
	if err := r.addFleetShipsToPlanet(ctx, planetsTable, fleet.TargetPlanetID, fleet.Ships, task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishExpeditionArrival(ctx context.Context, fleetTable string, queueTable string, task fleetQueueTask, fleet recallFleetRow) error {
	holdFleetID, err := r.insertFleetTransition(ctx, fleetTable, fleet.OwnerID, fleet, fleet.Mission+domaingame.FleetMissionOrbitingOffset, int64(fleet.DeployTime), int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, holdFleetID, fleet.Mission+domaingame.FleetMissionOrbitingOffset, task.End, int64(fleet.DeployTime)); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishExpeditionHold(ctx context.Context, fleetTable string, queueTable string, planetsTable string, messagesTable string, usersTable string, expeditionTable string, task fleetQueueTask, fleet recallFleetRow) error {
	settings, err := r.loadExpeditionSettings(ctx, expeditionTable)
	if err != nil {
		return err
	}
	target, err := r.loadExpeditionTargetState(ctx, planetsTable, fleet.TargetPlanetID)
	if err != nil {
		return err
	}
	result := expeditionForcedResult(settings)
	messageText := "Expedition report: Nothing happened."

	switch result {
	case expeditionResultDarkMatter:
		if err := r.addExpeditionDarkMatter(ctx, usersTable, fleet.OwnerID, maxInt(100, settings.DMFactor*100)); err != nil {
			return err
		}
		messageText = "Expedition report: You found Dark Matter."
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), fleet); err != nil {
			return err
		}
	case expeditionResultResources:
		returning := fleet
		returning.Metal += 1000
		messageText = "Expedition report: You got 1,000 Metal."
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), returning); err != nil {
			return err
		}
	case expeditionResultFleet:
		returning := fleet
		returning.Ships = copyFleetCounts(fleet.Ships)
		returning.Ships[domaingame.FleetSmallCargo]++
		messageText = "Expedition report: The following ships are now part of the fleet:<br>Small Cargo 1"
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), returning); err != nil {
			return err
		}
	case expeditionResultTrader:
		if err := r.activateExpeditionTrader(ctx, usersTable, fleet.OwnerID); err != nil {
			return err
		}
		messageText = "Expedition report: You met a representative with goods to trade."
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), fleet); err != nil {
			return err
		}
	case expeditionResultDelay:
		messageText = "Expedition report: The fleet will return later because the return trip will take longer."
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime+maxInt(1, fleet.FlightTime)*2), fleet); err != nil {
			return err
		}
	case expeditionResultAccel:
		messageText = "Expedition report: The fleet will return earlier after an expedited return jump."
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, maxInt64(1, int64(fleet.DeployTime/2)), fleet); err != nil {
			return err
		}
	case expeditionResultAliens:
		messageText = "Expedition report: An alien fleet attacked the expedition."
		if err := r.insertExpeditionBattleMessage(ctx, messagesTable, fleet.OwnerID, target, "Battle report: alien attackers engaged the expedition.", task.End); err != nil {
			return err
		}
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), fleet); err != nil {
			return err
		}
	case expeditionResultPirates:
		messageText = "Expedition report: Pirate ships attacked the expedition."
		if err := r.insertExpeditionBattleMessage(ctx, messagesTable, fleet.OwnerID, target, "Battle report: pirate attackers engaged the expedition.", task.End); err != nil {
			return err
		}
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), fleet); err != nil {
			return err
		}
	case expeditionResultBlackHole:
		messageText = "Expedition report: The entire expedition fleet was lost forever in a black hole."
	default:
		if err := r.insertExpeditionReturn(ctx, fleetTable, queueTable, task, fleet, int64(fleet.DeployTime), fleet); err != nil {
			return err
		}
	}

	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, 1, 0, 0, task.End); err != nil {
		return err
	}
	if err := r.insertExpeditionMessage(ctx, messagesTable, fleet.OwnerID, target, messageText, task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishReturningFleetArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, task fleetQueueTask, fleet recallFleetRow) error {
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.StartPlanetID, fleet.Metal, fleet.Crystal, fleet.Deuterium, task.End); err != nil {
		return err
	}
	if err := r.addFleetShipsToPlanet(ctx, planetsTable, fleet.StartPlanetID, fleet.Ships, task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) addFleetResourcesToPlanet(ctx context.Context, planetsTable string, planetID int, metal float64, crystal float64, deuterium float64, activityAt int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = `%d` + ?, `%d` = `%d` + ?, `%d` = `%d` + ?, lastakt = ? WHERE planet_id = ? LIMIT 1", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal, resourceDeuterium, resourceDeuterium),
		maxFloat(0, metal),
		maxFloat(0, crystal),
		maxFloat(0, deuterium),
		activityAt,
		planetID,
	)
	return err
}

func (r FleetRepository) addFleetShipsToPlanet(ctx context.Context, planetsTable string, planetID int, ships domaingame.FleetCounts, activityAt int64) error {
	setParts := []string{}
	args := []any{}
	for _, id := range domaingame.FleetIDs() {
		count := ships[id]
		if count <= 0 {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("`%d` = `%d` + ?", id, id))
		args = append(args, count)
	}
	setParts = append(setParts, "lastakt = ?")
	args = append(args, activityAt, planetID)
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET %s WHERE planet_id = ? LIMIT 1", planetsTable, strings.Join(setParts, ", ")),
		args...,
	)
	return err
}

func (r FleetRepository) removeCompletedFleetTask(ctx context.Context, fleetTable string, queueTable string, fleetID int, taskID int) error {
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE fleet_id = ? LIMIT 1", fleetTable), fleetID); err != nil {
		return err
	}
	return (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, taskID)
}

func (r FleetRepository) insertExpeditionReturn(ctx context.Context, fleetTable string, queueTable string, task fleetQueueTask, original recallFleetRow, seconds int64, returning recallFleetRow) error {
	returnFleetID, err := r.insertFleetTransition(ctx, fleetTable, original.OwnerID, returning, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, seconds, 0)
	if err != nil {
		return err
	}
	return r.insertRecallQueue(ctx, queueTable, original.OwnerID, returnFleetID, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, task.End, seconds)
}

func (r FleetRepository) insertExpeditionMessage(ctx context.Context, messagesTable string, ownerID int, target expeditionTargetState, text string, at int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypeExpedition,
		"Fleet command",
		fmt.Sprintf("Expedition result [%d:%d:%d]", target.Galaxy, target.System, target.Position),
		text,
		at,
	)
	return err
}

func (r FleetRepository) insertExpeditionBattleMessage(ctx context.Context, messagesTable string, ownerID int, target expeditionTargetState, text string, at int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypeBattleReportText,
		"Fleet command",
		fmt.Sprintf("Battle report [%d:%d:%d]", target.Galaxy, target.System, target.Position),
		text,
		at,
	)
	return err
}

func (r FleetRepository) loadExpeditionSettings(ctx context.Context, expeditionTable string) (expeditionSettings, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT chance_success, chance_alien, chance_pirates, chance_dm, chance_lost, chance_delay, chance_accel, chance_res, chance_fleet, dm_factor FROM %s LIMIT 1", expeditionTable),
	)
	if err != nil {
		return expeditionSettings{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return expeditionSettings{}, err
		}
		return expeditionSettings{}, errors.New("expedition settings not found")
	}
	var settings expeditionSettings
	if err := rows.Scan(
		&settings.ChanceSuccess,
		&settings.ChanceAlien,
		&settings.ChancePirates,
		&settings.ChanceDM,
		&settings.ChanceLost,
		&settings.ChanceDelay,
		&settings.ChanceAccel,
		&settings.ChanceRes,
		&settings.ChanceFleet,
		&settings.DMFactor,
	); err != nil {
		return expeditionSettings{}, err
	}
	if err := rows.Err(); err != nil {
		return expeditionSettings{}, err
	}
	return settings, nil
}

func (r FleetRepository) loadExpeditionTargetState(ctx context.Context, planetsTable string, planetID int) (expeditionTargetState, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT g, s, p, `%d` FROM %s WHERE planet_id = ? LIMIT 1", resourceMetal, planetsTable),
		planetID,
	)
	if err != nil {
		return expeditionTargetState{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return expeditionTargetState{}, err
		}
		return expeditionTargetState{}, errors.New("expedition target not found")
	}
	var target expeditionTargetState
	if err := rows.Scan(&target.Galaxy, &target.System, &target.Position, &target.VisitCounter); err != nil {
		return expeditionTargetState{}, err
	}
	if err := rows.Err(); err != nil {
		return expeditionTargetState{}, err
	}
	return target, nil
}

func (r FleetRepository) addExpeditionDarkMatter(ctx context.Context, usersTable string, ownerID int, amount int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET dmfree = dmfree + ? WHERE player_id = ? LIMIT 1", usersTable), amount, ownerID)
	return err
}

func (r FleetRepository) activateExpeditionTrader(ctx context.Context, usersTable string, ownerID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET trader = 1, rate_m = 3, rate_k = 2, rate_d = 1 WHERE player_id = ? LIMIT 1", usersTable), ownerID)
	return err
}

func expeditionForcedResult(settings expeditionSettings) expeditionResult {
	if settings.ChanceSuccess <= 0 {
		return expeditionResultNothing
	}
	if settings.ChanceAlien <= 0 {
		return expeditionResultAliens
	}
	if settings.ChancePirates <= 0 {
		return expeditionResultPirates
	}
	if settings.ChanceDM <= 0 {
		return expeditionResultDarkMatter
	}
	if settings.ChanceLost <= 0 {
		return expeditionResultBlackHole
	}
	if settings.ChanceDelay <= 0 {
		return expeditionResultDelay
	}
	if settings.ChanceAccel <= 0 {
		return expeditionResultAccel
	}
	if settings.ChanceRes <= 0 {
		return expeditionResultResources
	}
	if settings.ChanceFleet <= 0 {
		return expeditionResultFleet
	}
	if settings.ChanceSuccess >= 100 &&
		settings.ChanceAlien >= 100 &&
		settings.ChancePirates >= 100 &&
		settings.ChanceDM >= 100 &&
		settings.ChanceLost >= 100 &&
		settings.ChanceDelay >= 100 &&
		settings.ChanceAccel >= 100 &&
		settings.ChanceRes >= 100 &&
		settings.ChanceFleet >= 100 {
		return expeditionResultTrader
	}
	return expeditionResultNothing
}

func copyFleetCounts(counts domaingame.FleetCounts) domaingame.FleetCounts {
	copied := domaingame.FleetCounts{}
	for id, count := range counts {
		copied[id] = count
	}
	return copied
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

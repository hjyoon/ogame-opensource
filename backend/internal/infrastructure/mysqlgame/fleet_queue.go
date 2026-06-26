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
		if err := r.finishFleetQueueTask(ctx, fleetTable, queueTable, planetsTable, messagesTable, task); err != nil {
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

func (r FleetRepository) finishFleetQueueTask(ctx context.Context, fleetTable string, queueTable string, planetsTable string, messagesTable string, task fleetQueueTask) error {
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
		return r.finishExpeditionHold(ctx, fleetTable, queueTable, planetsTable, messagesTable, task, fleet)
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

func (r FleetRepository) finishExpeditionHold(ctx context.Context, fleetTable string, queueTable string, planetsTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	returnFleetID, err := r.insertFleetTransition(ctx, fleetTable, fleet.OwnerID, fleet, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, int64(fleet.DeployTime), 0)
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.DeployTime)); err != nil {
		return err
	}
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, 1, 0, 0, task.End); err != nil {
		return err
	}
	if err := r.insertExpeditionMessage(ctx, messagesTable, fleet.OwnerID, task.End); err != nil {
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

func (r FleetRepository) insertExpeditionMessage(ctx context.Context, messagesTable string, ownerID int, at int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypeExpedition,
		"Fleet command",
		"Expedition report",
		"Nothing happened.",
		at,
	)
	return err
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type fleetQueueTask struct {
	TaskID  int
	OwnerID int
	FleetID int
	End     int64
}

type fleetMessageContext struct {
	OriginOwnerID   int
	OriginOwnerName string
	OriginName      string
	OriginGalaxy    int
	OriginSystem    int
	OriginPosition  int
	OriginType      int
	OriginMetal     float64
	OriginCrystal   float64
	OriginDeuterium float64
	TargetOwnerID   int
	TargetOwnerName string
	TargetName      string
	TargetGalaxy    int
	TargetSystem    int
	TargetPosition  int
	TargetType      int
	TargetMetal     float64
	TargetCrystal   float64
	TargetDeuterium float64
}

type recycleTargetState struct {
	Type    int
	Metal   float64
	Crystal float64
}

type expeditionSettings struct {
	ChanceSuccess     int
	DepletedMin       int
	DepletedMed       int
	DepletedMax       int
	ChanceDepletedMin int
	ChanceDepletedMed int
	ChanceDepletedMax int
	ChanceAlien       int
	ChancePirates     int
	ChanceDM          int
	ChanceLost        int
	ChanceDelay       int
	ChanceAccel       int
	ChanceRes         int
	ChanceFleet       int
	DMFactor          int
	ScoreCaps         [8]int
	PointLimits       [8]int
	PointLimitMax     int
}

type expeditionTargetState struct {
	Galaxy       int
	System       int
	Position     int
	VisitCounter int
	Language     string
	Trader       domaingame.ExpeditionTraderState
	Weapon       int
	Shield       int
	Armour       int
	TopScore     int64
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
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	expeditionTable, err := tableName(r.prefix, "exptab")
	if err != nil {
		return err
	}
	fleetLogsTable, err := tableName(r.prefix, "fleetlogs")
	if err != nil {
		return err
	}
	battleTable, err := tableName(r.prefix, "battledata")
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

	tasks, err := r.loadDueFleetQueueTasks(ctx, queueTable, until, buildQueueBatch)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.finishFleetQueueTask(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, messagesTable, usersTable, expeditionTable, battleTable, unionTable, task); err != nil {
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

func (r FleetRepository) finishFleetQueueTask(ctx context.Context, uniTable string, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, messagesTable string, usersTable string, expeditionTable string, battleTable string, unionTable string, task fleetQueueTask) error {
	fleet, found, err := r.loadRecallFleetAnyOwner(ctx, fleetTable, task.FleetID)
	if err != nil {
		return err
	}
	if !found {
		return (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID)
	}
	if r.queueProduction {
		originFound, err := r.updateFleetQueuePlanetResources(ctx, usersTable, planetsTable, fleet.StartPlanetID, int(task.End))
		if err != nil {
			return err
		}
		if !originFound {
			return nil
		}
		targetFound, err := r.updateFleetQueuePlanetResources(ctx, usersTable, planetsTable, fleet.TargetPlanetID, int(task.End))
		if err != nil {
			return err
		}
		if !targetFound {
			return nil
		}
	}

	switch fleet.Mission {
	case domaingame.FleetMissionAttack, domaingame.FleetMissionDestroy:
		return r.finishAttackFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, task, fleet)
	case domaingame.FleetMissionACSAttack, domaingame.FleetMissionACSAttackHead:
		return r.finishACSAttackFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, unionTable, task, fleet)
	case domaingame.FleetMissionTransport:
		return r.finishTransportFleetArrival(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
	case domaingame.FleetMissionDeploy:
		return r.finishDeployFleetArrival(ctx, fleetTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
	case domaingame.FleetMissionColonize:
		return r.finishColonizationArrival(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
	case domaingame.FleetMissionACSHold:
		return r.finishACSHoldArrival(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, task, fleet)
	case domaingame.FleetMissionACSHold + domaingame.FleetMissionOrbitingOffset:
		return r.finishACSHoldOrbit(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, task, fleet)
	case domaingame.FleetMissionRecycle:
		return r.finishRecycleFleetArrival(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
	case domaingame.FleetMissionSpy:
		return r.finishSpyFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, task, fleet)
	case domaingame.FleetMissionExpedition:
		return r.finishExpeditionArrival(ctx, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, task, fleet)
	case domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset:
		return r.finishExpeditionHold(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, messagesTable, usersTable, expeditionTable, battleTable, task, fleet)
	case domaingame.FleetMissionMissile:
		return r.finishMissileArrival(ctx, fleetTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
	default:
		if fleet.Mission >= domaingame.FleetMissionReturnOffset && fleet.Mission < domaingame.FleetMissionOrbitingOffset {
			return r.finishReturningFleetArrival(ctx, fleetTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet)
		}
	}
	return nil
}

func (r FleetRepository) updateFleetQueuePlanetResources(ctx context.Context, usersTable string, planetsTable string, planetID int, until int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT owner_id FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
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
	var ownerID int
	if err := rows.Scan(&ownerID); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	if err := updatePlanetResources(ctx, r.queryer, r.execer, r.prefix, usersTable, planetsTable, ownerID, planetID, until, now); err != nil {
		return false, err
	}
	return true, nil
}

func (r FleetRepository) finishACSHoldArrival(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext := fleetMessageContext{}
	found := false
	if r.legacyEvents {
		var err error
		messageContext, found, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
	if err := r.updateFleetPlanetActivity(ctx, planetsTable, fleet.TargetPlanetID, task.End); err != nil {
		return err
	}
	orbiting := fleet
	orbiting.Fuel = 0
	duration := int64(max(0, fleet.DeployTime))
	orbitID, err := r.insertFleetTransition(ctx, fleetTable, fleet.OwnerID, orbiting, fleet.Mission+domaingame.FleetMissionOrbitingOffset, duration, int64(max(0, fleet.FlightTime)))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, orbitID, fleet.Mission+domaingame.FleetMissionOrbitingOffset, task.End, duration); err != nil {
		return err
	}
	if found {
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, orbiting, fleet.Mission+domaingame.FleetMissionOrbitingOffset, duration, int64(max(0, fleet.FlightTime)), task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishACSHoldOrbit(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext := fleetMessageContext{}
	found := false
	if r.legacyEvents {
		var err error
		messageContext, found, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
	returning := fleet
	returning.Fuel = 0
	duration := int64(max(0, fleet.DeployTime))
	returnID, err := r.insertFleetTransition(ctx, fleetTable, fleet.OwnerID, returning, domaingame.FleetMissionACSHold+domaingame.FleetMissionReturnOffset, duration, 0)
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnID, domaingame.FleetMissionACSHold+domaingame.FleetMissionReturnOffset, task.End, duration); err != nil {
		return err
	}
	if found {
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, returning, domaingame.FleetMissionACSHold+domaingame.FleetMissionReturnOffset, duration, 0, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishTransportFleetArrival(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext := fleetMessageContext{}
	found := false
	if r.legacyEvents {
		var err error
		messageContext, found, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
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
	if found {
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End); err != nil {
			return err
		}
		if err := r.insertTransportArrivalMessages(ctx, messagesTable, messageContext, fleet, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishDeployFleetArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext := fleetMessageContext{}
	found := false
	if r.legacyEvents {
		var err error
		messageContext, found, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, fleet.Metal, fleet.Crystal, fleet.Deuterium+float64(fleet.Fuel/2), task.End); err != nil {
		return err
	}
	if err := r.addFleetShipsToPlanet(ctx, planetsTable, fleet.TargetPlanetID, fleet.Ships, task.End); err != nil {
		return err
	}
	if found {
		if err := r.insertDeployArrivalMessage(ctx, messagesTable, messageContext, fleet, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishRecycleFleetArrival(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	target, found, err := r.loadRecycleTarget(ctx, planetsTable, fleet.TargetPlanetID)
	if err != nil {
		return err
	}
	recyclers := fleet.Ships[domaingame.FleetRecycler]
	if !found || target.Type != domaingame.PlanetTypeDebris || recyclers <= 0 {
		return errors.New("invalid recycle fleet target")
	}
	messageContext := fleetMessageContext{}
	messageFound := false
	if r.legacyEvents {
		messageContext, messageFound, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
	loaded := maxFloat(0, fleet.Metal) + maxFloat(0, fleet.Crystal) + maxFloat(0, fleet.Deuterium)
	totalCargo := 0
	for id, count := range fleet.Ships {
		if id != domaingame.FleetEspionageProbe && count > 0 {
			totalCargo += domaingame.FleetCargoCapacity(id) * count
		}
	}
	cargo := minFloat(float64(domaingame.FleetCargoCapacity(domaingame.FleetRecycler)*recyclers), maxFloat(0, float64(totalCargo)-loaded))
	harvestMetal, harvestCrystal := recycleHarvest(target.Metal, target.Crystal, cargo)
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET `%d` = `%d` - ?, `%d` = `%d` - ?, lastpeek = ? WHERE planet_id = ? LIMIT 1", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal), harvestMetal, harvestCrystal, task.End, fleet.TargetPlanetID); err != nil {
		return err
	}
	returning := fleet
	returning.Metal = maxFloat(0, fleet.Metal) + harvestMetal
	returning.Crystal = maxFloat(0, fleet.Crystal) + harvestCrystal
	returning.Deuterium = maxFloat(0, fleet.Deuterium)
	returnFleetID, err := r.insertRecallFleet(ctx, fleetTable, fleet.OwnerID, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, fleet.Mission+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.FlightTime)); err != nil {
		return err
	}
	if messageFound {
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End); err != nil {
			return err
		}
		if err := r.insertRecycleArrivalMessage(ctx, messagesTable, messageContext, recyclers, cargo, target, harvestMetal, harvestCrystal, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadRecycleTarget(ctx context.Context, planetsTable string, planetID int) (recycleTargetState, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT type, `%d`, `%d` FROM %s WHERE planet_id = ? LIMIT 1", resourceMetal, resourceCrystal, planetsTable), planetID)
	if err != nil {
		return recycleTargetState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return recycleTargetState{}, false, rows.Err()
	}
	var target recycleTargetState
	if err := rows.Scan(&target.Type, &target.Metal, &target.Crystal); err != nil {
		return recycleTargetState{}, false, err
	}
	return target, true, rows.Err()
}

func recycleHarvest(metal float64, crystal float64, cargo float64) (float64, float64) {
	harvestMetal := minFloat(metal, cargo/2)
	cargo -= harvestMetal
	harvestCrystal := minFloat(crystal, cargo)
	cargo = maxFloat(0, cargo-harvestCrystal)
	harvestMetal += minFloat(metal-harvestMetal, cargo)
	return harvestMetal, harvestCrystal
}

func (r FleetRepository) finishExpeditionArrival(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, task fleetQueueTask, fleet recallFleetRow) error {
	value, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("expedition arrival context unavailable")
	}
	orbiting := fleet
	orbiting.Fuel = 0
	holdFleetID, err := r.insertFleetTransition(ctx, fleetTable, fleet.OwnerID, orbiting, fleet.Mission+domaingame.FleetMissionOrbitingOffset, int64(fleet.DeployTime), int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, holdFleetID, fleet.Mission+domaingame.FleetMissionOrbitingOffset, task.End, int64(fleet.DeployTime)); err != nil {
		return err
	}
	if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, value, orbiting, fleet.Mission+domaingame.FleetMissionOrbitingOffset, int64(fleet.DeployTime), int64(fleet.FlightTime), task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishExpeditionHold(ctx context.Context, uniTable string, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, messagesTable string, usersTable string, expeditionTable string, battleTable string, task fleetQueueTask, fleet recallFleetRow) error {
	settings, err := r.loadExpeditionSettings(ctx, expeditionTable)
	if err != nil {
		return err
	}
	target, err := r.loadExpeditionTargetState(ctx, planetsTable, usersTable, fleet.TargetPlanetID, fleet.OwnerID)
	if err != nil {
		return err
	}
	value, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("expedition hold context unavailable")
	}
	result, err := domaingame.ResolveExpedition(domaingame.ExpeditionInput{
		Settings: expeditionDomainSettings(settings), VisitCounter: target.VisitCounter,
		HoldSeconds: fleet.FlightTime, FlightSeconds: fleet.DeployTime,
		Fleet: fleet.Ships, Loaded: domaingame.Resources{Metal: fleet.Metal, Crystal: fleet.Crystal, Deuterium: fleet.Deuterium},
		TopScore: target.TopScore, Trader: target.Trader,
	}, r.combatRandom)
	if err != nil {
		return err
	}
	if err := r.applyExpeditionOutcome(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, messagesTable, usersTable, battleTable, task, fleet, value, target, result); err != nil {
		return err
	}

	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.TargetPlanetID, 1, 0, 0, task.End); err != nil {
		return err
	}
	if err := r.insertExpeditionMessage(ctx, messagesTable, fleet.OwnerID, target, expeditionMessageText(target.Language, result), task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) finishReturningFleetArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext := fleetMessageContext{}
	found := false
	if r.legacyEvents {
		var err error
		messageContext, found, err = r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
		if err != nil {
			return err
		}
	}
	if err := r.addFleetResourcesToPlanet(ctx, planetsTable, fleet.StartPlanetID, fleet.Metal, fleet.Crystal, fleet.Deuterium, task.End); err != nil {
		return err
	}
	if err := r.addFleetShipsToPlanet(ctx, planetsTable, fleet.StartPlanetID, fleet.Ships, task.End); err != nil {
		return err
	}
	if found && fleet.Mission != domaingame.FleetMissionSpy+domaingame.FleetMissionReturnOffset {
		if err := r.insertFleetReturnMessage(ctx, messagesTable, messageContext, fleet, task.End); err != nil {
			return err
		}
	}
	if fleet.Mission == domaingame.FleetMissionColonize+domaingame.FleetMissionReturnOffset {
		if err := r.deleteColonizationPhantom(ctx, planetsTable, fleet.TargetPlanetID); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadFleetMessageContext(ctx context.Context, usersTable string, planetsTable string, fleet recallFleetRow) (fleetMessageContext, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT ou.player_id, ou.oname, op.name, op.g, op.s, op.p, op.type, op.`%d`, op.`%d`, op.`%d`, tu.player_id, tu.oname, tp.name, tp.g, tp.s, tp.p, tp.type, tp.`%d`, tp.`%d`, tp.`%d` FROM %s op JOIN %s ou ON ou.player_id = op.owner_id JOIN %s tp ON tp.planet_id = ? JOIN %s tu ON tu.player_id = tp.owner_id WHERE op.planet_id = ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, resourceMetal, resourceCrystal, resourceDeuterium, planetsTable, usersTable, planetsTable, usersTable), fleet.TargetPlanetID, fleet.StartPlanetID)
	if err != nil {
		return fleetMessageContext{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return fleetMessageContext{}, false, rows.Err()
	}
	var value fleetMessageContext
	if err := rows.Scan(&value.OriginOwnerID, &value.OriginOwnerName, &value.OriginName, &value.OriginGalaxy, &value.OriginSystem, &value.OriginPosition, &value.OriginType, &value.OriginMetal, &value.OriginCrystal, &value.OriginDeuterium, &value.TargetOwnerID, &value.TargetOwnerName, &value.TargetName, &value.TargetGalaxy, &value.TargetSystem, &value.TargetPosition, &value.TargetType, &value.TargetMetal, &value.TargetCrystal, &value.TargetDeuterium); err != nil {
		return fleetMessageContext{}, false, err
	}
	return value, true, rows.Err()
}

func (r FleetRepository) insertFleetTransitionLog(ctx context.Context, fleetLogsTable string, value fleetMessageContext, fleet recallFleetRow, mission int, seconds int64, deploySeconds int64, at int64) error {
	ids := domaingame.FleetIDs()
	args := []any{value.OriginOwnerID, value.TargetOwnerID, fleet.Fuel / 2, mission, seconds, deploySeconds, at, at + seconds, value.OriginGalaxy, value.OriginSystem, value.OriginPosition, value.OriginType, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, value.TargetType, int(value.OriginMetal), int(value.OriginCrystal), int(value.OriginDeuterium), fleet.Metal, fleet.Crystal, fleet.Deuterium}
	args = append(args, fleetCountValues(ids, fleet.Ships)...)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, target_id, union_id, fuel, mission, flight_time, deploy_time, start, end, origin_g, origin_s, origin_p, origin_type, target_g, target_s, target_p, target_type, `p%d`, `p%d`, `p%d`, `%d`, `%d`, `%d`, %s) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetLogsTable, resourceMetal, resourceCrystal, resourceDeuterium, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))), args...)
	return err
}

func (r FleetRepository) insertTransportArrivalMessages(ctx context.Context, messagesTable string, value fleetMessageContext, fleet recallFleetRow, at int64) error {
	targetLink := fleetGalaxyLink(value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
	ownText := fmt.Sprintf("Your fleet reaches the planet (\n%s\n) and delivers its cargo:.\n<br/>\n%s metal, %s crystal and %s deuterium.\n<br/>\n", targetLink, fleetLegacyNumber(fleet.Metal), fleetLegacyNumber(fleet.Crystal), fleetLegacyNumber(fleet.Deuterium))
	if err := r.insertFleetMessage(ctx, messagesTable, value.OriginOwnerID, "Fleet Command", "Reaching the planet", ownText, at); err != nil {
		return err
	}
	if value.OriginOwnerID == value.TargetOwnerID {
		return nil
	}
	otherText := fmt.Sprintf("Player %s\\'s fleet is delivering to your planet %s\n%s\n<br/>\n%s metal, %s crystal and %s deuterium\n<br/>\nBefore you had %s metal, %s crystal and %s deuterium.\n<br/>\nNow you have %s0 metal, %s1 crystal and %s2 deuterium.\n<br/>\n", value.OriginOwnerName, value.TargetName, targetLink, fleetLegacyNumber(fleet.Metal), fleetLegacyNumber(fleet.Crystal), fleetLegacyNumber(fleet.Deuterium), fleetLegacyNumber(value.TargetMetal), fleetLegacyNumber(value.TargetCrystal), fleetLegacyNumber(value.TargetDeuterium), value.OriginOwnerName, value.OriginOwnerName, value.OriginOwnerName)
	return r.insertFleetMessage(ctx, messagesTable, value.TargetOwnerID, "Observation", "Foreign fleet is delivering supplies", otherText, at)
}

func (r FleetRepository) insertDeployArrivalMessage(ctx context.Context, messagesTable string, value fleetMessageContext, fleet recallFleetRow, at int64) error {
	text := fmt.Sprintf("\nOne of your fleets (%s) reached %s\n%s\n. The fleet delivers %s metal, %s crystal and %s deuterium\n<br/>\n", fleetLegacyList(fleet.Ships), value.TargetName, fleetGalaxyLink(value.TargetGalaxy, value.TargetSystem, value.TargetPosition), fleetLegacyNumber(fleet.Metal), fleetLegacyNumber(fleet.Crystal), fleetLegacyNumber(fleet.Deuterium+float64(fleet.Fuel/2)))
	return r.insertFleetMessage(ctx, messagesTable, value.OriginOwnerID, "Fleet Command", "Fleet retention", text, at)
}

func (r FleetRepository) insertRecycleArrivalMessage(ctx context.Context, messagesTable string, value fleetMessageContext, recyclers int, cargo float64, target recycleTargetState, metal float64, crystal float64, at int64) error {
	subject := "\n<span class=\"espionagereport\">Intelligence</span>\n"
	text := fmt.Sprintf("The %s recyclers have a total capacity of %s. The debris field contains %s metal and %s crystal. Recycled %s metal and %s crystal.", fleetLegacyNumber(float64(recyclers)), fleetLegacyNumber(cargo), fleetLegacyNumber(target.Metal), fleetLegacyNumber(target.Crystal), fleetLegacyNumber(metal), fleetLegacyNumber(crystal))
	return r.insertFleetMessage(ctx, messagesTable, value.OriginOwnerID, "Fleet ", subject, text, at)
}

func (r FleetRepository) insertFleetReturnMessage(ctx context.Context, messagesTable string, value fleetMessageContext, fleet recallFleetRow, at int64) error {
	text := fmt.Sprintf("One of your fleets ( %s ), sent from %s, reaches %s %s . ", fleetLegacyList(fleet.Ships), fleetGalaxyLink(value.TargetGalaxy, value.TargetSystem, value.TargetPosition), value.OriginName, fleetGalaxyLink(value.OriginGalaxy, value.OriginSystem, value.OriginPosition))
	if fleet.Metal+fleet.Crystal+fleet.Deuterium != 0 {
		text += fmt.Sprintf("The fleet delivers %s metal, %s crystal and %s deuterium<br>", fleetLegacyNumber(fleet.Metal), fleetLegacyNumber(fleet.Crystal), fleetLegacyNumber(fleet.Deuterium))
	}
	return r.insertFleetMessage(ctx, messagesTable, value.OriginOwnerID, "Fleet Command", "Return of the fleet", text, at)
}

func (r FleetRepository) insertFleetMessage(ctx context.Context, messagesTable string, ownerID int, from string, subject string, text string, at int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeMisc, from, subject, text, at)
	return err
}

func fleetGalaxyLink(galaxy int, system int, position int) string {
	return fmt.Sprintf(`<a onclick=\"showGalaxy(%d,%d,%d);\" href=\"#\">[%d:%d:%d]</a>`, galaxy, system, position, galaxy, system, position)
}

func fleetLegacyList(ships domaingame.FleetCounts) string {
	var result strings.Builder
	for _, id := range domaingame.FleetIDs() {
		if ships[id] > 0 {
			fmt.Fprintf(&result, "%s: %s ", domaingame.FleetName(id), fleetLegacyNumber(float64(ships[id])))
		}
	}
	return result.String()
}

func fleetLegacyNumber(value float64) string {
	raw := strconv.FormatInt(int64(math.Round(value)), 10)
	for index := len(raw) - 3; index > 0; index -= 3 {
		raw = raw[:index] + "." + raw[index:]
	}
	return raw
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

func (r FleetRepository) updateFleetPlanetActivity(ctx context.Context, planetsTable string, planetID int, activityAt int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET lastakt = ? WHERE planet_id = ? LIMIT 1", planetsTable), activityAt, planetID)
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

func (r FleetRepository) insertExpeditionReturn(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, task fleetQueueTask, original recallFleetRow, value fleetMessageContext, seconds int64, returning recallFleetRow) error {
	returnFleetID, err := r.insertFleetTransition(ctx, fleetTable, original.OwnerID, returning, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, seconds, 0)
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, original.OwnerID, returnFleetID, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, task.End, seconds); err != nil {
		return err
	}
	return r.insertFleetTransitionLog(ctx, fleetLogsTable, value, returning, domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset, seconds, 0, task.End)
}

func (r FleetRepository) insertExpeditionMessage(ctx context.Context, messagesTable string, ownerID int, target expeditionTargetState, text string, at int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypeExpedition,
		expeditionLocaleValue(target.Language, "FLEET_MESSAGE_FROM"),
		expeditionFormat(expeditionLocaleValue(target.Language, "EXP_MESSAGE_SUBJ"), target.Galaxy, target.System, target.Position),
		text,
		at,
	)
	return err
}

func (r FleetRepository) loadExpeditionSettings(ctx context.Context, expeditionTable string) (expeditionSettings, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT chance_success, depleted_min, depleted_med, depleted_max, chance_depleted_min, chance_depleted_med, chance_depleted_max, chance_alien, chance_pirates, chance_dm, chance_lost, chance_delay, chance_accel, chance_res, chance_fleet, dm_factor, score_cap1, score_cap2, score_cap3, score_cap4, score_cap5, score_cap6, score_cap7, score_cap8, limit_cap1, limit_cap2, limit_cap3, limit_cap4, limit_cap5, limit_cap6, limit_cap7, limit_cap8, limit_max FROM %s LIMIT 1", expeditionTable),
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
		&settings.DepletedMin,
		&settings.DepletedMed,
		&settings.DepletedMax,
		&settings.ChanceDepletedMin,
		&settings.ChanceDepletedMed,
		&settings.ChanceDepletedMax,
		&settings.ChanceAlien,
		&settings.ChancePirates,
		&settings.ChanceDM,
		&settings.ChanceLost,
		&settings.ChanceDelay,
		&settings.ChanceAccel,
		&settings.ChanceRes,
		&settings.ChanceFleet,
		&settings.DMFactor,
		&settings.ScoreCaps[0], &settings.ScoreCaps[1], &settings.ScoreCaps[2], &settings.ScoreCaps[3],
		&settings.ScoreCaps[4], &settings.ScoreCaps[5], &settings.ScoreCaps[6], &settings.ScoreCaps[7],
		&settings.PointLimits[0], &settings.PointLimits[1], &settings.PointLimits[2], &settings.PointLimits[3],
		&settings.PointLimits[4], &settings.PointLimits[5], &settings.PointLimits[6], &settings.PointLimits[7],
		&settings.PointLimitMax,
	); err != nil {
		return expeditionSettings{}, err
	}
	if err := rows.Err(); err != nil {
		return expeditionSettings{}, err
	}
	return settings, nil
}

func (r FleetRepository) loadExpeditionTargetState(ctx context.Context, planetsTable string, usersTable string, planetID int, ownerID int) (expeditionTargetState, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT p.g, p.s, p.p, p.`%d`, COALESCE(u.lang,'en'), COALESCE(u.trader,0), COALESCE(u.rate_m,0), COALESCE(u.rate_k,0), COALESCE(u.rate_d,0), COALESCE(u.`%d`,0), COALESCE(u.`%d`,0), COALESCE(u.`%d`,0), COALESCE((SELECT MAX(score1) FROM %s),0) FROM %s p JOIN %s u ON u.player_id = ? WHERE p.planet_id = ? LIMIT 1", resourceMetal, domaingame.ResearchWeapon, domaingame.ResearchShield, domaingame.ResearchArmour, usersTable, planetsTable, usersTable),
		ownerID,
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
	if err := rows.Scan(&target.Galaxy, &target.System, &target.Position, &target.VisitCounter, &target.Language, &target.Trader.OfferID, &target.Trader.Metal, &target.Trader.Crystal, &target.Trader.Deut, &target.Weapon, &target.Shield, &target.Armour, &target.TopScore); err != nil {
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

func (r FleetRepository) activateExpeditionTrader(ctx context.Context, usersTable string, ownerID int, trader domaingame.ExpeditionTraderState) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET trader = ?, rate_m = ?, rate_k = ?, rate_d = ? WHERE player_id = ? LIMIT 1", usersTable), trader.OfferID, trader.Metal, trader.Crystal, trader.Deut, ownerID)
	return err
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

func minFloat(left float64, right float64) float64 {
	if left < right {
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

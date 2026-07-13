package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (r FleetRepository) finishACSAttackFleetArrival(
	ctx context.Context,
	uniTable string,
	fleetTable string,
	fleetLogsTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	messagesTable string,
	battleTable string,
	unionTable string,
	task fleetQueueTask,
	head recallFleetRow,
) error {
	if head.UnionID <= 0 {
		return errors.New("ACS attack union unavailable")
	}
	participants, err := r.loadACSCombatParticipants(ctx, fleetTable, queueTable, planetsTable, usersTable, head.UnionID)
	if err != nil {
		return err
	}
	if len(participants) == 0 {
		return errors.New("ACS attack fleets unavailable")
	}
	state, found, err := r.loadUnguardedAttackState(ctx, planetsTable, usersTable, head)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("ACS attack target unavailable")
	}
	settings, err := r.loadCombatUniverseSettings(ctx, uniTable)
	if err != nil {
		return err
	}
	holding, err := r.loadHoldingCombatParticipants(ctx, fleetTable, queueTable, planetsTable, usersTable, head.TargetPlanetID, settings.ACSLimit)
	if err != nil {
		return err
	}
	if r.combatRandom == nil {
		return errors.New("combat random source unavailable")
	}

	attackers := make([]domaingame.CombatSlot, 0, len(participants))
	for _, participant := range participants {
		attackers = append(attackers, domaingame.CombatSlot{
			ObjectID: participant.Fleet.ID,
			PlayerID: participant.Fleet.OwnerID,
			Name:     participant.Context.OriginOwnerName,
			Coords: domaingame.Coordinates{
				Galaxy:   participant.Context.OriginGalaxy,
				System:   participant.Context.OriginSystem,
				Position: participant.Context.OriginPosition,
			},
			Weapon: participant.Weapon,
			Shield: participant.Shield,
			Armour: participant.Armour,
			Units:  combatUnitCounts(participant.Fleet.Ships),
		})
	}
	value := participants[0].Context
	defenders := combatDefenderSlots(head.TargetPlanetID, value, state, holding)
	result, err := domaingame.ResolveCombat(attackers, defenders, settings.RapidFire, domaingame.CombatMaxRounds, r.combatRandom)
	if err != nil {
		return err
	}
	engineers := make([]bool, len(defenders))
	engineers[0] = state.DefenderEngineer
	repaired := domaingame.RepairCombatDefense(result, settings.DefenseRepair, settings.DefenseRepairDelta, engineers, r.combatRandom)
	writeback := domaingame.BuildCombatWriteback(result, repaired, settings.FleetDebrisPercent, settings.DefenseDebrisPercent)

	capacities := make([]int, len(participants))
	totalCargo := 0
	if result.Outcome == domaingame.CombatAttackerWon {
		for index, participant := range participants {
			capacities[index] = domaingame.FleetAvailableCargo(
				domaingame.FleetCounts(writeback.AttackerSurvivors[index]),
				domaingame.Resources{Metal: participant.Fleet.Metal, Crystal: participant.Fleet.Crystal, Deuterium: participant.Fleet.Deuterium},
				participant.Fleet.Fuel,
			)
			totalCargo += capacities[index]
		}
	}
	captured := domaingame.BattlePlunder(totalCargo, domaingame.Resources{
		Metal: value.TargetMetal, Crystal: value.TargetCrystal, Deuterium: value.TargetDeuterium,
	})
	if err := r.addGuardedBattleDebris(ctx, planetsTable, value, writeback.Debris); err != nil {
		return err
	}

	report := combatBattleReport(result, writeback, repaired, captured, task.End)
	battleID, err := r.insertBattleData(ctx, battleTable, acsBattleSource(result, settings.RapidFire), task.End)
	if err != nil {
		return err
	}
	if err := r.insertACSBattleMessages(ctx, messagesTable, participants, holding, value, report, result, writeback, task.End); err != nil {
		return err
	}
	if err := r.updateACSBattleData(ctx, battleTable, battleID, value, report, result.Outcome, writeback); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE date < ?", battleTable), task.End-2*7*24*60*60); err != nil {
		return err
	}
	if err := r.subtractAttackPlunder(ctx, planetsTable, head.TargetPlanetID, captured, task.End); err != nil {
		return err
	}
	if err := r.setPlanetCombatUnits(ctx, planetsTable, head.TargetPlanetID, writeback.DefenderSurvivors[0]); err != nil {
		return err
	}
	if err := r.writebackHoldingCombatFleets(ctx, fleetTable, queueTable, holding, writeback.DefenderSurvivors); err != nil {
		return err
	}

	for index, participant := range participants {
		share := combatPlunderShare(captured, capacities[index], totalCargo)
		participantTask := fleetQueueTask{TaskID: participant.Queue.TaskID, OwnerID: participant.Fleet.OwnerID, FleetID: participant.Fleet.ID, End: task.End}
		if err := r.returnGuardedAttackSurvivors(ctx, fleetTable, fleetLogsTable, queueTable, participantTask, participant.Fleet, participant.Context, writeback.AttackerSurvivors[index], share); err != nil {
			return err
		}
		if err := r.adjustCombatStats(ctx, usersTable, participant.Fleet.OwnerID, writeback.AttackerLosses[index]); err != nil {
			return err
		}
	}
	if err := r.adjustCombatDefenderStats(ctx, usersTable, value, holding, writeback.DefenderLosses); err != nil {
		return err
	}
	if err := (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable); err != nil {
		return err
	}
	for _, participant := range participants {
		if err := r.removeCompletedFleetTask(ctx, fleetTable, queueTable, participant.Fleet.ID, participant.Queue.TaskID); err != nil {
			return err
		}
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE union_id = ? LIMIT 1", unionTable), head.UnionID)
	return err
}

func (r FleetRepository) loadACSCombatParticipants(ctx context.Context, fleetTable string, queueTable string, planetsTable string, usersTable string, unionID int) ([]combatFleetParticipant, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fleet_id FROM %s WHERE union_id = ? ORDER BY fleet_id", fleetTable), unionID)
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	participants := make([]combatFleetParticipant, 0, len(ids))
	for _, id := range ids {
		participant, err := r.loadCombatFleetParticipant(ctx, fleetTable, queueTable, planetsTable, usersTable, id, "ACS participant")
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	return participants, nil
}

func (r FleetRepository) loadCombatPlayerTechnology(ctx context.Context, usersTable string, playerID int) (int, int, int, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0) FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchWeapon, domaingame.ResearchShield, domaingame.ResearchArmour, usersTable), playerID)
	if err != nil {
		return 0, 0, 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, 0, 0, false, rows.Err()
	}
	var weapon, shield, armour int
	if err := rows.Scan(&weapon, &shield, &armour); err != nil {
		return 0, 0, 0, false, err
	}
	return weapon, shield, armour, true, rows.Err()
}

func combatPlunderShare(captured domaingame.Resources, capacity int, totalCapacity int) domaingame.Resources {
	if capacity <= 0 || totalCapacity <= 0 {
		return domaingame.Resources{}
	}
	ratio := float64(capacity) / float64(totalCapacity)
	return domaingame.Resources{Metal: captured.Metal * ratio, Crystal: captured.Crystal * ratio, Deuterium: captured.Deuterium * ratio}
}

func combatLossTotals(writeback domaingame.CombatWriteback) (int64, int64) {
	var attackerLoss, defenderLoss int64
	for _, loss := range writeback.AttackerLosses {
		attackerLoss += loss.Points
	}
	for _, loss := range writeback.DefenderLosses {
		defenderLoss += loss.Points
	}
	return attackerLoss, defenderLoss
}

func (r FleetRepository) insertACSBattleMessages(ctx context.Context, messagesTable string, participants []combatFleetParticipant, holding []combatFleetParticipant, value fleetMessageContext, report string, result domaingame.CombatResult, writeback domaingame.CombatWriteback, at int64) error {
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	attackerStyle, defenderStyle := guardedBattleStyles(result.Outcome)
	seen := map[int]bool{}
	for _, ownerID := range combatDefenderOwnerIDs(value, holding) {
		if seen[ownerID] {
			continue
		}
		if err := r.insertBattleMessagePairWithLosses(ctx, messagesTable, ownerID, value, report, defenderStyle, defenderLoss, attackerLoss, at); err != nil {
			return err
		}
		seen[ownerID] = true
	}
	attackerReport := report
	if result.Outcome == domaingame.CombatDefenderWon && len(result.Rounds) <= 2 {
		attackerReport = fmt.Sprintf("Contact with the attacking fleet has been lost. <br> (That means it was destroyed during the first round.) <!--A:%d,W:%d-->", attackerLoss, defenderLoss)
	}
	for _, participant := range participants {
		if seen[participant.Fleet.OwnerID] {
			continue
		}
		if err := r.insertBattleMessagePairWithLosses(ctx, messagesTable, participant.Fleet.OwnerID, value, attackerReport, attackerStyle, defenderLoss, attackerLoss, at); err != nil {
			return err
		}
		seen[participant.Fleet.OwnerID] = true
	}
	return nil
}

func (r FleetRepository) updateACSBattleData(ctx context.Context, battleTable string, battleID int64, value fleetMessageContext, report string, outcome domaingame.CombatOutcome, writeback domaingame.CombatWriteback) error {
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	attackerStyle, _ := guardedBattleStyles(outcome)
	title := battleReportLinkSubjectWithLosses(battleID, value, attackerStyle, true, defenderLoss, attackerLoss)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET title = ?, report = ? WHERE battle_id = ? LIMIT 1", battleTable), title, report, battleID)
	return err
}

func acsBattleSource(result domaingame.CombatResult, rapidFire bool) string {
	var source strings.Builder
	fmt.Fprintf(&source, "MaxRound = %d\nRapidfire = %d\nAttackers = %d\nDefenders = %d\n", domaingame.CombatMaxRounds, combatBoolInt(rapidFire), len(result.Before.Attackers), len(result.Before.Defenders))
	for index, slot := range result.Before.Attackers {
		fmt.Fprintf(&source, "Attacker%d = %d %d %d", index, slot.Weapon, slot.Shield, slot.Armour)
		for _, id := range domaingame.FleetIDs() {
			fmt.Fprintf(&source, " %d %d", id, slot.Units[id])
		}
		fmt.Fprintf(&source, "\nAttacker%dName = %s\n", index, slot.Name)
	}
	for index, slot := range result.Before.Defenders {
		fmt.Fprintf(&source, "Defender%d = %d %d %d", index, slot.Weapon, slot.Shield, slot.Armour)
		for _, id := range combatReportDefenderIDs() {
			fmt.Fprintf(&source, " %d %d", id, slot.Units[id])
		}
		fmt.Fprintf(&source, "\nDefender%dName = %s\n", index, slot.Name)
	}
	return source.String()
}

func combatBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

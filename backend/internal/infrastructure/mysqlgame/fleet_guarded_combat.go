package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (r FleetRepository) finishGuardedAttackFleetArrival(
	ctx context.Context,
	uniTable string,
	fleetTable string,
	fleetLogsTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	messagesTable string,
	battleTable string,
	task fleetQueueTask,
	fleet recallFleetRow,
	messageContext fleetMessageContext,
	state unguardedAttackState,
) error {
	settings, err := r.loadCombatUniverseSettings(ctx, uniTable)
	if err != nil {
		return err
	}
	holding, err := r.loadHoldingCombatParticipants(ctx, fleetTable, queueTable, planetsTable, usersTable, fleet.TargetPlanetID, settings.ACSLimit)
	if err != nil {
		return err
	}
	if r.combatRandom == nil {
		return errors.New("combat random source unavailable")
	}

	attacker := domaingame.CombatSlot{
		ObjectID: fleet.ID, PlayerID: fleet.OwnerID, Name: messageContext.OriginOwnerName,
		Coords: domaingame.Coordinates{Galaxy: messageContext.OriginGalaxy, System: messageContext.OriginSystem, Position: messageContext.OriginPosition},
		Weapon: state.OriginWeapon, Shield: state.OriginShield, Armour: state.OriginArmour,
		Units: combatUnitCounts(fleet.Ships),
	}
	defenders := combatDefenderSlots(fleet.TargetPlanetID, messageContext, state, holding)
	result, err := domaingame.ResolveCombat([]domaingame.CombatSlot{attacker}, defenders, settings.RapidFire, domaingame.CombatMaxRounds, r.combatRandom)
	if err != nil {
		return err
	}
	engineers := make([]bool, len(defenders))
	engineers[0] = state.DefenderEngineer
	repaired := domaingame.RepairCombatDefense(result, settings.DefenseRepair, settings.DefenseRepairDelta, engineers, r.combatRandom)
	writeback := domaingame.BuildCombatWriteback(result, repaired, settings.FleetDebrisPercent, settings.DefenseDebrisPercent)

	captured := domaingame.Resources{}
	if result.Outcome == domaingame.CombatAttackerWon {
		survivors := domaingame.FleetCounts(writeback.AttackerSurvivors[0])
		availableCargo := domaingame.FleetAvailableCargo(survivors, domaingame.Resources{
			Metal: fleet.Metal, Crystal: fleet.Crystal, Deuterium: fleet.Deuterium,
		}, fleet.Fuel)
		captured = domaingame.BattlePlunder(availableCargo, domaingame.Resources{
			Metal: messageContext.TargetMetal, Crystal: messageContext.TargetCrystal, Deuterium: messageContext.TargetDeuterium,
		})
	}
	if err := r.addGuardedBattleDebris(ctx, planetsTable, messageContext, writeback.Debris); err != nil {
		return err
	}
	moon, err := r.maybeCreateBattleMoon(ctx, planetsTable, usersTable, fleet.TargetPlanetID, messageContext, writeback.Debris)
	if err != nil {
		return err
	}

	report := combatBattleReport(result, writeback, repaired, captured, moon, task.End)
	battleID, err := r.insertBattleData(ctx, battleTable, acsBattleSource(result, settings.RapidFire), task.End)
	if err != nil {
		return err
	}
	if err := r.insertGuardedBattleMessages(ctx, messagesTable, holding, messageContext, report, result, writeback, task.End); err != nil {
		return err
	}
	if err := r.updateGuardedBattleData(ctx, battleTable, battleID, messageContext, report, result.Outcome, writeback); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE date < ?", battleTable), task.End-2*7*24*60*60); err != nil {
		return err
	}
	moonDestruction, err := r.finishMoonDestruction(ctx, fleetTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet, messageContext, domaingame.FleetCounts(writeback.AttackerSurvivors[0]), result.Outcome == domaingame.CombatAttackerWon)
	if err != nil {
		return err
	}

	if err := r.subtractAttackPlunder(ctx, planetsTable, fleet.TargetPlanetID, captured, task.End); err != nil {
		return err
	}
	if err := r.setPlanetCombatUnits(ctx, planetsTable, fleet.TargetPlanetID, writeback.DefenderSurvivors[0]); err != nil {
		return err
	}
	if err := r.writebackHoldingCombatFleets(ctx, fleetTable, queueTable, holding, writeback.DefenderSurvivors); err != nil {
		return err
	}
	if moonDestruction.Code&domaingame.MoonDestroyFleet == 0 {
		if moonDestruction.ReturnTargetID > 0 {
			fleet.TargetPlanetID = moonDestruction.ReturnTargetID
		}
		if err := r.returnGuardedAttackSurvivors(ctx, fleetTable, fleetLogsTable, queueTable, task, fleet, messageContext, writeback.AttackerSurvivors[0], captured); err != nil {
			return err
		}
	}
	if err := r.adjustCombatStats(ctx, usersTable, fleet.OwnerID, writeback.AttackerLosses[0]); err != nil {
		return err
	}
	if err := r.adjustCombatDefenderStats(ctx, usersTable, messageContext, holding, writeback.DefenderLosses); err != nil {
		return err
	}
	if err := (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func combatUnitCounts(source map[int]int) map[int]int {
	units := make(map[int]int, len(source))
	for id, count := range source {
		if count > 0 {
			units[id] = count
		}
	}
	return units
}

func combatFleetTotal(ships domaingame.FleetCounts) int {
	total := 0
	for _, count := range ships {
		total += max(0, count)
	}
	return total
}

func (r FleetRepository) addGuardedBattleDebris(ctx context.Context, planetsTable string, value fleetMessageContext, debris domaingame.Resources) error {
	if err := r.ensureBattleDebris(ctx, planetsTable, value, r.now().Unix()); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET `%d` = `%d` + ?, `%d` = `%d` + ?, lastpeek = ? WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal), debris.Metal, debris.Crystal, r.now().Unix(), value.TargetGalaxy, value.TargetSystem, value.TargetPosition, domaingame.PlanetTypeDebris)
	return err
}

func (r FleetRepository) setPlanetCombatUnits(ctx context.Context, planetsTable string, planetID int, survivors map[int]int) error {
	ids := append([]int{}, domaingame.FleetIDs()...)
	for _, id := range domaingame.DefenseIDs() {
		if id < domaingame.DefenseAntiBallisticMissile {
			ids = append(ids, id)
		}
	}
	sets := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		sets = append(sets, fmt.Sprintf("`%d` = ?", id))
		args = append(args, max(0, survivors[id]))
	}
	args = append(args, planetID)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE planet_id = ? LIMIT 1", planetsTable, strings.Join(sets, ", ")), args...)
	return err
}

func (r FleetRepository) returnGuardedAttackSurvivors(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, task fleetQueueTask, fleet recallFleetRow, value fleetMessageContext, survivors map[int]int, captured domaingame.Resources) error {
	ships := domaingame.FleetCounts(combatUnitCounts(survivors))
	if combatFleetTotal(ships) == 0 {
		return nil
	}
	returning := fleet
	returning.Ships = ships
	returning.Metal = maxFloat(0, fleet.Metal) + captured.Metal
	returning.Crystal = maxFloat(0, fleet.Crystal) + captured.Crystal
	returning.Deuterium = maxFloat(0, fleet.Deuterium) + captured.Deuterium
	returnFleetID, err := r.insertRecallFleet(ctx, fleetTable, fleet.OwnerID, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, fleet.Mission+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.FlightTime)); err != nil {
		return err
	}
	return r.insertFleetTransitionLog(ctx, fleetLogsTable, value, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End)
}

func (r FleetRepository) adjustCombatStats(ctx context.Context, usersTable string, playerID int, loss domaingame.CombatParticipantLoss) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET score1 = score1 - ?, score2 = score2 - ? WHERE player_id = ? AND banned = 0 AND admin = 0", usersTable), loss.Points, loss.FleetUnits, playerID)
	return err
}

func (r FleetRepository) insertGuardedBattleMessages(ctx context.Context, messagesTable string, holding []combatFleetParticipant, value fleetMessageContext, report string, result domaingame.CombatResult, writeback domaingame.CombatWriteback, at int64) error {
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
	if seen[value.OriginOwnerID] {
		return nil
	}
	attackerReport := report
	if result.Outcome == domaingame.CombatDefenderWon && len(result.Rounds) <= 2 {
		attackerReport = fmt.Sprintf("Contact with the attacking fleet has been lost. <br> (That means it was destroyed during the first round.) <!--A:%d,W:%d-->", attackerLoss, defenderLoss)
	}
	return r.insertBattleMessagePairWithLosses(ctx, messagesTable, value.OriginOwnerID, value, attackerReport, attackerStyle, defenderLoss, attackerLoss, at)
}

func (r FleetRepository) insertBattleMessagePairWithLosses(ctx context.Context, messagesTable string, ownerID int, value fleetMessageContext, report string, style string, defenderLoss int64, attackerLoss int64, at int64) error {
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 1, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeBattleReportText, "Fleet Command", "Battle report", report, at)
	if err != nil {
		return err
	}
	reportID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	subject := battleReportLinkSubjectWithLosses(reportID, value, style, false, defenderLoss, attackerLoss)
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, '', 0, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeBattleReportLink, "Fleet Command", subject, at)
	return err
}

func (r FleetRepository) updateGuardedBattleData(ctx context.Context, battleTable string, battleID int64, value fleetMessageContext, report string, outcome domaingame.CombatOutcome, writeback domaingame.CombatWriteback) error {
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	attackerStyle, _ := guardedBattleStyles(outcome)
	title := battleReportLinkSubjectWithLosses(battleID, value, attackerStyle, true, defenderLoss, attackerLoss)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET title = ?, report = ? WHERE battle_id = ? LIMIT 1", battleTable), title, report, battleID)
	return err
}

func guardedBattleStyles(outcome domaingame.CombatOutcome) (string, string) {
	switch outcome {
	case domaingame.CombatAttackerWon:
		return "combatreport_ididattack_iwon", "combatreport_igotattacked_ilost"
	case domaingame.CombatDefenderWon:
		return "combatreport_ididattack_ilost", "combatreport_igotattacked_iwon"
	default:
		return "combatreport_ididattack_draw", "combatreport_igotattacked_draw"
	}
}

func battleReportLinkSubjectWithLosses(reportID int64, value fleetMessageContext, style string, admin bool, defenderLoss int64, attackerLoss int64) string {
	page := "bericht"
	window := "Bericht_Kampf"
	losses := fmt.Sprintf("V:%s,A:%s", fleetLegacyNumber(float64(defenderLoss)), fleetLegacyNumber(float64(attackerLoss)))
	if admin {
		page = "admin&session={PUBLIC_SESSION}&mode=BattleReport"
		return fmt.Sprintf("<a href=\"#\" onclick=\"fenster('index.php?page=%s&bericht=%d', '%s');\" ><span class=\"%s\">Battle report [%d:%d:%d] (%s)</span></a>", page, reportID, window, style, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, losses)
	}
	return fmt.Sprintf("<a href=\"#\" onclick=\"fenster(\\'index.php?page=%s&session={PUBLIC_SESSION}&bericht=%d\\', \\'%s\\');\" ><span class=\"%s\">Battle report [%d:%d:%d] (%s)</span></a>", page, reportID, window, style, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, losses)
}

func combatBattleReport(result domaingame.CombatResult, writeback domaingame.CombatWriteback, repaired []map[int]int, captured domaingame.Resources, moon battleMoonCreation, at int64) string {
	return renderCombatBattleReport(result, &combatBattleReportDetails{
		writeback: &writeback,
		repaired:  repaired,
		captured:  &captured,
		moon:      &moon,
	}, at)
}

type combatBattleReportDetails struct {
	writeback *domaingame.CombatWriteback
	repaired  []map[int]int
	captured  *domaingame.Resources
	moon      *battleMoonCreation
}

func combatExpeditionBattleReport(result domaingame.CombatResult, at int64) string {
	return renderCombatBattleReport(result, nil, at)
}

func renderCombatBattleReport(result domaingame.CombatResult, details *combatBattleReportDetails, at int64) string {
	var report strings.Builder
	fmt.Fprintf(&report, "At %s the following fleets met in battle::<br>", clientLocalTimeHTML(at, clientTimeFormatMessage))
	report.WriteString("<table border=1 width=100%><tr>")
	for _, slot := range result.Before.Attackers {
		report.WriteString(combatBattleSlot(slot, true, true, domaingame.FleetIDs(), slot.Units))
	}
	report.WriteString("</tr></table><table border=1 width=100%><tr>")
	for _, slot := range result.Before.Defenders {
		report.WriteString(combatBattleSlot(slot, false, true, combatReportDefenderIDs(), slot.Units))
	}
	report.WriteString("</tr></table>")
	for _, round := range result.Rounds {
		report.WriteString("<br><center>")
		fmt.Fprintf(&report, "The attacking fleet fires %s times with a total firepower of %s at the defender. The defending shields absorb %s damage", combatLegacyNumber(float64(round.AttackerShots)), combatLegacyNumber(round.AttackerPower), combatLegacyNumber(round.DefenderAbsorbed))
		report.WriteString("<br>")
		fmt.Fprintf(&report, "In total, the defending fleet fires %s times with a total firepower of %s at the attacker. The attackers shields absorb %s damage", combatLegacyNumber(float64(round.DefenderShots)), combatLegacyNumber(round.DefenderPower), combatLegacyNumber(round.AttackerAbsorbed))
		report.WriteString("</center><table border=1 width=100%><tr>")
		for index, slot := range result.Before.Attackers {
			report.WriteString(combatBattleSlot(slot, true, false, domaingame.FleetIDs(), round.Attackers[index].Units))
		}
		report.WriteString("</tr></table><table border=1 width=100%><tr>")
		for index, slot := range result.Before.Defenders {
			report.WriteString(combatBattleSlot(slot, false, false, combatReportDefenderIDs(), round.Defenders[index].Units))
		}
		report.WriteString("</tr></table>")
	}
	switch result.Outcome {
	case domaingame.CombatAttackerWon:
		report.WriteString("<p> The attacker has won the battle!")
		if details != nil && details.captured != nil {
			fmt.Fprintf(&report, "<br>He captured<br>%s metal %s crystal, and %s deuterium", combatLegacyNumber(details.captured.Metal), combatLegacyNumber(details.captured.Crystal), combatLegacyNumber(details.captured.Deuterium))
		}
	case domaingame.CombatDefenderWon:
		report.WriteString("<p> The defender has won the battle!")
	default:
		report.WriteString("<p> The battle ended in a draw, both fleets withdraw to their home planets.")
	}
	if details != nil && details.writeback != nil {
		attackerLoss, defenderLoss := int64(0), int64(0)
		for _, loss := range details.writeback.AttackerLosses {
			attackerLoss += loss.Points
		}
		for _, loss := range details.writeback.DefenderLosses {
			defenderLoss += loss.Points
		}
		fmt.Fprintf(&report, "<br><p><br>The attacker lost a total of %s units.<br>The defender lost a total of %s units.", combatLegacyNumber(float64(attackerLoss)), combatLegacyNumber(float64(defenderLoss)))
		fmt.Fprintf(&report, "<br>At these space coordinates now float %s metal and %s crystal.", combatLegacyNumber(details.writeback.Debris.Metal), combatLegacyNumber(details.writeback.Debris.Crystal))
	}
	if details != nil && details.moon != nil {
		appendBattleMoonReport(&report, *details.moon)
	}
	if details != nil && details.repaired != nil {
		appendCombatRepairReport(&report, details.repaired)
	}
	return report.String()
}

func appendBattleMoonReport(report *strings.Builder, moon battleMoonCreation) {
	if moon.Chance > 0 {
		fmt.Fprintf(report, "<br>The chance for a moon to be created is %d %%", moon.Chance)
	}
	if moon.Created {
		report.WriteString("<br>The enormous amounts of free metal and crystal draw together and form a moon around the planet.")
	}
}

func combatBattleSlot(slot domaingame.CombatSlot, attacker bool, showTechs bool, ids []int, units map[int]int) string {
	role := "Defender"
	if attacker {
		role = "Attacker"
	}
	var result strings.Builder
	result.WriteString("<th><br><center>")
	fmt.Fprintf(&result, "%s %s (<a href=# onclick=showGalaxy(%d,%d,%d); >[%d:%d:%d]</a>)", role, slot.Name, slot.Coords.Galaxy, slot.Coords.System, slot.Coords.Position, slot.Coords.Galaxy, slot.Coords.System, slot.Coords.Position)
	if showTechs {
		fmt.Fprintf(&result, "<br>Weapons: %d%% Shields: %d%% Armour: %d%% ", slot.Weapon*10, slot.Shield*10, slot.Armour*10)
	}
	weapon, shield, armour := slot.Weapon, slot.Shield, slot.Armour
	if !showTechs {
		weapon, shield, armour = 0, 0, 0
	}
	visible := make([]int, 0, len(ids))
	for _, id := range ids {
		if units[id] > 0 {
			visible = append(visible, id)
		}
	}
	if len(visible) == 0 {
		result.WriteString("<br>destroyed")
	} else {
		result.WriteString("<table border=1><tr><th>Type</th>")
		for _, id := range visible {
			fmt.Fprintf(&result, "<th>%s</th>", domaingame.CombatShortName(id))
		}
		result.WriteString("</tr><tr><th>Total</th>")
		for _, id := range visible {
			fmt.Fprintf(&result, "<th>%s</th>", combatLegacyNumber(float64(units[id])))
		}
		for _, row := range []struct {
			label string
			value func(domaingame.CombatUnitStats) float64
		}{
			{label: "Weapons", value: func(stats domaingame.CombatUnitStats) float64 {
				return float64(stats.Attack) * float64(10+weapon) / 10
			}},
			{label: "Shields", value: func(stats domaingame.CombatUnitStats) float64 {
				return float64(stats.Shield) * float64(10+shield) / 10
			}},
			{label: "Armour", value: func(stats domaingame.CombatUnitStats) float64 {
				return float64(stats.Structure) * float64(10+armour) / 100
			}},
		} {
			fmt.Fprintf(&result, "</tr><tr><th>%s</th>", row.label)
			for _, id := range visible {
				stats, _ := domaingame.CombatStatsForUnit(id)
				fmt.Fprintf(&result, "<th>%s</th>", combatLegacyNumber(row.value(stats)))
			}
		}
		result.WriteString("</tr></table>")
	}
	result.WriteString("</center></th>")
	return result.String()
}

func appendCombatRepairReport(report *strings.Builder, repaired []map[int]int) {
	repairOrder := []int{
		domaingame.DefenseRocketLauncher, domaingame.DefenseLightLaser, domaingame.DefenseHeavyLaser,
		domaingame.DefenseGaussCannon, domaingame.DefenseIonCannon, domaingame.DefenseSmallShieldDome,
		domaingame.DefensePlasmaTurret, domaingame.DefenseLargeShieldDome,
	}
	for _, slot := range repaired {
		total := 0
		for _, amount := range slot {
			total += amount
		}
		if total == 0 {
			continue
		}
		report.WriteString("<br>")
		comma := false
		for _, id := range repairOrder {
			if slot[id] == 0 {
				continue
			}
			if comma {
				report.WriteString(", ")
			}
			fmt.Fprintf(report, "%s %s", combatLegacyNumber(float64(slot[id])), combatDefenseName(id))
			comma = true
		}
		report.WriteString("could be repaired.<br>")
	}
}

func combatReportDefenderIDs() []int {
	ids := append([]int{}, domaingame.FleetIDs()...)
	for _, id := range domaingame.DefenseIDs() {
		if id < domaingame.DefenseAntiBallisticMissile {
			ids = append(ids, id)
		}
	}
	return ids
}

func combatDefenseName(id int) string {
	return map[int]string{
		domaingame.DefenseRocketLauncher: "Rocket Launcher", domaingame.DefenseLightLaser: "Light Laser",
		domaingame.DefenseHeavyLaser: "Heavy Laser", domaingame.DefenseGaussCannon: "Gauss Cannon",
		domaingame.DefenseIonCannon: "Ion Cannon", domaingame.DefensePlasmaTurret: "Plasma Turret",
		domaingame.DefenseSmallShieldDome: "Small Shield Dome", domaingame.DefenseLargeShieldDome: "Large Shield Dome",
	}[id]
}

func combatLegacyNumber(value float64) string {
	return fleetLegacyNumber(math.Round(value))
}

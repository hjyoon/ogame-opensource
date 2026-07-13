package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type unguardedAttackState struct {
	GuardCount       int
	DefenderUnits    map[int]int
	OriginWeapon     int
	OriginShield     int
	OriginArmour     int
	DefenderWeapon   int
	DefenderShield   int
	DefenderArmour   int
	DefenderEngineer bool
}

type combatUniverseSettings struct {
	FleetDebrisPercent   int
	DefenseDebrisPercent int
	RapidFire            bool
	DefenseRepair        int
	DefenseRepairDelta   int
	ACSLimit             int
}

func (r FleetRepository) finishAttackFleetArrival(ctx context.Context, uniTable string, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, battleTable string, task fleetQueueTask, fleet recallFleetRow) error {
	messageContext, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("attack fleet context unavailable")
	}
	state, found, err := r.loadUnguardedAttackState(ctx, planetsTable, usersTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("attack target unavailable")
	}
	if state.GuardCount > 0 {
		return r.finishGuardedAttackFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, task, fleet, messageContext, state)
	}
	holding, err := r.hasHoldingCombatFleet(ctx, fleetTable, fleet.TargetPlanetID)
	if err != nil {
		return err
	}
	if holding {
		return r.finishGuardedAttackFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, task, fleet, messageContext, state)
	}

	availableCargo := domaingame.FleetAvailableCargo(fleet.Ships, domaingame.Resources{
		Metal: fleet.Metal, Crystal: fleet.Crystal, Deuterium: fleet.Deuterium,
	}, fleet.Fuel)
	captured := domaingame.BattlePlunder(availableCargo, domaingame.Resources{
		Metal: messageContext.TargetMetal, Crystal: messageContext.TargetCrystal, Deuterium: messageContext.TargetDeuterium,
	})
	if err := r.subtractAttackPlunder(ctx, planetsTable, fleet.TargetPlanetID, captured, task.End); err != nil {
		return err
	}
	if err := r.ensureBattleDebris(ctx, planetsTable, messageContext, task.End); err != nil {
		return err
	}
	moon, err := r.maybeCreateBattleMoon(ctx, planetsTable, usersTable, fleet.TargetPlanetID, messageContext, domaingame.Resources{})
	if err != nil {
		return err
	}

	report := unguardedBattleReport(messageContext, state, fleet, captured, moon, task.End)
	battleID, err := r.insertBattleData(ctx, battleTable, unguardedBattleSource(messageContext, state, fleet), task.End)
	if err != nil {
		return err
	}
	if err := r.insertUnguardedBattleMessages(ctx, messagesTable, messageContext, report, task.End); err != nil {
		return err
	}
	if err := r.updateBattleData(ctx, battleTable, battleID, messageContext, report); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE date < ?", battleTable), task.End-2*7*24*60*60); err != nil {
		return err
	}
	moonDestruction, err := r.finishMoonDestruction(ctx, fleetTable, queueTable, planetsTable, usersTable, messagesTable, task, fleet, messageContext, fleet.Ships, true)
	if err != nil {
		return err
	}

	if moonDestruction.Code&domaingame.MoonDestroyFleet == 0 {
		returning := fleet
		if moonDestruction.ReturnTargetID > 0 {
			returning.TargetPlanetID = moonDestruction.ReturnTargetID
		}
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
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, returning, fleet.Mission+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadUnguardedAttackState(ctx context.Context, planetsTable string, usersTable string, fleet recallFleetRow) (unguardedAttackState, bool, error) {
	guardIDs := append([]int{}, domaingame.FleetIDs()...)
	for _, id := range domaingame.DefenseIDs() {
		if id < domaingame.DefenseAntiBallisticMissile {
			guardIDs = append(guardIDs, id)
		}
	}
	guardParts := make([]string, 0, len(guardIDs))
	for _, id := range guardIDs {
		guardParts = append(guardParts, fmt.Sprintf("COALESCE(tp.`%d`, 0)", id))
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT %s, COALESCE(ou.`%d`, 0), COALESCE(ou.`%d`, 0), COALESCE(ou.`%d`, 0), COALESCE(tu.`%d`, 0), COALESCE(tu.`%d`, 0), COALESCE(tu.`%d`, 0), COALESCE(tu.eng_until, 0) FROM %s tp JOIN %s tu ON tu.player_id = tp.owner_id JOIN %s op ON op.planet_id = ? JOIN %s ou ON ou.player_id = op.owner_id WHERE tp.planet_id = ? LIMIT 1",
		strings.Join(guardParts, ", "), domaingame.ResearchWeapon, domaingame.ResearchShield, domaingame.ResearchArmour, domaingame.ResearchWeapon, domaingame.ResearchShield, domaingame.ResearchArmour,
		planetsTable, usersTable, planetsTable, usersTable,
	), fleet.StartPlanetID, fleet.TargetPlanetID)
	if err != nil {
		return unguardedAttackState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return unguardedAttackState{}, false, rows.Err()
	}
	state := unguardedAttackState{DefenderUnits: make(map[int]int, len(guardIDs))}
	counts := make([]int, len(guardIDs))
	destinations := make([]any, 0, len(guardIDs)+7)
	for index := range counts {
		destinations = append(destinations, &counts[index])
	}
	var defenderEngineerUntil int64
	destinations = append(destinations, &state.OriginWeapon, &state.OriginShield, &state.OriginArmour, &state.DefenderWeapon, &state.DefenderShield, &state.DefenderArmour, &defenderEngineerUntil)
	if err := rows.Scan(destinations...); err != nil {
		return unguardedAttackState{}, false, err
	}
	for index, id := range guardIDs {
		if counts[index] > 0 {
			state.DefenderUnits[id] = counts[index]
			state.GuardCount += counts[index]
		}
	}
	state.DefenderEngineer = defenderEngineerUntil > r.now().Unix()
	return state, true, rows.Err()
}

func (r FleetRepository) loadCombatUniverseSettings(ctx context.Context, uniTable string) (combatUniverseSettings, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(fid, 0), COALESCE(did, 0), COALESCE(rapid, 0), COALESCE(defrepair, 0), COALESCE(defrepair_delta, 0), COALESCE(acs, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return combatUniverseSettings{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return combatUniverseSettings{}, err
		}
		return combatUniverseSettings{}, errors.New("combat universe settings unavailable")
	}
	var settings combatUniverseSettings
	var rapid int
	if err := rows.Scan(&settings.FleetDebrisPercent, &settings.DefenseDebrisPercent, &rapid, &settings.DefenseRepair, &settings.DefenseRepairDelta, &settings.ACSLimit); err != nil {
		return combatUniverseSettings{}, err
	}
	settings.RapidFire = rapid != 0
	return settings, rows.Err()
}

func (r FleetRepository) subtractAttackPlunder(ctx context.Context, planetsTable string, planetID int, captured domaingame.Resources, at int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET `%d` = `%d` - ?, `%d` = `%d` - ?, `%d` = `%d` - ?, lastakt = ? WHERE planet_id = ? LIMIT 1", planetsTable, resourceMetal, resourceMetal, resourceCrystal, resourceCrystal, resourceDeuterium, resourceDeuterium), captured.Metal, captured.Crystal, captured.Deuterium, at, planetID)
	return err
}

func (r FleetRepository) ensureBattleDebris(ctx context.Context, planetsTable string, value fleetMessageContext, at int64) error {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT planet_id FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable), value.TargetGalaxy, value.TargetSystem, value.TargetPosition, domaingame.PlanetTypeDebris)
	if err != nil {
		return err
	}
	found := rows.Next()
	if found {
		var debrisID int
		if err := rows.Scan(&debrisID); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 0, ?, 0, 0, 0, ?, ?, 0, 0)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium), "Debris Field", domaingame.PlanetTypeDebris, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, value.TargetOwnerID, at, at, at)
	return err
}

func (r FleetRepository) insertBattleData(ctx context.Context, battleTable string, source string, at int64) (int64, error) {
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (source, title, report, date) VALUES (?, '', '', ?)", battleTable), source, at)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r FleetRepository) updateBattleData(ctx context.Context, battleTable string, battleID int64, value fleetMessageContext, report string) error {
	title := battleReportLinkSubject(battleID, value, "combatreport_ididattack_iwon", true)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET title = ?, report = ? WHERE battle_id = ? LIMIT 1", battleTable), title, report, battleID)
	return err
}

func (r FleetRepository) insertUnguardedBattleMessages(ctx context.Context, messagesTable string, value fleetMessageContext, report string, at int64) error {
	if err := r.insertBattleMessagePair(ctx, messagesTable, value.TargetOwnerID, value, report, "combatreport_igotattacked_ilost", at); err != nil {
		return err
	}
	if value.OriginOwnerID != value.TargetOwnerID {
		if err := r.insertBattleMessagePair(ctx, messagesTable, value.OriginOwnerID, value, report, "combatreport_ididattack_iwon", at); err != nil {
			return err
		}
	}
	return nil
}

func (r FleetRepository) insertBattleMessagePair(ctx context.Context, messagesTable string, ownerID int, value fleetMessageContext, report string, style string, at int64) error {
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 1, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeBattleReportText, "Fleet Command", "Battle report", report, at)
	if err != nil {
		return err
	}
	reportID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	subject := battleReportLinkSubject(reportID, value, style, false)
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, '', 0, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeBattleReportLink, "Fleet Command", subject, at)
	return err
}

func battleReportLinkSubject(reportID int64, value fleetMessageContext, style string, admin bool) string {
	page := "bericht"
	window := "Bericht_Kampf"
	if admin {
		page = "admin&session={PUBLIC_SESSION}&mode=BattleReport"
		return fmt.Sprintf("<a href=\"#\" onclick=\"fenster('index.php?page=%s&bericht=%d', '%s');\" ><span class=\"%s\">Battle report [%d:%d:%d] (V:0,A:0)</span></a>", page, reportID, window, style, value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
	}
	return fmt.Sprintf("<a href=\"#\" onclick=\"fenster(\\'index.php?page=%s&session={PUBLIC_SESSION}&bericht=%d\\', \\'%s\\');\" ><span class=\"%s\">Battle report [%d:%d:%d] (V:0,A:0)</span></a>", page, reportID, window, style, value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
}

func unguardedBattleReport(value fleetMessageContext, state unguardedAttackState, fleet recallFleetRow, captured domaingame.Resources, moon battleMoonCreation, at int64) string {
	var report strings.Builder
	fmt.Fprintf(&report, "At %s the following fleets met in battle::<br>", time.Unix(at, 0).Format("01-02 15:04:05"))
	report.WriteString("<table border=1 width=100%><tr>")
	report.WriteString(unguardedBattleSlot("Attacker", value.OriginOwnerName, value.OriginGalaxy, value.OriginSystem, value.OriginPosition, state.OriginWeapon, state.OriginShield, state.OriginArmour, fleet.Ships))
	report.WriteString("</tr></table><table border=1 width=100%><tr>")
	report.WriteString(unguardedBattleSlot("Defender", value.TargetOwnerName, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, state.DefenderWeapon, state.DefenderShield, state.DefenderArmour, nil))
	report.WriteString("</tr></table><p> The attacker has won the battle!")
	fmt.Fprintf(&report, "<br>He captured<br>%s metal %s crystal, and %s deuterium", fleetLegacyNumber(captured.Metal), fleetLegacyNumber(captured.Crystal), fleetLegacyNumber(captured.Deuterium))
	report.WriteString("<br><p><br>The attacker lost a total of 0 units.<br>The defender lost a total of 0 units.")
	report.WriteString("<br>At these space coordinates now float 0 metal and 0 crystal.")
	appendBattleMoonReport(&report, moon)
	return report.String()
}

func unguardedBattleSlot(role string, name string, galaxy int, system int, position int, weapon int, shield int, armour int, units domaingame.FleetCounts) string {
	var slot strings.Builder
	fmt.Fprintf(&slot, "<th><br><center>%s %s (<a href=# onclick=showGalaxy(%d,%d,%d); >[%d:%d:%d]</a>)", role, name, galaxy, system, position, galaxy, system, position)
	fmt.Fprintf(&slot, "<br>Weapons: %d%% Shields: %d%% Armour: %d%% ", weapon*10, shield*10, armour*10)
	ids := make([]int, 0, len(domaingame.FleetIDs()))
	for _, id := range domaingame.FleetIDs() {
		if units[id] > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		slot.WriteString("<br>destroyed")
	} else {
		slot.WriteString("<table border=1><tr><th>Type</th>")
		for _, id := range ids {
			fmt.Fprintf(&slot, "<th>%s</th>", domaingame.CombatShortName(id))
		}
		slot.WriteString("</tr><tr><th>Total</th>")
		for _, id := range ids {
			fmt.Fprintf(&slot, "<th>%s</th>", fleetLegacyNumber(float64(units[id])))
		}
		for _, row := range []struct {
			label string
			value func(domaingame.CombatUnitStats) float64
		}{
			{label: "Weapons", value: func(stats domaingame.CombatUnitStats) float64 { return float64(stats.Attack) * float64(10+weapon) / 10 }},
			{label: "Shields", value: func(stats domaingame.CombatUnitStats) float64 { return float64(stats.Shield) * float64(10+shield) / 10 }},
			{label: "Armour", value: func(stats domaingame.CombatUnitStats) float64 {
				return float64(stats.Structure) * float64(10+armour) / 100
			}},
		} {
			fmt.Fprintf(&slot, "</tr><tr><th>%s</th>", row.label)
			for _, id := range ids {
				stats, _ := domaingame.CombatStatsForUnit(id)
				fmt.Fprintf(&slot, "<th>%s</th>", fleetLegacyNumber(row.value(stats)))
			}
		}
		slot.WriteString("</tr></table>")
	}
	slot.WriteString("</center></th>")
	return slot.String()
}

func unguardedBattleSource(value fleetMessageContext, state unguardedAttackState, fleet recallFleetRow) string {
	var source strings.Builder
	source.WriteString("MaxRound = 6\n")
	fmt.Fprintf(&source, "Attackers = 1\nDefenders = 1\nAttacker0 = %d %d %d", state.OriginWeapon, state.OriginShield, state.OriginArmour)
	for _, id := range domaingame.FleetIDs() {
		fmt.Fprintf(&source, " %d %d", id, fleet.Ships[id])
	}
	fmt.Fprintf(&source, "\nAttacker0Name = %s\nAttacker0Id = %d\nAttacker0GSP = %d %d %d\nAttacker0Type = 0\n", value.OriginOwnerName, fleet.ID, value.OriginGalaxy, value.OriginSystem, value.OriginPosition)
	fmt.Fprintf(&source, "Defender0 = %d %d %d\nDefender0Name = %s\nDefender0Id = %d\nDefender0GSP = %d %d %d\nDefender0Type = 1\n", state.DefenderWeapon, state.DefenderShield, state.DefenderArmour, value.TargetOwnerName, fleet.TargetPlanetID, value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
	return source.String()
}

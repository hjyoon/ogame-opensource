package mysqlgame

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (r AdminRepository) runAdminBattleSimulator(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	universe, err := r.loadAdminUniverse(ctx)
	if err != nil {
		return nil, err
	}
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	attackers := adminBattleSlots(query.Values, "a", query.Values["anum"], false, random)
	defenders := adminBattleSlots(query.Values, "d", query.Values["dnum"], true, random)
	if strings.TrimSpace(query.Text) != "" {
		attackers, defenders = parseAdminBattleSource(query.Text, random)
	}
	rapidFire := query.Values["rapid"] != 0
	maxRounds, hasMaxRounds := query.Values["max_round"]
	if !hasMaxRounds {
		maxRounds = domaingame.CombatMaxRounds
	}
	result, err := domaingame.ResolveCombat(attackers, defenders, rapidFire, maxRounds, random)
	if err != nil {
		return nil, err
	}
	repaired := domaingame.RepairCombatDefense(result, universe.DefenseRepair, universe.DefenseDelta, make([]bool, len(defenders)), random)
	writeback := domaingame.BuildCombatWriteback(result, repaired, query.Values["fid"], query.Values["did"])
	debrisTotal := max(0.0, writeback.Debris.Metal) + max(0.0, writeback.Debris.Crystal)
	moonChance := min(20, int(math.Floor(debrisTotal/100000)))
	moon := battleMoonCreation{Chance: moonChance, Created: random(100)+1 <= moonChance}
	now := r.now().Unix()
	report := combatBattleReport(result, writeback, repaired, domaingame.Resources{Metal: 1, Crystal: 2, Deuterium: 3}, moon, now)

	battleTable, err := tableName(r.prefix, "battledata")
	if err != nil {
		return nil, err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return nil, err
	}
	battleInsert, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (source, title, report, date) VALUES (?, '', '', ?)", battleTable), adminBattleSource(result, rapidFire, maxRounds), now)
	if err != nil {
		return nil, err
	}
	battleID, err := battleInsert.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE battle_id = ?", battleTable), battleID); err != nil {
		return nil, err
	}
	count, err := r.countAdminMessages(ctx, messagesTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	if count >= 127 {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? ORDER BY date ASC LIMIT 1", messagesTable), query.PlayerID); err != nil {
			return nil, err
		}
	}
	messageInsert, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 1, ?, 0)", messagesTable), query.PlayerID, domaingame.MessageTypeBattleReportText, "Fleet Command", "Battle report", report, now)
	if err != nil {
		return nil, err
	}
	messageID, err := messageInsert.LastInsertId()
	if err != nil {
		return nil, err
	}
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	target := domaingame.Coordinates{}
	if len(defenders) > 0 {
		target = defenders[0].Coords
	}
	style, _ := guardedBattleStyles(result.Outcome)
	link := adminBattleSimulatorLink(messageID, target, style, defenderLoss, attackerLoss)
	values := make(map[string]int, len(query.Values))
	for key, value := range query.Values {
		values[key] = value
	}
	if !hasMaxRounds {
		values["max_round"] = maxRounds
	}
	issue := domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Battle simulator completed.")
	issue.Result = &domaingame.AdminActionResult{Values: values, HTML: link, ItemID: int(messageID)}
	return issue, nil
}

func adminBattleSlots(values map[string]int, prefix string, count int, defender bool, random func(int) int) []domaingame.CombatSlot {
	count = max(0, count)
	slots := make([]domaingame.CombatSlot, count)
	for index := range slots {
		slotPrefix := fmt.Sprintf("%s%d_", prefix, index)
		slots[index] = domaingame.CombatSlot{
			ObjectID: random(10000) + 1, Name: map[bool]string{true: "Defender", false: "Attacker"}[defender] + strconv.Itoa(index),
			Coords: domaingame.Coordinates{Galaxy: random(9) + 1, System: random(499) + 1, Position: random(15) + 1},
			Weapon: values[slotPrefix+"weap"], Shield: values[slotPrefix+"shld"], Armour: values[slotPrefix+"armor"], Units: map[int]int{},
		}
		for _, id := range domaingame.FleetIDs() {
			slots[index].Units[id] = max(0, values[slotPrefix+strconv.Itoa(id)])
		}
		if defender {
			for _, id := range adminBattleDefenseIDs() {
				slots[index].Units[id] = max(0, values[slotPrefix+strconv.Itoa(id)])
			}
		}
	}
	return slots
}

func parseAdminBattleSource(source string, random func(int) int) ([]domaingame.CombatSlot, []domaingame.CombatSlot) {
	attackers := map[int]domaingame.CombatSlot{}
	defenders := map[int]domaingame.CombatSlot{}
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) != 2 {
			continue
		}
		defender := strings.HasPrefix(parts[0], "Defender")
		prefix := "Attacker"
		if defender {
			prefix = "Defender"
		} else if !strings.HasPrefix(parts[0], prefix) {
			continue
		}
		index, err := strconv.Atoi(strings.TrimPrefix(parts[0], prefix))
		fields := strings.Fields(parts[1])
		if err != nil || index < 0 || len(fields) < 3 {
			continue
		}
		slot := domaingame.CombatSlot{ObjectID: random(10000) + 1, Name: prefix + strconv.Itoa(index), Coords: domaingame.Coordinates{Galaxy: random(9) + 1, System: random(499) + 1, Position: random(15) + 1}, Units: map[int]int{}}
		slot.Weapon, _ = strconv.Atoi(fields[0])
		slot.Shield, _ = strconv.Atoi(fields[1])
		slot.Armour, _ = strconv.Atoi(fields[2])
		for field := 3; field+1 < len(fields); field += 2 {
			id, idErr := strconv.Atoi(fields[field])
			amount, amountErr := strconv.Atoi(fields[field+1])
			if idErr == nil && amountErr == nil {
				slot.Units[id] = max(0, amount)
			}
		}
		if defender {
			defenders[index] = slot
		} else {
			attackers[index] = slot
		}
	}
	return orderedAdminBattleSlots(attackers), orderedAdminBattleSlots(defenders)
}

func orderedAdminBattleSlots(values map[int]domaingame.CombatSlot) []domaingame.CombatSlot {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	result := make([]domaingame.CombatSlot, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func adminBattleDefenseIDs() []int {
	result := []int{}
	for _, id := range domaingame.DefenseIDs() {
		if id < domaingame.DefenseAntiBallisticMissile {
			result = append(result, id)
		}
	}
	return result
}

func adminBattleSource(result domaingame.CombatResult, rapidFire bool, maxRounds int) string {
	source := acsBattleSource(result, rapidFire)
	return strings.Replace(source, fmt.Sprintf("MaxRound = %d", domaingame.CombatMaxRounds), fmt.Sprintf("MaxRound = %d", maxRounds), 1)
}

func adminBattleSimulatorLink(reportID int64, target domaingame.Coordinates, style string, defenderLoss int64, attackerLoss int64) string {
	return fmt.Sprintf("<a href=\"#\" onclick=\"fenster('index.php?page=bericht&session={PUBLIC_SESSION}&bericht=%d', 'Bericht_Kampf');\" ><span class=\"%s\">Battle report [%d:%d:%d] (V:%s,A:%s)</span></a><br>", reportID, style, target.Galaxy, target.System, target.Position, fleetLegacyNumber(float64(defenderLoss)), fleetLegacyNumber(float64(attackerLoss)))
}

package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func expeditionDomainSettings(settings expeditionSettings) domaingame.ExpeditionSettings {
	return domaingame.ExpeditionSettings{
		ChanceSuccess: settings.ChanceSuccess, DepletedMin: settings.DepletedMin, DepletedMed: settings.DepletedMed, DepletedMax: settings.DepletedMax,
		ChanceDepletedMin: settings.ChanceDepletedMin, ChanceDepletedMed: settings.ChanceDepletedMed, ChanceDepletedMax: settings.ChanceDepletedMax,
		ChanceAlien: settings.ChanceAlien, ChancePirates: settings.ChancePirates, ChanceDM: settings.ChanceDM, ChanceLost: settings.ChanceLost,
		ChanceDelay: settings.ChanceDelay, ChanceAccel: settings.ChanceAccel, ChanceRes: settings.ChanceRes, ChanceFleet: settings.ChanceFleet,
		DMFactor: settings.DMFactor, ScoreCaps: settings.ScoreCaps, PointLimits: settings.PointLimits, PointLimitMax: settings.PointLimitMax,
	}
}

func (r FleetRepository) applyExpeditionOutcome(ctx context.Context, uniTable string, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, messagesTable string, usersTable string, battleTable string, task fleetQueueTask, fleet recallFleetRow, value fleetMessageContext, target expeditionTargetState, outcome domaingame.ExpeditionOutcome) error {
	returning := fleet
	returning.Ships = copyFleetCounts(fleet.Ships)
	returning.Fuel = 0
	switch outcome.Event {
	case domaingame.ExpeditionDarkMatter:
		if err := r.addExpeditionDarkMatter(ctx, usersTable, fleet.OwnerID, outcome.DarkMatter); err != nil {
			return err
		}
	case domaingame.ExpeditionResources:
		switch outcome.ResourceType {
		case 0:
			returning.Metal += outcome.ResourceAmount
		case 1:
			returning.Crystal += outcome.ResourceAmount
		case 2:
			returning.Deuterium += outcome.ResourceAmount
		}
	case domaingame.ExpeditionFleet:
		for id, amount := range outcome.FoundFleet {
			returning.Ships[id] += amount
		}
		if outcome.FleetUnits > 0 {
			if err := r.adjustExpeditionStats(ctx, usersTable, fleet.OwnerID, outcome.FleetPoints, outcome.FleetUnits, "+"); err != nil {
				return err
			}
			if err := (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable); err != nil {
				return err
			}
		}
	case domaingame.ExpeditionTrader:
		if outcome.UpdateTrader {
			if err := r.activateExpeditionTrader(ctx, usersTable, fleet.OwnerID, outcome.Trader); err != nil {
				return err
			}
		}
	case domaingame.ExpeditionBlackHole:
		score := domaingame.CalculatePlanetScore(nil, fleet.Ships, nil)
		if err := r.adjustExpeditionStats(ctx, usersTable, fleet.OwnerID, score.Points, score.FleetPoints, "-"); err != nil {
			return err
		}
		return (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable)
	case domaingame.ExpeditionAliens, domaingame.ExpeditionPirates:
		return r.finishExpeditionBattle(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, messagesTable, usersTable, battleTable, task, fleet, value, target, outcome)
	}
	return r.insertExpeditionReturn(ctx, fleetTable, fleetLogsTable, queueTable, task, fleet, value, int64(outcome.ReturnSeconds), returning)
}

func (r FleetRepository) finishExpeditionBattle(ctx context.Context, uniTable string, fleetTable string, fleetLogsTable string, queueTable string, messagesTable string, usersTable string, battleTable string, task fleetQueueTask, fleet recallFleetRow, value fleetMessageContext, target expeditionTargetState, outcome domaingame.ExpeditionOutcome) error {
	settings, err := r.loadCombatUniverseSettings(ctx, uniTable)
	if err != nil {
		return err
	}

	attacker := domaingame.CombatSlot{
		ObjectID: fleet.ID, PlayerID: fleet.OwnerID, Name: value.OriginOwnerName,
		Coords: domaingame.Coordinates{Galaxy: value.OriginGalaxy, System: value.OriginSystem, Position: value.OriginPosition},
		Weapon: target.Weapon, Shield: target.Shield, Armour: target.Armour,
		Units: combatUnitCounts(fleet.Ships),
	}
	opponentName := "Aliens"
	opponentWeapon, opponentShield, opponentArmour := target.Weapon+3, target.Shield+3, target.Armour+3
	if outcome.Event == domaingame.ExpeditionPirates {
		opponentName = "Piraten"
		opponentWeapon = max(0, target.Weapon-3)
		opponentShield = max(0, target.Shield-3)
		opponentArmour = max(0, target.Armour-3)
	}
	defender := domaingame.CombatSlot{
		Name:   opponentName,
		Coords: domaingame.Coordinates{Galaxy: target.Galaxy, System: target.System, Position: target.Position},
		Weapon: opponentWeapon, Shield: opponentShield, Armour: opponentArmour,
		Units: combatUnitCounts(outcome.OpponentFleet),
	}
	result, err := domaingame.ResolveCombat([]domaingame.CombatSlot{attacker}, []domaingame.CombatSlot{defender}, settings.RapidFire, domaingame.CombatMaxRounds, r.combatRandom)
	if err != nil {
		return err
	}
	writeback := domaingame.BuildCombatWriteback(result, []map[int]int{{}}, 0, 0)
	report := combatBattleReport(result, writeback, []map[int]int{{}}, domaingame.Resources{}, battleMoonCreation{}, task.End)
	battleID, err := r.insertBattleData(ctx, battleTable, acsBattleSource(result, settings.RapidFire), task.End)
	if err != nil {
		return err
	}
	if err := r.insertExpeditionBattleMessages(ctx, messagesTable, battleID, value, target, report, result, writeback, task.End); err != nil {
		return err
	}
	if err := r.updateExpeditionBattleData(ctx, battleTable, battleID, value, target.Language, report, result, writeback); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE date < ?", battleTable), task.End-2*7*24*60*60); err != nil {
		return err
	}

	survivors := domaingame.FleetCounts(writeback.AttackerSurvivors[0])
	if combatFleetTotal(survivors) > 0 {
		returning := fleet
		returning.Ships = survivors
		if err := r.insertExpeditionReturn(ctx, fleetTable, fleetLogsTable, queueTable, task, fleet, value, int64(outcome.ReturnSeconds), returning); err != nil {
			return err
		}
	}
	if err := r.adjustCombatStats(ctx, usersTable, fleet.OwnerID, writeback.AttackerLosses[0]); err != nil {
		return err
	}
	return (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable)
}

func (r FleetRepository) insertExpeditionBattleMessages(ctx context.Context, messagesTable string, battleID int64, value fleetMessageContext, target expeditionTargetState, report string, result domaingame.CombatResult, writeback domaingame.CombatWriteback, at int64) error {
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	style, _ := guardedBattleStyles(result.Outcome)
	message := report
	if result.Outcome == domaingame.CombatDefenderWon && len(result.Rounds) <= 2 {
		message = fmt.Sprintf("%s <!--A:%d,W:%d-->", expeditionLocaleValue(target.Language, "BATTLE_LOST"), attackerLoss, defenderLoss)
	}
	from := expeditionLocaleValue(target.Language, "FLEET_MESSAGE_FROM")
	subject := expeditionLocaleValue(target.Language, "FLEET_MESSAGE_BATTLE")
	inserted, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 1, ?, 0)", messagesTable), value.OriginOwnerID, domaingame.MessageTypeBattleReportText, from, subject, message, at)
	if err != nil {
		return err
	}
	reportID, err := inserted.LastInsertId()
	if err != nil {
		return err
	}
	if reportID <= 0 {
		return errors.New("expedition battle message id unavailable")
	}
	link := localizedBattleReportLink(reportID, value, style, false, defenderLoss, attackerLoss, subject)
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, '', 0, ?, 0)", messagesTable), value.OriginOwnerID, domaingame.MessageTypeBattleReportLink, from, link, at)
	return err
}

func (r FleetRepository) updateExpeditionBattleData(ctx context.Context, battleTable string, battleID int64, value fleetMessageContext, language string, report string, result domaingame.CombatResult, writeback domaingame.CombatWriteback) error {
	attackerLoss, defenderLoss := combatLossTotals(writeback)
	style, _ := guardedBattleStyles(result.Outcome)
	title := localizedBattleReportLink(battleID, value, style, true, defenderLoss, attackerLoss, expeditionLocaleValue(language, "FLEET_MESSAGE_BATTLE"))
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET title = ?, report = ? WHERE battle_id = ? LIMIT 1", battleTable), title, report, battleID)
	return err
}

func localizedBattleReportLink(reportID int64, value fleetMessageContext, style string, admin bool, defenderLoss int64, attackerLoss int64, subject string) string {
	link := battleReportLinkSubjectWithLosses(reportID, value, style, admin, defenderLoss, attackerLoss)
	return strings.Replace(link, "Battle report", subject, 1)
}

func (r FleetRepository) adjustExpeditionStats(ctx context.Context, usersTable string, ownerID int, points int64, fleetUnits int64, sign string) error {
	if sign != "+" {
		sign = "-"
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET score1 = score1 %s ?, score2 = score2 %s ? WHERE player_id = ? AND banned = 0 AND admin = 0", usersTable, sign, sign), points, fleetUnits, ownerID)
	return err
}

func expeditionMessageText(language string, outcome domaingame.ExpeditionOutcome) string {
	key := "EXP_NOTHING_" + fmt.Sprint(outcome.Variant+1)
	switch outcome.Event {
	case domaingame.ExpeditionAliens:
		key = "EXP_ALIENS_" + expeditionTierName(outcome.Tier, "WEAK", "MED", "STRONG") + "_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionPirates:
		key = "EXP_PIRATES_" + expeditionTierName(outcome.Tier, "WEAK", "MED", "STRONG") + "_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionDarkMatter:
		key = "EXP_DMFOUND_" + expeditionTierName(outcome.Tier, "SMALL", "MED", "LARGE") + "_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionBlackHole:
		key = "EXP_LOST_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionDelay:
		key = "EXP_DELAY_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionAccel:
		key = "EXP_ACCEL_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionResources:
		key = "EXP_RESFOUND_" + expeditionTierName(outcome.Tier, "SMALL", "MED", "LARGE") + "_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionFleet:
		key = "EXP_FLEET_" + expeditionTierName(outcome.Tier, "SMALL", "MED", "LARGE") + "_" + fmt.Sprint(outcome.Variant+1)
	case domaingame.ExpeditionTrader:
		key = "EXP_TRADER_" + fmt.Sprint(outcome.Variant+1)
	}
	text := expeditionLocaleValue(language, key)
	if outcome.Event == domaingame.ExpeditionDarkMatter {
		text += expeditionFormat(expeditionLocaleValue(language, "EXP_FOUND"), fleetLegacyNumber(float64(outcome.DarkMatter)), expeditionResourceName(language, 3))
	}
	if outcome.Event == domaingame.ExpeditionResources {
		text += expeditionFormat(expeditionLocaleValue(language, "EXP_FOUND"), fleetLegacyNumber(outcome.ResourceAmount), expeditionResourceName(language, outcome.ResourceType))
		if outcome.CargoLimited {
			text += "<br><br>" + expeditionLocaleValue(language, "EXP_RESFOUND_LOGBOOK_"+fmt.Sprint(outcome.FooterVariant+1))
		}
	}
	if outcome.Event == domaingame.ExpeditionFleet && len(outcome.FoundFleetOrder) > 0 {
		text += expeditionLocaleValue(language, "EXP_FLEET_FOUND")
		for _, id := range outcome.FoundFleetOrder {
			text += "<br>" + expeditionLocaleValue(language, fmt.Sprintf("NAME_%d", id)) + " " + fleetLegacyNumber(float64(outcome.FoundFleet[id]))
		}
	}
	if outcome.Event == domaingame.ExpeditionFleet && outcome.CargoLimited {
		text += "<br><br>" + expeditionLocaleValue(language, "EXP_FLEET_LOGBOOK_"+fmt.Sprint(outcome.FooterVariant+1))
	}
	if outcome.HasLogbook {
		prefix := []string{"EXP_NOT_DEPLETED_", "EXP_DEPLETED_MIN_", "EXP_DEPLETED_MED_", "EXP_DEPLETED_MAX_"}[max(0, min(3, outcome.LogbookTier))]
		text += "\n<br/>\n<br/>\n" + expeditionLocaleValue(language, prefix+fmt.Sprint(outcome.LogbookVariant+1))
	}
	return text
}

func expeditionTierName(tier int, low string, medium string, high string) string {
	if tier >= 2 {
		return high
	}
	if tier == 1 {
		return medium
	}
	return low
}

func expeditionFormat(template string, values ...any) string {
	for index, value := range values {
		template = strings.ReplaceAll(template, fmt.Sprintf("#%d", index+1), fmt.Sprint(value))
	}
	return template
}

var expeditionLocaleCache sync.Map

func expeditionLocaleValue(language string, key string) string {
	language = expeditionLanguage(language)
	loaded := cachedExpeditionLocale(language)
	values := loaded.(map[string]string)
	if value := values[key]; value != "" {
		return value
	}
	fallback := cachedExpeditionLocale("en")
	if value := fallback.(map[string]string)[key]; value != "" {
		return value
	}
	return key
}

func cachedExpeditionLocale(language string) any {
	if loaded, ok := expeditionLocaleCache.Load(language); ok {
		return loaded
	}
	loaded := loadExpeditionLocale(language)
	actual, _ := expeditionLocaleCache.LoadOrStore(language, loaded)
	return actual
}

func loadExpeditionLocale(language string) map[string]string {
	values := map[string]string{"FLEET_MESSAGE_FROM": "Fleet Command", "EXP_MESSAGE_SUBJ": "Expedition result [#1:#2:#3]"}
	for _, root := range []string{"game", "../game", "../../../../game"} {
		locaDir := filepath.Join(root, "loca", language+"_"+language)
		if _, err := os.Stat(locaDir); err != nil {
			continue
		}
		for _, name := range []string{"expedition.php", "espionage.php", "fleetmsg.php", "technames.php"} {
			fileValues, err := readAdminLocalizationFile(filepath.Join(locaDir, name), language)
			if err != nil {
				continue
			}
			for key, value := range fileValues {
				values[key] = value
			}
		}
		break
	}
	return values
}

func expeditionLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "de", "en", "es", "fr", "it", "jp", "ru":
		return strings.ToLower(strings.TrimSpace(language))
	default:
		return "en"
	}
}

func expeditionResourceName(language string, resourceType int) string {
	names := map[string][4]string{
		"de": {"Metall", "Kristall", "Deuterium", "Dunkle Materie"},
		"en": {"Metal", "Crystal", "Deuterium", "Dark Matter"},
		"es": {"Metal", "Cristal", "Deuterio", "Materia Oscura"},
		"fr": {"Métal", "Cristal", "Deutérium", "Matière noire"},
		"it": {"Metallo", "Cristallo", "Deuterio", "Materia Oscura"},
		"jp": {"メタル", "クリスタル", "重水素", "ダークマター"},
		"ru": {"Металл", "Кристалл", "Дейтерий", "Тёмная материя"},
	}
	return names[expeditionLanguage(language)][max(0, min(3, resourceType))]
}

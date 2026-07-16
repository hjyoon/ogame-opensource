package mysqlgame

import (
	"context"
	"fmt"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	adminQueueTypeUnloadAll        = "UnloadAll"
	adminQueueTypeCleanDebris      = "CleanDebris"
	adminQueueTypeCleanPlanets     = "CleanPlanets"
	adminQueueTypeCleanPlayers     = "CleanPlayers"
	adminQueueTypeUpdateStats      = "UpdateStats"
	adminQueueTypeRecalcAllyPoints = "RecalcAllyPoints"
	adminQueueTypeAllowName        = "AllowName"
	adminQueueTypeChangeEmail      = "ChangeEmail"
	adminQueueTypeUnban            = "UnbanPlayer"
	adminQueueTypeAllowAttacks     = "AllowAttacks"
	adminQueueTypeDebug            = "Debug"

	adminQueuePriorityUpdateStats  = 510
	adminQueuePriorityCleanDebris  = 600
	adminQueuePriorityCleanPlanets = 700
	adminQueuePriorityCleanPlayers = 900
)

type adminCronTables struct {
	uni        string
	users      string
	ally       string
	planets    string
	buildQueue string
	queue      string
	fleet      string
	fleetLogs  string
	messages   string
	expedition string
	battle     string
	union      string
	botstrat   string
	botvars    string
	debug      string
	reports    string
	notes      string
	browse     string
	template   string
	userLogs   string
	ipLogs     string
	allyApps   string
	buddy      string
}

func (r AdminRepository) runAdminCron(ctx context.Context, until int) ([]domaingame.AdminCouponMail, error) {
	tables, err := loadAdminCronTables(r.prefix)
	if err != nil {
		return nil, err
	}
	universe, err := r.loadAdminUniverse(ctx)
	if err != nil {
		return nil, err
	}
	if universe.Freeze {
		return nil, nil
	}
	tasks, err := r.loadAdminCronTasks(ctx, tables.queue, until)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if err := r.finishAdminCronTask(ctx, tables, task, universe.Language); err != nil {
			return nil, err
		}
	}
	return r.finishDueAdminCouponCronTasks(ctx, tables, until)
}

func loadAdminCronTables(prefix string) (adminCronTables, error) {
	var tables adminCronTables
	for suffix, destination := range map[string]*string{
		"uni": &tables.uni, "users": &tables.users, "ally": &tables.ally,
		"planets": &tables.planets, "buildqueue": &tables.buildQueue, "queue": &tables.queue,
		"fleet": &tables.fleet, "fleetlogs": &tables.fleetLogs, "messages": &tables.messages,
		"exptab": &tables.expedition, "battledata": &tables.battle, "union": &tables.union,
		"botstrat": &tables.botstrat, "botvars": &tables.botvars, "debug": &tables.debug,
		"reports": &tables.reports, "notes": &tables.notes, "browse": &tables.browse,
		"template": &tables.template, "userlogs": &tables.userLogs, "iplogs": &tables.ipLogs,
		"allyapps": &tables.allyApps, "buddy": &tables.buddy,
	} {
		name, err := tableName(prefix, suffix)
		if err != nil {
			return adminCronTables{}, err
		}
		*destination = name
	}
	return tables, nil
}

func (r AdminRepository) finishAdminCronTask(ctx context.Context, tables adminCronTables, task buildingQueueTask, language string) error {
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	switch task.Type {
	case queueTypeBuild, queueTypeDemolish:
		buildings := BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: true}
		return buildings.finishBuildingQueueTask(ctx, tables.users, tables.planets, tables.buildQueue, tables.queue, task)
	case queueTypeResearch:
		research := ResearchRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: true}
		return research.finishResearchQueueTask(ctx, tables.users, tables.planets, tables.queue, task)
	case queueTypeShipyard:
		shipyard := ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: true}
		return shipyard.finishShipyardQueueTask(ctx, tables.users, tables.planets, tables.queue, task, task.End)
	case queueTypeFleet:
		fleet := FleetRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, legacyEvents: true, queueProduction: true, combatRandom: random}
		return fleet.finishFleetQueueTask(ctx, tables.uni, tables.fleet, tables.fleetLogs, tables.queue, tables.planets, tables.messages, tables.users, tables.expedition, tables.battle, tables.union, fleetQueueTask{TaskID: task.TaskID, OwnerID: task.OwnerID, FleetID: task.SubID, End: int64(task.End)})
	case queueTypeRecalcPoints:
		if err := r.recalcAdminUserStats(ctx, tables.users, tables.planets, tables.fleet, task.OwnerID); err != nil {
			return err
		}
		return r.removeAdminCronTask(ctx, tables.queue, task.TaskID)
	case queueTypeAI:
		bots := BotRuntimeRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, randInt: random}
		return bots.finishBotQueueTask(ctx, tables.queue, tables.botstrat, tables.botvars, task)
	case adminQueueTypeUnloadAll:
		return r.finishAdminCronUnloadAll(ctx, tables, task)
	case adminQueueTypeCleanDebris:
		return r.finishAdminCronCleanDebris(ctx, tables, task)
	case adminQueueTypeUpdateStats:
		return r.finishAdminCronUpdateStats(ctx, tables, task, language)
	case adminQueueTypeRecalcAllyPoints:
		return r.finishAdminCronRecalcAllyPoints(ctx, tables, task)
	case adminQueueTypeAllowName:
		return r.finishAdminCronUserFlag(ctx, tables, task, fmt.Sprintf("UPDATE %s SET name_changed = 0 WHERE player_id = ?", tables.users))
	case adminQueueTypeChangeEmail:
		return r.finishAdminCronUserFlag(ctx, tables, task, fmt.Sprintf("UPDATE %s SET pemail = email WHERE player_id = ?", tables.users))
	case adminQueueTypeUnban:
		return r.finishAdminCronUserFlag(ctx, tables, task, fmt.Sprintf("UPDATE %s SET banned = 0, banned_until = 0 WHERE player_id = ?", tables.users))
	case adminQueueTypeAllowAttacks:
		return r.finishAdminCronUserFlag(ctx, tables, task, fmt.Sprintf("UPDATE %s SET noattack = 0, noattack_until = 0 WHERE player_id = ?", tables.users))
	case adminQueueTypeDebug:
		return r.removeAdminCronTask(ctx, tables.queue, task.TaskID)
	case adminQueueTypeCleanPlanets:
		return r.finishAdminCronCleanPlanets(ctx, tables, task, language)
	case adminQueueTypeCleanPlayers:
		return r.finishAdminCronCleanPlayers(ctx, tables, task)
	case adminCouponQueueType:
		// Coupon tasks are processed outside the locked core batch, matching legacy UpdateQueue.
		return nil
	default:
		if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
			return err
		}
		return r.insertAdminCronDebug(ctx, tables.debug, adminCronUnknownMessage(language)+task.Type, task.End)
	}
}

func (r AdminRepository) finishAdminCronUserFlag(ctx context.Context, tables adminCronTables, task buildingQueueTask, statement string) error {
	if _, err := r.execer.ExecContext(ctx, statement, task.OwnerID); err != nil {
		return err
	}
	return r.removeAdminCronTask(ctx, tables.queue, task.TaskID)
}

func (r AdminRepository) finishAdminCronUnloadAll(ctx context.Context, tables adminCronTables, task buildingQueueTask) error {
	missions := []int{
		domaingame.FleetMissionExpedition,
		domaingame.FleetMissionExpedition + domaingame.FleetMissionReturnOffset,
		domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset,
	}
	statement := fmt.Sprintf("DELETE p FROM %s p WHERE p.type = ? AND NOT EXISTS (SELECT 1 FROM %s f WHERE f.target_planet = p.planet_id AND f.mission IN (?, ?, ?))", tables.planets, tables.fleet)
	if r.dialect == DialectSQLite {
		statement = fmt.Sprintf("DELETE FROM %s AS p WHERE p.type = ? AND NOT EXISTS (SELECT 1 FROM %s AS f WHERE f.target_planet = p.planet_id AND f.mission IN (?, ?, ?))", tables.planets, tables.fleet)
	}
	if _, err := r.execer.ExecContext(ctx, statement, legacyPlanetTypeFarSpace, missions[0], missions[1], missions[2]); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET session = ''", tables.users)); err != nil {
		return err
	}
	if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET hacks = 0", tables.uni))
	return err
}

func (r AdminRepository) finishAdminCronCleanDebris(ctx context.Context, tables adminCronTables, task buildingQueueTask) error {
	statement := fmt.Sprintf("DELETE p FROM %s p WHERE p.type = ? AND p.`%d` = 0 AND p.`%d` = 0 AND NOT EXISTS (SELECT 1 FROM %s f WHERE f.target_planet = p.planet_id AND f.mission IN (?, ?))", tables.planets, resourceMetal, resourceCrystal, tables.fleet)
	if r.dialect == DialectSQLite {
		statement = fmt.Sprintf("DELETE FROM %s AS p WHERE p.type = ? AND p.`%d` = 0 AND p.`%d` = 0 AND NOT EXISTS (SELECT 1 FROM %s AS f WHERE f.target_planet = p.planet_id AND f.mission IN (?, ?))", tables.planets, resourceMetal, resourceCrystal, tables.fleet)
	}
	if _, err := r.execer.ExecContext(ctx, statement, legacyPlanetTypeDebris, domaingame.FleetMissionRecycle, domaingame.FleetMissionRecycle+domaingame.FleetMissionReturnOffset); err != nil {
		return err
	}
	if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
		return err
	}
	next := nextAdminCronWeekday(time.Unix(int64(task.End), 0).In(r.adminCronLocation()), time.Monday, 1, 10)
	return r.insertAdminCronTask(ctx, tables.queue, adminQueueTypeCleanDebris, task.End, int(next.Unix()), adminQueuePriorityCleanDebris)
}

func (r AdminRepository) finishAdminCronUpdateStats(ctx context.Context, tables adminCronTables, task buildingQueueTask, language string) error {
	for _, table := range []string{tables.users, tables.ally} {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET oldscore1 = score1, oldscore2 = score2, oldscore3 = score3, oldplace1 = place1, oldplace2 = place2, oldplace3 = place3, scoredate = ?", table), task.End); err != nil {
			return err
		}
	}
	if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
		return err
	}
	at := time.Unix(int64(task.End), 0).In(r.adminCronLocation())
	next := nextAdminCronStatsTime(at)
	if err := r.insertAdminCronTask(ctx, tables.queue, adminQueueTypeUpdateStats, task.End, int(next.Unix()), adminQueuePriorityUpdateStats); err != nil {
		return err
	}
	return r.insertAdminCronDebug(ctx, tables.debug, adminCronOldStatsMessage(language, at), int(r.adminCronNow().Unix()))
}

func (r AdminRepository) finishAdminCronRecalcAllyPoints(ctx context.Context, tables adminCronTables, task buildingQueueTask) error {
	updateStatement := fmt.Sprintf("UPDATE %s a SET score1 = GREATEST(0, COALESCE((SELECT SUM(u.score1) FROM %s u WHERE u.ally_id = a.ally_id), 0)), score2 = GREATEST(0, COALESCE((SELECT SUM(u.score2) FROM %s u WHERE u.ally_id = a.ally_id), 0)), score3 = GREATEST(0, COALESCE((SELECT SUM(u.score3) FROM %s u WHERE u.ally_id = a.ally_id), 0))", tables.ally, tables.users, tables.users, tables.users)
	statements := []string{
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place1 = (SELECT @pos := @pos+1) ORDER BY score1 DESC", tables.ally),
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place2 = (SELECT @pos := @pos+1) ORDER BY score2 DESC", tables.ally),
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place3 = (SELECT @pos := @pos+1) ORDER BY score3 DESC", tables.ally),
	}
	if r.dialect == DialectSQLite {
		updateStatement = fmt.Sprintf("UPDATE %s AS a SET score1 = CASE WHEN COALESCE((SELECT SUM(u.score1) FROM %s u WHERE u.ally_id = a.ally_id), 0) < 0 THEN 0 ELSE COALESCE((SELECT SUM(u.score1) FROM %s u WHERE u.ally_id = a.ally_id), 0) END, score2 = CASE WHEN COALESCE((SELECT SUM(u.score2) FROM %s u WHERE u.ally_id = a.ally_id), 0) < 0 THEN 0 ELSE COALESCE((SELECT SUM(u.score2) FROM %s u WHERE u.ally_id = a.ally_id), 0) END, score3 = CASE WHEN COALESCE((SELECT SUM(u.score3) FROM %s u WHERE u.ally_id = a.ally_id), 0) < 0 THEN 0 ELSE COALESCE((SELECT SUM(u.score3) FROM %s u WHERE u.ally_id = a.ally_id), 0) END", tables.ally, tables.users, tables.users, tables.users, tables.users, tables.users, tables.users)
		statements = []string{
			rankStatement(tables.ally, "ally_id", "score1", "place1"),
			rankStatement(tables.ally, "ally_id", "score2", "place2"),
			rankStatement(tables.ally, "ally_id", "score3", "place3"),
		}
	}
	if _, err := r.execer.ExecContext(ctx, updateStatement); err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := r.execer.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return r.removeAdminCronTask(ctx, tables.queue, task.TaskID)
}

func (r AdminRepository) loadAdminCronTasks(ctx context.Context, queueTable string, until int) ([]buildingQueueTask, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id, COALESCE(owner_id, 0), COALESCE(type, ''), COALESCE(sub_id, 0), COALESCE(obj_id, 0), COALESCE(level, 0), COALESCE(start, 0), COALESCE(end, 0), COALESCE(prio, 0), COALESCE(freeze, 0), COALESCE(frozen, 0) FROM %s WHERE end <= ? AND freeze = 0 ORDER BY end ASC, prio DESC LIMIT ?", queueTable), until, buildQueueBatch)
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
	return tasks, rows.Err()
}

func (r AdminRepository) removeAdminCronTask(ctx context.Context, queueTable string, taskID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ?", queueTable), taskID)
	return err
}

func (r AdminRepository) insertAdminCronTask(ctx context.Context, queueTable string, queueType string, start int, end int, priority int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, 0, 0, 0, ?, ?, ?)", queueTable), adminCouponQueueOwnerID, queueType, start, end, priority)
	return err
}

func (r AdminRepository) insertAdminCronDebug(ctx context.Context, debugTable string, message string, at int) error {
	message = strings.NewReplacer(`"`, "&quot;", "'", "&rsquo;", "`", "&lsquo;").Replace(message)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, ip, agent, url, text, date) VALUES (?, '0.0.0.0', 'cron', 'cron.php', ?, ?)", debugTable), adminCouponQueueOwnerID, message, at)
	return err
}

func (r AdminRepository) adminCronLocation() *time.Location {
	return legacyAdminTimeLocation
}

func nextAdminCronStatsTime(at time.Time) time.Time {
	day := at
	hour := 8
	if at.Hour() >= 8 && at.Hour() < 16 {
		hour = 16
	} else if at.Hour() >= 16 && at.Hour() < 20 {
		hour = 20
	} else {
		day = at.AddDate(0, 0, 1)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, 5, 0, 0, at.Location())
}

func nextAdminCronWeekday(at time.Time, weekday time.Weekday, hour int, minute int) time.Time {
	days := (int(weekday) - int(at.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	day := at.AddDate(0, 0, days)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, at.Location())
}

func nextAdminCronDaily(at time.Time, hour int, minute int) time.Time {
	day := at.AddDate(0, 0, 1)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, at.Location())
}

func adminCronUnknownMessage(language string) string {
	switch language {
	case "de":
		return "queue: Unbekannter Auftragstyp für globale Warteschlange: "
	case "ru":
		return "queue: Неизвестный тип задания для глобальной очереди: "
	default:
		return "queue: Unknown task type for global queue: "
	}
}

func adminCronOldStatsMessage(language string, at time.Time) string {
	stamp := at.Format("15:04")
	switch language {
	case "de":
		return "Alte Punkte beibehalten, Zeitstempel " + stamp
	case "ru":
		return "Старые очки сохранены, таймстамп " + stamp
	default:
		return "Old points saved, timestamp " + stamp
	}
}

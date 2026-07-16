package mysqlgame

import (
	"context"
	"fmt"
	"strconv"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	adminCronLegorID      = 1
	adminCronSpaceID      = 99999
	adminCronInactiveDays = 35
	adminCronDebrisType   = 10000
)

func (r AdminRepository) finishAdminCronCleanPlanets(ctx context.Context, tables adminCronTables, task buildingQueueTask, language string) error {
	planetIDs, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT planet_id FROM %s WHERE remove <= ? AND remove <> 0", tables.planets), task.End)
	if err != nil {
		return err
	}
	for _, planetID := range planetIDs {
		fleetIDs, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT fleet_id FROM %s WHERE target_planet = ? AND mission < ?", tables.fleet), planetID, domaingame.FleetMissionReturnOffset)
		if err != nil {
			return err
		}
		for _, fleetID := range fleetIDs {
			fleet := NewFleetRepositoryWithRunner(r.queryer, r.execer, r.prefix, adminCronClock(task.End, r.adminCronLocation()))
			fleet.legacyEvents = true
			fleet.dialect = r.dialect
			if err := fleet.RecallFleetAnyOwner(ctx, fleetID); err != nil {
				return err
			}
		}
		if err := r.overview.flushPlanetQueue(ctx, tables.queue, tables.buildQueue, planetID); err != nil {
			return err
		}
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE planet_id = ?", tables.planets), planetID); err != nil {
			return err
		}
	}
	if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
		return err
	}
	now := r.adminCronNow()
	localNow := now.In(r.adminCronLocation())
	if err := r.insertAdminCronTaskIfMissing(ctx, tables.queue, adminQueueTypeCleanPlanets, int(now.Unix()), int(nextAdminCronDaily(localNow, 1, 10).Unix()), adminQueuePriorityCleanPlanets); err != nil {
		return err
	}
	return r.insertAdminCronDebug(ctx, tables.debug, adminCronCleanPlanetsMessage(language, len(planetIDs)), int(now.Unix()))
}

func (r AdminRepository) finishAdminCronCleanPlayers(ctx context.Context, tables adminCronTables, task buildingQueueTask) error {
	disabled, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT player_id FROM %s WHERE disable_until <= ? AND disable_until <> 0 AND admin < 1 AND disable <> 0", tables.users), task.End)
	if err != nil {
		return err
	}
	for _, playerID := range disabled {
		if err := r.removeAdminCronUser(ctx, tables, playerID, task.End); err != nil {
			return err
		}
	}
	inactiveBefore := task.End - adminCronInactiveDays*24*60*60
	inactive, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT player_id FROM %s WHERE lastclick < ? AND admin < 1 AND lastclick <> 0 AND dm = 0", tables.users), inactiveBefore)
	if err != nil {
		return err
	}
	for _, playerID := range inactive {
		bot, err := r.isAdminCronBot(ctx, tables.queue, playerID)
		if err != nil {
			return err
		}
		if !bot {
			if err := r.removeAdminCronUser(ctx, tables, playerID, task.End); err != nil {
				return err
			}
		}
	}
	if err := r.removeAdminCronTask(ctx, tables.queue, task.TaskID); err != nil {
		return err
	}
	now := r.adminCronNow()
	localNow := now.In(r.adminCronLocation())
	return r.insertAdminCronTaskIfMissing(ctx, tables.queue, adminQueueTypeCleanPlayers, int(now.Unix()), int(nextAdminCronDaily(localNow, 1, 10).Unix()), adminQueuePriorityCleanPlayers)
}

func (r AdminRepository) removeAdminCronUser(ctx context.Context, tables adminCronTables, playerID int, at int) error {
	if playerID == adminCronLegorID || playerID == adminCronSpaceID {
		return nil
	}
	incoming, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT DISTINCT f.fleet_id FROM %s f JOIN %s p ON p.planet_id = f.target_planet JOIN %s q ON q.type = ? AND q.sub_id = f.fleet_id WHERE p.owner_id = ? AND p.type < ? AND f.owner_id <> ? AND f.mission < ?", tables.fleet, tables.planets, tables.queue), queueTypeFleet, playerID, adminCronDebrisType, playerID, domaingame.FleetMissionReturnOffset)
	if err != nil {
		return err
	}
	for _, fleetID := range incoming {
		fleet := NewFleetRepositoryWithRunner(r.queryer, r.execer, r.prefix, adminCronClock(at, r.adminCronLocation()))
		fleet.legacyEvents = true
		fleet.dialect = r.dialect
		if err := fleet.RecallFleetAnyOwner(ctx, fleetID); err != nil {
			return err
		}
	}
	unionQuery := fmt.Sprintf("DELETE FROM %s WHERE target_player = ? OR players REGEXP ?", tables.union)
	unionArgs := []any{playerID, fmt.Sprintf("(^|,)%d(,|$)", playerID)}
	if r.dialect == DialectSQLite {
		unionQuery = fmt.Sprintf("DELETE FROM %s WHERE target_player = ? OR players = ? OR players LIKE ? OR players LIKE ? OR players LIKE ?", tables.union)
		unionArgs = []any{playerID, strconv.Itoa(playerID), fmt.Sprintf("%d,%%", playerID), fmt.Sprintf("%%,%d,%%", playerID), fmt.Sprintf("%%,%d", playerID)}
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.fleet), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.queue), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.buildQueue), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? OR msg_id IN (SELECT msg_id FROM %s WHERE owner_id = ?)", tables.reports, tables.messages), []any{playerID, playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.messages), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.notes), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.browse), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.template), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.botvars), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", tables.userLogs), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? OR target_id = ?", tables.fleetLogs), []any{playerID, playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", tables.ipLogs), []any{playerID}},
		{unionQuery, unionArgs},
		{fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? AND type <> ?", tables.planets), []any{playerID, adminCronDebrisType}},
		{fmt.Sprintf("UPDATE %s SET owner_id = ? WHERE owner_id = ? AND type = ?", tables.planets), []any{adminCronSpaceID, playerID, adminCronDebrisType}},
		{fmt.Sprintf("DELETE FROM %s WHERE player_id = ?", tables.users), []any{playerID}},
		{fmt.Sprintf("UPDATE %s SET usercount = usercount - 1", tables.uni), nil},
		{fmt.Sprintf("DELETE FROM %s WHERE player_id = ?", tables.allyApps), []any{playerID}},
		{fmt.Sprintf("DELETE FROM %s WHERE request_from = ? OR request_to = ?", tables.buddy), []any{playerID, playerID}},
	}
	for _, statement := range statements {
		if _, err := r.execer.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return err
		}
	}
	return r.overview.recalcRanks(ctx, tables.users)
}

func (r AdminRepository) loadAdminCronIDs(ctx context.Context, statement string, args ...any) ([]int, error) {
	rows, err := r.queryer.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r AdminRepository) isAdminCronBot(ctx context.Context, queueTable string, playerID int) (bool, error) {
	ids, err := r.loadAdminCronIDs(ctx, fmt.Sprintf("SELECT task_id FROM %s WHERE type = ? AND owner_id = ? LIMIT 1", queueTable), queueTypeAI, playerID)
	return len(ids) > 0, err
}

func (r AdminRepository) insertAdminCronTaskIfMissing(ctx context.Context, queueTable string, queueType string, start int, end int, priority int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) SELECT ?, ?, 0, 0, 0, ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM %s WHERE type = ?)", queueTable, queueTable), adminCronSpaceID, queueType, start, end, priority, queueType)
	return err
}

func (r AdminRepository) adminCronNow() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

func adminCronClock(at int, location *time.Location) func() time.Time {
	fixed := time.Unix(int64(at), 0).In(location)
	return func() time.Time { return fixed }
}

func adminCronCleanPlanetsMessage(language string, count int) string {
	switch language {
	case "de":
		return fmt.Sprintf("Aufräumen zerstörter Planeten (%d)", count)
	case "ru":
		return fmt.Sprintf("Чистка уничтоженных планет (%d)", count)
	default:
		return fmt.Sprintf("Cleanup of destroyed planets (%d)", count)
	}
}

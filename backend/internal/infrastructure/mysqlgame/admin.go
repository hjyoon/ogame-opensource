package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type AdminRepository struct {
	queryer       Queryer
	overview      OverviewRepository
	prefix        string
	legacyGameDir string
}

func NewAdminRepository(db *sql.DB, prefix string) AdminRepository {
	runner := SQLQueryer{DB: db}
	return AdminRepository{
		queryer:       runner,
		overview:      NewOverviewRepository(db, prefix),
		prefix:        prefix,
		legacyGameDir: "game",
	}
}

func NewAdminRepositoryWithQueryer(queryer Queryer, prefix string) AdminRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return AdminRepository{
		queryer:       queryer,
		overview:      NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:        prefix,
		legacyGameDir: "game",
	}
}

func (r AdminRepository) WithLegacyGameDir(path string) AdminRepository {
	if path != "" {
		r.legacyGameDir = path
	}
	return r
}

func (r AdminRepository) GetAdmin(ctx context.Context, query appgame.AdminQuery) (domaingame.Admin, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Admin{}, err
	}
	viewer, err := r.loadAdminViewer(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Admin{}, err
	}
	admin := domaingame.NewAdmin(overview, viewer, query.Mode)
	switch admin.Mode {
	case "Debug":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "debug", true)
	case "Errors":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "errors", false)
	case "Queue":
		admin.QueueRows, err = r.loadAdminQueueRows(ctx)
	case "UserLogs":
		admin.UserLogRows, err = r.loadAdminUserLogRows(ctx)
	case "Users":
		admin.UserRows, admin.ActiveUsers, err = r.loadAdminUsers(ctx)
	case "BattleReport":
		admin.BattleReports, err = r.loadAdminBattleReports(ctx)
	case "Checksum":
		admin.ChecksumGroups, err = r.loadAdminChecksumGroups(ctx)
	}
	if err != nil {
		return domaingame.Admin{}, err
	}
	return admin, nil
}

func (r AdminRepository) loadAdminViewer(ctx context.Context, playerID int) (domaingame.AdminViewer, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.AdminViewer{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(oname, ''), COALESCE(admin, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.AdminViewer{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.AdminViewer{}, err
		}
		return domaingame.AdminViewer{}, errors.New("admin viewer not found")
	}
	var viewer domaingame.AdminViewer
	if err := rows.Scan(&viewer.PlayerID, &viewer.Name, &viewer.Level); err != nil {
		return domaingame.AdminViewer{}, err
	}
	return viewer, rows.Err()
}

func (r AdminRepository) loadAdminMessageRows(ctx context.Context, rawTable string, includeErrorIDOrder bool) ([]domaingame.AdminMessageRow, error) {
	messagesTable, err := tableName(r.prefix, rawTable)
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	order := "m.date DESC"
	if includeErrorIDOrder {
		order += ", m.error_id DESC"
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT m.error_id, COALESCE(m.owner_id, 0), COALESCE(u.oname, ''), COALESCE(m.ip, ''), COALESCE(m.agent, ''), COALESCE(m.text, ''), COALESCE(m.date, 0) FROM %s m LEFT JOIN %s u ON u.player_id = m.owner_id ORDER BY %s LIMIT 50",
			messagesTable,
			usersTable,
			order,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminMessageRow, 0, 50)
	for rows.Next() {
		var row domaingame.AdminMessageRow
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.IP, &row.Agent, &row.Text, &row.Date); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminUserLogRows(ctx context.Context) ([]domaingame.AdminUserLogRow, error) {
	userLogsTable, err := tableName(r.prefix, "userlogs")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT l.id, COALESCE(l.owner_id, 0), COALESCE(u.oname, ''), COALESCE(l.date, 0), COALESCE(l.type, ''), COALESCE(l.text, '') FROM %s l LEFT JOIN %s u ON u.player_id = l.owner_id WHERE l.owner_id > 0 ORDER BY l.date DESC LIMIT 50",
			userLogsTable,
			usersTable,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminUserLogRow, 0, 50)
	for rows.Next() {
		var row domaingame.AdminUserLogRow
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.Date, &row.Type, &row.Text); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminUsers(ctx context.Context) ([]domaingame.AdminUserRow, []domaingame.AdminUserRow, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, nil, err
	}
	newUsers, err := r.queryAdminUsers(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(p.planet_id, 0), COALESCE(p.name, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid ORDER BY u.regdate DESC LIMIT 25",
			usersTable,
			planetsTable,
		),
	)
	if err != nil {
		return nil, nil, err
	}
	activeUsers, err := r.queryAdminUsers(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(p.planet_id, 0), COALESCE(p.name, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.lastclick >= ? ORDER BY u.oname ASC",
			usersTable,
			planetsTable,
		),
		time.Now().Unix()-24*60*60,
	)
	if err != nil {
		return nil, nil, err
	}
	return newUsers, activeUsers, nil
}

func (r AdminRepository) queryAdminUsers(ctx context.Context, sql string, args ...any) ([]domaingame.AdminUserRow, error) {
	rows, err := r.queryer.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminUserRow, 0)
	for rows.Next() {
		var row domaingame.AdminUserRow
		var vacation, banned, noattack, disable int
		var planetID, galaxy, system, position int
		var planetName string
		if err := rows.Scan(
			&row.PlayerID,
			&row.Name,
			&row.RegDate,
			&row.LastClick,
			&vacation,
			&banned,
			&noattack,
			&disable,
			&planetID,
			&planetName,
			&galaxy,
			&system,
			&position,
		); err != nil {
			return nil, err
		}
		row.Vacation = vacation != 0
		row.Banned = banned != 0
		row.NoAttack = noattack != 0
		row.Disable = disable != 0
		if planetID != 0 {
			row.HomePlanet = &domaingame.AdminUserPlanet{
				ID:   planetID,
				Name: planetName,
				Coordinates: domaingame.Coordinates{
					Galaxy:   galaxy,
					System:   system,
					Position: position,
				},
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminQueueRows(ctx context.Context) ([]domaingame.AdminQueueRow, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.task_id, COALESCE(q.owner_id, 0), COALESCE(u.oname, ''), COALESCE(q.type, ''), COALESCE(q.sub_id, 0), COALESCE(q.obj_id, 0), COALESCE(q.level, 0), COALESCE(q.start, 0), COALESCE(q.end, 0), COALESCE(q.prio, 0), COALESCE(q.freeze, 0), COALESCE(q.frozen, 0) FROM %s q LEFT JOIN %s u ON u.player_id = q.owner_id WHERE q.type <> ? ORDER BY q.end ASC, q.prio DESC LIMIT 50",
			queueTable,
			usersTable,
		),
		"Fleet",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminQueueRow, 0, 50)
	for rows.Next() {
		var row domaingame.AdminQueueRow
		var subID, objID, level int
		var freeze int
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.Type, &subID, &objID, &level, &row.Start, &row.End, &row.Priority, &freeze, &row.Frozen); err != nil {
			return nil, err
		}
		row.Freeze = freeze != 0
		row.Description = legacyAdminQueueDescription(row.Type, subID, objID, level)
		result = append(result, row)
	}
	return result, rows.Err()
}

func legacyAdminQueueDescription(queueType string, subID int, objID int, level int) string {
	switch queueType {
	case "UpdateStats":
		return "Save old statistics"
	case "RecalcPoints":
		return "Recalculate statistics"
	case "RecalcAllyPoints":
		return "Recalculate alliance statistics"
	case "AllowName":
		return "Allow name change"
	case "ChangeEmail":
		return "Update permanent mail address"
	case "UnloadAll":
		return "Unload all the players"
	case "CleanDebris":
		return "Cleaning virtual debris"
	case "CleanPlanets":
		return "Cleanup of destroyed planets"
	case "CleanPlayers":
		return "Deleting inactive players and players put up for deletion"
	case "UnbanPlayer":
		return "Unban a player"
	case "AllowAttacks":
		return "Allow attacks"
	}
	return fmt.Sprintf("Unknown task type (type=%s, sub_id=%d, obj_id=%d, level=%d)", queueType, subID, objID, level)
}

func (r AdminRepository) loadAdminBattleReports(ctx context.Context) ([]domaingame.AdminBattleReportRow, error) {
	battleTable, err := tableName(r.prefix, "battledata")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT battle_id, COALESCE(source, ''), COALESCE(title, ''), COALESCE(report, ''), COALESCE(date, 0) FROM %s ORDER BY date DESC", battleTable),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminBattleReportRow, 0)
	for rows.Next() {
		var row domaingame.AdminBattleReportRow
		var source, report string
		if err := rows.Scan(&row.ID, &source, &row.Title, &report, &row.Date); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

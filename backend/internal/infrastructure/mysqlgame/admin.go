package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type AdminRepository struct {
	queryer       Queryer
	execer        Execer
	overview      OverviewRepository
	prefix        string
	legacyGameDir string
	now           func() time.Time
}

func NewAdminRepository(db *sql.DB, prefix string) AdminRepository {
	runner := SQLQueryer{DB: db}
	return AdminRepository{
		queryer:       runner,
		execer:        runner,
		overview:      NewOverviewRepository(db, prefix),
		prefix:        prefix,
		legacyGameDir: "game",
		now:           time.Now,
	}
}

func NewAdminRepositoryWithQueryer(queryer Queryer, prefix string) AdminRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return AdminRepository{
		queryer:       queryer,
		execer:        execer,
		overview:      NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:        prefix,
		legacyGameDir: "game",
		now:           time.Now,
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
	if !admin.CanAccessMode() {
		return admin, nil
	}
	switch admin.Mode {
	case "Debug":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "debug", true)
	case "Errors":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "errors", false)
	case "Queue":
		admin.QueueRows, err = r.loadAdminQueueRows(ctx)
	case "UserLogs":
		admin.UserLogRows, err = r.loadAdminUserLogRows(ctx)
	case "Users", "Bans":
		admin.UserRows, admin.ActiveUsers, err = r.loadAdminUsers(ctx)
	case "Planets":
		admin.PlanetRows, err = r.loadAdminPlanetRows(ctx)
	case "Uni":
		admin.Universe, err = r.loadAdminUniverse(ctx)
	case "Expedition":
		admin.Expedition, err = r.loadAdminExpeditionSettings(ctx)
	case "BattleReport":
		admin.BattleReports, err = r.loadAdminBattleReports(ctx)
	case "Checksum":
		admin.ChecksumGroups, err = r.loadAdminChecksumGroups(ctx)
	case "BotEdit":
		admin.BotStrategies, err = r.loadAdminBotStrategies(ctx)
	}
	if err != nil {
		return domaingame.Admin{}, err
	}
	return admin, nil
}

func (r AdminRepository) MutateAdmin(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if r.execer == nil {
		return nil, errors.New("admin mutation unavailable")
	}
	mode := domaingame.NormalizeAdminMode(query.Mode)
	if mode == "Expedition" && query.Action == "settings" {
		expeditionTable, err := tableName(r.prefix, "exptab")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminExpeditionSettings(ctx, expeditionTable, query.Values)
	}
	if mode != "Bans" || query.Action != "ban" {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	prangerTable, err := tableName(r.prefix, "pranger")
	if err != nil {
		return nil, err
	}
	return r.mutateAdminBans(ctx, usersTable, queueTable, prangerTable, query)
}

func (r AdminRepository) mutateAdminExpeditionSettings(ctx context.Context, expeditionTable string, values map[string]int) (*domaingame.AdminActionIssue, error) {
	assignments := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for _, column := range adminExpeditionColumns {
		value, ok := values[column]
		if !ok {
			continue
		}
		assignments = append(assignments, "`"+column+"` = ?")
		args = append(args, value)
	}
	if len(assignments) == 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s", expeditionTable, strings.Join(assignments, ", ")), args...)
	if err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

type adminBanUser struct {
	ID   int
	Name string
}

func (r AdminRepository) mutateAdminBans(ctx context.Context, usersTable string, queueTable string, prangerTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	targetIDs := uniquePositiveIDs(query.TargetIDs)
	if len(targetIDs) == 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	actor, _, err := r.loadAdminBanUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	now := int(r.now().Unix())
	seconds := max(0, query.Days)*24*60*60 + max(0, query.Hours)*60*60
	until := now + seconds
	reason := sanitizeAdminBanReason(query.Reason)

	for _, targetID := range targetIDs {
		target, found, err := r.loadAdminBanUser(ctx, usersTable, targetID)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		switch query.BanMode {
		case 0:
			if err := r.insertAdminBanPranger(ctx, prangerTable, actor, target, now, until, reason); err != nil {
				return nil, err
			}
			if err := r.banAdminUser(ctx, usersTable, queueTable, targetID, now, until, false); err != nil {
				return nil, err
			}
		case 1:
			if err := r.insertAdminBanPranger(ctx, prangerTable, actor, target, now, until, reason); err != nil {
				return nil, err
			}
			if err := r.banAdminUser(ctx, usersTable, queueTable, targetID, now, until, true); err != nil {
				return nil, err
			}
		case 2:
			if err := r.insertAdminBanPranger(ctx, prangerTable, actor, target, now, until, reason); err != nil {
				return nil, err
			}
			if err := r.banAdminUserAttacks(ctx, usersTable, queueTable, targetID, now, until); err != nil {
				return nil, err
			}
		case 3:
			if err := r.unbanAdminUser(ctx, usersTable, queueTable, targetID); err != nil {
				return nil, err
			}
		case 4:
			if err := r.unbanAdminUserAttacks(ctx, usersTable, queueTable, targetID); err != nil {
				return nil, err
			}
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) loadAdminBanUser(ctx context.Context, usersTable string, playerID int) (adminBanUser, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT player_id, COALESCE(oname, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return adminBanUser{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return adminBanUser{}, false, err
		}
		return adminBanUser{}, false, nil
	}
	var user adminBanUser
	if err := rows.Scan(&user.ID, &user.Name); err != nil {
		return adminBanUser{}, false, err
	}
	return user, true, rows.Err()
}

func (r AdminRepository) insertAdminBanPranger(ctx context.Context, prangerTable string, actor adminBanUser, target adminBanUser, now int, until int, reason string) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (admin_name, user_name, admin_id, user_id, ban_when, ban_until, reason) VALUES (?, ?, ?, ?, ?, ?, ?)", prangerTable),
		actor.Name,
		target.Name,
		actor.ID,
		target.ID,
		now,
		until,
		reason,
	)
	return err
}

func (r AdminRepository) banAdminUser(ctx context.Context, usersTable string, queueTable string, playerID int, now int, until int, vacation bool) error {
	if err := r.deleteAdminQueue(ctx, queueTable, playerID, "UnbanPlayer"); err != nil {
		return err
	}
	if err := r.insertAdminQueue(ctx, queueTable, playerID, "UnbanPlayer", now, now+until); err != nil {
		return err
	}
	if vacation {
		_, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET score1 = 0, score2 = 0, score3 = 0, banned = 1, banned_until = ?, vacation = 1, vacation_until = ? WHERE player_id = ?", usersTable),
			until,
			until,
			playerID,
		)
		return err
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET score1 = 0, score2 = 0, score3 = 0, banned = 1, banned_until = ? WHERE player_id = ?", usersTable),
		until,
		playerID,
	)
	return err
}

func (r AdminRepository) banAdminUserAttacks(ctx context.Context, usersTable string, queueTable string, playerID int, now int, until int) error {
	if err := r.deleteAdminQueue(ctx, queueTable, playerID, "AllowAttacks"); err != nil {
		return err
	}
	if err := r.insertAdminQueue(ctx, queueTable, playerID, "AllowAttacks", now, now+until); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET noattack = 1, noattack_until = ? WHERE player_id = ?", usersTable), until, playerID)
	return err
}

func (r AdminRepository) unbanAdminUser(ctx context.Context, usersTable string, queueTable string, playerID int) error {
	if err := r.deleteAdminQueue(ctx, queueTable, playerID, "UnbanPlayer"); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET banned = 0, banned_until = 0 WHERE player_id = ?", usersTable), playerID)
	return err
}

func (r AdminRepository) unbanAdminUserAttacks(ctx context.Context, usersTable string, queueTable string, playerID int) error {
	if err := r.deleteAdminQueue(ctx, queueTable, playerID, "AllowAttacks"); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET noattack = 0, noattack_until = 0 WHERE player_id = ?", usersTable), playerID)
	return err
}

func (r AdminRepository) deleteAdminQueue(ctx context.Context, queueTable string, playerID int, queueType string) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE type = ? AND owner_id = ?", queueTable), queueType, playerID)
	return err
}

func (r AdminRepository) insertAdminQueue(ctx context.Context, queueTable string, playerID int, queueType string, start int, end int) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, 0, 0, 0, ?, ?, 0)", queueTable),
		playerID,
		queueType,
		start,
		end,
	)
	return err
}

func uniquePositiveIDs(ids []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func sanitizeAdminBanReason(reason string) string {
	replacer := strings.NewReplacer(`\"`, "&quot;", `'`, "&rsquo;", "`", "&lsquo;")
	return replacer.Replace(reason)
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

func (r AdminRepository) loadAdminBotStrategies(ctx context.Context) ([]domaingame.AdminBotStrategy, error) {
	botstratTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id, COALESCE(name, '') FROM %s ORDER BY id ASC", botstratTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domaingame.AdminBotStrategy{}
	for rows.Next() {
		var row domaingame.AdminBotStrategy
		if err := rows.Scan(&row.ID, &row.Name); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
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

func (r AdminRepository) loadAdminPlanetRows(ctx context.Context) ([]domaingame.AdminPlanetRow, error) {
	planetsTable, err := tableName(r.prefix, "planets")
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
			"SELECT p.planet_id, COALESCE(p.name, ''), COALESCE(p.date, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), COALESCE(u.player_id, 0), COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0) FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id ORDER BY p.date DESC LIMIT 25",
			planetsTable,
			usersTable,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminPlanetRow, 0, 25)
	for rows.Next() {
		var row domaingame.AdminPlanetRow
		var owner domaingame.AdminUserRow
		var vacation, banned, noattack, disable int
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.Date,
			&row.Coordinates.Galaxy,
			&row.Coordinates.System,
			&row.Coordinates.Position,
			&owner.PlayerID,
			&owner.Name,
			&owner.RegDate,
			&owner.LastClick,
			&vacation,
			&banned,
			&noattack,
			&disable,
		); err != nil {
			return nil, err
		}
		if owner.PlayerID != 0 {
			owner.Vacation = vacation != 0
			owner.Banned = banned != 0
			owner.NoAttack = noattack != 0
			owner.Disable = disable != 0
			row.Owner = &owner
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminUniverse(ctx context.Context) (*domaingame.AdminUniverseSettings, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT COALESCE(num, 0), COALESCE(speed, 0), COALESCE(fspeed, 0), COALESCE(galaxies, 0), COALESCE(systems, 0), COALESCE(maxusers, 0), COALESCE(acs, 0), COALESCE(fid, 0), COALESCE(did, 0), COALESCE(rapid, 0), COALESCE(moons, 0), COALESCE(defrepair, 0), COALESCE(defrepair_delta, 0), COALESCE(usercount, 0), COALESCE(freeze, 0), COALESCE(news1, ''), COALESCE(news2, ''), COALESCE(news_until, 0), COALESCE(startdate, 0), COALESCE(battle_engine, ''), COALESCE(lang, ''), COALESCE(hacks, 0), COALESCE(ext_board, ''), COALESCE(ext_discord, ''), COALESCE(ext_tutorial, ''), COALESCE(ext_rules, ''), COALESCE(ext_impressum, ''), COALESCE(php_battle, 0), COALESCE(battle_max, 0), COALESCE(force_lang, 0), COALESCE(start_dm, 0), COALESCE(max_werf, 0), COALESCE(feedage, 0) FROM %s LIMIT 1",
			uniTable,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("admin universe settings not found")
	}
	var universe domaingame.AdminUniverseSettings
	var rapid, moons, freeze, phpBattle, forceLang int
	if err := rows.Scan(
		&universe.Number,
		&universe.Speed,
		&universe.FleetSpeed,
		&universe.Galaxies,
		&universe.Systems,
		&universe.MaxUsers,
		&universe.ACS,
		&universe.FleetDebris,
		&universe.DefenseDebris,
		&rapid,
		&moons,
		&universe.DefenseRepair,
		&universe.DefenseDelta,
		&universe.UserCount,
		&freeze,
		&universe.News1,
		&universe.News2,
		&universe.NewsUntil,
		&universe.StartDate,
		&universe.BattleEngine,
		&universe.Language,
		&universe.Hacks,
		&universe.ExtBoard,
		&universe.ExtDiscord,
		&universe.ExtTutorial,
		&universe.ExtRules,
		&universe.ExtImpressum,
		&phpBattle,
		&universe.BattleMax,
		&forceLang,
		&universe.StartDarkMatter,
		&universe.MaxShipyard,
		&universe.FeedAge,
	); err != nil {
		return nil, err
	}
	universe.RapidFire = rapid != 0
	universe.Moons = moons != 0
	universe.Freeze = freeze != 0
	universe.PHPBattle = phpBattle != 0
	universe.ForceLanguage = forceLang != 0
	return &universe, rows.Err()
}

var adminExpeditionColumns = []string{
	"dm_factor",
	"chance_success",
	"depleted_min",
	"depleted_med",
	"depleted_max",
	"chance_depleted_min",
	"chance_depleted_med",
	"chance_depleted_max",
	"chance_alien",
	"chance_pirates",
	"chance_dm",
	"chance_lost",
	"chance_delay",
	"chance_accel",
	"chance_res",
	"chance_fleet",
	"score_cap1",
	"limit_cap1",
	"score_cap2",
	"limit_cap2",
	"score_cap3",
	"limit_cap3",
	"score_cap4",
	"limit_cap4",
	"score_cap5",
	"limit_cap5",
	"score_cap6",
	"limit_cap6",
	"score_cap7",
	"limit_cap7",
	"score_cap8",
	"limit_cap8",
	"limit_max",
}

func numericColumnsByName(names []string) string {
	columns := make([]string, 0, len(names))
	for _, name := range names {
		columns = append(columns, "`"+name+"`")
	}
	return strings.Join(columns, ", ")
}

func (r AdminRepository) loadAdminExpeditionSettings(ctx context.Context) (map[string]int, error) {
	expeditionTable, err := tableName(r.prefix, "exptab")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s LIMIT 1", numericColumnsByName(adminExpeditionColumns), expeditionTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("admin expedition settings not found")
	}
	values := make([]int, len(adminExpeditionColumns))
	dest := make([]any, len(values))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	result := make(map[string]int, len(adminExpeditionColumns))
	for index, column := range adminExpeditionColumns {
		result[column] = values[index]
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

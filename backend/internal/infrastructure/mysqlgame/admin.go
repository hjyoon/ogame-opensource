package mysqlgame

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"math/big"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type AdminRepository struct {
	queryer       Queryer
	execer        Execer
	masterQueryer Queryer
	masterExecer  Execer
	overview      OverviewRepository
	prefix        string
	legacyGameDir string
	uniNumber     int
	now           func() time.Time
	couponCode    func() (string, error)
}

func NewAdminRepository(db *sql.DB, prefix string) AdminRepository {
	runner := SQLQueryer{DB: db}
	return AdminRepository{
		queryer:       runner,
		execer:        runner,
		overview:      NewOverviewRepository(db, prefix),
		prefix:        prefix,
		legacyGameDir: "game",
		uniNumber:     1,
		now:           time.Now,
		couponCode:    randomCouponCode,
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
		uniNumber:     1,
		now:           time.Now,
		couponCode:    randomCouponCode,
	}
}

func (r AdminRepository) WithMasterRunner(queryer Queryer, execer Execer) AdminRepository {
	r.masterQueryer = queryer
	r.masterExecer = execer
	return r
}

func (r AdminRepository) WithUniverseNumber(number int) AdminRepository {
	if number > 0 {
		r.uniNumber = number
	}
	return r
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
	case "Fleetlogs":
		admin.FleetLogRows, err = r.loadAdminFleetLogRows(ctx)
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
	case "Reports":
		admin.ReportRows, err = r.loadAdminReportRows(ctx)
	case "Uni":
		admin.Universe, err = r.loadAdminUniverse(ctx)
	case "Expedition":
		admin.Expedition, err = r.loadAdminExpeditionSettings(ctx)
	case "BattleReport":
		admin.BattleReports, err = r.loadAdminBattleReports(ctx)
	case "Checksum":
		admin.ChecksumGroups, err = r.loadAdminChecksumGroups(ctx)
	case "DB":
		admin.DatabaseBackups, err = r.loadAdminDatabaseBackups(ctx)
	case "BotEdit":
		admin.BotStrategies, err = r.loadAdminBotStrategies(ctx)
	case "Coupons":
		admin.CouponRows, err = r.loadAdminCouponRows(ctx)
		if err == nil {
			admin.CouponQueueRows, err = r.loadAdminCouponQueueRows(ctx)
		}
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
	if mode == "Expedition" && query.Action == domaingame.AdminActionSettings {
		expeditionTable, err := tableName(r.prefix, "exptab")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminExpeditionSettings(ctx, expeditionTable, query.Values)
	}
	if mode == "Uni" && query.Action == domaingame.AdminActionSettings {
		uniTable, err := tableName(r.prefix, "uni")
		if err != nil {
			return nil, err
		}
		usersTable, err := tableName(r.prefix, "users")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminUniverseSettings(ctx, uniTable, usersTable, query.Values)
	}
	if mode == "Broadcast" && query.Action == domaingame.AdminActionBroadcastSend {
		return r.mutateAdminBroadcast(ctx, query)
	}
	if mode == "Reports" && query.Action == domaingame.AdminActionReportsDelete {
		return r.mutateAdminReports(ctx, query)
	}
	if mode == "BattleSim" {
		return r.mutateAdminBattleSim(ctx, query)
	}
	if mode == "RakSim" {
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Missile attack simulator completed. Defense result rendered."), nil
	}
	if mode == "Expedition" && query.Action == domaingame.AdminActionExpeditionSim {
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Expedition simulation result myChart generated."), nil
	}
	if mode == "Queue" {
		queueTable, err := tableName(r.prefix, "queue")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminQueue(ctx, queueTable, query)
	}
	if mode == "Fleetlogs" {
		queueTable, err := tableName(r.prefix, "queue")
		if err != nil {
			return nil, err
		}
		fleetTable, err := tableName(r.prefix, "fleet")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminFleetlogs(ctx, queueTable, fleetTable, query)
	}
	if mode == "DB" {
		return r.mutateAdminDatabase(ctx, query)
	}
	if mode == "Coupons" {
		return r.mutateAdminCoupons(ctx, query)
	}
	if mode == "Users" {
		usersTable, err := tableName(r.prefix, "users")
		if err != nil {
			return nil, err
		}
		planetsTable, err := tableName(r.prefix, "planets")
		if err != nil {
			return nil, err
		}
		fleetTable, err := tableName(r.prefix, "fleet")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminUsers(ctx, usersTable, planetsTable, fleetTable, query)
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

func (r AdminRepository) mutateAdminBattleSim(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return nil, err
	}
	reportText := `<table class="battleReport"><tr><th>Battle report</th></tr><tr><td>Simulator result</td></tr></table>`
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 1, ?, ?)", messagesTable),
		query.PlayerID,
		domaingame.MessageTypeBattleReportText,
		"Battle simulator",
		"Battle report",
		reportText,
		r.now().Unix(),
		query.PlanetID,
	); err != nil {
		return nil, err
	}
	return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Battle report simulator completed."), nil
}

func (r AdminRepository) mutateAdminUniverseSettings(ctx context.Context, uniTable string, usersTable string, values map[string]int) (*domaingame.AdminActionIssue, error) {
	freeze := 0
	if values["freeze"] != 0 {
		freeze = 1
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET freeze = ?", uniTable), freeze); err != nil {
		return nil, err
	}
	if freeze != 0 {
		now := int(r.now().Unix())
		activeSince := now - 7*24*60*60
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET vacation = 1, vacation_until = ? WHERE lastclick >= ? AND admin = 0", usersTable), now, activeSince); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) mutateAdminBroadcast(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	subject := strings.TrimSpace(query.Subject)
	text := strings.TrimSpace(query.Text)
	if subject == "" || text == "" {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return nil, err
	}
	actor, err := r.loadAdminBroadcastActor(ctx, usersTable, planetsTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	recipients, err := r.loadAdminBroadcastRecipients(ctx, usersTable, query.Category)
	if err != nil {
		return nil, err
	}
	from := fmt.Sprintf(
		"%s <a href=\"index.php?page=galaxy&galaxy=%d&system=%d&position=%d&session={PUBLIC_SESSION}\">[%d:%d:%d]</a>\n",
		actor.Name,
		actor.Galaxy,
		actor.System,
		actor.Position,
		actor.Galaxy,
		actor.System,
		actor.Position,
	)
	messageSubject := fmt.Sprintf(
		"%s <a href=\"index.php?page=writemessages&session={PUBLIC_SESSION}&messageziel=%d&re=1&betreff=Re:%s\">\n</a>\n",
		subject,
		query.PlayerID,
		subject,
	)
	messageText := sanitizeAdminBroadcastText(text)
	now := int(r.now().Unix())
	for _, recipientID := range recipients {
		if err := r.insertAdminBroadcastMessage(ctx, messagesTable, recipientID, from, messageSubject, messageText, now); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) mutateAdminReports(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	reportsTable, err := tableName(r.prefix, "reports")
	if err != nil {
		return nil, err
	}
	if query.DeleteMode == "deleteall" {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s ORDER BY date DESC LIMIT 50", reportsTable)); err != nil {
			return nil, err
		}
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	for _, reportID := range uniquePositiveIDs(query.ReportIDs) {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", reportsTable), reportID); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

type adminBroadcastActor struct {
	Name     string
	Galaxy   int
	System   int
	Position int
}

func (r AdminRepository) loadAdminBroadcastActor(ctx context.Context, usersTable string, planetsTable string, playerID int) (adminBroadcastActor, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(u.oname, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.player_id = ? LIMIT 1", usersTable, planetsTable),
		playerID,
	)
	if err != nil {
		return adminBroadcastActor{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return adminBroadcastActor{}, err
		}
		return adminBroadcastActor{}, errors.New("admin broadcast actor not found")
	}
	var actor adminBroadcastActor
	if err := rows.Scan(&actor.Name, &actor.Galaxy, &actor.System, &actor.Position); err != nil {
		return adminBroadcastActor{}, err
	}
	return actor, rows.Err()
}

func (r AdminRepository) loadAdminBroadcastRecipients(ctx context.Context, usersTable string, category int) ([]int, error) {
	query := fmt.Sprintf("SELECT player_id FROM %s", usersTable)
	args := []any{}
	switch category {
	case 1:
		query += " WHERE score1 < ?"
		args = append(args, domaingame.GalaxyNoobScoreLimit)
	case 2:
		query += " WHERE place1 < ?"
		args = append(args, 100)
	case 3:
		query += " WHERE admin = ?"
		args = append(args, domaingame.AdminLevelOperator)
	}
	rows, err := r.queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recipients := []int{}
	for rows.Next() {
		var recipientID int
		if err := rows.Scan(&recipientID); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipientID)
	}
	return recipients, rows.Err()
}

func (r AdminRepository) insertAdminBroadcastMessage(ctx context.Context, messagesTable string, ownerID int, from string, subject string, text string, now int) error {
	count, err := r.countAdminMessages(ctx, messagesTable, ownerID)
	if err != nil {
		return err
	}
	if count >= 127 {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? ORDER BY date ASC, msg_id ASC LIMIT 1", messagesTable), ownerID); err != nil {
			return err
		}
	}
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypeMisc,
		from,
		subject,
		text,
		now,
	)
	return err
}

func (r AdminRepository) countAdminMessages(ctx context.Context, messagesTable string, ownerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ?", messagesTable), ownerID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func sanitizeAdminBroadcastText(text string) string {
	replacer := strings.NewReplacer(`\"`, "&quot;", `'`, "&rsquo;", "`", "&lsquo;")
	return replacer.Replace(text)
}

func (r AdminRepository) mutateAdminQueue(ctx context.Context, queueTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if query.TaskID <= 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	now := int(r.now().Unix())
	var err error
	switch query.Action {
	case domaingame.AdminActionQueueEnd:
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET end = ? WHERE task_id = ?", queueTable), now, query.TaskID)
	case domaingame.AdminActionQueueRemove:
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ?", queueTable), query.TaskID)
	case domaingame.AdminActionQueueFreeze:
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET freeze = 1, frozen = ? WHERE task_id = ? AND freeze = 0", queueTable), now, query.TaskID)
	case domaingame.AdminActionQueueUnfreeze:
		err = r.unfreezeAdminQueue(ctx, queueTable, query.TaskID, now)
	}
	if err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) mutateAdminFleetlogs(ctx context.Context, queueTable string, fleetTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if query.TaskID <= 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	now := int(r.now().Unix())
	switch query.Action {
	case domaingame.AdminActionFleetlogsTwoMinutes:
		if err := r.updateAdminFleetlogTaskEnd(ctx, queueTable, fleetTable, query.TaskID, now+2*60); err != nil {
			return nil, err
		}
	case domaingame.AdminActionFleetlogsEnd:
		if err := r.updateAdminFleetlogTaskEnd(ctx, queueTable, fleetTable, query.TaskID, now); err != nil {
			return nil, err
		}
	case domaingame.AdminActionFleetlogsReturn:
		fleetID, found, err := r.loadAdminFleetlogFleetID(ctx, queueTable, query.TaskID)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		fleetRepository := NewFleetRepositoryWithRunner(r.queryer, r.execer, r.prefix, r.now)
		if err := fleetRepository.RecallFleetAnyOwner(ctx, fleetID); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) updateAdminFleetlogTaskEnd(ctx context.Context, queueTable string, fleetTable string, taskID int, end int) error {
	unionID, found, err := r.loadAdminFleetlogUnionID(ctx, queueTable, fleetTable, taskID)
	if err != nil || !found {
		return err
	}
	if unionID > 0 {
		_, err = r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s q JOIN %s f ON f.fleet_id = q.sub_id SET q.end = ? WHERE q.type = ? AND f.union_id = ?", queueTable, fleetTable),
			end,
			queueTypeFleet,
			unionID,
		)
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET end = ? WHERE task_id = ? AND type = ?", queueTable), end, taskID, queueTypeFleet)
	return err
}

func (r AdminRepository) loadAdminFleetlogUnionID(ctx context.Context, queueTable string, fleetTable string, taskID int) (int, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(f.union_id, 0) FROM %s q JOIN %s f ON f.fleet_id = q.sub_id WHERE q.task_id = ? AND q.type = ? LIMIT 1", queueTable, fleetTable),
		taskID,
		queueTypeFleet,
	)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	var unionID int
	if err := rows.Scan(&unionID); err != nil {
		return 0, false, err
	}
	return unionID, true, rows.Err()
}

func (r AdminRepository) loadAdminFleetlogFleetID(ctx context.Context, queueTable string, taskID int) (int, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT sub_id FROM %s WHERE task_id = ? AND type = ? LIMIT 1", queueTable),
		taskID,
		queueTypeFleet,
	)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	var fleetID int
	if err := rows.Scan(&fleetID); err != nil {
		return 0, false, err
	}
	return fleetID, true, rows.Err()
}

func (r AdminRepository) unfreezeAdminQueue(ctx context.Context, queueTable string, taskID int, now int) error {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT freeze, frozen, end FROM %s WHERE task_id = ? LIMIT 1", queueTable), taskID)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return rows.Err()
	}
	var freeze, frozen, end int
	if err := rows.Scan(&freeze, &frozen, &end); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if freeze == 0 {
		return nil
	}
	frozenSeconds := now - frozen
	if frozenSeconds > 0 {
		end += frozenSeconds
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET freeze = 0, frozen = 0, end = ? WHERE task_id = ?", queueTable), end, taskID)
	return err
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

func (r AdminRepository) loadAdminFleetLogRows(ctx context.Context) ([]domaingame.AdminFleetLogRow, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.task_id, q.start, q.end, f.mission, f.flight_time, f.fuel, COALESCE(f.union_id, 0), f.start_planet, f.target_planet, COALESCE(o.name, ''), COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(o.type, 0), COALESCE(o.owner_id, 0), COALESCE(ou.oname, 'space'), COALESCE(t.name, ''), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, 0), COALESCE(t.owner_id, 0), COALESCE(tu.oname, 'space'), f.`%d`, f.`%d`, f.`%d`, %s FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s ou ON ou.player_id = o.owner_id LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s tu ON tu.player_id = t.owner_id WHERE q.type = ? ORDER BY q.end ASC, q.prio DESC LIMIT 50",
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
			prefixedNumericColumns("f", fleetIDs),
			queueTable,
			fleetTable,
			planetsTable,
			usersTable,
			planetsTable,
			usersTable,
		),
		queueTypeFleet,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminFleetLogRow, 0, 50)
	for rows.Next() {
		row, err := scanAdminFleetLogRow(rows, fleetIDs)
		if err != nil {
			return nil, err
		}
		row.Number = len(result) + 1
		result = append(result, row)
	}
	return result, rows.Err()
}

func scanAdminFleetLogRow(rows Rows, fleetIDs []int) (domaingame.AdminFleetLogRow, error) {
	var row domaingame.AdminFleetLogRow
	var cargoMetal, cargoCrystal, cargoDeuterium int
	shipValues := make([]int, len(fleetIDs))
	dest := []any{
		&row.TaskID,
		&row.Start,
		&row.End,
		&row.Mission,
		&row.FlightTime,
		&row.Fuel,
		&row.UnionID,
		&row.Origin.ID,
		&row.Target.ID,
		&row.Origin.Name,
		&row.Origin.Coordinates.Galaxy,
		&row.Origin.Coordinates.System,
		&row.Origin.Coordinates.Position,
		&row.Origin.Type,
		&row.Origin.OwnerID,
		&row.Origin.OwnerName,
		&row.Target.Name,
		&row.Target.Coordinates.Galaxy,
		&row.Target.Coordinates.System,
		&row.Target.Coordinates.Position,
		&row.Target.Type,
		&row.Target.OwnerID,
		&row.Target.OwnerName,
		&cargoMetal,
		&cargoCrystal,
		&cargoDeuterium,
	}
	dest = appendIntDest(dest, shipValues)
	if err := rows.Scan(dest...); err != nil {
		return domaingame.AdminFleetLogRow{}, err
	}
	row.Cargo = adminFleetLogCargoRows(cargoMetal, cargoCrystal, cargoDeuterium)
	row.Ships = adminFleetLogShipRows(fleetIDs, shipValues)
	return row, nil
}

func adminFleetLogCargoRows(metal int, crystal int, deuterium int) []domaingame.FleetResourceLoad {
	values := []domaingame.FleetResourceLoad{
		{ID: domaingame.ResourceMetal, Name: "Metal", Loaded: metal},
		{ID: domaingame.ResourceCrystal, Name: "Crystal", Loaded: crystal},
		{ID: domaingame.ResourceDeuterium, Name: "Deuterium", Loaded: deuterium},
	}
	rows := make([]domaingame.FleetResourceLoad, 0, len(values))
	for _, value := range values {
		if value.Loaded > 0 {
			rows = append(rows, value)
		}
	}
	return rows
}

func adminFleetLogShipRows(ids []int, values []int) []domaingame.FleetShipCount {
	ships := make([]domaingame.FleetShipCount, 0, len(ids))
	for index, id := range ids {
		count := values[index]
		if count > 0 {
			ships = append(ships, domaingame.FleetShipCount{ID: id, Name: domaingame.TechnologyName(id), Count: count})
		}
	}
	return ships
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

const (
	adminCouponQueueType     = "Coupon"
	adminCouponQueueOwnerID  = 99999
	adminCouponQueuePriority = 520
)

func (r AdminRepository) loadAdminCouponRows(ctx context.Context) ([]domaingame.AdminCouponRow, error) {
	if r.masterQueryer == nil {
		return []domaingame.AdminCouponRow{}, nil
	}
	rows, err := r.masterQueryer.QueryContext(ctx, "SELECT id, COALESCE(code, ''), COALESCE(amount, 0), COALESCE(used, 0), COALESCE(user_uni, 0), COALESCE(user_id, 0), COALESCE(user_name, '') FROM coupons ORDER BY id DESC LIMIT 15")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminCouponRow, 0, 15)
	for rows.Next() {
		var row domaingame.AdminCouponRow
		var used int
		if err := rows.Scan(&row.ID, &row.Code, &row.Amount, &used, &row.UserUniverse, &row.UserID, &row.UserName); err != nil {
			return nil, err
		}
		row.Used = used != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminCouponQueueRows(ctx context.Context) ([]domaingame.AdminCouponQueueRow, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, COALESCE(sub_id, 0), COALESCE(obj_id, 0), COALESCE(level, 0), COALESCE(start, 0), COALESCE(end, 0), COALESCE(prio, 0) FROM %s WHERE type = ? ORDER BY end ASC, task_id ASC", queueTable),
		adminCouponQueueType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminCouponQueueRow, 0)
	for rows.Next() {
		var row domaingame.AdminCouponQueueRow
		var packed int
		if err := rows.Scan(&row.ID, &row.Amount, &packed, &row.PeriodicDays, &row.Start, &row.End, &row.Priority); err != nil {
			return nil, err
		}
		row.InactiveDays = (packed >> 16) & 0xffff
		row.IngameDays = packed & 0xffff
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) mutateAdminCoupons(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	switch query.Action {
	case domaingame.AdminActionCouponAddOne:
		code, err := r.insertAdminCoupon(ctx, query.Amount)
		if err != nil {
			return nil, err
		}
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Coupon added: "+code), nil
	case domaingame.AdminActionCouponRemoveOne:
		return r.deleteAdminCoupon(ctx, query.ItemID)
	case domaingame.AdminActionCouponAddDate:
		return r.insertAdminCouponQueue(ctx, query)
	case domaingame.AdminActionCouponRemoveDate:
		return r.deleteAdminCouponQueue(ctx, query.ItemID)
	default:
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
}

func (r AdminRepository) insertAdminCoupon(ctx context.Context, amount int) (string, error) {
	if r.masterQueryer == nil || r.masterExecer == nil {
		return "", errors.New("admin coupons master DB unavailable")
	}
	if amount < 0 {
		amount = 0
	}
	generator := r.couponCode
	if generator == nil {
		generator = randomCouponCode
	}
	for attempts := 0; attempts < 10; attempts++ {
		code, err := generator()
		if err != nil {
			return "", err
		}
		exists, err := r.adminCouponCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}
		if _, err := r.masterExecer.ExecContext(ctx, "INSERT INTO coupons (code, amount, used, user_uni, user_id, user_name) VALUES (?, ?, 0, 0, 0, '')", code, amount); err != nil {
			return "", err
		}
		return code, nil
	}
	return "", errors.New("admin coupon code generation exhausted")
}

func (r AdminRepository) adminCouponCodeExists(ctx context.Context, code string) (bool, error) {
	rows, err := r.masterQueryer.QueryContext(ctx, "SELECT id FROM coupons WHERE code = ? LIMIT 1", code)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		return true, rows.Err()
	}
	return false, rows.Err()
}

func (r AdminRepository) deleteAdminCoupon(ctx context.Context, itemID int) (*domaingame.AdminActionIssue, error) {
	if r.masterExecer == nil {
		return nil, errors.New("admin coupons master DB unavailable")
	}
	if itemID <= 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	if _, err := r.masterExecer.ExecContext(ctx, "DELETE FROM coupons WHERE id = ?", itemID); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) insertAdminCouponQueue(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	end := parseAdminCouponQueueEnd(query.DayMonth, query.HourMinute, r.now())
	packedCriteria := ((query.InactiveDays & 0xffff) << 16) | (query.IngameDays & 0xffff)
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		adminCouponQueueOwnerID,
		adminCouponQueueType,
		maxInt(query.Amount, 0),
		packedCriteria,
		maxInt(query.PeriodicDays, 0),
		int(r.now().Unix()),
		end,
		adminCouponQueuePriority,
	)
	if err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) deleteAdminCouponQueue(ctx context.Context, itemID int) (*domaingame.AdminActionIssue, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	if itemID <= 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ? AND type = ?", queueTable), itemID, adminCouponQueueType); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func parseAdminCouponQueueEnd(dayMonth string, hourMinute string, now time.Time) int {
	day := 1
	month := int(now.Month())
	hour := 0
	minute := 0
	_, _ = fmt.Sscanf(dayMonth, "%d.%d", &day, &month)
	_, _ = fmt.Sscanf(hourMinute, "%d:%d", &hour, &minute)
	if day < 1 {
		day = 1
	}
	if month < 1 || month > 12 {
		month = int(now.Month())
	}
	if hour < 0 || hour > 23 {
		hour = 0
	}
	if minute < 0 || minute > 59 {
		minute = 0
	}
	return int(time.Date(now.Year(), time.Month(month), day, hour, minute, 0, 0, now.Location()).Unix())
}

func randomCouponCode() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, 20)
	for index := range bytes {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		bytes[index] = alphabet[value.Int64()]
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", bytes[0:4], bytes[4:8], bytes[8:12], bytes[12:16], bytes[16:20]), nil
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
			"SELECT l.id, COALESCE(l.owner_id, 0), COALESCE(u.oname, ''), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(l.date, 0), COALESCE(l.type, ''), COALESCE(l.text, '') FROM %s l LEFT JOIN %s u ON u.player_id = l.owner_id WHERE l.owner_id > 0 ORDER BY l.date DESC, l.id DESC LIMIT 50",
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
		var vacation, banned, noattack, disable int
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.LastClick, &vacation, &banned, &noattack, &disable, &row.Date, &row.Type, &row.Text); err != nil {
			return nil, err
		}
		row.Vacation = vacation != 0
		row.Banned = banned != 0
		row.NoAttack = noattack != 0
		row.Disable = disable != 0
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
			"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(p.planet_id, 0), COALESCE(p.name, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid ORDER BY u.regdate DESC, u.player_id DESC LIMIT 25",
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
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.task_id, COALESCE(q.owner_id, 0), COALESCE(u.oname, ''), COALESCE(q.type, ''), COALESCE(q.sub_id, 0), COALESCE(q.obj_id, 0), COALESCE(q.level, 0), COALESCE(q.start, 0), COALESCE(q.end, 0), COALESCE(q.prio, 0), COALESCE(q.freeze, 0), COALESCE(q.frozen, 0), COALESCE(p.name, '') FROM %s q LEFT JOIN %s u ON u.player_id = q.owner_id LEFT JOIN %s bq ON bq.id = q.sub_id LEFT JOIN %s p ON p.planet_id = CASE WHEN q.type IN (?, ?) THEN bq.planet_id WHEN q.type IN (?, ?) THEN q.sub_id ELSE NULL END WHERE q.type <> ? ORDER BY q.end ASC, q.prio DESC, q.task_id ASC LIMIT 50",
			queueTable,
			usersTable,
			buildQueueTable,
			planetsTable,
		),
		queueTypeBuild,
		queueTypeDemolish,
		queueTypeShipyard,
		queueTypeResearch,
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
		var planetName string
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.Type, &subID, &objID, &level, &row.Start, &row.End, &row.Priority, &freeze, &row.Frozen, &planetName); err != nil {
			return nil, err
		}
		row.Freeze = freeze != 0
		row.Description = legacyAdminQueueDescription(row.Type, subID, objID, level, planetName)
		result = append(result, row)
	}
	return result, rows.Err()
}

func legacyAdminQueueDescription(queueType string, subID int, objID int, level int, planetName string) string {
	technologyName := domaingame.TechnologyName(objID)
	planetLink := legacyAdminQueuePlanetLinkHTML(planetName)
	switch queueType {
	case queueTypeBuild:
		return fmt.Sprintf("Building '%s' (%d) on planet %s", technologyName, level, planetLink)
	case queueTypeDemolish:
		return fmt.Sprintf("Demolition of '%s' (%d) on planet %s", technologyName, level, planetLink)
	case queueTypeShipyard:
		return fmt.Sprintf("Shipyard assignment: '%s' (%d) on planet %s", technologyName, level, planetLink)
	case queueTypeResearch:
		return fmt.Sprintf("Research is underway '%s' (%d) from planet %s", technologyName, level, planetLink)
	}
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

func legacyAdminQueuePlanetLinkHTML(planetName string) string {
	if planetName == "" {
		return ""
	}
	return "<a>" + html.EscapeString(planetName) + "</a>"
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

func (r AdminRepository) loadAdminReportRows(ctx context.Context) ([]domaingame.AdminReportRow, error) {
	reportsTable, err := tableName(r.prefix, "reports")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT r.id, COALESCE(r.owner_id, 0), COALESCE(u.oname, ''), COALESCE(r.msg_id, 0), COALESCE(r.msgfrom, ''), COALESCE(r.subj, ''), COALESCE(r.text, ''), COALESCE(r.date, 0) FROM %s r LEFT JOIN %s u ON u.player_id = r.owner_id ORDER BY r.date DESC LIMIT 50", reportsTable, usersTable),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminReportRow, 0, 50)
	for rows.Next() {
		var row domaingame.AdminReportRow
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.MessageID, &row.From, &row.Subject, &row.Text, &row.Date); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

package mysqlgame

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	secret        string
	now           func() time.Time
	couponCode    func() (string, error)
	botPassword   func() (string, error)
	randomIntN    func(int) int
	randomRead    func([]byte) (int, error)
	dialect       SQLDialect
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
		botPassword:   randomBotPassword,
		randomIntN:    randomAdminIntN,
		randomRead:    rand.Read,
		dialect:       detectSQLDialect(db),
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
		botPassword:   randomBotPassword,
		randomIntN:    randomAdminIntN,
		randomRead:    rand.Read,
		dialect:       detectSQLDialectFromQueryer(queryer),
	}
}

func (r AdminRepository) WithDialect(dialect SQLDialect) AdminRepository {
	r.dialect = normalizeDialect(dialect)
	r.overview.dialect = r.dialect
	return r
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

func (r AdminRepository) WithSecret(secret string) AdminRepository {
	r.secret = secret
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
	case "Browse":
		admin.BrowseRows, err = r.loadAdminBrowseRows(ctx)
	case "Debug":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "debug", true, query.Filter)
	case "Errors":
		admin.MessageRows, err = r.loadAdminMessageRows(ctx, "errors", false, "")
	case "Logins":
		admin.LoginRows, err = r.loadAdminLoginRows(ctx, query.LoginName, query.LoginUserID, query.LoginUserIDSet, query.LoginIP)
	case "Bots":
		admin.BotRows, err = r.loadAdminBotRows(ctx)
	case "Queue":
		admin.QueueRows, err = r.loadAdminQueueRows(ctx)
	case "UserLogs":
		if query.UserLogSearch == nil {
			admin.UserLogRows, err = r.loadAdminUserLogRows(ctx)
		} else {
			admin.UserLogSearched = true
			admin.UserLogType = query.UserLogSearch.Type
			admin.UserLogGroups, err = r.loadAdminUserLogGroups(ctx, *query.UserLogSearch)
		}
	case "Users", "Bans":
		if admin.Mode == "Users" && query.TargetPlayerID > 0 {
			admin.SelectedUser, err = r.loadAdminUserDetail(ctx, query.TargetPlayerID)
		} else {
			admin.UserRows, admin.ActiveUsers, err = r.loadAdminUsers(ctx)
		}
	case "Planets":
		if query.TargetPlanetID > 0 {
			admin.SelectedPlanet, err = r.loadAdminPlanetDetail(ctx, query.TargetPlanetID)
		} else {
			admin.PlanetRows, err = r.loadAdminPlanetRows(ctx)
			if err == nil && query.PlanetSearch != nil {
				admin.PlanetSearchAttempted = true
				admin.PlanetSearchBlank = strings.TrimSpace(query.PlanetSearch.Text) == ""
				if !admin.PlanetSearchBlank {
					admin.PlanetSearchRows, err = r.loadAdminPlanetSearchRows(ctx, *query.PlanetSearch)
				}
			}
		}
	case "Reports":
		admin.ReportRows, err = r.loadAdminReportRows(ctx)
	case "Uni":
		admin.Universe, err = r.loadAdminUniverse(ctx)
	case "Expedition":
		admin.Expedition, err = r.loadAdminExpeditionSettings(ctx)
	case "BattleSim":
		admin.Universe, err = r.loadAdminUniverse(ctx)
	case "BattleReport":
		admin.BattleReports, err = r.loadAdminBattleReports(ctx)
	case "Checksum":
		admin.ChecksumGroups, err = r.loadAdminChecksumGroups(ctx)
	case "DB":
		admin.DatabaseBackups, err = r.loadAdminDatabaseBackups(ctx)
	case "BotEdit":
		admin.BotStrategies, err = r.loadAdminBotStrategies(ctx)
	case "Mods":
		admin.ModRows, err = r.loadAdminMods(ctx)
	case "Loca":
		admin.Localization, err = r.loadAdminLocalization(query.LocaSource, query.LocaTarget)
	case "ColonySettings":
		admin.ColonySettings, err = r.loadAdminColonySettings(ctx)
	case "Coupons":
		admin.CouponRows, admin.CouponTotal, err = r.loadAdminCouponRows(ctx, query.CouponFrom)
		admin.CouponFrom = normalizeAdminCouponFrom(query.CouponFrom)
		admin.CouponPageSize = adminCouponPageSize
		if err == nil {
			admin.CouponQueueRows, err = r.loadAdminCouponQueueRows(ctx)
		}
	}
	if err != nil {
		return domaingame.Admin{}, err
	}
	return admin, nil
}

type adminModManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Website     string `json:"website"`
}

func (r AdminRepository) loadAdminMods(ctx context.Context) ([]domaingame.AdminModInfo, error) {
	modsDir := filepath.Join(r.legacyGameDir, "mods")
	entries, err := os.ReadDir(modsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	mods := make([]domaingame.AdminModInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		modDir := filepath.Join(modsDir, entry.Name())
		manifestPath := filepath.Join(modDir, "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var manifest adminModManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		runtimeHooks, err := loadAdminModRuntimeHooks(modDir)
		if err != nil {
			return nil, err
		}
		runtimePolicy := domaingame.AdminModRuntimePolicyNoHooks
		if len(runtimeHooks) > 0 {
			runtimePolicy = domaingame.AdminModRuntimePolicyLegacyPHPOnly
		}
		mods = append(mods, domaingame.AdminModInfo{
			Folder:        entry.Name(),
			Name:          strings.TrimSpace(manifest.Name),
			Version:       strings.TrimSpace(manifest.Version),
			Author:        strings.TrimSpace(manifest.Author),
			Description:   strings.TrimSpace(manifest.Description),
			Website:       strings.TrimSpace(manifest.Website),
			RuntimeHooks:  runtimeHooks,
			RuntimePolicy: runtimePolicy,
		})
	}
	installed := map[string]int{}
	if r.queryer != nil {
		uniTable, err := tableName(r.prefix, "uni")
		if err != nil {
			return nil, err
		}
		list, err := r.loadAdminModList(ctx, uniTable)
		if err != nil {
			return nil, err
		}
		available := make(map[string]bool, len(mods))
		for _, mod := range mods {
			available[mod.Folder] = true
		}
		cleanList := make([]string, 0, len(list))
		changed := false
		for _, mod := range list {
			if !available[mod] {
				changed = true
				continue
			}
			cleanList = append(cleanList, mod)
		}
		if changed && r.execer != nil {
			if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET modlist = ?", uniTable), domaingame.JoinAdminModList(cleanList)); err != nil {
				return nil, err
			}
		}
		for index, mod := range cleanList {
			installed[mod] = index + 1
		}
		for index := range mods {
			mods[index].Installed = installed[mods[index].Folder] > 0
			mods[index].Active = mods[index].Installed
		}
	}
	sort.SliceStable(mods, func(i, j int) bool {
		leftInstalled := installed[mods[i].Folder]
		rightInstalled := installed[mods[j].Folder]
		if leftInstalled != 0 || rightInstalled != 0 {
			if leftInstalled == 0 {
				return false
			}
			if rightInstalled == 0 {
				return true
			}
			return leftInstalled < rightInstalled
		}
		return mods[i].Folder < mods[j].Folder
	})
	return mods, nil
}

func loadAdminModRuntimeHooks(modDir string) ([]string, error) {
	hooks := make([]string, 0, 4)
	if exists, err := regularFileExists(filepath.Join(modDir, "main.php")); err != nil {
		return nil, err
	} else if exists {
		hooks = append(hooks, "main.php")
	}
	for _, relDir := range []string{"pages", "pages_admin"} {
		entries, err := os.ReadDir(filepath.Join(modDir, relDir))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".php") {
				continue
			}
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			hooks = append(hooks, filepath.ToSlash(filepath.Join(relDir, name)))
		}
	}
	return hooks, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
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
	if mode == "ColonySettings" && query.Action == domaingame.AdminActionSettings {
		colonyTable, err := tableName(r.prefix, "coltab")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminColonySettings(ctx, colonyTable, query.Values)
	}
	if mode == "Checksum" && query.Action == domaingame.AdminActionChecksumFix {
		return r.fixAdminChecksums()
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
		return r.mutateAdminUniverseSettings(ctx, uniTable, usersTable, query.Universe)
	}
	if mode == "Broadcast" && query.Action == domaingame.AdminActionBroadcastSend {
		return r.mutateAdminBroadcast(ctx, query)
	}
	if mode == "Reports" && query.Action == domaingame.AdminActionReportsDelete {
		return r.mutateAdminReports(ctx, query)
	}
	if (mode == "Debug" || mode == "Errors") && query.Action == domaingame.AdminActionMessagesDelete {
		return r.mutateAdminMessages(ctx, mode, query)
	}
	if mode == "Debug" && query.Action == domaingame.AdminActionMessagesFilter {
		return nil, nil
	}
	if mode == "UserLogs" && query.Action == domaingame.AdminActionUserLogsSearch {
		return nil, nil
	}
	if mode == "BattleSim" {
		return r.mutateAdminBattleSim(ctx, query)
	}
	if mode == "RakSim" {
		return r.mutateAdminRakSim(query), nil
	}
	if mode == "Expedition" && query.Action == domaingame.AdminActionExpeditionSim {
		return r.mutateAdminExpeditionSim(ctx, query)
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
	if mode == "Mods" {
		uniTable, err := tableName(r.prefix, "uni")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminMods(ctx, uniTable, query)
	}
	if mode == "Bots" && query.Action == domaingame.AdminActionBotStop {
		queueTable, err := tableName(r.prefix, "queue")
		if err != nil {
			return nil, err
		}
		return r.mutateAdminBotStop(ctx, queueTable, query.TargetIDs)
	}
	if mode == "Bots" && query.Action == domaingame.AdminActionBotAdd {
		return r.mutateAdminBotAdd(ctx, query)
	}
	if mode == "Users" {
		uniTable, err := tableName(r.prefix, "uni")
		if err != nil {
			return nil, err
		}
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
		return r.mutateAdminUsers(ctx, uniTable, usersTable, planetsTable, fleetTable, query)
	}
	if mode == "Planets" {
		return r.mutateAdminPlanets(ctx, query)
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
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return nil, err
	}
	prangerTable, err := tableName(r.prefix, "pranger")
	if err != nil {
		return nil, err
	}
	return r.mutateAdminBans(ctx, usersTable, planetsTable, fleetTable, queueTable, prangerTable, query)
}

func (r AdminRepository) mutateAdminMods(ctx context.Context, uniTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if query.ModName == "" {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	available := true
	if query.Action == domaingame.AdminActionModInstall {
		var err error
		available, err = r.adminModAvailable(query.ModName)
		if err != nil {
			return nil, err
		}
		if available {
			hooks, err := loadAdminModRuntimeHooks(filepath.Join(r.legacyGameDir, "mods", query.ModName))
			if err != nil {
				return nil, err
			}
			if len(hooks) > 0 {
				return domaingame.AdminIssue(domaingame.AdminIssueModRuntimeExcluded), nil
			}
		}
	}
	installed, err := r.loadAdminModList(ctx, uniTable)
	if err != nil {
		return nil, err
	}
	next, changed := domaingame.ApplyAdminModAction(installed, query.Action, query.ModName, available)
	if !changed {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET modlist = ?", uniTable), domaingame.JoinAdminModList(next)); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) mutateAdminBotStop(ctx context.Context, queueTable string, targetIDs []int) (*domaingame.AdminActionIssue, error) {
	if len(targetIDs) == 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueBotStopped), nil
	}
	for _, playerID := range targetIDs {
		if playerID <= 0 {
			continue
		}
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE type = ? AND owner_id = ?", queueTable), "AI", playerID); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueBotStopped), nil
}

type adminBotStartStrategy struct {
	ID           int
	StartBlockID int
	HasStart     bool
}

type adminBotUniverse struct {
	Systems  int
	Galaxies int
	Language string
	StartDM  int
}

type adminBotCoordinates struct {
	Galaxy   int
	System   int
	Position int
}

func (r AdminRepository) mutateAdminBotAdd(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	iplogsTable, err := tableName(r.prefix, "iplogs")
	if err != nil {
		return nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	botvarsTable, err := tableName(r.prefix, "botvars")
	if err != nil {
		return nil, err
	}
	botstratTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return nil, err
	}

	start, found, err := r.loadAdminBotStartStrategy(ctx, botstratTable)
	if err != nil {
		return nil, err
	}
	if !found {
		return domaingame.AdminIssue(domaingame.AdminIssueBotNoStart), nil
	}
	name := query.Name
	lowerName := strings.ToLower(name)
	exists, err := r.adminBotUserExists(ctx, usersTable, lowerName)
	if err != nil {
		return nil, err
	}
	if exists {
		return domaingame.AdminIssue(domaingame.AdminIssueBotExists), nil
	}
	universe, err := r.loadAdminBotUniverse(ctx, uniTable)
	if err != nil {
		return nil, err
	}
	coords, err := r.nextAdminBotHomePlanet(ctx, planetsTable, universe)
	if err != nil {
		return nil, err
	}
	passwordGenerator := r.botPassword
	if passwordGenerator == nil {
		passwordGenerator = randomBotPassword
	}
	password, err := passwordGenerator()
	if err != nil {
		return nil, err
	}
	now := int(r.now().Unix())
	remoteAddr := query.RemoteAddr
	if remoteAddr == "" {
		remoteAddr = "0.0.0.0"
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET usercount = usercount + 1", uniTable)); err != nil {
		return nil, err
	}
	playerID, err := r.insertAdminBotUser(ctx, usersTable, name, lowerName, password, remoteAddr, universe, now)
	if err != nil {
		return nil, err
	}
	if err := r.insertAdminBotIPLog(ctx, iplogsTable, playerID, remoteAddr, now); err != nil {
		return nil, err
	}
	planetID, err := r.insertAdminBotHomePlanet(ctx, planetsTable, playerID, coords, now)
	if err != nil {
		return nil, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET hplanetid = ?, aktplanet = ? WHERE player_id = ?", usersTable), planetID, planetID, playerID); err != nil {
		return nil, err
	}
	if err := r.insertAdminBotVar(ctx, botvarsTable, playerID, "TimeLimit", "94608000"); err != nil {
		return nil, err
	}
	if err := r.insertAdminBotVar(ctx, botvarsTable, playerID, "password", password); err != nil {
		return nil, err
	}
	if start.HasStart {
		if _, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
			playerID,
			"AI",
			start.ID,
			start.StartBlockID,
			0,
			now,
			now,
			1000,
		); err != nil {
			return nil, err
		}
	}
	if err := r.overview.recalcRanks(ctx, usersTable); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueBotAdded), nil
}

func (r AdminRepository) loadAdminBotStartStrategy(ctx context.Context, botstratTable string) (adminBotStartStrategy, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id, COALESCE(source, '') FROM %s WHERE name = ? LIMIT 1", botstratTable), "_start")
	if err != nil {
		return adminBotStartStrategy{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return adminBotStartStrategy{}, false, rows.Err()
	}
	var id int
	var source string
	if err := rows.Scan(&id, &source); err != nil {
		return adminBotStartStrategy{}, false, err
	}
	if err := rows.Err(); err != nil {
		return adminBotStartStrategy{}, false, err
	}
	start := adminBotStartStrategy{ID: id}
	var graph struct {
		Nodes []struct {
			Key      int    `json:"key"`
			Category string `json:"category"`
		} `json:"nodeDataArray"`
	}
	if err := json.Unmarshal([]byte(source), &graph); err != nil {
		return start, true, nil
	}
	for _, node := range graph.Nodes {
		if node.Category == "Start" {
			start.StartBlockID = node.Key
			start.HasStart = true
			break
		}
	}
	return start, true, nil
}

func (r AdminRepository) adminBotUserExists(ctx context.Context, usersTable string, lowerName string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT player_id FROM %s WHERE name = ? LIMIT 1", usersTable), lowerName)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	return exists, rows.Err()
}

func (r AdminRepository) loadAdminBotUniverse(ctx context.Context, uniTable string) (adminBotUniverse, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT systems, galaxies, lang, start_dm FROM %s LIMIT 1", uniTable))
	if err != nil {
		return adminBotUniverse{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return adminBotUniverse{}, errors.New("admin bot universe row not found")
	}
	var universe adminBotUniverse
	if err := rows.Scan(&universe.Systems, &universe.Galaxies, &universe.Language, &universe.StartDM); err != nil {
		return adminBotUniverse{}, err
	}
	if err := rows.Err(); err != nil {
		return adminBotUniverse{}, err
	}
	if universe.Systems <= 0 || universe.Galaxies <= 0 {
		return adminBotUniverse{}, errors.New("admin bot universe has invalid galaxy layout")
	}
	if strings.TrimSpace(universe.Language) == "" {
		universe.Language = "en"
	}
	return universe, nil
}

func (r AdminRepository) nextAdminBotHomePlanet(ctx context.Context, planetsTable string, universe adminBotUniverse) (adminBotCoordinates, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT g, s, p FROM %s WHERE g >= 1 AND p <= 15 AND type <> ? ORDER BY g, s, p", planetsTable), planetTypeDestroyedMoon)
	if err != nil {
		return adminBotCoordinates{}, err
	}
	defer rows.Close()
	occupied := make(map[int]bool)
	planetsPerGalaxy := 15 * universe.Systems
	for rows.Next() {
		var coords adminBotCoordinates
		if err := rows.Scan(&coords.Galaxy, &coords.System, &coords.Position); err != nil {
			return adminBotCoordinates{}, err
		}
		if coords.Galaxy < 1 || coords.System < 1 || coords.Position < 1 {
			continue
		}
		index := ((coords.Galaxy - 1) * planetsPerGalaxy) + ((coords.System - 1) * 15) + coords.Position - 1
		occupied[index] = true
	}
	if err := rows.Err(); err != nil {
		return adminBotCoordinates{}, err
	}
	for distance := 0.0; distance < float64(planetsPerGalaxy*universe.Galaxies); distance += 1.3 {
		index := int(math.Floor(distance))
		galaxy := index/planetsPerGalaxy + 1
		if galaxy > universe.Galaxies {
			break
		}
		withinGalaxy := index - ((galaxy - 1) * planetsPerGalaxy)
		system := withinGalaxy/15 + 1
		position := withinGalaxy%15 + 1
		if position > 3 && position < 13 && !occupied[index] {
			return adminBotCoordinates{Galaxy: galaxy, System: system, Position: position}, nil
		}
	}
	return adminBotCoordinates{}, errors.New("no admin bot home planet slots available")
}

func (r AdminRepository) insertAdminBotUser(ctx context.Context, usersTable string, originalName string, lowerName string, password string, remoteAddr string, universe adminBotUniverse, now int) (int, error) {
	columns := []string{
		"regdate", "ally_id", "joindate", "allyrank", "session", "private_session", "name", "oname", "name_changed", "name_until",
		"password", "temp_pass", "pemail", "email", "email_changed", "email_until", "disable", "disable_until", "vacation", "vacation_until",
		"banned", "banned_until", "noattack", "noattack_until", "lastlogin", "lastclick", "ip_addr", "validated", "validatemd", "hplanetid",
		"admin", "sortby", "sortorder", "skin", "useskin", "deact_ip", "maxspy", "maxfleetmsg", "lang", "aktplanet",
		"dm", "dmfree", "sniff", "debug", "trader", "rate_m", "rate_k", "rate_d", "score1", "score2", "score3", "place1", "place2", "place3",
		"oldscore1", "oldscore2", "oldscore3", "oldplace1", "oldplace2", "oldplace3", "scoredate", "flags", "feedid", "lastfeed",
		"com_until", "adm_until", "eng_until", "geo_until", "tec_until",
	}
	args := []any{
		now, 0, 0, 0, "", "", lowerName, originalName, 0, 0,
		hashOverviewPassword(password, r.secret), "", "", "", 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, remoteAddr, 1, "", 0,
		0, 0, 0, "/evolution/", 1, 0, 1, 3, universe.Language, 0,
		0, universe.StartDM, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 31, "", 0,
		0, 0, 0, 0, 0,
	}
	result, err := r.execer.ExecContext(ctx, adminInsertStatement(usersTable, columns), args...)
	if err != nil {
		return 0, err
	}
	return adminLastInsertID(result)
}

func (r AdminRepository) insertAdminBotIPLog(ctx context.Context, iplogsTable string, playerID int, remoteAddr string, now int) error {
	_, err := r.execer.ExecContext(ctx, adminInsertStatement(iplogsTable, []string{"ip", "user_id", "reg", "date"}), remoteAddr, playerID, 1, now)
	return err
}

func (r AdminRepository) insertAdminBotHomePlanet(ctx context.Context, planetsTable string, playerID int, coords adminBotCoordinates, now int) (int, error) {
	columns := []string{"name", "type", "g", "s", "p", "owner_id", "diameter", "temp", "fields", "maxfields", "date", "700", "701", "702", "lastpeek", "lastakt", "gate_until", "remove"}
	args := []any{
		"Homeplanet",
		1,
		coords.Galaxy,
		coords.System,
		coords.Position,
		playerID,
		12800,
		adminBotHomePlanetTemperature(coords.Position, now%10),
		0,
		163,
		now,
		500,
		500,
		0,
		now,
		now,
		0,
		0,
	}
	result, err := r.execer.ExecContext(ctx, adminInsertStatement(planetsTable, columns), args...)
	if err != nil {
		return 0, err
	}
	return adminLastInsertID(result)
}

func (r AdminRepository) insertAdminBotVar(ctx context.Context, botvarsTable string, playerID int, name string, value string) error {
	_, err := r.execer.ExecContext(ctx, adminInsertStatement(botvarsTable, []string{"owner_id", "var", "value"}), playerID, name, value)
	return err
}

func adminBotHomePlanetTemperature(position int, jitter int) int {
	switch {
	case position <= 3:
		return 80 + jitter - 2*position
	case position >= 4 && position <= 6:
		return 30 + jitter - 2*position
	case position >= 7 && position <= 9:
		return 10 + jitter - 2*position
	case position >= 10 && position <= 12:
		return -10 + jitter - 2*position
	default:
		return -60 + jitter - 2*position
	}
}

func adminInsertStatement(table string, columns []string) string {
	quoted := make([]string, 0, len(columns))
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, "`"+column+"`")
		values = append(values, "?")
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(quoted, ", "), strings.Join(values, ", "))
}

func adminLastInsertID(result sql.Result) (int, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("database returned empty id")
	}
	return int(id), nil
}

func randomBotPassword() (string, error) {
	entropy := make([]byte, 8)
	if _, err := randomBotPasswordRead(entropy); err != nil {
		return "", err
	}
	return formatRandomBotPassword(entropy), nil
}

var randomBotPasswordRead = rand.Read

func formatRandomBotPassword(entropy []byte) string {
	syllables := []string{"er", "in", "tia", "wol", "fe", "pre", "vet", "jo", "nes", "al", "len", "son", "cha", "ir", "ler", "bo", "ok", "tio", "nar", "sim", "ple", "bla", "ten", "toe", "cho", "co", "lat", "spe", "ak", "er", "po", "co", "lor", "pen", "cil", "li", "ght", "wh", "at", "the", "he", "ck", "is", "mam", "bo", "no", "fi", "ve", "any", "way", "pol", "iti", "cs", "ra", "dio", "sou", "rce", "sea", "rch", "pa", "per", "com"}
	var builder strings.Builder
	for count := 0; count < 4; count++ {
		mode := entropy[count*2]
		value := entropy[count*2+1]
		if mode%10 == 1 {
			builder.WriteString(strconv.Itoa(int(value%50) + 1))
			continue
		}
		builder.WriteString(syllables[int(value)%len(syllables)])
	}
	return builder.String()
}

func (r AdminRepository) loadAdminModList(ctx context.Context, uniTable string) ([]string, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(modlist, '') FROM %s LIMIT 1", uniTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var raw string
	if err := rows.Scan(&raw); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domaingame.NormalizeAdminModList(raw), nil
}

func (r AdminRepository) adminModAvailable(modName string) (bool, error) {
	if modName == "" || filepath.Base(modName) != modName || strings.ContainsAny(modName, `/\`) {
		return false, nil
	}
	manifestPath := filepath.Join(r.legacyGameDir, "mods", modName, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var manifest adminModManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false, err
	}
	return strings.TrimSpace(manifest.Name) != "", nil
}

var adminLocaAssignmentPattern = regexp.MustCompile(`(?s)\$LOCA\[\s*"([^"]+)"\s*\]\[\s*"([^"]+)"\s*(?:\.\s*([A-Z][A-Z0-9_]*))?\s*\]\s*=\s*((?:"(?:\\.|[^"\\])*"\s*(?:\.\s*)?)+);`)
var adminLocaNestedAssignmentPattern = regexp.MustCompile(`(?s)\$LOCA\[\s*"([^"]+)"\s*\]\[\s*"([^"]+)"\s*\]\s*=\s*"((?:\\.|[^"\\])*)"\s*\.\s*\$LOCA\[\s*"([^"]+)"\s*\]\[\s*"([^"]+)"\s*\]\s*=\s*((?:"(?:\\.|[^"\\])*"\s*(?:\.\s*)?)+);`)
var adminLocaStringPattern = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)

var adminLocaConstants = map[string]string{
	"GID_RC_METAL":     "700",
	"GID_RC_CRYSTAL":   "701",
	"GID_RC_DEUTERIUM": "702",
	"GID_RC_ENERGY":    "703",
	"GID_RC_DM":        "704",
}

type adminLocalizationEntry struct {
	key   string
	value string
}

type positionedAdminLocalizationEntry struct {
	position int
	entry    adminLocalizationEntry
}

func (r AdminRepository) loadAdminLocalization(source string, target string) (*domaingame.AdminLocalization, error) {
	locaDir := filepath.Join(r.legacyGameDir, "loca")
	entries, err := os.ReadDir(locaDir)
	if errors.Is(err, os.ErrNotExist) {
		return &domaingame.AdminLocalization{}, nil
	}
	if err != nil {
		return nil, err
	}
	languages := make([]string, 0, len(entries))
	valid := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			languages = append(languages, entry.Name())
			valid[entry.Name()] = true
		}
	}
	sort.Strings(languages)
	result := &domaingame.AdminLocalization{Languages: languages}
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if !valid[source] || !valid[target] {
		return result, nil
	}
	result.Source = source
	result.Target = target
	sourceDir := filepath.Join(locaDir, source)
	sourceFiles, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range sourceFiles {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".php") {
			continue
		}
		file, err := r.compareAdminLocalizationFile(locaDir, source, target, entry.Name())
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, file)
	}
	sort.SliceStable(result.Files, func(i, j int) bool {
		return result.Files[i].Name < result.Files[j].Name
	})
	return result, nil
}

func (r AdminRepository) compareAdminLocalizationFile(locaDir string, source string, target string, fileName string) (domaingame.AdminLocalizationFile, error) {
	sourcePath := filepath.Join(locaDir, source, fileName)
	targetPath := filepath.Join(locaDir, target, fileName)
	sourceLang := adminLocaLanguage(source)
	targetLang := adminLocaLanguage(target)
	sourceEntries, err := readAdminLocalizationEntries(sourcePath, sourceLang)
	if err != nil {
		return domaingame.AdminLocalizationFile{}, err
	}
	targetEntries, err := readAdminLocalizationEntries(targetPath, targetLang)
	if errors.Is(err, os.ErrNotExist) {
		targetEntries = nil
	} else if err != nil {
		return domaingame.AdminLocalizationFile{}, err
	}
	targetValues := make(map[string]string, len(targetEntries))
	for _, entry := range targetEntries {
		targetValues[entry.key] = entry.value
	}
	file := domaingame.AdminLocalizationFile{
		Name:          fileName,
		TargetMissing: len(sourceEntries) == 0,
		Rows:          make([]domaingame.AdminLocalizationRow, 0, len(sourceEntries)),
	}
	for _, entry := range sourceEntries {
		targetValue, ok := targetValues[entry.key]
		status := "ok"
		if !ok {
			targetValue = "The string is missing!"
			status = "missing"
		} else if entry.value != "" && entry.value == targetValue {
			status = "same"
		}
		file.Rows = append(file.Rows, domaingame.AdminLocalizationRow{
			Key:    entry.key,
			Source: entry.value,
			Target: targetValue,
			Status: status,
		})
	}
	return file, nil
}

func readAdminLocalizationFile(path string, language string) (map[string]string, error) {
	entries, err := readAdminLocalizationEntries(path, language)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		values[entry.key] = entry.value
	}
	return values, nil
}

func readAdminLocalizationEntries(path string, language string) ([]adminLocalizationEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	parsed := make([]positionedAdminLocalizationEntry, 0)
	for _, match := range adminLocaAssignmentPattern.FindAllStringSubmatchIndex(text, -1) {
		matchLanguage := adminLocaRegexpGroup(text, match, 1)
		if language != "" && matchLanguage != language {
			continue
		}
		key := adminLocaRegexpGroup(text, match, 2) + adminLocaConstants[adminLocaRegexpGroup(text, match, 3)]
		value := parseAdminLocaStringExpression(adminLocaRegexpGroup(text, match, 4))
		parsed = append(parsed, positionedAdminLocalizationEntry{position: match[0], entry: adminLocalizationEntry{key: key, value: value}})
	}
	for _, match := range adminLocaNestedAssignmentPattern.FindAllStringSubmatchIndex(text, -1) {
		matchLanguage := adminLocaRegexpGroup(text, match, 1)
		if language != "" && matchLanguage != language {
			continue
		}
		prefix := parseAdminLocaStringExpression(`"` + adminLocaRegexpGroup(text, match, 3) + `"`)
		value := prefix + parseAdminLocaStringExpression(adminLocaRegexpGroup(text, match, 6))
		parsed = append(parsed, positionedAdminLocalizationEntry{
			position: match[1],
			entry:    adminLocalizationEntry{key: adminLocaRegexpGroup(text, match, 2), value: value},
		})
	}
	sort.SliceStable(parsed, func(i, j int) bool { return parsed[i].position < parsed[j].position })

	entries := make([]adminLocalizationEntry, 0)
	indexes := map[string]int{}
	for _, item := range parsed {
		entry := item.entry
		if index, ok := indexes[entry.key]; ok {
			entries[index] = entry
		} else {
			indexes[entry.key] = len(entries)
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func adminLocaRegexpGroup(text string, match []int, group int) string {
	start := match[group*2]
	end := match[group*2+1]
	if start < 0 || end < 0 {
		return ""
	}
	return text[start:end]
}

func parseAdminLocaStringExpression(expression string) string {
	var value strings.Builder
	for _, literal := range adminLocaStringPattern.FindAllStringSubmatch(expression, -1) {
		value.WriteString(unescapePHPDoubleQuotedString(literal[1]))
	}
	return value.String()
}

func unescapePHPDoubleQuotedString(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			builder.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case 'v':
			builder.WriteByte('\v')
		case 'e':
			builder.WriteByte(0x1b)
		case 'f':
			builder.WriteByte('\f')
		case '\\', '"', '$':
			builder.WriteByte(value[index])
		case 'x':
			parsed, consumed := parseAdminLocaEscapedInteger(value[index+1:], 16, 2)
			if consumed == 0 {
				builder.WriteString(`\x`)
				continue
			}
			builder.WriteByte(byte(parsed))
			index += consumed
		default:
			if value[index] >= '0' && value[index] <= '7' {
				parsed, consumed := parseAdminLocaEscapedInteger(value[index:], 8, 3)
				builder.WriteByte(byte(parsed))
				index += consumed - 1
				continue
			}
			builder.WriteByte('\\')
			builder.WriteByte(value[index])
		}
	}
	return builder.String()
}

func parseAdminLocaEscapedInteger(value string, base int, limit int) (int64, int) {
	consumed := 0
	for consumed < len(value) && consumed < limit {
		character := value[consumed]
		valid := character >= '0' && character <= '7'
		if base == 16 {
			valid = valid || character == '8' || character == '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F'
		}
		if !valid {
			break
		}
		consumed++
	}
	if consumed == 0 {
		return 0, 0
	}
	parsed, err := strconv.ParseInt(value[:consumed], base, 16)
	if err != nil {
		return 0, 0
	}
	return parsed, consumed
}

func adminLocaLanguage(directory string) string {
	if index := strings.Index(directory, "_"); index > 0 {
		return directory[:index]
	}
	return directory
}

func (r AdminRepository) mutateAdminBattleSim(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	return r.runAdminBattleSimulator(ctx, query)
}

func (r AdminRepository) mutateAdminRakSim(query appgame.AdminMutationQuery) *domaingame.AdminActionIssue {
	target := domaingame.DefenseCounts{}
	values := make(map[string]int, len(domaingame.DefenseIDs())+4)
	for _, name := range []string{"a_weap", "d_armor", "anz", "pziel"} {
		values[name] = query.Values[name]
	}
	for _, id := range domaingame.DefenseIDs() {
		value := query.Values[fmt.Sprintf("d_%d", id)]
		target[id] = value
		values[fmt.Sprintf("d_%d", id)] = value
	}
	result := domaingame.ResolveMissileAttack(domaingame.MissileAttackInput{
		Amount:           values["anz"],
		PrimaryDefenseID: values["pziel"],
		Target:           target,
		AttackerWeapon:   values["a_weap"],
		DefenderArmour:   values["d_armor"],
	})
	for _, id := range domaingame.DefenseIDs() {
		values[fmt.Sprintf("d_%d", id)] = result.Target[id]
	}
	issue := domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Missile attack simulator completed.")
	issue.Result = &domaingame.AdminActionResult{Values: values}
	return issue
}

func (r AdminRepository) mutateAdminExpeditionSim(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	values, err := r.loadAdminExpeditionSettings(ctx)
	if err != nil {
		return nil, err
	}
	settings := adminExpeditionSettings(values)
	count := max(0, query.Values["expcount"])
	series := make([]int, int(domaingame.ExpeditionTrader)+1)
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	for range count {
		event, err := domaingame.ResolveExpeditionEvent(settings, 0, 1, random)
		if err != nil {
			return nil, err
		}
		series[int(event)]++
	}
	issue := domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, "Expedition simulation completed.")
	issue.Result = &domaingame.AdminActionResult{Values: map[string]int{"expcount": query.Values["expcount"]}, Series: series}
	return issue, nil
}

func adminExpeditionSettings(values map[string]int) domaingame.ExpeditionSettings {
	settings := domaingame.ExpeditionSettings{
		ChanceSuccess: values["chance_success"], DepletedMin: values["depleted_min"], DepletedMed: values["depleted_med"], DepletedMax: values["depleted_max"],
		ChanceDepletedMin: values["chance_depleted_min"], ChanceDepletedMed: values["chance_depleted_med"], ChanceDepletedMax: values["chance_depleted_max"],
		ChanceAlien: values["chance_alien"], ChancePirates: values["chance_pirates"], ChanceDM: values["chance_dm"], ChanceLost: values["chance_lost"],
		ChanceDelay: values["chance_delay"], ChanceAccel: values["chance_accel"], ChanceRes: values["chance_res"], ChanceFleet: values["chance_fleet"], DMFactor: values["dm_factor"],
		PointLimitMax: values["limit_max"],
	}
	for index := range settings.ScoreCaps {
		number := index + 1
		settings.ScoreCaps[index] = values[fmt.Sprintf("score_cap%d", number)]
		settings.PointLimits[index] = values[fmt.Sprintf("limit_cap%d", number)]
	}
	return settings
}

func (r AdminRepository) mutateAdminUniverseSettings(ctx context.Context, uniTable string, usersTable string, settings *domaingame.AdminUniverseMutation) (*domaingame.AdminActionIssue, error) {
	if settings == nil {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	now := int(r.now().Unix())
	if settings.NewsUpdateDays > 0 {
		newsUntil := now + settings.NewsUpdateDays*24*60*60
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET news1 = ?, news2 = ?, news_until = ?", uniTable), settings.News1, settings.News2, newsUntil); err != nil {
			return nil, err
		}
	}
	if settings.NewsOff {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET news_until = 0", uniTable)); err != nil {
			return nil, err
		}
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET lang = ?, battle_engine = ?, freeze = ?, speed = ?, fspeed = ?, acs = ?, fid = ?, did = ?, defrepair = ?, defrepair_delta = ?, galaxies = ?, systems = ?, rapid = ?, moons = ?, php_battle = ?, battle_max = ?, force_lang = ?, start_dm = ?, max_werf = ?, feedage = ?", uniTable),
		settings.Language, settings.BattleEngine, legacyBoolInt(settings.Freeze), settings.Speed, settings.FleetSpeed,
		settings.ACS, settings.FleetDebris, settings.DefenseDebris, settings.DefenseRepair, settings.DefenseDelta,
		settings.Galaxies, settings.Systems, legacyBoolInt(settings.RapidFire), legacyBoolInt(settings.Moons),
		legacyBoolInt(settings.PHPBattle), settings.BattleMax, legacyBoolInt(settings.ForceLanguage),
		settings.StartDarkMatter, settings.MaxShipyard, settings.FeedAge,
	); err != nil {
		return nil, err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET ext_board = ?, ext_discord = ?, ext_tutorial = ?, ext_rules = ?, ext_impressum = ?", uniTable),
		settings.ExtBoard, settings.ExtDiscord, settings.ExtTutorial, settings.ExtRules, settings.ExtImpressum,
	); err != nil {
		return nil, err
	}
	if settings.MaxUsers > 0 {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET maxusers = ?", uniTable), settings.MaxUsers); err != nil {
			return nil, err
		}
	}
	if settings.Freeze {
		activeSince := now - 7*24*60*60
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET vacation = 1, vacation_until = ? WHERE lastclick >= ? AND admin = 0", usersTable), now, activeSince); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func legacyBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (r AdminRepository) mutateAdminBroadcast(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	subject := query.Subject
	text := query.Text
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
	messageText := sanitizeAdminBroadcastText(legacyAdminBroadcastBBCode(text))
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

func (r AdminRepository) mutateAdminMessages(ctx context.Context, mode string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	rawTable := "errors"
	includeErrorIDOrder := false
	if mode == "Debug" {
		rawTable = "debug"
		includeErrorIDOrder = true
		if query.Filter != "" {
			return nil, nil
		}
	}
	table, err := tableName(r.prefix, rawTable)
	if err != nil {
		return nil, err
	}
	if mode == "Debug" && query.DeleteMode == "deleteall" {
		statement := "TRUNCATE TABLE " + table
		if r.dialect == DialectSQLite {
			statement = "DELETE FROM " + table
		}
		_, err := r.execer.ExecContext(ctx, statement)
		return nil, err
	}
	ids, err := r.loadAdminMessageIDs(ctx, table, includeErrorIDOrder)
	if err != nil {
		return nil, err
	}
	selected := make(map[int]struct{}, len(query.TargetIDs))
	for _, id := range query.TargetIDs {
		selected[id] = struct{}{}
	}
	deleteShown := mode == "Debug" && query.DeleteMode == "deleteshown"
	deleteAll := mode == "Errors" && query.DeleteMode == "deleteall"
	for _, id := range ids {
		if _, ok := selected[id]; !ok && !deleteShown && !deleteAll {
			continue
		}
		if _, err := r.execer.ExecContext(ctx, "DELETE FROM "+table+" WHERE error_id = ?", id); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r AdminRepository) loadAdminMessageIDs(ctx context.Context, table string, includeErrorIDOrder bool) ([]int, error) {
	order := "date DESC"
	if includeErrorIDOrder {
		order += ", error_id DESC"
	}
	rows, err := r.queryer.QueryContext(ctx, "SELECT error_id FROM "+table+" ORDER BY "+order+" LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0, 50)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
	text = strings.NewReplacer(`\"`, "&quot;", `'`, "&rsquo;", "\\`", "&lsquo;").Replace(text)
	return strings.ReplaceAll(text, `"`, `\"`)
}

var adminBroadcastSimpleBBTags = []struct {
	pattern *regexp.Regexp
	open    string
	close   string
}{
	{regexp.MustCompile(`(?is)\[b\](.*?)\[/b\]`), "<strong>", "</strong>"},
	{regexp.MustCompile(`(?is)\[i\](.*?)\[/i\]`), "<i>", "</i>"},
	{regexp.MustCompile(`(?is)\[u\](.*?)\[/u\]`), "<u>", "</u>"},
	{regexp.MustCompile(`(?is)\[(?:s|strike)\](.*?)\[/(?:s|strike)\]`), "<del>", "</del>"},
	{regexp.MustCompile(`(?is)\[sub\](.*?)\[/sub\]`), "<sub>", "</sub>"},
	{regexp.MustCompile(`(?is)\[sup\](.*?)\[/sup\]`), "<sup>", "</sup>"},
}

var (
	adminBroadcastURLAttribute = regexp.MustCompile(`(?is)\[url=([^\]]+)\](.*?)\[/url\]`)
	adminBroadcastURLText      = regexp.MustCompile(`(?is)\[url\](.*?)\[/url\]`)
	adminBroadcastColor        = regexp.MustCompile(`(?is)\[color=([^\]]+)\](.*?)\[/color\]`)
	adminBroadcastFont         = regexp.MustCompile(`(?is)\[font([^\]]*)\](.*?)\[/font\]`)
	adminBroadcastSize         = regexp.MustCompile(`(?is)\[size=([^\]]+)\](.*?)\[/size\]`)
	adminBroadcastEmail        = regexp.MustCompile(`(?is)\[email(?:=([^\]]+))?\](.*?)\[/email\]`)
	adminBroadcastAlign        = regexp.MustCompile(`(?is)\[align=([^\]]+)\](.*?)\[/align\]`)
	adminBroadcastHorizontal   = regexp.MustCompile(`(?is)\s*\[hr\]\s*`)
	adminBroadcastImage        = regexp.MustCompile(`(?is)\[img([^\]]*)\](.*?)\[/img\]`)
	adminBroadcastQuote        = regexp.MustCompile(`(?is)\[quote(?:=([^\]]+))?\](.*?)\[/quote\]`)
	adminBroadcastAttribute    = regexp.MustCompile(`(?i)([a-z]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s\]]+))`)
)

func legacyAdminBroadcastBBCode(text string) string {
	text = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
	text = strings.ReplaceAll(text, "\n", "<br />\n")
	for _, tag := range adminBroadcastSimpleBBTags {
		for {
			next := tag.pattern.ReplaceAllString(text, tag.open+"$1"+tag.close)
			if next == text {
				break
			}
			text = next
		}
	}
	text = adminBroadcastColor.ReplaceAllString(text, `<font color="$1">$2</font>`)
	text = replaceAdminBroadcastFonts(text)
	text = replaceAdminBroadcastSizes(text)
	text = replaceAdminBroadcastEmails(text)
	text = replaceAdminBroadcastAlignments(text)
	text = adminBroadcastHorizontal.ReplaceAllString(text, `<hr class="bb" />`)
	text = replaceAdminBroadcastImages(text)
	text = replaceAdminBroadcastQuotes(text)
	text = replaceAdminBroadcastURLs(text, adminBroadcastURLAttribute, true)
	return replaceAdminBroadcastURLs(text, adminBroadcastURLText, false)
}

func replaceAdminBroadcastFonts(text string) string {
	return adminBroadcastFont.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastFont.FindStringSubmatch(value)
		attributes := parseAdminBroadcastAttributes("font" + parts[1])
		result := `<font face="` + adminBroadcastAttributeValue(attributes["font"]) + `"`
		for _, name := range []string{"color", "size"} {
			if attributes[name] != "" {
				result += ` ` + name + `="` + adminBroadcastAttributeValue(attributes[name]) + `"`
			}
		}
		return result + `>` + parts[2] + `</font>`
	})
}

func replaceAdminBroadcastSizes(text string) string {
	return adminBroadcastSize.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastSize.FindStringSubmatch(value)
		raw := strings.TrimSpace(parts[1])
		size, _ := strconv.Atoi(raw)
		sign := ""
		if strings.HasPrefix(raw, "+") {
			sign = "+"
		}
		switch {
		case size > 7:
			size = 7
			sign = ""
		case size < -6:
			size = -6
			sign = ""
		case size == 0:
			size = 3
		}
		return fmt.Sprintf(`<font size="%s%d">%s</font>`, sign, size, parts[2])
	})
}

func replaceAdminBroadcastEmails(text string) string {
	return adminBroadcastEmail.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastEmail.FindStringSubmatch(value)
		href := parts[1]
		if href == "" {
			href = parts[2]
		}
		if !strings.HasPrefix(href, "mailto:") {
			href = "mailto:" + href
		}
		return `<a class="bb_email" href="` + adminBroadcastAttributeValue(href) + `">` + parts[2] + `</a>`
	})
}

func replaceAdminBroadcastAlignments(text string) string {
	return adminBroadcastAlign.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastAlign.FindStringSubmatch(value)
		align := strings.ToLower(parts[1])
		switch align {
		case "left", "right", "center", "justify":
		default:
			align = ""
		}
		return `<div class="bb" align="` + align + `">` + parts[2] + `</div>`
	})
}

func replaceAdminBroadcastImages(text string) string {
	return adminBroadcastImage.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastImage.FindStringSubmatch(value)
		attributes := parseAdminBroadcastAttributes(parts[1])
		result := `<img class="reloadimage" title="pic.php?url=` + strings.ReplaceAll(url.QueryEscape(strings.TrimSpace(parts[2])), "+", "%20") + `" src="/game/img/preload.gif"alt=""`
		for _, name := range []string{"width", "height", "border"} {
			if attributes[name] == "" {
				continue
			}
			number, _ := strconv.Atoi(attributes[name])
			if name != "border" && number == 0 {
				continue
			}
			result += fmt.Sprintf(` %s="%d"`, name, number)
		}
		return result + ` />`
	})
}

func replaceAdminBroadcastQuotes(text string) string {
	return adminBroadcastQuote.ReplaceAllStringFunc(text, func(value string) string {
		parts := adminBroadcastQuote.FindStringSubmatch(value)
		author := ""
		if parts[1] != "" {
			author = "(\n<b style=\"color: white;\">" + adminBroadcastAttributeValue(parts[1]) + "</b>\n)"
		}
		header := "<div style=\"border: 3px double rgb(65, 86, 128); padding: 1px 4px 2px;\">\n\u0426\u0438\u0442\u0430\u0442\u0430 " + author + " </div>"
		return header + `<div style="border-style: none double double; border-color: -moz-use-text-color rgb(65, 86, 128) rgb(65, 86, 128); border-width: medium 3px 3px; padding: 4px 4px 6px;">` + parts[2] + `</div>`
	})
}

func parseAdminBroadcastAttributes(value string) map[string]string {
	result := map[string]string{}
	for _, match := range adminBroadcastAttribute.FindAllStringSubmatch(value, -1) {
		attributeValue := ""
		for index := 2; index < len(match); index++ {
			if match[index] != "" {
				attributeValue = match[index]
				break
			}
		}
		result[strings.ToLower(match[1])] = attributeValue
	}
	return result
}

func adminBroadcastAttributeValue(value string) string {
	return strings.NewReplacer(`"`, "&quot;", `'`, "&#039;").Replace(value)
}

func replaceAdminBroadcastURLs(text string, pattern *regexp.Regexp, withAttribute bool) string {
	return pattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := pattern.FindStringSubmatch(value)
		if len(parts) < 2 {
			return value
		}
		href := parts[1]
		label := parts[1]
		if withAttribute {
			if len(parts) < 3 {
				return value
			}
			label = parts[2]
		}
		if !adminBroadcastURLHasProtocol(href) {
			href = "http://" + href
		}
		return `<a class="bb" href="` + html.EscapeString(href) + `">` + label + `</a>`
	})
}

func adminBroadcastURLHasProtocol(value string) bool {
	for _, prefix := range []string{"http://", "https://", "ftp://", "file://", "mailto:", "#", "/", "?", "./", "../"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func (r AdminRepository) mutateAdminQueue(ctx context.Context, queueTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if query.Action == domaingame.AdminActionQueueCron {
		mails, err := r.runAdminCron(ctx, int(r.now().Unix()))
		if err != nil {
			return nil, err
		}
		issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
		issue.OutboundCouponMails = mails
		return issue, nil
	}
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
		fleetRepository.legacyEvents = true
		fleetRepository.dialect = r.dialect
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
		statement := fmt.Sprintf("UPDATE %s q JOIN %s f ON f.fleet_id = q.sub_id SET q.end = ? WHERE q.type = ? AND f.union_id = ?", queueTable, fleetTable)
		if r.dialect == DialectSQLite {
			statement = fmt.Sprintf("UPDATE %s SET end = ? WHERE type = ? AND sub_id IN (SELECT fleet_id FROM %s WHERE union_id = ?)", queueTable, fleetTable)
		}
		_, err = r.execer.ExecContext(
			ctx,
			statement,
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
			return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
		}
		assignments = append(assignments, "`"+column+"` = ?")
		args = append(args, value)
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s", expeditionTable, strings.Join(assignments, ", ")), args...)
	if err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) mutateAdminColonySettings(ctx context.Context, colonyTable string, values map[string]int) (*domaingame.AdminActionIssue, error) {
	assignments := make([]string, 0, len(adminColonySettingsColumns))
	args := make([]any, 0, len(adminColonySettingsColumns))
	for _, column := range adminColonySettingsColumns {
		value, ok := values[column]
		if !ok {
			return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
		}
		assignments = append(assignments, "`"+column+"` = ?")
		args = append(args, value)
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s", colonyTable, strings.Join(assignments, ", ")), args...); err != nil {
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

func (r AdminRepository) mutateAdminBans(ctx context.Context, usersTable string, planetsTable string, fleetTable string, queueTable string, prangerTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	targetIDs := uniquePositiveIDs(query.TargetIDs)
	if len(targetIDs) == 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	actor, _, err := r.loadAdminBanUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	now := int(r.now().Unix())
	seconds := query.Days*24*60*60 + query.Hours*60*60
	until := now + seconds
	reason := sanitizeAdminBanReason(query.Reason)
	recalculateRanks := false

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
			recalculateRanks = true
		case 1:
			if err := r.insertAdminBanPranger(ctx, prangerTable, actor, target, now, until, reason); err != nil {
				return nil, err
			}
			if err := r.banAdminUser(ctx, usersTable, queueTable, targetID, now, until, true); err != nil {
				return nil, err
			}
			recalculateRanks = true
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
			if err := r.recalcAdminUserStats(ctx, usersTable, planetsTable, fleetTable, targetID); err != nil {
				return nil, err
			}
		case 4:
			if err := r.unbanAdminUserAttacks(ctx, usersTable, queueTable, targetID); err != nil {
				return nil, err
			}
		}
	}
	if recalculateRanks {
		if err := r.overview.recalcRanks(ctx, usersTable); err != nil {
			return nil, err
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
	adminCouponPageSize      = 15
)

var legacyAdminTimeLocation = time.FixedZone("Europe/Moscow", 3*60*60)

func normalizeAdminCouponFrom(from int) int {
	if from < 0 {
		return 0
	}
	return from
}

func (r AdminRepository) loadAdminCouponRows(ctx context.Context, from int) ([]domaingame.AdminCouponRow, int, error) {
	if r.masterQueryer == nil {
		return []domaingame.AdminCouponRow{}, 0, nil
	}
	totalRows, err := r.masterQueryer.QueryContext(ctx, "SELECT COUNT(*) FROM coupons")
	if err != nil {
		return nil, 0, err
	}
	total := 0
	if totalRows.Next() {
		if err := totalRows.Scan(&total); err != nil {
			totalRows.Close()
			return nil, 0, err
		}
	}
	if err := totalRows.Err(); err != nil {
		totalRows.Close()
		return nil, 0, err
	}
	if err := totalRows.Close(); err != nil {
		return nil, 0, err
	}
	rows, err := r.masterQueryer.QueryContext(ctx, "SELECT id, COALESCE(code, ''), COALESCE(amount, 0), COALESCE(used, 0), COALESCE(user_uni, 0), COALESCE(user_id, 0), COALESCE(user_name, '') FROM coupons ORDER BY id DESC LIMIT ? OFFSET ?", adminCouponPageSize, normalizeAdminCouponFrom(from))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminCouponRow, 0, adminCouponPageSize)
	for rows.Next() {
		var row domaingame.AdminCouponRow
		var used int
		if err := rows.Scan(&row.ID, &row.Code, &row.Amount, &used, &row.UserUniverse, &row.UserID, &row.UserName); err != nil {
			return nil, 0, err
		}
		row.Used = used != 0
		result = append(result, row)
	}
	return result, total, rows.Err()
}

func (r AdminRepository) loadAdminCouponQueueRows(ctx context.Context) ([]domaingame.AdminCouponQueueRow, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, COALESCE(sub_id, 0), COALESCE(obj_id, 0), COALESCE(level, 0), COALESCE(start, 0), COALESCE(end, 0), COALESCE(prio, 0) FROM %s WHERE type = ? ORDER BY end ASC", queueTable),
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
		// The legacy INSERT targets an unsigned INT and ignores strict-mode range
		// errors after still reporting the generated code as successful.
		if amount < 0 || int64(amount) > int64(^uint32(0)) {
			return code, nil
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
	now := r.now()
	end := int(now.Unix()) + parseAdminCouponQueueEnd(query.DayMonth, query.HourMinute, now)
	packedCriteria := (query.InactiveDays << 16) | query.IngameDays
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		adminCouponQueueOwnerID,
		adminCouponQueueType,
		query.Amount,
		packedCriteria,
		query.PeriodicDays,
		int(now.Unix()),
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
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ?", queueTable), itemID); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func parseAdminCouponQueueEnd(dayMonth string, hourMinute string, now time.Time) int {
	day, month := 0, 0
	hour, minute := 0, 0
	_, _ = fmt.Sscanf(dayMonth, "%d.%d", &day, &month)
	_, _ = fmt.Sscanf(hourMinute, "%d:%d", &hour, &minute)
	return int(time.Date(now.In(legacyAdminTimeLocation).Year(), time.Month(month), day, hour, minute, 0, 0, legacyAdminTimeLocation).Unix())
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

func randomAdminIntN(maximum int) int {
	if maximum <= 0 {
		return 0
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(maximum)))
	if err != nil {
		return 0
	}
	return int(value.Int64())
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

func (r AdminRepository) loadAdminBotRows(ctx context.Context) ([]domaingame.AdminBotRow, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
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
			"SELECT bots.owner_id, COALESCE(u.oname, ''), COALESCE(p.planet_id, 0), COALESCE(p.name, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM (SELECT owner_id FROM %s WHERE type = ? GROUP BY owner_id) bots LEFT JOIN %s u ON u.player_id = bots.owner_id LEFT JOIN %s p ON p.planet_id = u.hplanetid ORDER BY bots.owner_id ASC",
			queueTable,
			usersTable,
			planetsTable,
		),
		"AI",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminBotRow, 0)
	for rows.Next() {
		var row domaingame.AdminBotRow
		var planetID, galaxy, system, position int
		var planetName string
		if err := rows.Scan(&row.PlayerID, &row.Name, &planetID, &planetName, &galaxy, &system, &position); err != nil {
			return nil, err
		}
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

func (r AdminRepository) MutateAdminBotEdit(ctx context.Context, query appgame.AdminBotEditMutationQuery) (appgame.AdminBotEditMutationResult, error) {
	if r.execer == nil {
		return appgame.AdminBotEditMutationResult{}, errors.New("admin botedit mutation unavailable")
	}
	botstratTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return appgame.AdminBotEditMutationResult{}, err
	}
	switch query.Action {
	case domaingame.AdminActionBotEditLoad:
		source, name, err := r.loadAdminBotStrategy(ctx, botstratTable, query.StrategyID)
		if err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		return appgame.AdminBotEditMutationResult{Source: source, Name: name, SelectedStrategyID: query.StrategyID}, nil
	case domaingame.AdminActionBotEditSave:
		if query.StrategyID > 1 {
			source, _, err := r.loadAdminBotStrategy(ctx, botstratTable, query.StrategyID)
			if err != nil {
				return appgame.AdminBotEditMutationResult{}, err
			}
			if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET source = ? WHERE id = 1", botstratTable), source); err != nil {
				return appgame.AdminBotEditMutationResult{}, err
			}
			if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET source = ? WHERE id = ?", botstratTable), query.Source, query.StrategyID); err != nil {
				return appgame.AdminBotEditMutationResult{}, err
			}
		}
		return appgame.AdminBotEditMutationResult{SelectedStrategyID: query.StrategyID}, nil
	case domaingame.AdminActionBotEditImport:
		if query.StrategyID == 0 {
			return appgame.AdminBotEditMutationResult{ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionFailed)}, nil
		}
		source, name, err := r.loadAdminBotStrategy(ctx, botstratTable, query.StrategyID)
		if err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET source = ? WHERE id = 1", botstratTable), source); err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET source = ? WHERE id = ?", botstratTable), query.Source, query.StrategyID); err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		return appgame.AdminBotEditMutationResult{
			ActionIssue:        domaingame.AdminIssue(domaingame.AdminIssueActionSaved),
			Name:               name,
			SelectedStrategyID: query.StrategyID,
		}, nil
	case domaingame.AdminActionBotEditNew:
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, source) VALUES (?, ?)", botstratTable), query.Name, defaultAdminBotStrategySource()); err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		return appgame.AdminBotEditMutationResult{}, nil
	case domaingame.AdminActionBotEditRename:
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET name = ? WHERE id = ?", botstratTable), query.Name, query.StrategyID); err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		strategies, err := r.loadAdminBotStrategies(ctx)
		if err != nil {
			return appgame.AdminBotEditMutationResult{}, err
		}
		return appgame.AdminBotEditMutationResult{Strategies: strategies, SelectedStrategyID: query.StrategyID}, nil
	default:
		return appgame.AdminBotEditMutationResult{}, nil
	}
}

func (r AdminRepository) loadAdminBotStrategy(ctx context.Context, botstratTable string, strategyID int) (string, string, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(source, ''), COALESCE(name, '') FROM %s WHERE id = ? LIMIT 1", botstratTable), strategyID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", "", err
		}
		return "", "", nil
	}
	var source string
	var name string
	if err := rows.Scan(&source, &name); err != nil {
		return "", "", err
	}
	return source, name, rows.Err()
}

func defaultAdminBotStrategySource() string {
	return "{ \"class\": \"go.GraphLinksModel\",\n" +
		"                             \"linkFromPortIdProperty\": \"fromPort\",\n" +
		"                             \"linkToPortIdProperty\": \"toPort\",\n" +
		"                             \"nodeDataArray\": [ ],\n" +
		"                             \"linkDataArray\": [ ]}"
}

func (r AdminRepository) loadAdminBrowseRows(ctx context.Context) ([]domaingame.AdminBrowseRow, error) {
	browseTable, err := tableName(r.prefix, "browse")
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
			"SELECT b.log_id, COALESCE(b.owner_id, 0), COALESCE(u.oname, ''), COALESCE(b.url, ''), COALESCE(b.method, ''), COALESCE(b.getdata, ''), COALESCE(b.postdata, ''), COALESCE(b.date, 0) FROM %s b LEFT JOIN %s u ON u.player_id = b.owner_id ORDER BY b.date DESC LIMIT 50",
			browseTable,
			usersTable,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminBrowseRow, 0, 50)
	for rows.Next() {
		var row domaingame.AdminBrowseRow
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.URL, &row.Method, &row.GetData, &row.PostData, &row.Date); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminLoginRows(ctx context.Context, name string, userID int, userIDSet bool, ip string) ([]domaingame.AdminLoginRow, error) {
	var result []domaingame.AdminLoginRow
	if name != "" {
		usersTable, err := tableName(r.prefix, "users")
		if err != nil {
			return nil, err
		}
		users, err := r.queryer.QueryContext(ctx, "SELECT player_id FROM "+usersTable+" WHERE oname LIKE ? LIMIT 25", name+"%")
		if err != nil {
			return nil, err
		}
		var userIDs []int
		for users.Next() {
			var id int
			if err := users.Scan(&id); err != nil {
				users.Close()
				return nil, err
			}
			userIDs = append(userIDs, id)
		}
		if err := users.Err(); err != nil {
			users.Close()
			return nil, err
		}
		if err := users.Close(); err != nil {
			return nil, err
		}
		for _, id := range userIDs {
			rows, err := r.queryAdminLoginRows(ctx, "l.user_id = ?", id)
			if err != nil {
				return nil, err
			}
			result = append(result, rows...)
		}
	}
	if userIDSet {
		rows, err := r.queryAdminLoginRows(ctx, "l.user_id = ?", userID)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
	}
	if ip != "" {
		rows, err := r.queryAdminLoginRows(ctx, "l.ip = ?", ip)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
	}
	return result, nil
}

func (r AdminRepository) queryAdminLoginRows(ctx context.Context, where string, args ...any) ([]domaingame.AdminLoginRow, error) {
	iplogsTable, err := tableName(r.prefix, "iplogs")
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
			"SELECT l.log_id, COALESCE(l.user_id, 0), COALESCE(u.oname, ''), COALESCE(l.ip, ''), COALESCE(l.date, 0) FROM %s l LEFT JOIN %s u ON u.player_id = l.user_id WHERE l.reg = 0 AND %s",
			iplogsTable,
			usersTable,
			where,
		),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminLoginRow, 0)
	for rows.Next() {
		var row domaingame.AdminLoginRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.UserName, &row.IP, &row.Date); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r AdminRepository) loadAdminMessageRows(ctx context.Context, rawTable string, includeErrorIDOrder bool, filter string) ([]domaingame.AdminMessageRow, error) {
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
	where := ""
	args := []any{}
	if filter != "" {
		where = " WHERE m.text LIKE ?"
		args = append(args, "%"+filter+"%")
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT m.error_id, COALESCE(m.owner_id, 0), COALESCE(u.oname, ''), COALESCE(m.ip, ''), COALESCE(m.agent, ''), COALESCE(m.text, ''), COALESCE(m.date, 0) FROM %s m LEFT JOIN %s u ON u.player_id = m.owner_id%s ORDER BY %s LIMIT 50",
			messagesTable,
			usersTable,
			where,
			order,
		),
		args...,
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

func (r AdminRepository) loadAdminUserLogGroups(ctx context.Context, search domaingame.AdminUserLogSearch) ([]domaingame.AdminUserLogGroup, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	userLogsTable, err := tableName(r.prefix, "userlogs")
	if err != nil {
		return nil, err
	}
	users, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT player_id, COALESCE(oname, ''), COALESCE(lastclick, 0), COALESCE(vacation, 0), COALESCE(banned, 0), COALESCE(noattack, 0), COALESCE(disable, 0) FROM %s WHERE player_id > 0 ORDER BY player_id",
		usersTable,
	))
	if err != nil {
		return nil, err
	}
	var matched []domaingame.AdminUserLogRow
	for users.Next() {
		var user domaingame.AdminUserLogRow
		var vacation, banned, noattack, disable int
		if err := users.Scan(&user.OwnerID, &user.OwnerName, &user.LastClick, &vacation, &banned, &noattack, &disable); err != nil {
			users.Close()
			return nil, err
		}
		user.Vacation = vacation != 0
		user.Banned = banned != 0
		user.NoAttack = noattack != 0
		user.Disable = disable != 0
		if legacySimilarTextPercent(search.Name, user.OwnerName) > 75 {
			matched = append(matched, user)
		}
	}
	if err := users.Err(); err != nil {
		users.Close()
		return nil, err
	}
	if err := users.Close(); err != nil {
		return nil, err
	}
	since := legacyUserLogSince(search.Since)
	until := since + int64(search.Days)*24*60*60 + int64(search.Hours)*60*60
	groups := make([]domaingame.AdminUserLogGroup, 0, len(matched))
	for _, user := range matched {
		query := fmt.Sprintf("SELECT id, COALESCE(date, 0), COALESCE(type, ''), COALESCE(text, '') FROM %s WHERE owner_id = ? AND (date >= ? AND date <= ?)", userLogsTable)
		args := []any{user.OwnerID, since, until}
		if search.Type != "ALL" {
			query += " AND type = ?"
			args = append(args, search.Type)
		}
		query += " ORDER BY date ASC"
		rows, err := r.queryer.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		group := domaingame.AdminUserLogGroup{User: user, Rows: []domaingame.AdminUserLogRow{}}
		for rows.Next() {
			row := user
			if err := rows.Scan(&row.ID, &row.Date, &row.Type, &row.Text); err != nil {
				rows.Close()
				return nil, err
			}
			group.Rows = append(group.Rows, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func legacyUserLogSince(value string) int64 {
	parsed, err := time.ParseInLocation("2.1.2006", value, time.UTC)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}

func legacySimilarTextPercent(first string, second string) float64 {
	a := []rune(strings.ToLower(first))
	b := []rune(strings.ToLower(second))
	if len(a)+len(b) == 0 {
		return 0
	}
	return 200 * float64(legacySimilarText(a, b)) / float64(len(a)+len(b))
}

func legacySimilarText(first []rune, second []rune) int {
	max, firstPos, secondPos := 0, 0, 0
	for i := range first {
		for j := range second {
			length := 0
			for i+length < len(first) && j+length < len(second) && first[i+length] == second[j+length] {
				length++
			}
			if length > max {
				max, firstPos, secondPos = length, i, j
			}
		}
	}
	if max == 0 {
		return 0
	}
	total := max
	if firstPos > 0 && secondPos > 0 {
		total += legacySimilarText(first[:firstPos], second[:secondPos])
	}
	if firstPos+max < len(first) && secondPos+max < len(second) {
		total += legacySimilarText(first[firstPos+max:], second[secondPos+max:])
	}
	return total
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

func (r AdminRepository) loadAdminUserDetail(ctx context.Context, targetID int) (*domaingame.AdminUserDetail, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	researchIDs := domaingame.ResearchIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(u.pemail, ''), COALESCE(u.email, ''), COALESCE(u.joindate, 0), COALESCE(u.disable_until, 0), COALESCE(u.vacation_until, 0), COALESCE(u.banned_until, 0), COALESCE(u.noattack_until, 0), COALESCE(u.lastlogin, 0), COALESCE(u.ip_addr, ''), COALESCE(u.validated, 0), COALESCE(u.admin, 0), COALESCE(u.sniff, 0), COALESCE(u.debug, 0), COALESCE(u.sortby, 0), COALESCE(u.sortorder, 0), COALESCE(u.skin, ''), COALESCE(u.useskin, 0), COALESCE(u.deact_ip, 0), COALESCE(u.maxspy, 0), COALESCE(u.maxfleetmsg, 0), COALESCE(u.oldscore1, 0), COALESCE(u.oldplace1, 0), COALESCE(u.oldscore2, 0), COALESCE(u.oldplace2, 0), COALESCE(u.oldscore3, 0), COALESCE(u.oldplace3, 0), COALESCE(u.score1, 0), COALESCE(u.place1, 0), COALESCE(u.score2, 0), COALESCE(u.place2, 0), COALESCE(u.score3, 0), COALESCE(u.place3, 0), COALESCE(u.scoredate, 0), COALESCE(u.dmfree, 0), COALESCE(u.dm, 0), COALESCE(a.tag, ''), COALESCE(a.name, ''), COALESCE(h.planet_id, 0), COALESCE(h.name, ''), COALESCE(h.g, 0), COALESCE(h.s, 0), COALESCE(h.p, 0), COALESCE(ap.planet_id, 0), COALESCE(ap.name, ''), COALESCE(ap.g, 0), COALESCE(ap.s, 0), COALESCE(ap.p, 0), %s FROM %s u LEFT JOIN %s a ON a.ally_id = u.ally_id LEFT JOIN %s h ON h.planet_id = u.hplanetid LEFT JOIN %s ap ON ap.planet_id = u.aktplanet WHERE u.player_id = ? LIMIT 1",
			adminNumericColumns("u", researchIDs),
			usersTable,
			allyTable,
			planetsTable,
			planetsTable,
		),
		targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var detail domaingame.AdminUserDetail
	var vacation, banned, noattack, disable, validated, adminLevel, sniff, debug, useSkin, deactivateIP int
	var allyTag, allyName string
	var homeID, homeGalaxy, homeSystem, homePosition int
	var homeName string
	var activeID, activeGalaxy, activeSystem, activePosition int
	var activeName string
	researchValues := make([]int, len(researchIDs))
	dest := []any{
		&detail.PlayerID,
		&detail.Name,
		&detail.RegDate,
		&detail.LastClick,
		&vacation,
		&banned,
		&noattack,
		&disable,
		&detail.PermanentEmail,
		&detail.Email,
		&detail.JoinDate,
		&detail.DisableUntil,
		&detail.VacationUntil,
		&detail.BannedUntil,
		&detail.NoAttackUntil,
		&detail.LastLogin,
		&detail.IPAddress,
		&validated,
		&adminLevel,
		&sniff,
		&debug,
		&detail.SortBy,
		&detail.SortOrder,
		&detail.Skin,
		&useSkin,
		&deactivateIP,
		&detail.MaxSpy,
		&detail.MaxFleetMsg,
		&detail.OldScore1,
		&detail.OldPlace1,
		&detail.OldScore2,
		&detail.OldPlace2,
		&detail.OldScore3,
		&detail.OldPlace3,
		&detail.Score1,
		&detail.Place1,
		&detail.Score2,
		&detail.Place2,
		&detail.Score3,
		&detail.Place3,
		&detail.ScoreDate,
		&detail.DarkMatterFree,
		&detail.DarkMatter,
		&allyTag,
		&allyName,
		&homeID,
		&homeName,
		&homeGalaxy,
		&homeSystem,
		&homePosition,
		&activeID,
		&activeName,
		&activeGalaxy,
		&activeSystem,
		&activePosition,
	}
	dest = appendIntDest(dest, researchValues)
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	detail.Vacation = vacation != 0
	detail.Banned = banned != 0
	detail.NoAttack = noattack != 0
	detail.Disable = disable != 0
	detail.Validated = validated != 0
	detail.AdminLevel = adminLevel
	detail.Sniff = sniff != 0
	detail.Debug = debug != 0
	detail.UseSkin = useSkin != 0
	detail.DeactivateIP = deactivateIP != 0
	if allyTag != "" || allyName != "" {
		detail.Alliance = fmt.Sprintf("[%s] %s", allyTag, allyName)
	}
	if homeID != 0 {
		detail.HomePlanet = &domaingame.AdminUserPlanet{ID: homeID, Name: homeName, Coordinates: domaingame.Coordinates{Galaxy: homeGalaxy, System: homeSystem, Position: homePosition}}
	}
	if activeID != 0 {
		detail.ActivePlanet = &domaingame.AdminUserPlanet{ID: activeID, Name: activeName, Coordinates: domaingame.Coordinates{Galaxy: activeGalaxy, System: activeSystem, Position: activePosition}}
	}
	detail.Research = make(domaingame.ResearchLevels, len(researchIDs))
	for index, id := range researchIDs {
		detail.Research[id] = researchValues[index]
	}
	detail.Planets, err = r.loadAdminUserPlanetRows(ctx, planetsTable, targetID)
	if err != nil {
		return nil, err
	}
	return &detail, rows.Err()
}

func (r AdminRepository) loadAdminUserPlanetRows(ctx context.Context, planetsTable string, targetID int) ([]domaingame.AdminPlanetRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, COALESCE(name, ''), COALESCE(date, 0), COALESCE(g, 0), COALESCE(s, 0), COALESCE(p, 0) FROM %s WHERE owner_id = ? ORDER BY g ASC, s ASC, p ASC, type DESC", planetsTable),
		targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminPlanetRow, 0)
	for rows.Next() {
		var row domaingame.AdminPlanetRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Date, &row.Coordinates.Galaxy, &row.Coordinates.System, &row.Coordinates.Position); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func adminNumericColumns(alias string, ids []int) string {
	columns := make([]string, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, fmt.Sprintf("%s.`%d`", alias, id))
	}
	return strings.Join(columns, ", ")
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

func (r AdminRepository) loadAdminPlanetDetail(ctx context.Context, planetID int) (*domaingame.AdminPlanetDetail, error) {
	planetsTable, err := tableName(r.prefix, "planets")
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
	buildingIDs := domaingame.BuildingIDs()
	fleetIDs := domaingame.FleetIDs()
	defenseIDs := domaingame.DefenseIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT p.planet_id, COALESCE(p.name, ''), COALESCE(p.date, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), COALESCE(u.player_id, 0), COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0), COALESCE(p.type, 1), COALESCE(p.diameter, 0), COALESCE(p.temp, 0), COALESCE(p.fields, 0), COALESCE(p.maxfields, 0), COALESCE(p.`remove`, 0), COALESCE(p.lastakt, 0), COALESCE(p.lastpeek, 0), COALESCE(p.gate_until, 0), COALESCE(p.`%d`, 0), COALESCE(p.`%d`, 0), COALESCE(p.`%d`, 0), COALESCE(p.prod1, 0), COALESCE(p.prod2, 0), COALESCE(p.prod3, 0), COALESCE(p.prod4, 0), COALESCE(p.prod12, 0), COALESCE(p.prod212, 0), %s, %s, %s FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id WHERE p.planet_id = ? LIMIT 1",
			domaingame.ResourceMetal,
			domaingame.ResourceCrystal,
			domaingame.ResourceDeuterium,
			adminNumericColumns("p", buildingIDs),
			adminNumericColumns("p", fleetIDs),
			adminNumericColumns("p", defenseIDs),
			planetsTable,
			usersTable,
		),
		planetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var detail domaingame.AdminPlanetDetail
	var owner domaingame.AdminUserRow
	var vacation, banned, noattack, disable int
	var prod1, prod2, prod3, prod4, prod12, prod212 float64
	buildingValues := make([]int, len(buildingIDs))
	fleetValues := make([]int, len(fleetIDs))
	defenseValues := make([]int, len(defenseIDs))
	dest := []any{
		&detail.ID,
		&detail.Name,
		&detail.Date,
		&detail.Coordinates.Galaxy,
		&detail.Coordinates.System,
		&detail.Coordinates.Position,
		&owner.PlayerID,
		&owner.Name,
		&owner.RegDate,
		&owner.LastClick,
		&vacation,
		&banned,
		&noattack,
		&disable,
		&detail.Type,
		&detail.Diameter,
		&detail.Temperature,
		&detail.Fields,
		&detail.MaxFields,
		&detail.RemoveDate,
		&detail.LastActivity,
		&detail.LastUpdate,
		&detail.GateUntil,
		&detail.Resources.Metal,
		&detail.Resources.Crystal,
		&detail.Resources.Deuterium,
		&prod1,
		&prod2,
		&prod3,
		&prod4,
		&prod12,
		&prod212,
	}
	dest = appendIntDest(dest, buildingValues)
	dest = appendIntDest(dest, fleetValues)
	dest = appendIntDest(dest, defenseValues)
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	if owner.PlayerID != 0 {
		owner.Vacation = vacation != 0
		owner.Banned = banned != 0
		owner.NoAttack = noattack != 0
		owner.Disable = disable != 0
		detail.Owner = &owner
	}
	buildings := make(domaingame.BuildingLevels, len(buildingIDs))
	for index, id := range buildingIDs {
		buildings[id] = buildingValues[index]
	}
	fleet := make(domaingame.FleetCounts, len(fleetIDs))
	for index, id := range fleetIDs {
		fleet[id] = fleetValues[index]
	}
	defense := make(domaingame.DefenseCounts, len(defenseIDs))
	for index, id := range defenseIDs {
		defense[id] = defenseValues[index]
	}
	percents := map[int]int{
		domaingame.BuildingMetalMine:      adminProductionPercent(prod1),
		domaingame.BuildingCrystalMine:    adminProductionPercent(prod2),
		domaingame.BuildingDeuteriumSynth: adminProductionPercent(prod3),
		domaingame.BuildingSolarPlant:     adminProductionPercent(prod4),
		domaingame.BuildingFusionReactor:  adminProductionPercent(prod12),
		domaingame.FleetSolarSatellite:    adminProductionPercent(prod212),
	}
	detail.Score = domaingame.CalculatePlanetScore(buildings, fleet, defense)
	detail.Buildings = adminTechnologyValues(buildingIDs, buildingValues, percents)
	detail.Fleet = adminTechnologyValues(fleetIDs, fleetValues, percents)
	detail.Defense = adminTechnologyValues(defenseIDs, defenseValues, nil)
	production := domaingame.BuildResourceProduction(
		domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{
			ID:          detail.ID,
			Name:        detail.Name,
			Type:        detail.Type,
			Coordinates: detail.Coordinates,
			Fields:      detail.Fields,
			MaxFields:   detail.MaxFields,
			Resources:   detail.Resources,
		}},
		domaingame.ResourceProductionInputs{
			Levels:            buildings,
			SolarSatellites:   fleet[domaingame.FleetSolarSatellite],
			ProductionFactors: adminProductionFactors(percents),
			UniverseSpeed:     1,
		},
	)
	detail.EnergyBalance = int(production.Totals.Hour.EnergyRaw)
	detail.EnergyCapacity = int(production.Totals.Hour.Energy)
	detail.ProductionFactor = production.Factor
	detail.Moon, err = r.loadAdminRelatedPlanet(ctx, planetsTable, detail.Coordinates, domaingame.PlanetTypeMoon)
	if err != nil {
		return nil, err
	}
	detail.Debris, err = r.loadAdminRelatedPlanet(ctx, planetsTable, detail.Coordinates, domaingame.PlanetTypeDebris)
	if err != nil {
		return nil, err
	}
	if detail.Type != domaingame.PlanetTypePlanet {
		detail.Parent, err = r.loadAdminParentPlanet(ctx, planetsTable, detail.Coordinates)
		if err != nil {
			return nil, err
		}
	}
	detail.BuildQueue, err = (BuildingsRepository{queryer: r.queryer}).loadBuildingQueueEntries(ctx, buildQueueTable, planetID, int(r.now().Unix()))
	if err != nil {
		return nil, err
	}
	return &detail, rows.Err()
}

func (r AdminRepository) loadAdminParentPlanet(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates) (*domaingame.AdminPlanetRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, COALESCE(name, ''), COALESCE(date, 0), COALESCE(g, 0), COALESCE(s, 0), COALESCE(p, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0) FROM %s WHERE g = ? AND s = ? AND p = ? AND type IN (?, ?, ?) LIMIT 1",
			resourceMetal, resourceCrystal, resourceDeuterium, planetsTable,
		),
		coordinates.Galaxy, coordinates.System, coordinates.Position,
		domaingame.PlanetTypePlanet, domaingame.PlanetTypeDestroyedPlanet, domaingame.PlanetTypeAbandoned,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var row domaingame.AdminPlanetRow
	if err := rows.Scan(&row.ID, &row.Name, &row.Date, &row.Coordinates.Galaxy, &row.Coordinates.System, &row.Coordinates.Position, &row.Resources.Metal, &row.Resources.Crystal, &row.Resources.Deuterium); err != nil {
		return nil, err
	}
	return &row, rows.Err()
}

func (r AdminRepository) loadAdminRelatedPlanet(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, planetType int) (*domaingame.AdminPlanetRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, COALESCE(name, ''), COALESCE(date, 0), COALESCE(g, 0), COALESCE(s, 0), COALESCE(p, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0) FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1",
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
			planetsTable,
		),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		planetType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var row domaingame.AdminPlanetRow
	if err := rows.Scan(
		&row.ID,
		&row.Name,
		&row.Date,
		&row.Coordinates.Galaxy,
		&row.Coordinates.System,
		&row.Coordinates.Position,
		&row.Resources.Metal,
		&row.Resources.Crystal,
		&row.Resources.Deuterium,
	); err != nil {
		return nil, err
	}
	return &row, rows.Err()
}

func adminTechnologyValues(ids []int, values []int, percents map[int]int) []domaingame.AdminTechnologyValue {
	result := make([]domaingame.AdminTechnologyValue, 0, len(ids))
	for index, id := range ids {
		result = append(result, domaingame.AdminTechnologyValue{
			ID:      id,
			Name:    domaingame.TechnologyName(id),
			Value:   values[index],
			Percent: percents[id],
		})
	}
	return result
}

func adminProductionPercent(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(value*100 + 0.5)
}

func adminProductionFactors(percents map[int]int) domaingame.ProductionFactors {
	factors := make(domaingame.ProductionFactors, len(percents))
	for id, percent := range percents {
		factors[id] = float64(percent) / 100
	}
	return factors
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

var adminColonySettingsColumns = []string{
	"t1_a", "t1_b", "t1_c",
	"t2_a", "t2_b", "t2_c",
	"t3_a", "t3_b", "t3_c",
	"t4_a", "t4_b", "t4_c",
	"t5_a", "t5_b", "t5_c",
}

func (r AdminRepository) loadAdminColonySettings(ctx context.Context) (map[string]int, error) {
	colonyTable, err := tableName(r.prefix, "coltab")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s LIMIT 1", numericColumnsByName(adminColonySettingsColumns), colonyTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("admin colony settings not found")
	}
	values := make([]int, len(adminColonySettingsColumns))
	dest := make([]any, len(values))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	result := make(map[string]int, len(adminColonySettingsColumns))
	for index, column := range adminColonySettingsColumns {
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
	botStrategyTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.task_id, COALESCE(q.owner_id, 0), COALESCE(u.oname, ''), COALESCE(q.type, ''), COALESCE(q.sub_id, 0), COALESCE(q.obj_id, 0), COALESCE(q.level, 0), COALESCE(q.start, 0), COALESCE(q.end, 0), COALESCE(q.prio, 0), COALESCE(q.freeze, 0), COALESCE(q.frozen, 0), COALESCE(p.name, ''), COALESCE(bs.name, ''), COALESCE(bs.source, '') FROM %s q LEFT JOIN %s u ON u.player_id = q.owner_id LEFT JOIN %s bq ON bq.id = q.sub_id LEFT JOIN %s p ON p.planet_id = CASE WHEN q.type IN (?, ?) THEN bq.planet_id WHEN q.type IN (?, ?) THEN q.sub_id ELSE NULL END LEFT JOIN %s bs ON q.type = ? AND bs.id = q.sub_id WHERE q.type <> ? ORDER BY q.end ASC, q.prio DESC, q.task_id ASC LIMIT 50",
			queueTable,
			usersTable,
			buildQueueTable,
			planetsTable,
			botStrategyTable,
		),
		queueTypeBuild,
		queueTypeDemolish,
		queueTypeShipyard,
		queueTypeResearch,
		queueTypeAI,
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
		var planetName, botStrategyName, botStrategySource string
		if err := rows.Scan(&row.ID, &row.OwnerID, &row.OwnerName, &row.Type, &subID, &objID, &level, &row.Start, &row.End, &row.Priority, &freeze, &row.Frozen, &planetName, &botStrategyName, &botStrategySource); err != nil {
			return nil, err
		}
		row.Freeze = freeze != 0
		row.Description = legacyAdminQueueDescription(row.Type, subID, objID, level, planetName)
		if row.Type == queueTypeAI {
			row.Description = legacyAdminQueueAIDescription(subID, objID, botStrategyName, botStrategySource)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func legacyAdminQueueAIDescription(strategyID int, blockID int, strategyName string, source string) string {
	if strategyName == "" {
		return fmt.Sprintf("Bot Task (Strategy #%d)", strategyID)
	}
	blockText := ""
	if graph, err := parseBotStrategyGraph(source); err == nil {
		for _, node := range graph.Nodes {
			if node.Key == blockID {
				blockText = node.Text
				break
			}
		}
	}
	return fmt.Sprintf("Bot Task (Strategy %s) : <br>%s", strategyName, blockText)
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
	case queueTypeAI:
		return fmt.Sprintf("Bot Task (Strategy #%d)", subID)
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

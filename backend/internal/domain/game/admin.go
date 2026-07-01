package game

import "strings"

const (
	AdminLevelPlayer   = 0
	AdminLevelOperator = 1
	AdminLevelAdmin    = 2

	AdminIssueAccessDenied = "access_denied"
	AdminIssueActionSaved  = "action_saved"
)

const (
	AdminActionQueueEnd      = "queue_end"
	AdminActionQueueRemove   = "queue_remove"
	AdminActionQueueFreeze   = "queue_freeze"
	AdminActionQueueUnfreeze = "queue_unfreeze"

	AdminActionFleetlogsTwoMinutes = "fleetlogs_2min"
	AdminActionFleetlogsEnd        = "fleetlogs_end"
	AdminActionFleetlogsReturn     = "fleetlogs_return"

	AdminActionBroadcastSend = "broadcast_send"
	AdminActionReportsDelete = "reports_delete"

	AdminActionBattleSimRun  = "battle_sim"
	AdminActionRakSimRun     = "rak_sim"
	AdminActionExpeditionSim = "sim"

	AdminActionSettings = "settings"

	AdminActionDatabaseCreate  = "create"
	AdminActionDatabaseRestore = "restore"
	AdminActionDatabaseDelete  = "delete"

	AdminActionUsersRecalcStats  = "recalc_stats"
	AdminActionUsersUpdate       = "update"
	AdminActionUsersCreatePlanet = "create_planet"

	AdminActionCouponAddOne     = "add_one"
	AdminActionCouponRemoveOne  = "remove_one"
	AdminActionCouponAddDate    = "add_date"
	AdminActionCouponRemoveDate = "remove_date"

	AdminActionBotEditLoad   = "load"
	AdminActionBotEditSave   = "save"
	AdminActionBotEditNew    = "new"
	AdminActionBotEditRename = "rename"
)

type Admin struct {
	Commander       string
	CurrentPlanet   PlanetOverview
	PlanetSwitcher  []PlanetSummary
	Viewer          AdminViewer
	Mode            string
	Menu            []AdminMenuItem
	MessageRows     []AdminMessageRow
	UserLogRows     []AdminUserLogRow
	UserRows        []AdminUserRow
	ActiveUsers     []AdminUserRow
	SelectedUser    *AdminUserDetail
	PlanetRows      []AdminPlanetRow
	SelectedPlanet  *AdminPlanetDetail
	ReportRows      []AdminReportRow
	Universe        *AdminUniverseSettings
	Expedition      map[string]int
	FleetLogRows    []AdminFleetLogRow
	QueueRows       []AdminQueueRow
	BattleReports   []AdminBattleReportRow
	ChecksumGroups  []AdminChecksumGroup
	DatabaseBackups []AdminDatabaseBackup
	BotStrategies   []AdminBotStrategy
	CouponRows      []AdminCouponRow
	CouponQueueRows []AdminCouponQueueRow
}

type AdminViewer struct {
	PlayerID int
	Name     string
	Level    int
}

type AdminMenuItem struct {
	Mode  string
	Label string
	Image string
}

type AdminActionIssue struct {
	Code    string
	Message string
}

type AdminMessageRow struct {
	ID        int
	OwnerID   int
	OwnerName string
	IP        string
	Agent     string
	Text      string
	Date      int64
}

type AdminUserLogRow struct {
	ID        int
	OwnerID   int
	OwnerName string
	LastClick int64
	Vacation  bool
	Banned    bool
	NoAttack  bool
	Disable   bool
	Type      string
	Text      string
	Date      int64
}

type AdminUserRow struct {
	PlayerID   int
	Name       string
	RegDate    int64
	LastClick  int64
	Vacation   bool
	Banned     bool
	NoAttack   bool
	Disable    bool
	HomePlanet *AdminUserPlanet
}

type AdminUserPlanet struct {
	ID          int
	Name        string
	Coordinates Coordinates
}

type AdminUserDetail struct {
	AdminUserRow
	PermanentEmail string
	Email          string
	Alliance       string
	JoinDate       int64
	DisableUntil   int64
	VacationUntil  int64
	BannedUntil    int64
	NoAttackUntil  int64
	LastLogin      int64
	IPAddress      string
	Validated      bool
	AdminLevel     int
	Sniff          bool
	Debug          bool
	SortBy         int
	SortOrder      int
	Skin           string
	UseSkin        bool
	DeactivateIP   bool
	MaxSpy         int
	MaxFleetMsg    int
	OldScore1      int64
	OldPlace1      int
	OldScore2      int64
	OldPlace2      int
	OldScore3      int64
	OldPlace3      int
	Score1         int64
	Place1         int
	Score2         int64
	Place2         int
	Score3         int64
	Place3         int
	ScoreDate      int64
	DarkMatterFree int
	DarkMatter     int
	Research       ResearchLevels
	ActivePlanet   *AdminUserPlanet
	Planets        []AdminPlanetRow
}

type AdminPlanetRow struct {
	ID          int
	Name        string
	Date        int64
	Coordinates Coordinates
	Owner       *AdminUserRow
}

type AdminTechnologyValue struct {
	ID      int
	Name    string
	Value   int
	Percent int
}

type AdminPlanetDetail struct {
	AdminPlanetRow
	Type             int
	Diameter         int
	Temperature      int
	Fields           int
	MaxFields        int
	RemoveDate       int64
	LastActivity     int64
	LastUpdate       int64
	GateUntil        int64
	Score            PlanetScore
	Resources        Resources
	EnergyBalance    int
	EnergyCapacity   int
	ProductionFactor float64
	Buildings        []AdminTechnologyValue
	Fleet            []AdminTechnologyValue
	Defense          []AdminTechnologyValue
	BuildQueue       []BuildingQueueEntry
	Moon             *AdminPlanetRow
	Debris           *AdminPlanetRow
}

type AdminReportRow struct {
	ID        int
	OwnerID   int
	OwnerName string
	MessageID int
	From      string
	Subject   string
	Text      string
	Date      int64
}

type AdminFleetLogRow struct {
	TaskID     int
	Number     int
	Mission    int
	Start      int64
	End        int64
	FlightTime int
	Fuel       int
	UnionID    int
	Origin     AdminFleetLogPlanet
	Target     AdminFleetLogPlanet
	Ships      []FleetShipCount
	Cargo      []FleetResourceLoad
}

type AdminFleetLogPlanet struct {
	ID          int
	Name        string
	OwnerID     int
	OwnerName   string
	Coordinates Coordinates
	Type        int
}

type AdminUniverseSettings struct {
	Number          int
	Speed           float64
	FleetSpeed      float64
	Galaxies        int
	Systems         int
	MaxUsers        int
	ACS             int
	FleetDebris     int
	DefenseDebris   int
	RapidFire       bool
	Moons           bool
	DefenseRepair   int
	DefenseDelta    int
	UserCount       int
	Freeze          bool
	News1           string
	News2           string
	NewsUntil       int64
	StartDate       int64
	BattleEngine    string
	Language        string
	Hacks           int
	ExtBoard        string
	ExtDiscord      string
	ExtTutorial     string
	ExtRules        string
	ExtImpressum    string
	PHPBattle       bool
	BattleMax       int
	ForceLanguage   bool
	StartDarkMatter int
	MaxShipyard     int
	FeedAge         int
}

type AdminQueueRow struct {
	ID          int
	OwnerID     int
	OwnerName   string
	Type        string
	Description string
	Priority    int
	Start       int64
	End         int64
	Freeze      bool
	Frozen      int64
}

type AdminBattleReportRow struct {
	ID    int
	Date  int64
	Title string
}

type AdminChecksumGroup struct {
	Title string
	Rows  []AdminChecksumRow
}

type AdminChecksumRow struct {
	Path     string
	Checksum string
	Status   string
}

type AdminDatabaseBackup struct {
	FileName string
}

type AdminBotStrategy struct {
	ID   int
	Name string
}

type AdminCouponRow struct {
	ID           int
	Code         string
	Amount       int
	Used         bool
	UserUniverse int
	UserID       int
	UserName     string
}

type AdminCouponQueueRow struct {
	ID           int
	Amount       int
	InactiveDays int
	IngameDays   int
	PeriodicDays int
	Start        int64
	End          int64
	Priority     int
}

var adminMenuItems = []AdminMenuItem{
	{Mode: "Fleetlogs", Label: "Fleet Logs", Image: "/public-assets/game-img/admin_fleetlogs.png"},
	{Mode: "Browse", Label: "Browse History", Image: "/public-assets/game-img/admin_browse.png"},
	{Mode: "Reports", Label: "Reports", Image: "/public-assets/game-img/admin_report.png"},
	{Mode: "Bans", Label: "Bans", Image: "/public-assets/game-img/admin_ban.png"},
	{Mode: "Users", Label: "Users", Image: "/public-assets/game-img/admin_users.png"},
	{Mode: "Planets", Label: "Planets", Image: "/public-assets/game-img/admin_planets.png"},
	{Mode: "Queue", Label: "Queue", Image: "/public-assets/game-img/admin_queue.png"},
	{Mode: "Uni", Label: "Universe Settings", Image: "/public-assets/game-img/admin_uni.png"},
	{Mode: "Errors", Label: "Errors", Image: "/public-assets/game-img/admin_error.png"},
	{Mode: "Debug", Label: "Debug", Image: "/public-assets/game-img/admin_debug.png"},
	{Mode: "BattleSim", Label: "Battlesim", Image: "/public-assets/game-img/admin_sim.png"},
	{Mode: "Broadcast", Label: "Broadcast", Image: "/public-assets/game-img/admin_broadcast.png"},
	{Mode: "Expedition", Label: "Expedition Settings", Image: "/public-assets/evolution/gebaeude/210.gif"},
	{Mode: "Logins", Label: "Logins", Image: "/public-assets/game-img/admin_logins.png"},
	{Mode: "Checksum", Label: "Source Check", Image: "/public-assets/game-img/admin_checksum.png"},
	{Mode: "Bots", Label: "Bot Controls", Image: "/public-assets/game-img/admin_bots.png"},
	{Mode: "BattleReport", Label: "Battle Reports", Image: "/public-assets/game-img/admin_battle.png"},
	{Mode: "UserLogs", Label: "User Logs", Image: "/public-assets/game-img/admin_userlogs.png"},
	{Mode: "BotEdit", Label: "Botstrat Editor", Image: "/public-assets/game-img/admin_botedit.png"},
	{Mode: "Coupons", Label: "Coupons", Image: "/public-assets/game-img/admin_coupons.png"},
	{Mode: "RakSim", Label: "Rocket Attack Sim", Image: "/public-assets/game-img/admin_raksim.png"},
	{Mode: "DB", Label: "DB Integrity", Image: "/public-assets/game-img/admin_db.png"},
	{Mode: "ColonySettings", Label: "Colonization Settings", Image: "/public-assets/game-img/admin_colony_settings.png"},
	{Mode: "Loca", Label: "Localization", Image: "/public-assets/game-img/admin_loca.png"},
	{Mode: "Mods", Label: "Modifications", Image: "/public-assets/game-img/admin_mods.png"},
}

func NewAdmin(overview Overview, viewer AdminViewer, mode string) Admin {
	return Admin{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Viewer:         viewer,
		Mode:           NormalizeAdminMode(mode),
		Menu:           AdminMenu(),
	}
}

func AdminMenu() []AdminMenuItem {
	items := make([]AdminMenuItem, len(adminMenuItems))
	copy(items, adminMenuItems)
	return items
}

func NormalizeAdminMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" || mode == "Home" {
		return "Home"
	}
	for _, item := range adminMenuItems {
		if item.Mode == mode {
			return mode
		}
	}
	return "Home"
}

func (a Admin) CanAccess() bool {
	return a.Viewer.Level > AdminLevelPlayer
}

func (a Admin) CanAccessMode() bool {
	if !a.CanAccess() {
		return false
	}
	if a.Viewer.Level >= AdminLevelAdmin {
		return true
	}
	return !AdminModeRequiresAdmin(a.Mode)
}

func (a Admin) CanMutate(action string) bool {
	if !a.CanAccessMode() {
		return false
	}
	if a.Viewer.Level >= AdminLevelAdmin {
		return true
	}
	return !AdminMutationRequiresAdmin(a.Mode, action)
}

func AdminModeRequiresAdmin(mode string) bool {
	switch NormalizeAdminMode(mode) {
	case "Bots", "BotEdit":
		return true
	default:
		return false
	}
}

func AdminMutationRequiresAdmin(mode string, action string) bool {
	switch NormalizeAdminMode(mode) {
	case "Bans", "Reports", "Broadcast", "BattleSim", "RakSim", "UserLogs":
		return false
	case "Expedition":
		return action == "settings"
	case "Queue", "Uni", "Coupons", "Planets", "Users", "Debug", "Errors", "Bots", "BotEdit", "DB", "ColonySettings", "Loca", "Mods":
		return true
	default:
		return true
	}
}

func AdminIssue(code string) *AdminActionIssue {
	switch code {
	case AdminIssueAccessDenied:
		return &AdminActionIssue{Code: code, Message: "Access denied."}
	case AdminIssueActionSaved:
		return &AdminActionIssue{Code: code, Message: "Action saved."}
	default:
		return &AdminActionIssue{Code: code, Message: "Admin action could not be completed."}
	}
}

func AdminIssueWithMessage(code string, message string) *AdminActionIssue {
	if strings.TrimSpace(message) == "" {
		return AdminIssue(code)
	}
	return &AdminActionIssue{Code: code, Message: message}
}

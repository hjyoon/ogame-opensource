package game

import "strings"

const (
	AdminLevelPlayer   = 0
	AdminLevelOperator = 1
	AdminLevelAdmin    = 2

	AdminIssueAccessDenied = "access_denied"
)

type Admin struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Viewer         AdminViewer
	Mode           string
	Menu           []AdminMenuItem
	MessageRows    []AdminMessageRow
	UserLogRows    []AdminUserLogRow
	UserRows       []AdminUserRow
	ActiveUsers    []AdminUserRow
	PlanetRows     []AdminPlanetRow
	QueueRows      []AdminQueueRow
	BattleReports  []AdminBattleReportRow
	ChecksumGroups []AdminChecksumGroup
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

type AdminPlanetRow struct {
	ID          int
	Name        string
	Date        int64
	Coordinates Coordinates
	Owner       *AdminUserRow
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

func AdminIssue(code string) *AdminActionIssue {
	switch code {
	case AdminIssueAccessDenied:
		return &AdminActionIssue{Code: code, Message: "Access denied."}
	default:
		return &AdminActionIssue{Code: code, Message: "Admin action could not be completed."}
	}
}

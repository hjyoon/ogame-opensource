package httpdelivery

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameAdminResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Admin         *gameAdminSummary          `json:"admin,omitempty"`
	ActionIssue   *gameAdminActionIssue      `json:"actionIssue,omitempty"`
}

type gameAdminActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameAdminMutationRequest struct {
	Action       string         `json:"action"`
	TaskID       int            `json:"taskId"`
	TargetIDs    []int          `json:"targetIds"`
	BanMode      int            `json:"banMode"`
	Days         int            `json:"days"`
	Hours        int            `json:"hours"`
	Reason       string         `json:"reason"`
	Values       map[string]int `json:"values"`
	Category     int            `json:"category"`
	Subject      string         `json:"subject"`
	Text         string         `json:"text"`
	ReportIDs    []int          `json:"reportIds"`
	DeleteMode   string         `json:"deleteMode"`
	FileName     string         `json:"fileName"`
	Amount       int            `json:"amount"`
	ItemID       int            `json:"itemId"`
	DayMonth     string         `json:"dayMonth"`
	HourMinute   string         `json:"hourMinute"`
	InactiveDays int            `json:"inactiveDays"`
	IngameDays   int            `json:"ingameDays"`
	PeriodicDays int            `json:"periodicDays"`
}

type gameAdminSummary struct {
	Commander       string                      `json:"commander"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Viewer          gameAdminViewer             `json:"viewer"`
	Mode            string                      `json:"mode"`
	Menu            []gameAdminMenuItem         `json:"menu"`
	MessageRows     []gameAdminMessageRow       `json:"messageRows,omitempty"`
	UserLogRows     []gameAdminUserLogRow       `json:"userLogRows,omitempty"`
	UserRows        []gameAdminUserRow          `json:"userRows,omitempty"`
	ActiveUsers     []gameAdminUserRow          `json:"activeUsers,omitempty"`
	SelectedUser    *gameAdminUserDetail        `json:"selectedUser,omitempty"`
	PlanetRows      []gameAdminPlanetRow        `json:"planetRows,omitempty"`
	SelectedPlanet  *gameAdminPlanetDetail      `json:"selectedPlanet,omitempty"`
	ReportRows      []gameAdminReportRow        `json:"reportRows,omitempty"`
	Universe        *gameAdminUniverseSettings  `json:"universe,omitempty"`
	Expedition      map[string]int              `json:"expedition,omitempty"`
	FleetLogRows    []gameAdminFleetLogRow      `json:"fleetLogRows,omitempty"`
	QueueRows       []gameAdminQueueRow         `json:"queueRows,omitempty"`
	BattleReports   []gameAdminBattleReportRow  `json:"battleReports,omitempty"`
	ChecksumGroups  []gameAdminChecksumGroup    `json:"checksumGroups,omitempty"`
	DatabaseBackups []gameAdminDatabaseBackup   `json:"databaseBackups,omitempty"`
	BotStrategies   []gameAdminBotStrategy      `json:"botStrategies,omitempty"`
	CouponRows      []gameAdminCouponRow        `json:"couponRows,omitempty"`
	CouponQueueRows []gameAdminCouponQueueRow   `json:"couponQueueRows,omitempty"`
}

type gameAdminViewer struct {
	PlayerID int    `json:"playerId"`
	Name     string `json:"name"`
	Level    int    `json:"level"`
}

type gameAdminMenuItem struct {
	Mode  string `json:"mode"`
	Label string `json:"label"`
	Image string `json:"image"`
}

type gameAdminMessageRow struct {
	ID        int    `json:"id"`
	OwnerID   int    `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	IP        string `json:"ip"`
	Agent     string `json:"agent"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type gameAdminUserLogRow struct {
	ID        int    `json:"id"`
	OwnerID   int    `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	LastClick int64  `json:"lastClick"`
	Vacation  bool   `json:"vacation"`
	Banned    bool   `json:"banned"`
	NoAttack  bool   `json:"noAttack"`
	Disable   bool   `json:"disable"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type gameAdminUserRow struct {
	PlayerID   int                  `json:"playerId"`
	Name       string               `json:"name"`
	RegDate    int64                `json:"regDate"`
	LastClick  int64                `json:"lastClick"`
	Vacation   bool                 `json:"vacation"`
	Banned     bool                 `json:"banned"`
	NoAttack   bool                 `json:"noAttack"`
	Disable    bool                 `json:"disable"`
	HomePlanet *gameAdminUserPlanet `json:"homePlanet,omitempty"`
}

type gameAdminUserPlanet struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
}

type gameAdminUserDetail struct {
	gameAdminUserRow
	PermanentEmail string               `json:"permanentEmail"`
	Email          string               `json:"email"`
	Alliance       string               `json:"alliance"`
	JoinDate       int64                `json:"joinDate"`
	DisableUntil   int64                `json:"disableUntil"`
	VacationUntil  int64                `json:"vacationUntil"`
	BannedUntil    int64                `json:"bannedUntil"`
	NoAttackUntil  int64                `json:"noAttackUntil"`
	LastLogin      int64                `json:"lastLogin"`
	IPAddress      string               `json:"ipAddress"`
	Validated      bool                 `json:"validated"`
	AdminLevel     int                  `json:"adminLevel"`
	Sniff          bool                 `json:"sniff"`
	Debug          bool                 `json:"debug"`
	SortBy         int                  `json:"sortBy"`
	SortOrder      int                  `json:"sortOrder"`
	Skin           string               `json:"skin"`
	UseSkin        bool                 `json:"useSkin"`
	DeactivateIP   bool                 `json:"deactivateIP"`
	MaxSpy         int                  `json:"maxSpy"`
	MaxFleetMsg    int                  `json:"maxFleetMsg"`
	OldScore1      int64                `json:"oldScore1"`
	OldPlace1      int                  `json:"oldPlace1"`
	OldScore2      int64                `json:"oldScore2"`
	OldPlace2      int                  `json:"oldPlace2"`
	OldScore3      int64                `json:"oldScore3"`
	OldPlace3      int                  `json:"oldPlace3"`
	Score1         int64                `json:"score1"`
	Place1         int                  `json:"place1"`
	Score2         int64                `json:"score2"`
	Place2         int                  `json:"place2"`
	Score3         int64                `json:"score3"`
	Place3         int                  `json:"place3"`
	ScoreDate      int64                `json:"scoreDate"`
	DarkMatterFree int                  `json:"darkMatterFree"`
	DarkMatter     int                  `json:"darkMatter"`
	Research       map[int]int          `json:"research"`
	ActivePlanet   *gameAdminUserPlanet `json:"activePlanet,omitempty"`
	Planets        []gameAdminPlanetRow `json:"planets"`
}

type gameAdminPlanetRow struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Date        int64                   `json:"date"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Owner       *gameAdminUserRow       `json:"owner,omitempty"`
}

type gameAdminTechnologyValue struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Value   int    `json:"value"`
	Percent int    `json:"percent"`
}

type gameAdminPlanetDetail struct {
	gameAdminPlanetRow
	Type             int                        `json:"type"`
	Diameter         int                        `json:"diameter"`
	Temperature      int                        `json:"temperature"`
	Fields           int                        `json:"fields"`
	MaxFields        int                        `json:"maxFields"`
	RemoveDate       int64                      `json:"removeDate"`
	LastActivity     int64                      `json:"lastActivity"`
	LastUpdate       int64                      `json:"lastUpdate"`
	GateUntil        int64                      `json:"gateUntil"`
	Score            gamePlanetScoreResponse    `json:"score"`
	Resources        gameResourcesResponse      `json:"resources"`
	EnergyBalance    int                        `json:"energyBalance"`
	EnergyCapacity   int                        `json:"energyCapacity"`
	ProductionFactor float64                    `json:"productionFactor"`
	Buildings        []gameAdminTechnologyValue `json:"buildings"`
	Fleet            []gameAdminTechnologyValue `json:"fleet"`
	Defense          []gameAdminTechnologyValue `json:"defense"`
	BuildQueue       []gameBuildQueueResponse   `json:"buildQueue"`
	Moon             *gameAdminPlanetRow        `json:"moon,omitempty"`
	Debris           *gameAdminPlanetRow        `json:"debris,omitempty"`
}

type gamePlanetScoreResponse struct {
	Points        int64 `json:"points"`
	FleetPoints   int64 `json:"fleetPoints"`
	DefensePoints int64 `json:"defensePoints"`
}

type gameAdminReportRow struct {
	ID        int    `json:"id"`
	OwnerID   int    `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	MessageID int    `json:"messageId"`
	From      string `json:"from"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type gameAdminFleetLogRow struct {
	TaskID     int                          `json:"taskId"`
	Number     int                          `json:"number"`
	Mission    int                          `json:"mission"`
	Start      int64                        `json:"start"`
	End        int64                        `json:"end"`
	FlightTime int                          `json:"flightTime"`
	Fuel       int                          `json:"fuel"`
	UnionID    int                          `json:"unionId"`
	Origin     gameAdminFleetLogPlanet      `json:"origin"`
	Target     gameAdminFleetLogPlanet      `json:"target"`
	Ships      []gameFleetShipCountResponse `json:"ships"`
	Cargo      []gameFleetResourceLoad      `json:"cargo"`
}

type gameAdminFleetLogPlanet struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	OwnerID     int                     `json:"ownerId"`
	OwnerName   string                  `json:"ownerName"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Type        int                     `json:"type"`
}

type gameAdminUniverseSettings struct {
	Number          int     `json:"number"`
	Speed           float64 `json:"speed"`
	FleetSpeed      float64 `json:"fleetSpeed"`
	Galaxies        int     `json:"galaxies"`
	Systems         int     `json:"systems"`
	MaxUsers        int     `json:"maxUsers"`
	ACS             int     `json:"acs"`
	FleetDebris     int     `json:"fleetDebris"`
	DefenseDebris   int     `json:"defenseDebris"`
	RapidFire       bool    `json:"rapidFire"`
	Moons           bool    `json:"moons"`
	DefenseRepair   int     `json:"defenseRepair"`
	DefenseDelta    int     `json:"defenseDelta"`
	UserCount       int     `json:"userCount"`
	Freeze          bool    `json:"freeze"`
	News1           string  `json:"news1"`
	News2           string  `json:"news2"`
	NewsUntil       int64   `json:"newsUntil"`
	StartDate       int64   `json:"startDate"`
	BattleEngine    string  `json:"battleEngine"`
	Language        string  `json:"language"`
	Hacks           int     `json:"hacks"`
	ExtBoard        string  `json:"extBoard"`
	ExtDiscord      string  `json:"extDiscord"`
	ExtTutorial     string  `json:"extTutorial"`
	ExtRules        string  `json:"extRules"`
	ExtImpressum    string  `json:"extImpressum"`
	PHPBattle       bool    `json:"phpBattle"`
	BattleMax       int     `json:"battleMax"`
	ForceLanguage   bool    `json:"forceLanguage"`
	StartDarkMatter int     `json:"startDarkMatter"`
	MaxShipyard     int     `json:"maxShipyard"`
	FeedAge         int     `json:"feedAge"`
}

type gameAdminQueueRow struct {
	ID          int    `json:"id"`
	OwnerID     int    `json:"ownerId"`
	OwnerName   string `json:"ownerName"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Freeze      bool   `json:"freeze"`
	Frozen      int64  `json:"frozen"`
}

type gameAdminBattleReportRow struct {
	ID    int    `json:"id"`
	Date  int64  `json:"date"`
	Title string `json:"title"`
}

type gameAdminChecksumGroup struct {
	Title string                 `json:"title"`
	Rows  []gameAdminChecksumRow `json:"rows"`
}

type gameAdminChecksumRow struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Status   string `json:"status"`
}

type gameAdminDatabaseBackup struct {
	FileName string `json:"fileName"`
}

type gameAdminBotStrategy struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type gameAdminCouponRow struct {
	ID           int    `json:"id"`
	Code         string `json:"code"`
	Amount       int    `json:"amount"`
	Used         bool   `json:"used"`
	UserUniverse int    `json:"userUniverse"`
	UserID       int    `json:"userId"`
	UserName     string `json:"userName"`
}

type gameAdminCouponQueueRow struct {
	ID           int   `json:"id"`
	Amount       int   `json:"amount"`
	InactiveDays int   `json:"inactiveDays"`
	IngameDays   int   `json:"ingameDays"`
	PeriodicDays int   `json:"periodicDays"`
	Start        int64 `json:"start"`
	End          int64 `json:"end"`
	Priority     int   `json:"priority"`
}

func (a app) handleGameAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameAdminGet(w, r)
	case http.MethodPost:
		a.handleGameAdminPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameAdminGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	targetPlayerID, err := selectedAdminPlayerID(r)
	if err != nil {
		http.Error(w, "invalid selected player", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.GetAdmin(r.Context(), appgame.AdminCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            r.URL.Query().Get("mode"),
		TargetPlayerID:  targetPlayerID,
		TargetPlanetID:  planetID,
		Filter:          r.URL.Query().Get("filter"),
	})
	if err != nil {
		logGameAdminError(a.deps.Logger, r, "game admin get failed", err)
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameAdminResponse(w, result)
}

func (a app) handleGameAdminPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	targetPlayerID, err := selectedAdminPlayerID(r)
	if err != nil {
		http.Error(w, "invalid selected player", http.StatusBadRequest)
		return
	}
	var request gameAdminMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid admin request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.MutateAdmin(r.Context(), appgame.AdminMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            r.URL.Query().Get("mode"),
		TargetPlayerID:  targetPlayerID,
		TargetPlanetID:  planetID,
		Filter:          r.URL.Query().Get("filter"),
		Action:          request.Action,
		TaskID:          request.TaskID,
		TargetIDs:       request.TargetIDs,
		BanMode:         request.BanMode,
		Days:            request.Days,
		Hours:           request.Hours,
		Reason:          request.Reason,
		Values:          request.Values,
		Category:        request.Category,
		Subject:         request.Subject,
		Text:            request.Text,
		ReportIDs:       request.ReportIDs,
		DeleteMode:      request.DeleteMode,
		FileName:        request.FileName,
		Amount:          request.Amount,
		ItemID:          request.ItemID,
		DayMonth:        request.DayMonth,
		HourMinute:      request.HourMinute,
		InactiveDays:    request.InactiveDays,
		IngameDays:      request.IngameDays,
		PeriodicDays:    request.PeriodicDays,
	})
	if err != nil {
		logGameAdminError(a.deps.Logger, r, "game admin mutation failed", err)
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameAdminResponse(w, result)
}

func selectedAdminPlayerID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("player_id")
	if raw == "" {
		return 0, nil
	}
	playerID, err := strconv.Atoi(raw)
	if err != nil || playerID < 0 {
		return 0, strconv.ErrSyntax
	}
	return playerID, nil
}

func logGameAdminError(logger *slog.Logger, r *http.Request, message string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Error(message, "error", err, "method", r.Method, "path", r.URL.Path, "mode", r.URL.Query().Get("mode"))
}

func writeGameAdminResponse(w http.ResponseWriter, result appgame.AdminResult) {
	status := http.StatusOK
	var admin *gameAdminSummary
	if result.Authenticated {
		mapped := toGameAdminSummary(result.Admin)
		admin = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameAdminResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Admin:         admin,
		ActionIssue:   toGameAdminActionIssue(result.ActionIssue),
	})
}

func toGameAdminSummary(admin domaingame.Admin) gameAdminSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(admin.PlanetSwitcher))
	for _, planet := range admin.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	menu := make([]gameAdminMenuItem, 0, len(admin.Menu))
	for _, item := range admin.Menu {
		menu = append(menu, gameAdminMenuItem{Mode: item.Mode, Label: item.Label, Image: item.Image})
	}
	messageRows := make([]gameAdminMessageRow, 0, len(admin.MessageRows))
	for _, row := range admin.MessageRows {
		messageRows = append(messageRows, gameAdminMessageRow{
			ID:        row.ID,
			OwnerID:   row.OwnerID,
			OwnerName: row.OwnerName,
			IP:        row.IP,
			Agent:     row.Agent,
			Text:      row.Text,
			Date:      row.Date,
		})
	}
	userLogRows := make([]gameAdminUserLogRow, 0, len(admin.UserLogRows))
	for _, row := range admin.UserLogRows {
		userLogRows = append(userLogRows, gameAdminUserLogRow{
			ID:        row.ID,
			OwnerID:   row.OwnerID,
			OwnerName: row.OwnerName,
			LastClick: row.LastClick,
			Vacation:  row.Vacation,
			Banned:    row.Banned,
			NoAttack:  row.NoAttack,
			Disable:   row.Disable,
			Type:      row.Type,
			Text:      row.Text,
			Date:      row.Date,
		})
	}
	userRows := make([]gameAdminUserRow, 0, len(admin.UserRows))
	for _, row := range admin.UserRows {
		userRows = append(userRows, toGameAdminUserRow(row))
	}
	activeUsers := make([]gameAdminUserRow, 0, len(admin.ActiveUsers))
	for _, row := range admin.ActiveUsers {
		activeUsers = append(activeUsers, toGameAdminUserRow(row))
	}
	selectedUser := toGameAdminUserDetail(admin.SelectedUser)
	planetRows := make([]gameAdminPlanetRow, 0, len(admin.PlanetRows))
	for _, row := range admin.PlanetRows {
		planetRows = append(planetRows, toGameAdminPlanetRow(row))
	}
	selectedPlanet := toGameAdminPlanetDetail(admin.SelectedPlanet)
	reportRows := make([]gameAdminReportRow, 0, len(admin.ReportRows))
	for _, row := range admin.ReportRows {
		reportRows = append(reportRows, gameAdminReportRow{
			ID:        row.ID,
			OwnerID:   row.OwnerID,
			OwnerName: row.OwnerName,
			MessageID: row.MessageID,
			From:      row.From,
			Subject:   row.Subject,
			Text:      row.Text,
			Date:      row.Date,
		})
	}
	universe := toGameAdminUniverseSettings(admin.Universe)
	fleetLogRows := make([]gameAdminFleetLogRow, 0, len(admin.FleetLogRows))
	for _, row := range admin.FleetLogRows {
		fleetLogRows = append(fleetLogRows, toGameAdminFleetLogRow(row))
	}
	queueRows := make([]gameAdminQueueRow, 0, len(admin.QueueRows))
	for _, row := range admin.QueueRows {
		queueRows = append(queueRows, gameAdminQueueRow{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			OwnerName:   row.OwnerName,
			Type:        row.Type,
			Description: row.Description,
			Priority:    row.Priority,
			Start:       row.Start,
			End:         row.End,
			Freeze:      row.Freeze,
			Frozen:      row.Frozen,
		})
	}
	battleReports := make([]gameAdminBattleReportRow, 0, len(admin.BattleReports))
	for _, row := range admin.BattleReports {
		battleReports = append(battleReports, gameAdminBattleReportRow{
			ID:    row.ID,
			Date:  row.Date,
			Title: row.Title,
		})
	}
	checksumGroups := make([]gameAdminChecksumGroup, 0, len(admin.ChecksumGroups))
	for _, group := range admin.ChecksumGroups {
		rows := make([]gameAdminChecksumRow, 0, len(group.Rows))
		for _, row := range group.Rows {
			rows = append(rows, gameAdminChecksumRow{
				Path:     row.Path,
				Checksum: row.Checksum,
				Status:   row.Status,
			})
		}
		checksumGroups = append(checksumGroups, gameAdminChecksumGroup{
			Title: group.Title,
			Rows:  rows,
		})
	}
	databaseBackups := make([]gameAdminDatabaseBackup, 0, len(admin.DatabaseBackups))
	for _, backup := range admin.DatabaseBackups {
		databaseBackups = append(databaseBackups, gameAdminDatabaseBackup{FileName: backup.FileName})
	}
	botStrategies := make([]gameAdminBotStrategy, 0, len(admin.BotStrategies))
	for _, strategy := range admin.BotStrategies {
		botStrategies = append(botStrategies, gameAdminBotStrategy{
			ID:   strategy.ID,
			Name: strategy.Name,
		})
	}
	couponRows := make([]gameAdminCouponRow, 0, len(admin.CouponRows))
	for _, row := range admin.CouponRows {
		couponRows = append(couponRows, gameAdminCouponRow{
			ID:           row.ID,
			Code:         row.Code,
			Amount:       row.Amount,
			Used:         row.Used,
			UserUniverse: row.UserUniverse,
			UserID:       row.UserID,
			UserName:     row.UserName,
		})
	}
	couponQueueRows := make([]gameAdminCouponQueueRow, 0, len(admin.CouponQueueRows))
	for _, row := range admin.CouponQueueRows {
		couponQueueRows = append(couponQueueRows, gameAdminCouponQueueRow{
			ID:           row.ID,
			Amount:       row.Amount,
			InactiveDays: row.InactiveDays,
			IngameDays:   row.IngameDays,
			PeriodicDays: row.PeriodicDays,
			Start:        row.Start,
			End:          row.End,
			Priority:     row.Priority,
		})
	}
	return gameAdminSummary{
		Commander:      admin.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(admin.CurrentPlanet),
		PlanetSwitcher: planets,
		Viewer: gameAdminViewer{
			PlayerID: admin.Viewer.PlayerID,
			Name:     admin.Viewer.Name,
			Level:    admin.Viewer.Level,
		},
		Mode:            admin.Mode,
		Menu:            menu,
		MessageRows:     messageRows,
		UserLogRows:     userLogRows,
		UserRows:        userRows,
		ActiveUsers:     activeUsers,
		SelectedUser:    selectedUser,
		PlanetRows:      planetRows,
		SelectedPlanet:  selectedPlanet,
		ReportRows:      reportRows,
		Universe:        universe,
		Expedition:      admin.Expedition,
		FleetLogRows:    fleetLogRows,
		QueueRows:       queueRows,
		BattleReports:   battleReports,
		ChecksumGroups:  checksumGroups,
		DatabaseBackups: databaseBackups,
		BotStrategies:   botStrategies,
		CouponRows:      couponRows,
		CouponQueueRows: couponQueueRows,
	}
}

func toGameAdminUniverseSettings(universe *domaingame.AdminUniverseSettings) *gameAdminUniverseSettings {
	if universe == nil {
		return nil
	}
	return &gameAdminUniverseSettings{
		Number:          universe.Number,
		Speed:           universe.Speed,
		FleetSpeed:      universe.FleetSpeed,
		Galaxies:        universe.Galaxies,
		Systems:         universe.Systems,
		MaxUsers:        universe.MaxUsers,
		ACS:             universe.ACS,
		FleetDebris:     universe.FleetDebris,
		DefenseDebris:   universe.DefenseDebris,
		RapidFire:       universe.RapidFire,
		Moons:           universe.Moons,
		DefenseRepair:   universe.DefenseRepair,
		DefenseDelta:    universe.DefenseDelta,
		UserCount:       universe.UserCount,
		Freeze:          universe.Freeze,
		News1:           universe.News1,
		News2:           universe.News2,
		NewsUntil:       universe.NewsUntil,
		StartDate:       universe.StartDate,
		BattleEngine:    universe.BattleEngine,
		Language:        universe.Language,
		Hacks:           universe.Hacks,
		ExtBoard:        universe.ExtBoard,
		ExtDiscord:      universe.ExtDiscord,
		ExtTutorial:     universe.ExtTutorial,
		ExtRules:        universe.ExtRules,
		ExtImpressum:    universe.ExtImpressum,
		PHPBattle:       universe.PHPBattle,
		BattleMax:       universe.BattleMax,
		ForceLanguage:   universe.ForceLanguage,
		StartDarkMatter: universe.StartDarkMatter,
		MaxShipyard:     universe.MaxShipyard,
		FeedAge:         universe.FeedAge,
	}
}

func toGameAdminFleetLogRow(row domaingame.AdminFleetLogRow) gameAdminFleetLogRow {
	ships := make([]gameFleetShipCountResponse, 0, len(row.Ships))
	for _, ship := range row.Ships {
		ships = append(ships, gameFleetShipCountResponse{ID: ship.ID, Name: ship.Name, Count: ship.Count})
	}
	cargo := make([]gameFleetResourceLoad, 0, len(row.Cargo))
	for _, resource := range row.Cargo {
		cargo = append(cargo, gameFleetResourceLoad{
			ID:     resource.ID,
			Name:   resource.Name,
			Loaded: resource.Loaded,
		})
	}
	return gameAdminFleetLogRow{
		TaskID:     row.TaskID,
		Number:     row.Number,
		Mission:    row.Mission,
		Start:      row.Start,
		End:        row.End,
		FlightTime: row.FlightTime,
		Fuel:       row.Fuel,
		UnionID:    row.UnionID,
		Origin:     toGameAdminFleetLogPlanet(row.Origin),
		Target:     toGameAdminFleetLogPlanet(row.Target),
		Ships:      ships,
		Cargo:      cargo,
	}
}

func toGameAdminFleetLogPlanet(planet domaingame.AdminFleetLogPlanet) gameAdminFleetLogPlanet {
	return gameAdminFleetLogPlanet{
		ID:          planet.ID,
		Name:        planet.Name,
		OwnerID:     planet.OwnerID,
		OwnerName:   planet.OwnerName,
		Coordinates: toGameCoordinatesResponse(planet.Coordinates),
		Type:        planet.Type,
	}
}

func toGameAdminPlanetRow(row domaingame.AdminPlanetRow) gameAdminPlanetRow {
	var owner *gameAdminUserRow
	if row.Owner != nil {
		mapped := toGameAdminUserRow(*row.Owner)
		owner = &mapped
	}
	return gameAdminPlanetRow{
		ID:          row.ID,
		Name:        row.Name,
		Date:        row.Date,
		Coordinates: toGameCoordinatesResponse(row.Coordinates),
		Owner:       owner,
	}
}

func toGameAdminPlanetRowPointer(row *domaingame.AdminPlanetRow) *gameAdminPlanetRow {
	if row == nil {
		return nil
	}
	mapped := toGameAdminPlanetRow(*row)
	return &mapped
}

func toGameAdminPlanetDetail(detail *domaingame.AdminPlanetDetail) *gameAdminPlanetDetail {
	if detail == nil {
		return nil
	}
	buildings := make([]gameAdminTechnologyValue, 0, len(detail.Buildings))
	for _, row := range detail.Buildings {
		buildings = append(buildings, toGameAdminTechnologyValue(row))
	}
	fleet := make([]gameAdminTechnologyValue, 0, len(detail.Fleet))
	for _, row := range detail.Fleet {
		fleet = append(fleet, toGameAdminTechnologyValue(row))
	}
	defense := make([]gameAdminTechnologyValue, 0, len(detail.Defense))
	for _, row := range detail.Defense {
		defense = append(defense, toGameAdminTechnologyValue(row))
	}
	buildQueue := make([]gameBuildQueueResponse, 0, len(detail.BuildQueue))
	for _, entry := range detail.BuildQueue {
		buildQueue = append(buildQueue, gameBuildQueueResponse{
			TechID:  entry.TechID,
			Name:    entry.Name,
			Level:   entry.Level,
			Destroy: entry.Destroy,
			End:     int64(entry.End),
		})
	}
	return &gameAdminPlanetDetail{
		gameAdminPlanetRow: toGameAdminPlanetRow(detail.AdminPlanetRow),
		Type:               detail.Type,
		Diameter:           detail.Diameter,
		Temperature:        detail.Temperature,
		Fields:             detail.Fields,
		MaxFields:          detail.MaxFields,
		RemoveDate:         detail.RemoveDate,
		LastActivity:       detail.LastActivity,
		LastUpdate:         detail.LastUpdate,
		GateUntil:          detail.GateUntil,
		Score: gamePlanetScoreResponse{
			Points:        detail.Score.Points,
			FleetPoints:   detail.Score.FleetPoints,
			DefensePoints: detail.Score.DefensePoints,
		},
		Resources: gameResourcesResponse{
			Metal:      detail.Resources.Metal,
			Crystal:    detail.Resources.Crystal,
			Deuterium:  detail.Resources.Deuterium,
			DarkMatter: detail.Resources.DarkMatter,
		},
		EnergyBalance:    detail.EnergyBalance,
		EnergyCapacity:   detail.EnergyCapacity,
		ProductionFactor: detail.ProductionFactor,
		Buildings:        buildings,
		Fleet:            fleet,
		Defense:          defense,
		BuildQueue:       buildQueue,
		Moon:             toGameAdminPlanetRowPointer(detail.Moon),
		Debris:           toGameAdminPlanetRowPointer(detail.Debris),
	}
}

func toGameAdminTechnologyValue(row domaingame.AdminTechnologyValue) gameAdminTechnologyValue {
	return gameAdminTechnologyValue{ID: row.ID, Name: row.Name, Value: row.Value, Percent: row.Percent}
}

func toGameAdminUserRow(row domaingame.AdminUserRow) gameAdminUserRow {
	var planet *gameAdminUserPlanet
	if row.HomePlanet != nil {
		planet = &gameAdminUserPlanet{
			ID:          row.HomePlanet.ID,
			Name:        row.HomePlanet.Name,
			Coordinates: toGameCoordinatesResponse(row.HomePlanet.Coordinates),
		}
	}
	return gameAdminUserRow{
		PlayerID:   row.PlayerID,
		Name:       row.Name,
		RegDate:    row.RegDate,
		LastClick:  row.LastClick,
		Vacation:   row.Vacation,
		Banned:     row.Banned,
		NoAttack:   row.NoAttack,
		Disable:    row.Disable,
		HomePlanet: planet,
	}
}

func toGameAdminUserPlanetPointer(planet *domaingame.AdminUserPlanet) *gameAdminUserPlanet {
	if planet == nil {
		return nil
	}
	return &gameAdminUserPlanet{
		ID:          planet.ID,
		Name:        planet.Name,
		Coordinates: toGameCoordinatesResponse(planet.Coordinates),
	}
}

func toGameAdminUserDetail(detail *domaingame.AdminUserDetail) *gameAdminUserDetail {
	if detail == nil {
		return nil
	}
	planets := make([]gameAdminPlanetRow, 0, len(detail.Planets))
	for _, row := range detail.Planets {
		planets = append(planets, toGameAdminPlanetRow(row))
	}
	return &gameAdminUserDetail{
		gameAdminUserRow: toGameAdminUserRow(detail.AdminUserRow),
		PermanentEmail:   detail.PermanentEmail,
		Email:            detail.Email,
		Alliance:         detail.Alliance,
		JoinDate:         detail.JoinDate,
		DisableUntil:     detail.DisableUntil,
		VacationUntil:    detail.VacationUntil,
		BannedUntil:      detail.BannedUntil,
		NoAttackUntil:    detail.NoAttackUntil,
		LastLogin:        detail.LastLogin,
		IPAddress:        detail.IPAddress,
		Validated:        detail.Validated,
		AdminLevel:       detail.AdminLevel,
		Sniff:            detail.Sniff,
		Debug:            detail.Debug,
		SortBy:           detail.SortBy,
		SortOrder:        detail.SortOrder,
		Skin:             detail.Skin,
		UseSkin:          detail.UseSkin,
		DeactivateIP:     detail.DeactivateIP,
		MaxSpy:           detail.MaxSpy,
		MaxFleetMsg:      detail.MaxFleetMsg,
		OldScore1:        detail.OldScore1,
		OldPlace1:        detail.OldPlace1,
		OldScore2:        detail.OldScore2,
		OldPlace2:        detail.OldPlace2,
		OldScore3:        detail.OldScore3,
		OldPlace3:        detail.OldPlace3,
		Score1:           detail.Score1,
		Place1:           detail.Place1,
		Score2:           detail.Score2,
		Place2:           detail.Place2,
		Score3:           detail.Score3,
		Place3:           detail.Place3,
		ScoreDate:        detail.ScoreDate,
		DarkMatterFree:   detail.DarkMatterFree,
		DarkMatter:       detail.DarkMatter,
		Research:         map[int]int(detail.Research),
		ActivePlanet:     toGameAdminUserPlanetPointer(detail.ActivePlanet),
		Planets:          planets,
	}
}

func toGameAdminActionIssue(issue *domaingame.AdminActionIssue) *gameAdminActionIssue {
	if issue == nil {
		return nil
	}
	return &gameAdminActionIssue{Code: issue.Code, Message: issue.Message}
}

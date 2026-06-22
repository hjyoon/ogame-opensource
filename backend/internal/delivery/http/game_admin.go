package httpdelivery

import (
	"encoding/json"
	"net/http"

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

type gameAdminSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Viewer         gameAdminViewer             `json:"viewer"`
	Mode           string                      `json:"mode"`
	Menu           []gameAdminMenuItem         `json:"menu"`
	MessageRows    []gameAdminMessageRow       `json:"messageRows,omitempty"`
	UserLogRows    []gameAdminUserLogRow       `json:"userLogRows,omitempty"`
	UserRows       []gameAdminUserRow          `json:"userRows,omitempty"`
	ActiveUsers    []gameAdminUserRow          `json:"activeUsers,omitempty"`
	QueueRows      []gameAdminQueueRow         `json:"queueRows,omitempty"`
	BattleReports  []gameAdminBattleReportRow  `json:"battleReports,omitempty"`
	ChecksumGroups []gameAdminChecksumGroup    `json:"checksumGroups,omitempty"`
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

func (a app) handleGameAdmin(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.GetAdmin(r.Context(), appgame.AdminCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            r.URL.Query().Get("mode"),
	})
	if err != nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameAdminResponse(w, result)
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
	return gameAdminSummary{
		Commander:      admin.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(admin.CurrentPlanet),
		PlanetSwitcher: planets,
		Viewer: gameAdminViewer{
			PlayerID: admin.Viewer.PlayerID,
			Name:     admin.Viewer.Name,
			Level:    admin.Viewer.Level,
		},
		Mode:           admin.Mode,
		Menu:           menu,
		MessageRows:    messageRows,
		UserLogRows:    userLogRows,
		UserRows:       userRows,
		ActiveUsers:    activeUsers,
		QueueRows:      queueRows,
		BattleReports:  battleReports,
		ChecksumGroups: checksumGroups,
	}
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

func toGameAdminActionIssue(issue *domaingame.AdminActionIssue) *gameAdminActionIssue {
	if issue == nil {
		return nil
	}
	return &gameAdminActionIssue{Code: issue.Code, Message: issue.Message}
}

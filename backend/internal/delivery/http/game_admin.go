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
	QueueRows      []gameAdminQueueRow         `json:"queueRows,omitempty"`
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
	return gameAdminSummary{
		Commander:      admin.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(admin.CurrentPlanet),
		PlanetSwitcher: planets,
		Viewer: gameAdminViewer{
			PlayerID: admin.Viewer.PlayerID,
			Name:     admin.Viewer.Name,
			Level:    admin.Viewer.Level,
		},
		Mode:        admin.Mode,
		Menu:        menu,
		MessageRows: messageRows,
		UserLogRows: userLogRows,
		QueueRows:   queueRows,
	}
}

func toGameAdminActionIssue(issue *domaingame.AdminActionIssue) *gameAdminActionIssue {
	if issue == nil {
		return nil
	}
	return &gameAdminActionIssue{Code: issue.Code, Message: issue.Message}
}

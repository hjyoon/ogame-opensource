package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameBuddyResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	ActionIssue   *gameBuddyActionIssue      `json:"actionIssue,omitempty"`
	Buddy         *gameBuddySummary          `json:"buddy,omitempty"`
}

type gameBuddyActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameBuddyMutationRequest struct {
	Action  int    `json:"action"`
	BuddyID int    `json:"buddyId"`
	Text    string `json:"text"`
}

type gameBuddySummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Action         int                         `json:"action"`
	Rows           []gameBuddyRow              `json:"rows"`
	Target         *gameBuddyPlayer            `json:"target,omitempty"`
}

type gameBuddyRow struct {
	BuddyID int             `json:"buddyId"`
	Player  gameBuddyPlayer `json:"player"`
	Text    string          `json:"text"`
	Status  gameBuddyStatus `json:"status"`
}

type gameBuddyPlayer struct {
	PlayerID    int                     `json:"playerId"`
	Name        string                  `json:"name"`
	Alliance    *gameBuddyAlliance      `json:"alliance,omitempty"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
}

type gameBuddyAlliance struct {
	ID      int    `json:"id"`
	Tag     string `json:"tag"`
	Founder bool   `json:"founder"`
}

type gameBuddyStatus struct {
	Text  string `json:"text"`
	Color string `json:"color"`
}

func (a app) handleGameBuddy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameBuddyGet(w, r)
	case http.MethodPost:
		a.handleGameBuddyPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameBuddyGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameBuddy == nil {
		http.Error(w, "game buddy unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	action, err := optionalIntQuery(r, "action")
	if err != nil {
		http.Error(w, "invalid buddy action", http.StatusBadRequest)
		return
	}
	buddyID, err := optionalIntQuery(r, "buddy_id")
	if err != nil {
		http.Error(w, "invalid buddy id", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameBuddy.GetBuddy(r.Context(), appgame.BuddyCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          action,
		BuddyID:         buddyID,
	})
	if err != nil {
		http.Error(w, "game buddy unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameBuddyResponse(w, result)
}

func (a app) handleGameBuddyPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameBuddy == nil {
		http.Error(w, "game buddy unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	var mutation gameBuddyMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		http.Error(w, "invalid buddy mutation", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameBuddy.MutateBuddy(r.Context(), appgame.BuddyMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          mutation.Action,
		BuddyID:         mutation.BuddyID,
		Text:            mutation.Text,
	})
	if err != nil {
		http.Error(w, "game buddy unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameBuddyResponse(w, result)
}

func writeGameBuddyResponse(w http.ResponseWriter, result appgame.BuddyResult) {
	status := http.StatusOK
	var buddy *gameBuddySummary
	if result.Authenticated {
		mapped := toGameBuddySummary(result.Buddy)
		buddy = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameBuddyResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		ActionIssue:   toGameBuddyActionIssue(result.ActionIssue),
		Buddy:         buddy,
	})
}

func optionalIntQuery(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func toGameBuddySummary(buddy domaingame.Buddy) gameBuddySummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(buddy.PlanetSwitcher))
	for _, planet := range buddy.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameBuddyRow, 0, len(buddy.Rows))
	for _, row := range buddy.Rows {
		rows = append(rows, toGameBuddyRow(row))
	}
	var target *gameBuddyPlayer
	if buddy.Target != nil {
		mapped := toGameBuddyPlayer(*buddy.Target)
		target = &mapped
	}
	return gameBuddySummary{
		Commander:      buddy.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(buddy.CurrentPlanet),
		PlanetSwitcher: planets,
		Action:         buddy.Action,
		Rows:           rows,
		Target:         target,
	}
}

func toGameBuddyActionIssue(issue *domaingame.BuddyActionIssue) *gameBuddyActionIssue {
	if issue == nil {
		return nil
	}
	return &gameBuddyActionIssue{Code: issue.Code, Message: issue.Message}
}

func toGameBuddyRow(row domaingame.BuddyRow) gameBuddyRow {
	return gameBuddyRow{
		BuddyID: row.BuddyID,
		Player:  toGameBuddyPlayer(row.Player),
		Text:    row.Text,
		Status: gameBuddyStatus{
			Text:  row.Status.Text,
			Color: row.Status.Color,
		},
	}
}

func toGameBuddyPlayer(player domaingame.BuddyPlayer) gameBuddyPlayer {
	var alliance *gameBuddyAlliance
	if player.Alliance != nil {
		alliance = &gameBuddyAlliance{ID: player.Alliance.ID, Tag: player.Alliance.Tag, Founder: player.Alliance.Founder}
	}
	return gameBuddyPlayer{
		PlayerID:    player.PlayerID,
		Name:        player.Name,
		Alliance:    alliance,
		Coordinates: toGameCoordinatesResponse(player.Coordinates),
	}
}

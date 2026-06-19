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
	Buddy         *gameBuddySummary          `json:"buddy,omitempty"`
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

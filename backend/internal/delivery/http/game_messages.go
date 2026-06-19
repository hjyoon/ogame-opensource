package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameMessagesResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Messages      *gameMessagesSummary       `json:"messages,omitempty"`
}

type gameMessagesSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Action         string                      `json:"action"`
	Rows           []gameMessageResponse       `json:"rows"`
	Compose        *gameMessageComposeResponse `json:"compose,omitempty"`
}

type gameMessageResponse struct {
	ID         int    `json:"id"`
	Type       int    `json:"type"`
	From       string `json:"from"`
	Subject    string `json:"subject"`
	Text       string `json:"text"`
	Date       int64  `json:"date"`
	Unread     bool   `json:"unread"`
	Reportable bool   `json:"reportable"`
}

type gameMessageComposeResponse struct {
	Target   gameMessageTargetResponse `json:"target"`
	Subject  string                    `json:"subject"`
	MaxChars int                       `json:"maxChars"`
}

type gameMessageTargetResponse struct {
	PlayerID    int                     `json:"playerId"`
	Name        string                  `json:"name"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
}

func (a app) handleGameMessages(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameMessages == nil {
		http.Error(w, "game messages unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	targetPlayerID, err := selectedMessageTargetID(r)
	if err != nil {
		http.Error(w, "invalid message target", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameMessages.GetMessages(r.Context(), appgame.MessagesCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		TargetPlayerID:  targetPlayerID,
	})
	if err != nil {
		http.Error(w, "game messages unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var messages *gameMessagesSummary
	if result.Authenticated {
		mapped := toGameMessagesSummary(result.Messages)
		messages = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameMessagesResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Messages:      messages,
	})
}

func selectedMessageTargetID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("messageziel")
	if raw == "" {
		return 0, nil
	}
	targetID, err := strconv.Atoi(raw)
	if err != nil || targetID < 0 {
		return 0, strconv.ErrSyntax
	}
	return targetID, nil
}

func toGameMessagesSummary(messages domaingame.Messages) gameMessagesSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(messages.PlanetSwitcher))
	for _, planet := range messages.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameMessageResponse, 0, len(messages.Rows))
	for _, row := range messages.Rows {
		rows = append(rows, toGameMessageResponse(row))
	}
	var compose *gameMessageComposeResponse
	if messages.Compose != nil {
		mapped := toGameMessageComposeResponse(*messages.Compose)
		compose = &mapped
	}
	return gameMessagesSummary{
		Commander:      messages.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(messages.CurrentPlanet),
		PlanetSwitcher: planets,
		Action:         messages.Action,
		Rows:           rows,
		Compose:        compose,
	}
}

func toGameMessageResponse(message domaingame.Message) gameMessageResponse {
	return gameMessageResponse{
		ID:         message.ID,
		Type:       message.Type,
		From:       message.From,
		Subject:    message.Subject,
		Text:       message.Text,
		Date:       message.Date,
		Unread:     message.Unread,
		Reportable: message.Reportable,
	}
}

func toGameMessageComposeResponse(compose domaingame.MessageCompose) gameMessageComposeResponse {
	return gameMessageComposeResponse{
		Target: gameMessageTargetResponse{
			PlayerID: compose.Target.PlayerID,
			Name:     compose.Target.Name,
			Coordinates: gameCoordinatesResponse{
				Galaxy:   compose.Target.Coordinates.Galaxy,
				System:   compose.Target.Coordinates.System,
				Position: compose.Target.Coordinates.Position,
			},
		},
		Subject:  compose.Subject,
		MaxChars: compose.MaxChars,
	}
}

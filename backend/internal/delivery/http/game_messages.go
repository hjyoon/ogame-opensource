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
	ActionIssue   *gameMessageIssueResponse  `json:"actionIssue,omitempty"`
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

type gameMessagesMutationRequest struct {
	Action         string `json:"action"`
	TargetPlayerID int    `json:"targetPlayerId"`
	Subject        string `json:"subject"`
	Text           string `json:"text"`
	DeleteMode     string `json:"deleteMode"`
	MessageIDs     []int  `json:"messageIds"`
	ReportIDs      []int  `json:"reportIds"`
}

type gameMessageIssueResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a app) handleGameMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameMessagesGet(w, r)
	case http.MethodPost:
		a.handleGameMessagesPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameMessagesGet(w http.ResponseWriter, r *http.Request) {
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
		Subject:         r.URL.Query().Get("betreff"),
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

	writeGameMessagesResponse(w, status, result, messages)
}

func (a app) handleGameMessagesPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameMessages == nil {
		http.Error(w, "game messages unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	var request gameMessagesMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid messages request", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameMessages.MutateMessages(r.Context(), appgame.MessagesMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          request.Action,
		TargetPlayerID:  request.TargetPlayerID,
		Subject:         request.Subject,
		Text:            request.Text,
		DeleteMode:      request.DeleteMode,
		MessageIDs:      request.MessageIDs,
		ReportIDs:       request.ReportIDs,
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
	writeGameMessagesResponse(w, status, result, messages)
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

func writeGameMessagesResponse(w http.ResponseWriter, status int, result appgame.MessagesResult, messages *gameMessagesSummary) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameMessagesResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		ActionIssue:   toGameMessageIssueResponse(result.ActionIssue),
		Messages:      messages,
	})
}

func toGameMessageIssueResponse(issue *domaingame.MessageActionIssue) *gameMessageIssueResponse {
	if issue == nil {
		return nil
	}
	return &gameMessageIssueResponse{Code: issue.Code, Message: issue.Message}
}

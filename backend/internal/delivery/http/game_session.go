package httpdelivery

import (
	"encoding/json"
	"net/http"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type gameSessionResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Session       *gameSessionSummary        `json:"session,omitempty"`
}

type gameSessionIssueResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	BannedUntil int    `json:"bannedUntil,omitempty"`
}

type gameSessionSummary struct {
	PlayerID       int    `json:"playerId"`
	Commander      string `json:"commander"`
	UniverseNumber int    `json:"universeNumber"`
	HomePlanetID   int    `json:"homePlanetId"`
	VacationMode   bool   `json:"vacationMode"`
	VacationUntil  int    `json:"vacationUntil,omitempty"`
	DeletionQueued bool   `json:"deletionQueued"`
	DeletionAt     int    `json:"deletionAt,omitempty"`
}

func (a app) handleGameSession(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameSessions == nil {
		http.Error(w, "game session unavailable", http.StatusServiceUnavailable)
		return
	}

	result, err := a.deps.GameSessions.GetGameSession(r.Context(), apppublicsite.GameSessionCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
	})
	if err != nil {
		http.Error(w, "game session unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var session *gameSessionSummary
	if result.Authenticated {
		session = &gameSessionSummary{
			PlayerID:       result.Session.PlayerID,
			Commander:      result.Session.Commander,
			UniverseNumber: result.Session.UniverseNumber,
			HomePlanetID:   result.Session.HomePlanetID,
			VacationMode:   result.Session.VacationMode,
			VacationUntil:  result.Session.VacationUntil,
			DeletionQueued: result.Session.DeletionQueued,
			DeletionAt:     result.Session.DeletionAt,
		}
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameSessionResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Session:       session,
	})
}

func toGameSessionIssueResponses(issues []domain.SessionIssue) []gameSessionIssueResponse {
	responses := make([]gameSessionIssueResponse, 0, len(issues))
	for _, issue := range issues {
		responses = append(responses, gameSessionIssueResponse{
			Code:        issue.Code,
			Message:     issue.Message,
			BannedUntil: issue.BannedUntil,
		})
	}
	return responses
}

func cookieMap(r *http.Request) map[string]string {
	cookies := make(map[string]string)
	for _, cookie := range r.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	return cookies
}

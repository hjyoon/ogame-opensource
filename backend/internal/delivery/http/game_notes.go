package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameNotesResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Notes         *gameNotesSummary          `json:"notes,omitempty"`
}

type gameNotesSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Action         string                      `json:"action"`
	Rows           []gameNoteResponse          `json:"rows"`
	EditNote       *gameNoteResponse           `json:"editNote,omitempty"`
}

type gameNoteResponse struct {
	ID            int    `json:"id"`
	Subject       string `json:"subject"`
	Text          string `json:"text"`
	TextSize      int    `json:"textSize"`
	Priority      int    `json:"priority"`
	PriorityColor string `json:"priorityColor"`
	Date          int64  `json:"date"`
}

func (a app) handleGameNotes(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameNotes == nil {
		http.Error(w, "game notes unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	action, err := selectedNotesAction(r)
	if err != nil {
		http.Error(w, "invalid notes action", http.StatusBadRequest)
		return
	}
	noteID, err := selectedNoteID(r)
	if err != nil {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}
	if action == 2 && r.URL.Query().Get("n") == "" {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameNotes.GetNotes(r.Context(), appgame.NotesCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          action,
		NoteID:          noteID,
	})
	if err != nil {
		http.Error(w, "game notes unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var notes *gameNotesSummary
	if result.Authenticated {
		mapped := toGameNotesSummary(result.Notes)
		notes = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameNotesResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Notes:         notes,
	})
}

func selectedNotesAction(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("a")
	if raw == "" {
		return 0, nil
	}
	action, err := strconv.Atoi(raw)
	if err != nil || action < 1 || action > 2 {
		return 0, strconv.ErrSyntax
	}
	return action, nil
}

func selectedNoteID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("n")
	if raw == "" {
		return 0, nil
	}
	noteID, err := strconv.Atoi(raw)
	if err != nil || noteID < 0 {
		return 0, strconv.ErrSyntax
	}
	return noteID, nil
}

func toGameNotesSummary(notes domaingame.Notes) gameNotesSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(notes.PlanetSwitcher))
	for _, planet := range notes.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameNoteResponse, 0, len(notes.Rows))
	for _, note := range notes.Rows {
		rows = append(rows, toGameNoteResponse(note))
	}
	var editNote *gameNoteResponse
	if notes.EditNote != nil {
		mapped := toGameNoteResponse(*notes.EditNote)
		editNote = &mapped
	}
	return gameNotesSummary{
		Commander:      notes.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(notes.CurrentPlanet),
		PlanetSwitcher: planets,
		Action:         notes.Action,
		Rows:           rows,
		EditNote:       editNote,
	}
}

func toGameNoteResponse(note domaingame.Note) gameNoteResponse {
	return gameNoteResponse{
		ID:            note.ID,
		Subject:       note.Subject,
		Text:          note.Text,
		TextSize:      note.TextSize,
		Priority:      note.Priority,
		PriorityColor: note.PriorityColor(),
		Date:          note.Date,
	}
}

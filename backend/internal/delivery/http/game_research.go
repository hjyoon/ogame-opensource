package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameResearchResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	ActionIssue   *gameBuildingsActionIssue  `json:"actionIssue,omitempty"`
	Research      *gameResearchSummary       `json:"research,omitempty"`
}

type gameResearchSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasLab         bool                        `json:"hasLab"`
	Active         *gameResearchQueueResponse  `json:"active,omitempty"`
	Items          []gameBuildingItemResponse  `json:"items"`
}

type gameResearchQueueResponse struct {
	TaskID           int  `json:"taskId"`
	PlanetID         int  `json:"planetId"`
	TechID           int  `json:"techId"`
	Level            int  `json:"level"`
	Start            int  `json:"start"`
	End              int  `json:"end"`
	RemainingSeconds int  `json:"remainingSeconds"`
	Cancelable       bool `json:"cancelable"`
}

type gameResearchMutationRequest struct {
	Action string `json:"action"`
	TechID int    `json:"techId"`
}

func (a app) handleGameResearch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameResearchGet(w, r)
	case http.MethodPost:
		a.handleGameResearchPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameResearchGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameResearch == nil {
		http.Error(w, "game research unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameResearch.GetResearch(r.Context(), appgame.ResearchCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game research unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameResearchResponse(w, result)
}

func (a app) handleGameResearchPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameResearch == nil {
		http.Error(w, "game research unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	var mutation gameResearchMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		http.Error(w, "invalid research mutation", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameResearch.MutateResearch(r.Context(), appgame.ResearchMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          mutation.Action,
		TechID:          mutation.TechID,
	})
	if err != nil {
		http.Error(w, "game research unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameResearchResponse(w, result)
}

func writeGameResearchResponse(w http.ResponseWriter, result appgame.ResearchResult) {
	status := http.StatusOK
	var research *gameResearchSummary
	if result.Authenticated {
		mapped := toGameResearchSummary(result.Research)
		research = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameResearchResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		ActionIssue:   toGameBuildingsActionIssue(result.ActionIssue),
		Research:      research,
	})
}

func toGameResearchSummary(research domaingame.Research) gameResearchSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(research.PlanetSwitcher))
	for _, planet := range research.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	items := make([]gameBuildingItemResponse, 0, len(research.Items))
	for _, item := range research.Items {
		items = append(items, toGameBuildingItemResponse(item))
	}
	var active *gameResearchQueueResponse
	if research.Active != nil {
		active = &gameResearchQueueResponse{
			TaskID:           research.Active.TaskID,
			PlanetID:         research.Active.PlanetID,
			TechID:           research.Active.TechID,
			Level:            research.Active.Level,
			Start:            research.Active.Start,
			End:              research.Active.End,
			RemainingSeconds: research.Active.RemainingSeconds,
			Cancelable:       research.Active.Cancelable,
		}
	}
	return gameResearchSummary{
		Commander:      research.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(research.CurrentPlanet),
		PlanetSwitcher: planets,
		HasLab:         research.HasLab,
		Active:         active,
		Items:          items,
	}
}

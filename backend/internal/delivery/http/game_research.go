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
	Research      *gameResearchSummary       `json:"research,omitempty"`
}

type gameResearchSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasLab         bool                        `json:"hasLab"`
	Items          []gameBuildingItemResponse  `json:"items"`
}

func (a app) handleGameResearch(w http.ResponseWriter, r *http.Request) {
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
	return gameResearchSummary{
		Commander:      research.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(research.CurrentPlanet),
		PlanetSwitcher: planets,
		HasLab:         research.HasLab,
		Items:          items,
	}
}

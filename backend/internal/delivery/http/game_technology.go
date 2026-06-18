package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameTechnologyResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Technology    *gameTechnologySummary     `json:"technology,omitempty"`
}

type gameTechnologySummary struct {
	Commander      string                        `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse    `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse   `json:"planetSwitcher"`
	Groups         []gameTechnologyGroupResponse `json:"groups"`
}

type gameTechnologyGroupResponse struct {
	Key   string                       `json:"key"`
	Name  string                       `json:"name"`
	Items []gameTechnologyItemResponse `json:"items"`
}

type gameTechnologyItemResponse struct {
	ID               int                                 `json:"id"`
	Name             string                              `json:"name"`
	Requirements     []gameTechnologyRequirementResponse `json:"requirements"`
	DetailsAvailable bool                                `json:"detailsAvailable"`
}

type gameTechnologyRequirementResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	CurrentLevel int    `json:"currentLevel"`
	Met          bool   `json:"met"`
}

func (a app) handleGameTechnology(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameTechnology == nil {
		http.Error(w, "game technology unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameTechnology.GetTechnology(r.Context(), appgame.TechnologyCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game technology unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var technology *gameTechnologySummary
	if result.Authenticated {
		mapped := toGameTechnologySummary(result.Technology)
		technology = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameTechnologyResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Technology:    technology,
	})
}

func toGameTechnologySummary(technology domaingame.Technology) gameTechnologySummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(technology.PlanetSwitcher))
	for _, planet := range technology.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	groups := make([]gameTechnologyGroupResponse, 0, len(technology.Groups))
	for _, group := range technology.Groups {
		groups = append(groups, toGameTechnologyGroupResponse(group))
	}
	return gameTechnologySummary{
		Commander:      technology.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(technology.CurrentPlanet),
		PlanetSwitcher: planets,
		Groups:         groups,
	}
}

func toGameTechnologyGroupResponse(group domaingame.TechnologyGroup) gameTechnologyGroupResponse {
	items := make([]gameTechnologyItemResponse, 0, len(group.Items))
	for _, item := range group.Items {
		items = append(items, toGameTechnologyItemResponse(item))
	}
	return gameTechnologyGroupResponse{
		Key:   group.Key,
		Name:  group.Name,
		Items: items,
	}
}

func toGameTechnologyItemResponse(item domaingame.TechnologyItem) gameTechnologyItemResponse {
	requirements := make([]gameTechnologyRequirementResponse, 0, len(item.Requirements))
	for _, requirement := range item.Requirements {
		requirements = append(requirements, gameTechnologyRequirementResponse{
			ID:           requirement.ID,
			Name:         requirement.Name,
			Level:        requirement.Level,
			CurrentLevel: requirement.CurrentLevel,
			Met:          requirement.Met,
		})
	}
	return gameTechnologyItemResponse{
		ID:               item.ID,
		Name:             item.Name,
		Requirements:     requirements,
		DetailsAvailable: item.DetailsAvailable,
	}
}

package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameBuildingsResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Buildings     *gameBuildingsSummary      `json:"buildings,omitempty"`
}

type gameBuildingsSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Items          []gameBuildingItemResponse  `json:"items"`
}

type gameBuildingItemResponse struct {
	ID              int                      `json:"id"`
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	Level           int                      `json:"level"`
	NextLevel       int                      `json:"nextLevel"`
	Cost            gameBuildingCostResponse `json:"cost"`
	DurationSeconds int                      `json:"durationSeconds"`
	CanBuild        bool                     `json:"canBuild"`
	Action          string                   `json:"action"`
}

type gameBuildingCostResponse struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
	Energy    float64 `json:"energy"`
}

func (a app) handleGameBuildings(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameBuildings == nil {
		http.Error(w, "game buildings unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameBuildings.GetBuildings(r.Context(), appgame.BuildingsCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game buildings unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var buildings *gameBuildingsSummary
	if result.Authenticated {
		mapped := toGameBuildingsSummary(result.Buildings)
		buildings = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameBuildingsResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Buildings:     buildings,
	})
}

func toGameBuildingsSummary(buildings domaingame.Buildings) gameBuildingsSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(buildings.PlanetSwitcher))
	for _, planet := range buildings.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	items := make([]gameBuildingItemResponse, 0, len(buildings.Items))
	for _, item := range buildings.Items {
		items = append(items, toGameBuildingItemResponse(item))
	}
	return gameBuildingsSummary{
		Commander:      buildings.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(buildings.CurrentPlanet),
		PlanetSwitcher: planets,
		Items:          items,
	}
}

func toGameBuildingItemResponse(item domaingame.BuildingItem) gameBuildingItemResponse {
	return gameBuildingItemResponse{
		ID:              item.ID,
		Name:            item.Name,
		Description:     item.Description,
		Level:           item.Level,
		NextLevel:       item.NextLevel,
		Cost:            toGameBuildingCostResponse(item.Cost),
		DurationSeconds: item.DurationSeconds,
		CanBuild:        item.CanBuild,
		Action:          item.Action,
	}
}

func toGameBuildingCostResponse(cost domaingame.BuildingCost) gameBuildingCostResponse {
	return gameBuildingCostResponse{
		Metal:     cost.Metal,
		Crystal:   cost.Crystal,
		Deuterium: cost.Deuterium,
		Energy:    cost.Energy,
	}
}

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
	ActionIssue   *gameBuildingsActionIssue  `json:"actionIssue,omitempty"`
	Buildings     *gameBuildingsSummary      `json:"buildings,omitempty"`
}

type gameBuildingsActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameBuildingsMutationRequest struct {
	Action string `json:"action"`
	TechID int    `json:"techId"`
	ListID int    `json:"listId"`
}

type gameBuildingsSummary struct {
	Commander       string                      `json:"commander"`
	CommanderActive bool                        `json:"commanderActive"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Queue           []gameBuildingQueueResponse `json:"queue"`
	Items           []gameBuildingItemResponse  `json:"items"`
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

type gameBuildingQueueResponse struct {
	ListID           int    `json:"listId"`
	TechID           int    `json:"techId"`
	Name             string `json:"name"`
	Level            int    `json:"level"`
	Destroy          bool   `json:"destroy"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	RemainingSeconds int    `json:"remainingSeconds"`
}

func (a app) handleGameBuildings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameBuildingsGet(w, r)
	case http.MethodPost:
		a.handleGameBuildingsPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameBuildingsGet(w http.ResponseWriter, r *http.Request) {
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

	writeGameBuildingsResponse(w, result)
}

func (a app) handleGameBuildingsPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameBuildings == nil {
		http.Error(w, "game buildings unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	var mutation gameBuildingsMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		http.Error(w, "invalid buildings mutation", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameBuildings.MutateBuildings(r.Context(), appgame.BuildingsMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          mutation.Action,
		TechID:          mutation.TechID,
		ListID:          mutation.ListID,
	})
	if err != nil {
		http.Error(w, "game buildings unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameBuildingsResponse(w, result)
}

func writeGameBuildingsResponse(w http.ResponseWriter, result appgame.BuildingsResult) {
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
		ActionIssue:   toGameBuildingsActionIssue(result.ActionIssue),
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
	queue := make([]gameBuildingQueueResponse, 0, len(buildings.Queue))
	for _, entry := range buildings.Queue {
		queue = append(queue, toGameBuildingQueueResponse(entry))
	}
	return gameBuildingsSummary{
		Commander:       buildings.Commander,
		CommanderActive: buildings.CommanderActive,
		CurrentPlanet:   toGamePlanetOverviewResponse(buildings.CurrentPlanet),
		PlanetSwitcher:  planets,
		Queue:           queue,
		Items:           items,
	}
}

func toGameBuildingsActionIssue(issue *domaingame.BuildingsActionIssue) *gameBuildingsActionIssue {
	if issue == nil {
		return nil
	}
	return &gameBuildingsActionIssue{Code: issue.Code, Message: issue.Message}
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

func toGameBuildingQueueResponse(entry domaingame.BuildingQueueEntry) gameBuildingQueueResponse {
	return gameBuildingQueueResponse{
		ListID:           entry.ListID,
		TechID:           entry.TechID,
		Name:             entry.Name,
		Level:            entry.Level,
		Destroy:          entry.Destroy,
		Start:            entry.Start,
		End:              entry.End,
		RemainingSeconds: entry.RemainingSeconds,
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

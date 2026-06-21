package httpdelivery

import (
	"encoding/json"
	"fmt"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameShipyardResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	ActionIssue   *gameBuildingsActionIssue  `json:"actionIssue,omitempty"`
	Shipyard      *gameShipyardSummary       `json:"shipyard,omitempty"`
}

type gameShipyardSummary struct {
	Commander       string                      `json:"commander"`
	CommanderActive bool                        `json:"commanderActive"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasShipyard     bool                        `json:"hasShipyard"`
	Busy            bool                        `json:"busy"`
	Queue           []gameShipyardQueueResponse `json:"queue"`
	Items           []gameShipyardItemResponse  `json:"items"`
}

type gameShipyardQueueResponse struct {
	TaskID           int    `json:"taskId"`
	UnitID           int    `json:"unitId"`
	Name             string `json:"name"`
	Count            int    `json:"count"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	RemainingSeconds int    `json:"remainingSeconds"`
}

type gameShipyardItemResponse struct {
	ID               int                      `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description"`
	Count            int                      `json:"count"`
	Cost             gameBuildingCostResponse `json:"cost"`
	DurationSeconds  int                      `json:"durationSeconds"`
	CanBuild         bool                     `json:"canBuild"`
	MeetsRequirement bool                     `json:"meetsRequirement"`
	MaxBuild         int                      `json:"maxBuild"`
	BlockedReason    string                   `json:"blockedReason"`
}

type gameShipyardMutationRequest struct {
	Orders map[string]int `json:"orders"`
}

func (a app) handleGameShipyard(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameShipyardGet(w, r)
	case http.MethodPost:
		a.handleGameShipyardPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameShipyardGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameShipyard == nil {
		http.Error(w, "game shipyard unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameShipyard.GetShipyard(r.Context(), appgame.ShipyardCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game shipyard unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameShipyardResponse(w, result)
}

func (a app) handleGameShipyardPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameShipyard == nil {
		http.Error(w, "game shipyard unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	var mutation gameShipyardMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		http.Error(w, "invalid shipyard mutation", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameShipyard.MutateShipyard(r.Context(), appgame.ShipyardMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Orders:          parseIntegerOrderMap(mutation.Orders),
	})
	if err != nil {
		http.Error(w, "game shipyard unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameShipyardResponse(w, result)
}

func writeGameShipyardResponse(w http.ResponseWriter, result appgame.ShipyardResult) {
	status := http.StatusOK
	var shipyard *gameShipyardSummary
	if result.Authenticated {
		mapped := toGameShipyardSummary(result.Shipyard)
		shipyard = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameShipyardResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		ActionIssue:   toGameBuildingsActionIssue(result.ActionIssue),
		Shipyard:      shipyard,
	})
}

func parseIntegerOrderMap(raw map[string]int) map[int]int {
	orders := make(map[int]int, len(raw))
	for key, amount := range raw {
		var id int
		if _, err := fmt.Sscanf(key, "%d", &id); err != nil || id <= 0 {
			continue
		}
		orders[id] = amount
	}
	return orders
}

func toGameShipyardSummary(shipyard domaingame.Shipyard) gameShipyardSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(shipyard.PlanetSwitcher))
	for _, planet := range shipyard.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	items := make([]gameShipyardItemResponse, 0, len(shipyard.Items))
	for _, item := range shipyard.Items {
		items = append(items, toGameShipyardItemResponse(item))
	}
	queue := make([]gameShipyardQueueResponse, 0, len(shipyard.Queue))
	for _, entry := range shipyard.Queue {
		queue = append(queue, toGameShipyardQueueResponse(entry))
	}
	return gameShipyardSummary{
		Commander:       shipyard.Commander,
		CommanderActive: shipyard.CommanderActive,
		CurrentPlanet:   toGamePlanetOverviewResponse(shipyard.CurrentPlanet),
		PlanetSwitcher:  planets,
		HasShipyard:     shipyard.HasShipyard,
		Busy:            shipyard.Busy,
		Queue:           queue,
		Items:           items,
	}
}

func toGameShipyardQueueResponse(entry domaingame.ShipyardQueueEntry) gameShipyardQueueResponse {
	return gameShipyardQueueResponse{
		TaskID:           entry.TaskID,
		UnitID:           entry.UnitID,
		Name:             entry.Name,
		Count:            entry.Count,
		Start:            entry.Start,
		End:              entry.End,
		RemainingSeconds: entry.RemainingSeconds,
	}
}

func toGameShipyardItemResponse(item domaingame.ShipyardItem) gameShipyardItemResponse {
	return gameShipyardItemResponse{
		ID:               item.ID,
		Name:             item.Name,
		Description:      item.Description,
		Count:            item.Count,
		Cost:             toGameBuildingCostResponse(item.Cost),
		DurationSeconds:  item.DurationSeconds,
		CanBuild:         item.CanBuild,
		MeetsRequirement: item.MeetsRequirement,
		MaxBuild:         item.MaxBuild,
		BlockedReason:    item.BlockedReason,
	}
}

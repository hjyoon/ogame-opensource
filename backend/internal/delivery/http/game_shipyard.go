package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameShipyardResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Shipyard      *gameShipyardSummary       `json:"shipyard,omitempty"`
}

type gameShipyardSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasShipyard    bool                        `json:"hasShipyard"`
	Busy           bool                        `json:"busy"`
	Items          []gameShipyardItemResponse  `json:"items"`
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

func (a app) handleGameShipyard(w http.ResponseWriter, r *http.Request) {
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
		Shipyard:      shipyard,
	})
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
	return gameShipyardSummary{
		Commander:      shipyard.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(shipyard.CurrentPlanet),
		PlanetSwitcher: planets,
		HasShipyard:    shipyard.HasShipyard,
		Busy:           shipyard.Busy,
		Items:          items,
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

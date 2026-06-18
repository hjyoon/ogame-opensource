package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameDefenseResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Defense       *gameDefenseSummary        `json:"defense,omitempty"`
}

type gameDefenseSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasShipyard    bool                        `json:"hasShipyard"`
	Busy           bool                        `json:"busy"`
	Items          []gameShipyardItemResponse  `json:"items"`
}

func (a app) handleGameDefense(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameDefense == nil {
		http.Error(w, "game defense unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameDefense.GetDefense(r.Context(), appgame.DefenseCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game defense unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var defense *gameDefenseSummary
	if result.Authenticated {
		mapped := toGameDefenseSummary(result.Defense)
		defense = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameDefenseResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Defense:       defense,
	})
}

func toGameDefenseSummary(defense domaingame.Defense) gameDefenseSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(defense.PlanetSwitcher))
	for _, planet := range defense.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	items := make([]gameShipyardItemResponse, 0, len(defense.Items))
	for _, item := range defense.Items {
		items = append(items, toGameShipyardItemResponse(item))
	}
	return gameDefenseSummary{
		Commander:      defense.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(defense.CurrentPlanet),
		PlanetSwitcher: planets,
		HasShipyard:    defense.HasShipyard,
		Busy:           defense.Busy,
		Items:          items,
	}
}

package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameResourceProductionResponse struct {
	Authenticated bool                           `json:"authenticated"`
	Issues        []gameSessionIssueResponse     `json:"issues"`
	Resources     *gameResourceProductionSummary `json:"resources,omitempty"`
}

type gameResourceProductionSummary struct {
	Commander      string                       `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse   `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse  `json:"planetSwitcher"`
	Factor         float64                      `json:"factor"`
	Natural        gameResourceProductionValues `json:"natural"`
	Rows           []gameResourceProductionRow  `json:"rows"`
	Storage        gameResourceProductionValues `json:"storage"`
	Totals         gameResourceProductionTotals `json:"totals"`
}

type gameResourceProductionRow struct {
	ID      int                          `json:"id"`
	Name    string                       `json:"name"`
	Level   int                          `json:"level"`
	Percent int                          `json:"percent"`
	Values  gameResourceProductionValues `json:"values"`
}

type gameResourceProductionValues struct {
	Metal        float64 `json:"metal"`
	Crystal      float64 `json:"crystal"`
	Deuterium    float64 `json:"deuterium"`
	Energy       float64 `json:"energy"`
	EnergyRaw    float64 `json:"energyRaw"`
	EnergyStored bool    `json:"energyStored"`
}

type gameResourceProductionTotals struct {
	Hour gameResourceProductionValues `json:"hour"`
	Day  gameResourceProductionValues `json:"day"`
	Week gameResourceProductionValues `json:"week"`
}

func (a app) handleGameResources(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameResources == nil {
		http.Error(w, "game resources unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameResources.GetResources(r.Context(), appgame.ResourcesCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game resources unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var resources *gameResourceProductionSummary
	if result.Authenticated {
		mapped := toGameResourceProductionSummary(result.Resources)
		resources = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameResourceProductionResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Resources:     resources,
	})
}

func toGameResourceProductionSummary(resources domaingame.ResourceProduction) gameResourceProductionSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(resources.PlanetSwitcher))
	for _, planet := range resources.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameResourceProductionRow, 0, len(resources.Rows))
	for _, row := range resources.Rows {
		rows = append(rows, gameResourceProductionRow{
			ID:      row.ID,
			Name:    row.Name,
			Level:   row.Level,
			Percent: row.Percent,
			Values:  toGameResourceProductionValues(row.Values),
		})
	}
	return gameResourceProductionSummary{
		Commander:      resources.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(resources.CurrentPlanet),
		PlanetSwitcher: planets,
		Factor:         resources.Factor,
		Natural:        toGameResourceProductionValues(resources.Natural),
		Rows:           rows,
		Storage:        toGameResourceProductionValues(resources.Storage),
		Totals: gameResourceProductionTotals{
			Hour: toGameResourceProductionValues(resources.Totals.Hour),
			Day:  toGameResourceProductionValues(resources.Totals.Day),
			Week: toGameResourceProductionValues(resources.Totals.Week),
		},
	}
}

func toGameResourceProductionValues(values domaingame.ResourceProductionValues) gameResourceProductionValues {
	return gameResourceProductionValues{
		Metal:        values.Metal,
		Crystal:      values.Crystal,
		Deuterium:    values.Deuterium,
		Energy:       values.Energy,
		EnergyRaw:    values.EnergyRaw,
		EnergyStored: values.EnergyStored,
	}
}

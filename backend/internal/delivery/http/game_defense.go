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
	ActionIssue   *gameBuildingsActionIssue  `json:"actionIssue,omitempty"`
	Defense       *gameDefenseSummary        `json:"defense,omitempty"`
}

type gameDefenseSummary struct {
	Commander       string                      `json:"commander"`
	CommanderActive bool                        `json:"commanderActive"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	HasShipyard     bool                        `json:"hasShipyard"`
	Busy            bool                        `json:"busy"`
	Queue           []gameShipyardQueueResponse `json:"queue"`
	Items           []gameShipyardItemResponse  `json:"items"`
}

func (a app) handleGameDefense(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameDefenseGet(w, r)
	case http.MethodPost:
		a.handleGameDefensePost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameDefenseGet(w http.ResponseWriter, r *http.Request) {
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

	writeGameDefenseResponse(w, result)
}

func (a app) handleGameDefensePost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameDefense == nil {
		http.Error(w, "game defense unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	var mutation gameShipyardMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		http.Error(w, "invalid defense mutation", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameDefense.MutateDefense(r.Context(), appgame.DefenseMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Orders:          parseIntegerOrderMap(mutation.Orders),
	})
	if err != nil {
		http.Error(w, "game defense unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameDefenseResponse(w, result)
}

func writeGameDefenseResponse(w http.ResponseWriter, result appgame.DefenseResult) {
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
		ActionIssue:   toGameBuildingsActionIssue(result.ActionIssue),
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
	queue := make([]gameShipyardQueueResponse, 0, len(defense.Queue))
	for _, entry := range defense.Queue {
		queue = append(queue, toGameShipyardQueueResponse(entry))
	}
	return gameDefenseSummary{
		Commander:       defense.Commander,
		CommanderActive: defense.CommanderActive,
		CurrentPlanet:   toGamePlanetOverviewResponse(defense.CurrentPlanet),
		PlanetSwitcher:  planets,
		HasShipyard:     defense.HasShipyard,
		Busy:            defense.Busy,
		Queue:           queue,
		Items:           items,
	}
}

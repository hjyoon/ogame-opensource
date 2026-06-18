package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameOverviewResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Overview      *gameOverviewSummary       `json:"overview,omitempty"`
}

type gameOverviewSummary struct {
	Commander      string                      `json:"commander"`
	Score          gameScoreResponse           `json:"score"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
}

type gameScoreResponse struct {
	Points          int64 `json:"points"`
	RawScore        int64 `json:"rawScore"`
	Rank            int   `json:"rank"`
	UniversePlayers int   `json:"universePlayers"`
}

type gameCoordinatesResponse struct {
	Galaxy   int `json:"galaxy"`
	System   int `json:"system"`
	Position int `json:"position"`
}

type gameResourcesResponse struct {
	Metal             float64 `json:"metal"`
	Crystal           float64 `json:"crystal"`
	Deuterium         float64 `json:"deuterium"`
	MetalCapacity     int     `json:"metalCapacity"`
	CrystalCapacity   int     `json:"crystalCapacity"`
	DeuteriumCapacity int     `json:"deuteriumCapacity"`
}

type gamePlanetOverviewResponse struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Type        int                     `json:"type"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Diameter    int                     `json:"diameter"`
	Temperature int                     `json:"temperature"`
	Fields      int                     `json:"fields"`
	MaxFields   int                     `json:"maxFields"`
	Resources   gameResourcesResponse   `json:"resources"`
}

type gamePlanetSummaryResponse struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Type        int                     `json:"type"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Current     bool                    `json:"current"`
}

type gameOverviewMutationRequest struct {
	Action string `json:"action"`
	Name   string `json:"name"`
}

func (a app) handleGameOverview(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameOverviewGet(w, r)
	case http.MethodPost:
		a.handleGameOverviewPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameOverviewGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOverview == nil {
		http.Error(w, "game overview unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameOverview.GetOverview(r.Context(), appgame.OverviewCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game overview unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameOverviewResponse(w, result)
}

func (a app) handleGameOverviewPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOverview == nil {
		http.Error(w, "game overview unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	mutation, err := decodeGameOverviewMutation(r)
	if err != nil || mutation.Action != "rename" {
		http.Error(w, "invalid overview request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameOverview.RenamePlanet(r.Context(), appgame.OverviewRenameCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Name:            mutation.Name,
	})
	if err != nil {
		http.Error(w, "game overview unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameOverviewResponse(w, result)
}

func decodeGameOverviewMutation(r *http.Request) (gameOverviewMutationRequest, error) {
	defer r.Body.Close()
	var request gameOverviewMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return gameOverviewMutationRequest{}, err
	}
	return request, nil
}

func writeGameOverviewResponse(w http.ResponseWriter, result appgame.OverviewResult) {
	status := http.StatusOK
	var overview *gameOverviewSummary
	if result.Authenticated {
		mapped := toGameOverviewSummary(result.Overview)
		overview = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameOverviewResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Overview:      overview,
	})
}

func selectedPlanetID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("cp")
	if raw == "" {
		return 0, nil
	}
	planetID, err := strconv.Atoi(raw)
	if err != nil || planetID < 0 {
		return 0, strconv.ErrSyntax
	}
	return planetID, nil
}

func toGameOverviewSummary(overview domaingame.Overview) gameOverviewSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(overview.PlanetSwitcher))
	for _, planet := range overview.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	return gameOverviewSummary{
		Commander: overview.Commander,
		Score: gameScoreResponse{
			Points:          overview.Score.DisplayPoints(),
			RawScore:        overview.Score.RawScore,
			Rank:            overview.Score.Rank,
			UniversePlayers: overview.Score.UniversePlayers,
		},
		CurrentPlanet:  toGamePlanetOverviewResponse(overview.CurrentPlanet),
		PlanetSwitcher: planets,
	}
}

func toGamePlanetOverviewResponse(planet domaingame.PlanetOverview) gamePlanetOverviewResponse {
	return gamePlanetOverviewResponse{
		ID:          planet.ID,
		Name:        planet.Name,
		Type:        planet.Type,
		Coordinates: toGameCoordinatesResponse(planet.Coordinates),
		Diameter:    planet.Diameter,
		Temperature: planet.Temperature,
		Fields:      planet.Fields,
		MaxFields:   planet.MaxFields,
		Resources: gameResourcesResponse{
			Metal:             planet.Resources.Metal,
			Crystal:           planet.Resources.Crystal,
			Deuterium:         planet.Resources.Deuterium,
			MetalCapacity:     planet.Resources.MetalCapacity,
			CrystalCapacity:   planet.Resources.CrystalCapacity,
			DeuteriumCapacity: planet.Resources.DeuteriumCapacity,
		},
	}
}

func toGamePlanetSummaryResponse(planet domaingame.PlanetSummary) gamePlanetSummaryResponse {
	return gamePlanetSummaryResponse{
		ID:          planet.ID,
		Name:        planet.Name,
		Type:        planet.Type,
		Coordinates: toGameCoordinatesResponse(planet.Coordinates),
		Current:     planet.Current,
	}
}

func toGameCoordinatesResponse(coordinates domaingame.Coordinates) gameCoordinatesResponse {
	return gameCoordinatesResponse{
		Galaxy:   coordinates.Galaxy,
		System:   coordinates.System,
		Position: coordinates.Position,
	}
}

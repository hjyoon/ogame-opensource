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
	ActionIssue   *gameOverviewActionIssue   `json:"actionIssue,omitempty"`
}

type gameOverviewActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameOverviewSummary struct {
	Commander      string                      `json:"commander"`
	ServerTime     string                      `json:"serverTime"`
	Score          gameScoreResponse           `json:"score"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Messages       []string                    `json:"messages,omitempty"`
	UnreadMessages int                         `json:"unreadMessages"`
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
	BuildQueue  *gameBuildQueueResponse `json:"buildQueue,omitempty"`
}

type gamePlanetSummaryResponse struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Type        int                     `json:"type"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Current     bool                    `json:"current"`
	BuildQueue  *gameBuildQueueResponse `json:"buildQueue,omitempty"`
}

type gameBuildQueueResponse struct {
	TechID  int    `json:"techId"`
	Name    string `json:"name"`
	Level   int    `json:"level"`
	Destroy bool   `json:"destroy"`
	End     int64  `json:"end"`
}

type gameOverviewMutationRequest struct {
	Action   string `json:"action"`
	Name     string `json:"name"`
	DeleteID int    `json:"deleteId"`
	Password string `json:"password"`
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
		Login:           hasOverviewLoginMarker(r),
	})
	if err != nil {
		http.Error(w, "game overview unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameOverviewResponse(w, result)
}

func hasOverviewLoginMarker(r *http.Request) bool {
	_, ok := r.URL.Query()["lgn"]
	return ok
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
	if err != nil {
		http.Error(w, "invalid overview request", http.StatusBadRequest)
		return
	}
	var result appgame.OverviewResult
	switch mutation.Action {
	case "rename":
		result, err = a.deps.GameOverview.RenamePlanet(r.Context(), appgame.OverviewRenameCommand{
			PublicSession:   r.URL.Query().Get("session"),
			PrivateSessions: cookieMap(r),
			RemoteAddr:      remoteIP(r.RemoteAddr),
			PlanetID:        planetID,
			Name:            mutation.Name,
		})
	case "delete":
		if mutation.DeleteID <= 0 {
			http.Error(w, "invalid overview request", http.StatusBadRequest)
			return
		}
		result, err = a.deps.GameOverview.DeletePlanet(r.Context(), appgame.OverviewDeleteCommand{
			PublicSession:   r.URL.Query().Get("session"),
			PrivateSessions: cookieMap(r),
			RemoteAddr:      remoteIP(r.RemoteAddr),
			PlanetID:        planetID,
			DeleteID:        mutation.DeleteID,
			Password:        mutation.Password,
		})
	default:
		http.Error(w, "invalid overview request", http.StatusBadRequest)
		return
	}
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
		ActionIssue:   toGameOverviewActionIssue(result.ActionIssue),
	})
}

func toGameOverviewActionIssue(issue *domaingame.OverviewActionIssue) *gameOverviewActionIssue {
	if issue == nil {
		return nil
	}
	return &gameOverviewActionIssue{Code: issue.Code, Message: issue.Message}
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
		Commander:  overview.Commander,
		ServerTime: overview.ServerTime,
		Score: gameScoreResponse{
			Points:          overview.Score.DisplayPoints(),
			RawScore:        overview.Score.RawScore,
			Rank:            overview.Score.Rank,
			UniversePlayers: overview.Score.UniversePlayers,
		},
		CurrentPlanet:  toGamePlanetOverviewResponse(overview.CurrentPlanet),
		PlanetSwitcher: planets,
		Messages:       overview.Messages,
		UnreadMessages: overview.UnreadMessages,
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
		BuildQueue: toGameBuildQueueResponse(planet.BuildQueue),
	}
}

func toGamePlanetSummaryResponse(planet domaingame.PlanetSummary) gamePlanetSummaryResponse {
	return gamePlanetSummaryResponse{
		ID:          planet.ID,
		Name:        planet.Name,
		Type:        planet.Type,
		Coordinates: toGameCoordinatesResponse(planet.Coordinates),
		Current:     planet.Current,
		BuildQueue:  toGameBuildQueueResponse(planet.BuildQueue),
	}
}

func toGameBuildQueueResponse(queue *domaingame.OverviewBuildQueue) *gameBuildQueueResponse {
	if queue == nil {
		return nil
	}
	return &gameBuildQueueResponse{
		TechID:  queue.TechID,
		Name:    queue.Name,
		Level:   queue.Level,
		Destroy: queue.Destroy,
		End:     queue.End,
	}
}

func toGameCoordinatesResponse(coordinates domaingame.Coordinates) gameCoordinatesResponse {
	return gameCoordinatesResponse{
		Galaxy:   coordinates.Galaxy,
		System:   coordinates.System,
		Position: coordinates.Position,
	}
}

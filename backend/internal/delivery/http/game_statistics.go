package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameStatisticsResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Statistics    *gameStatisticsSummary     `json:"statistics,omitempty"`
}

type gameStatisticsSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Who            string                      `json:"who"`
	Type           string                      `json:"type"`
	Start          int                         `json:"start"`
	Total          int                         `json:"total"`
	GeneratedAt    int64                       `json:"generatedAt"`
	Rows           []gameStatisticsRowResponse `json:"rows"`
}

type gameStatisticsRowResponse struct {
	Place         int                             `json:"place"`
	PreviousPlace int                             `json:"previousPlace"`
	Delta         int                             `json:"delta"`
	Score         int64                           `json:"score"`
	DisplayScore  int64                           `json:"displayScore"`
	Members       int                             `json:"members"`
	PerMember     int64                           `json:"perMember"`
	ScoreDate     int64                           `json:"scoreDate"`
	Player        gameStatisticsPlayerResponse    `json:"player"`
	Alliance      *gameStatisticsAllianceResponse `json:"alliance,omitempty"`
	Coordinates   gameCoordinatesResponse         `json:"coordinates"`
	Own           bool                            `json:"own"`
	SameAlliance  bool                            `json:"sameAlliance"`
}

type gameStatisticsPlayerResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type gameStatisticsAllianceResponse struct {
	ID  int    `json:"id"`
	Tag string `json:"tag"`
}

func (a app) handleGameStatistics(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameStatistics == nil {
		http.Error(w, "game statistics unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	start, err := selectedStatisticsStart(r)
	if err != nil {
		http.Error(w, "invalid statistics start", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameStatistics.GetStatistics(r.Context(), appgame.StatisticsCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Who:             r.URL.Query().Get("who"),
		Type:            r.URL.Query().Get("type"),
		Start:           start,
	})
	if err != nil {
		http.Error(w, "game statistics unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var statistics *gameStatisticsSummary
	if result.Authenticated {
		mapped := toGameStatisticsSummary(result.Statistics)
		statistics = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameStatisticsResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Statistics:    statistics,
	})
}

func toGameStatisticsSummary(statistics domaingame.Statistics) gameStatisticsSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(statistics.PlanetSwitcher))
	for _, planet := range statistics.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameStatisticsRowResponse, 0, len(statistics.Rows))
	for _, row := range statistics.Rows {
		rows = append(rows, toGameStatisticsRowResponse(row, statistics.Type))
	}
	return gameStatisticsSummary{
		Commander:      statistics.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(statistics.CurrentPlanet),
		PlanetSwitcher: planets,
		Who:            statistics.Who,
		Type:           statistics.Type,
		Start:          statistics.Start,
		Total:          statistics.Total,
		GeneratedAt:    statistics.GeneratedAt,
		Rows:           rows,
	}
}

func toGameStatisticsRowResponse(row domaingame.StatisticsRow, statType string) gameStatisticsRowResponse {
	var alliance *gameStatisticsAllianceResponse
	if row.Alliance != nil {
		alliance = &gameStatisticsAllianceResponse{ID: row.Alliance.ID, Tag: row.Alliance.Tag}
	}
	return gameStatisticsRowResponse{
		Place:         row.Place,
		PreviousPlace: row.PreviousPlace,
		Delta:         row.PlaceDelta(),
		Score:         row.Score,
		DisplayScore:  row.DisplayScore(statType),
		Members:       row.Members,
		PerMember:     row.DisplayScorePerMember(statType),
		ScoreDate:     row.ScoreDate,
		Player: gameStatisticsPlayerResponse{
			ID:   row.Player.ID,
			Name: row.Player.Name,
		},
		Alliance:     alliance,
		Coordinates:  toGameCoordinatesResponse(row.Coordinates),
		Own:          row.Own,
		SameAlliance: row.SameAlliance,
	}
}

func selectedStatisticsStart(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("start")
	if raw == "" {
		return 0, nil
	}
	start, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return start, nil
}

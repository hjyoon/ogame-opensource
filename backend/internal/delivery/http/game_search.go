package httpdelivery

import (
	"encoding/json"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameSearchResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Search        *gameSearchSummary         `json:"search,omitempty"`
}

type gameSearchSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Type           string                      `json:"type"`
	Text           string                      `json:"text"`
	Message        string                      `json:"message"`
	PlayerRows     []gameSearchPlayerRow       `json:"playerRows"`
	AllianceRows   []gameSearchAllianceRow     `json:"allianceRows"`
}

type gameSearchPlayerRow struct {
	PlayerID     int                             `json:"playerId"`
	PlayerName   string                          `json:"playerName"`
	Alliance     *gameStatisticsAllianceResponse `json:"alliance,omitempty"`
	PlanetID     int                             `json:"planetId"`
	PlanetName   string                          `json:"planetName"`
	Coordinates  gameCoordinatesResponse         `json:"coordinates"`
	Place        int                             `json:"place"`
	Own          bool                            `json:"own"`
	SameAlliance bool                            `json:"sameAlliance"`
}

type gameSearchAllianceRow struct {
	AllianceID   int    `json:"allianceId"`
	Tag          string `json:"tag"`
	Name         string `json:"name"`
	Members      int    `json:"members"`
	Score        int64  `json:"score"`
	DisplayScore int64  `json:"displayScore"`
	Own          bool   `json:"own"`
}

func (a app) handleGameSearch(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameSearch == nil {
		http.Error(w, "game search unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameSearch.GetSearch(r.Context(), appgame.SearchCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Type:            r.URL.Query().Get("type"),
		Text:            r.URL.Query().Get("searchtext"),
	})
	if err != nil {
		http.Error(w, "game search unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var search *gameSearchSummary
	if result.Authenticated {
		mapped := toGameSearchSummary(result.Search)
		search = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameSearchResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Search:        search,
	})
}

func toGameSearchSummary(search domaingame.Search) gameSearchSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(search.PlanetSwitcher))
	for _, planet := range search.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	playerRows := make([]gameSearchPlayerRow, 0, len(search.PlayerRows))
	for _, row := range search.PlayerRows {
		playerRows = append(playerRows, toGameSearchPlayerRow(row))
	}
	allianceRows := make([]gameSearchAllianceRow, 0, len(search.AllianceRows))
	for _, row := range search.AllianceRows {
		allianceRows = append(allianceRows, toGameSearchAllianceRow(row))
	}
	return gameSearchSummary{
		Commander:      search.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(search.CurrentPlanet),
		PlanetSwitcher: planets,
		Type:           search.Type,
		Text:           search.Text,
		Message:        search.Message,
		PlayerRows:     playerRows,
		AllianceRows:   allianceRows,
	}
}

func toGameSearchPlayerRow(row domaingame.SearchPlayerRow) gameSearchPlayerRow {
	var alliance *gameStatisticsAllianceResponse
	if row.Alliance != nil {
		alliance = &gameStatisticsAllianceResponse{ID: row.Alliance.ID, Tag: row.Alliance.Tag}
	}
	return gameSearchPlayerRow{
		PlayerID:     row.PlayerID,
		PlayerName:   row.PlayerName,
		Alliance:     alliance,
		PlanetID:     row.PlanetID,
		PlanetName:   row.PlanetName,
		Coordinates:  toGameCoordinatesResponse(row.Coordinates),
		Place:        row.Place,
		Own:          row.Own,
		SameAlliance: row.SameAlliance,
	}
}

func toGameSearchAllianceRow(row domaingame.SearchAllianceRow) gameSearchAllianceRow {
	return gameSearchAllianceRow{
		AllianceID:   row.AllianceID,
		Tag:          row.Tag,
		Name:         row.Name,
		Members:      row.Members,
		Score:        row.Score,
		DisplayScore: row.DisplayScore(),
		Own:          row.Own,
	}
}

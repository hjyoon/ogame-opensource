package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gamePhalanxResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Phalanx       *gamePhalanxSummary        `json:"phalanx,omitempty"`
}

type gamePhalanxSummary struct {
	Commander          string                      `json:"commander"`
	CurrentPlanet      gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher     []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Source             gamePhalanxPlanetResponse   `json:"source"`
	Target             gamePhalanxPlanetResponse   `json:"target"`
	Events             []gameFleetMissionResponse  `json:"events"`
	ActionIssue        *gamePhalanxActionIssue     `json:"actionIssue,omitempty"`
	Cost               int                         `json:"cost"`
	RemainingDeuterium float64                     `json:"remainingDeuterium"`
	ReportHeading      string                      `json:"reportHeading"`
	EventsHeading      string                      `json:"eventsHeading"`
}

type gamePhalanxPlanetResponse struct {
	ID           int                     `json:"id"`
	OwnerID      int                     `json:"ownerId"`
	Name         string                  `json:"name"`
	Type         int                     `json:"type"`
	Coordinates  gameCoordinatesResponse `json:"coordinates"`
	PhalanxLevel int                     `json:"phalanxLevel"`
	Deuterium    float64                 `json:"deuterium"`
}

type gamePhalanxActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a app) handleGamePhalanx(w http.ResponseWriter, r *http.Request) {
	if a.deps.GamePhalanx == nil {
		http.Error(w, "game phalanx unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	targetPlanetID, err := selectedPhalanxTargetID(r)
	if err != nil {
		http.Error(w, "invalid phalanx target", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GamePhalanx.GetPhalanx(r.Context(), appgame.PhalanxCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		TargetPlanetID:  targetPlanetID,
	})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("game phalanx unavailable", "error", err.Error())
		}
		http.Error(w, "game phalanx unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var phalanx *gamePhalanxSummary
	if result.Authenticated {
		mapped := toGamePhalanxSummary(result.Phalanx)
		phalanx = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gamePhalanxResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Phalanx:       phalanx,
	})
}

func selectedPhalanxTargetID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("spid")
	if raw == "" {
		raw = r.URL.Query().Get("targetPlanetId")
	}
	targetID, err := strconv.Atoi(raw)
	if err != nil || targetID <= 0 {
		return 0, strconv.ErrSyntax
	}
	return targetID, nil
}

func toGamePhalanxSummary(phalanx domaingame.Phalanx) gamePhalanxSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(phalanx.PlanetSwitcher))
	for _, planet := range phalanx.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	events := make([]gameFleetMissionResponse, 0, len(phalanx.Events))
	for _, event := range phalanx.Events {
		events = append(events, toGameFleetMissionResponse(event))
	}
	return gamePhalanxSummary{
		Commander:          phalanx.Commander,
		CurrentPlanet:      toGamePlanetOverviewResponse(phalanx.CurrentPlanet),
		PlanetSwitcher:     planets,
		Source:             toGamePhalanxPlanetResponse(phalanx.Source),
		Target:             toGamePhalanxPlanetResponse(phalanx.Target),
		Events:             events,
		ActionIssue:        toGamePhalanxActionIssue(phalanx.ActionIssue),
		Cost:               phalanx.Cost,
		RemainingDeuterium: phalanx.RemainingDeuterium,
		ReportHeading:      domaingame.PhalanxReportHeading,
		EventsHeading:      domaingame.PhalanxEventsHeading,
	}
}

func toGamePhalanxPlanetResponse(planet domaingame.PhalanxPlanet) gamePhalanxPlanetResponse {
	return gamePhalanxPlanetResponse{
		ID:           planet.ID,
		OwnerID:      planet.OwnerID,
		Name:         planet.Name,
		Type:         planet.Type,
		Coordinates:  toGameCoordinatesResponse(planet.Coordinates),
		PhalanxLevel: planet.PhalanxLevel,
		Deuterium:    planet.Deuterium,
	}
}

func toGamePhalanxActionIssue(issue *domaingame.PhalanxActionIssue) *gamePhalanxActionIssue {
	if issue == nil {
		return nil
	}
	return &gamePhalanxActionIssue{Code: issue.Code, Message: issue.Message}
}

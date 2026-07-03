package httpdelivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameJumpGateResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	JumpGate      *gameJumpGateSummary       `json:"jumpGate,omitempty"`
}

type gameJumpGateSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Source         gameJumpGateMoonResponse    `json:"source"`
	Targets        []gameJumpGateMoonResponse  `json:"targets"`
	Ships          []gameJumpGateShipResponse  `json:"ships"`
	ActionIssue    *gameJumpGateActionIssue    `json:"actionIssue,omitempty"`
}

type gameJumpGateMoonResponse struct {
	ID          int                     `json:"id"`
	OwnerID     int                     `json:"ownerId"`
	Name        string                  `json:"name"`
	Type        int                     `json:"type"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	GateLevel   int                     `json:"gateLevel"`
	GateUntil   int64                   `json:"gateUntil"`
}

type gameJumpGateShipResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type gameJumpGateActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameJumpGateMutationRequest struct {
	SourceMoonID int         `json:"sourceMoonId"`
	TargetMoonID int         `json:"targetMoonId"`
	Ships        map[int]int `json:"ships"`
}

func (a app) handleGameJumpGate(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameJumpGate == nil {
		http.Error(w, "game jump gate unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.handleGameJumpGateGet(w, r)
	case http.MethodPost:
		a.handleGameJumpGatePost(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameJumpGateGet(w http.ResponseWriter, r *http.Request) {
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameJumpGate.GetJumpGate(r.Context(), appgame.JumpGateCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	a.writeGameJumpGateResult(w, result, err)
}

func (a app) handleGameJumpGatePost(w http.ResponseWriter, r *http.Request) {
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	var body gameJumpGateMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid jump gate request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameJumpGate.Jump(r.Context(), appgame.JumpGateMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		SourceMoonID:    body.SourceMoonID,
		TargetMoonID:    body.TargetMoonID,
		Ships:           body.Ships,
	})
	a.writeGameJumpGateResult(w, result, err)
}

func (a app) writeGameJumpGateResult(w http.ResponseWriter, result appgame.JumpGateResult, err error) {
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("game jump gate unavailable", "error", err.Error())
		}
		http.Error(w, "game jump gate unavailable", http.StatusServiceUnavailable)
		return
	}
	status := http.StatusOK
	var jumpGate *gameJumpGateSummary
	if result.Authenticated {
		mapped := toGameJumpGateSummary(result.JumpGate)
		jumpGate = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameJumpGateResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		JumpGate:      jumpGate,
	})
}

func (a app) handleLegacyJumpGatePost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameJumpGate == nil {
		http.Error(w, "game jump gate unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid jump gate request", http.StatusBadRequest)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		planetID = legacyJumpGateInt(r.FormValue("qm"))
	}
	sourceID := legacyJumpGateInt(r.FormValue("qm"))
	targetID := legacyJumpGateInt(r.FormValue("zm"))
	result, err := a.deps.GameJumpGate.Jump(r.Context(), appgame.JumpGateMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		SourceMoonID:    sourceID,
		TargetMoonID:    targetID,
		Ships:           legacyJumpGateShips(r),
	})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("legacy jump gate unavailable", "error", err.Error())
		}
		http.Error(w, "game jump gate unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Authenticated {
		http.Error(w, "unauthenticated", http.StatusForbidden)
		return
	}
	if result.JumpGate.ActionIssue != nil && result.JumpGate.ActionIssue.Code != domaingame.JumpGateIssueMoved {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<center>\n%s<br></center>\n", result.JumpGate.ActionIssue.Message)
		return
	}
	target := result.JumpGate.Source.ID
	if targetID > 0 {
		target = targetID
	}
	redirect := "/game/index.php?page=infos&session=" + url.QueryEscape(r.URL.Query().Get("session")) + "&cp=" + strconv.Itoa(target) + "&gid=43"
	http.Redirect(w, r, redirect, http.StatusFound)
}

func toGameJumpGateSummary(jumpGate domaingame.JumpGate) gameJumpGateSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(jumpGate.PlanetSwitcher))
	for _, planet := range jumpGate.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	targets := make([]gameJumpGateMoonResponse, 0, len(jumpGate.Targets))
	for _, target := range jumpGate.Targets {
		targets = append(targets, toGameJumpGateMoonResponse(target))
	}
	ships := make([]gameJumpGateShipResponse, 0, len(jumpGate.Ships))
	for _, ship := range jumpGate.Ships {
		ships = append(ships, gameJumpGateShipResponse{ID: ship.ID, Name: ship.Name, Count: ship.Count})
	}
	return gameJumpGateSummary{
		Commander:      jumpGate.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(jumpGate.CurrentPlanet),
		PlanetSwitcher: planets,
		Source:         toGameJumpGateMoonResponse(jumpGate.Source),
		Targets:        targets,
		Ships:          ships,
		ActionIssue:    toGameJumpGateActionIssue(jumpGate.ActionIssue),
	}
}

func toGameJumpGateMoonResponse(moon domaingame.JumpGateMoon) gameJumpGateMoonResponse {
	return gameJumpGateMoonResponse{
		ID:          moon.ID,
		OwnerID:     moon.OwnerID,
		Name:        moon.Name,
		Type:        moon.Type,
		Coordinates: toGameCoordinatesResponse(moon.Coordinates),
		GateLevel:   moon.GateLevel,
		GateUntil:   moon.GateUntil,
	}
}

func toGameJumpGateActionIssue(issue *domaingame.JumpGateActionIssue) *gameJumpGateActionIssue {
	if issue == nil {
		return nil
	}
	return &gameJumpGateActionIssue{Code: issue.Code, Message: issue.Message}
}

func legacyJumpGateShips(r *http.Request) map[int]int {
	ships := make(map[int]int)
	for _, id := range domaingame.FleetIDs() {
		value := legacyJumpGateInt(r.FormValue("c" + strconv.Itoa(id)))
		if value != 0 {
			ships[id] = value
		}
	}
	return ships
}

func legacyJumpGateInt(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return number
}

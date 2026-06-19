package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameFleetResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Fleet         *gameFleetSummary          `json:"fleet,omitempty"`
}

type gameFleetSummary struct {
	Commander       string                      `json:"commander"`
	CommanderActive bool                        `json:"commanderActive"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Slots           gameFleetSlotsResponse      `json:"slots"`
	Expeditions     gameExpeditionSlotsResponse `json:"expeditions"`
	Missions        []gameFleetMissionResponse  `json:"missions"`
	Ships           []gameFleetShipResponse     `json:"ships"`
	Templates       gameFleetTemplatesResponse  `json:"templates"`
}

type gameFleetSlotsResponse struct {
	Used    int  `json:"used"`
	Max     int  `json:"max"`
	BaseMax int  `json:"baseMax"`
	Admiral bool `json:"admiral"`
}

type gameExpeditionSlotsResponse struct {
	Used int `json:"used"`
	Max  int `json:"max"`
}

type gameFleetMissionResponse struct {
	ID              int                          `json:"id"`
	Mission         int                          `json:"mission"`
	MissionName     string                       `json:"missionName"`
	StateTitle      string                       `json:"stateTitle"`
	StateShort      string                       `json:"stateShort"`
	Ships           []gameFleetShipCountResponse `json:"ships"`
	TotalShips      int                          `json:"totalShips"`
	Origin          gameCoordinatesResponse      `json:"origin"`
	Target          gameCoordinatesResponse      `json:"target"`
	TargetType      int                          `json:"targetType"`
	TargetOwnerName string                       `json:"targetOwnerName"`
	DepartureAt     int64                        `json:"departureAt"`
	ArrivalAt       int64                        `json:"arrivalAt"`
	CanRecall       bool                         `json:"canRecall"`
	CanCreateUnion  bool                         `json:"canCreateUnion"`
}

type gameFleetShipCountResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type gameFleetShipResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Speed       int    `json:"speed"`
	Cargo       int    `json:"cargo"`
	Consumption int    `json:"consumption"`
	Selectable  bool   `json:"selectable"`
}

type gameFleetTemplatesResponse struct {
	CommanderActive bool                        `json:"commanderActive"`
	Max             int                         `json:"max"`
	Items           []gameFleetTemplateResponse `json:"items"`
}

type gameFleetTemplateResponse struct {
	ID        int                             `json:"id"`
	Name      string                          `json:"name"`
	UpdatedAt int64                           `json:"updatedAt"`
	Ships     []gameFleetTemplateShipResponse `json:"ships"`
}

type gameFleetTemplateShipResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type gameFleetTemplateMutationRequest struct {
	Action     string         `json:"action"`
	TemplateID int            `json:"templateId"`
	Name       string         `json:"name"`
	Ships      map[string]int `json:"ships"`
}

type gameFleetMutationRequest struct {
	Action  string `json:"action"`
	FleetID int    `json:"fleetId"`
}

func (a app) handleGameFleet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameFleetGet(w, r)
	case http.MethodPost:
		a.handleGameFleetPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameFleetGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameFleet == nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameFleet.GetFleet(r.Context(), appgame.FleetCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameFleetResponse(w, result)
}

func (a app) handleGameFleetPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameFleet == nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	var payload gameFleetMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid fleet payload", http.StatusBadRequest)
		return
	}
	if payload.Action != "recall" {
		http.Error(w, "unsupported fleet action", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameFleet.RecallFleet(r.Context(), appgame.FleetRecallCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		FleetID:         payload.FleetID,
	})
	if err != nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameFleetResponse(w, result)
}

func (a app) handleGameFleetTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameFleetTemplatesGet(w, r)
	case http.MethodPost:
		a.handleGameFleetTemplatesPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameFleetTemplatesGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameFleet == nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameFleet.GetFleet(r.Context(), appgame.FleetCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameFleetResponse(w, result)
}

func (a app) handleGameFleetTemplatesPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameFleet == nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	var payload gameFleetTemplateMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid fleet template payload", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameFleet.MutateFleetTemplate(r.Context(), appgame.FleetTemplateMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		TemplateID:      payload.TemplateID,
		Action:          payload.Action,
		Name:            payload.Name,
		Ships:           parseFleetTemplateShips(payload.Ships),
	})
	if err != nil {
		http.Error(w, "game fleet unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameFleetResponse(w, result)
}

func writeGameFleetResponse(w http.ResponseWriter, result appgame.FleetResult) {
	status := http.StatusOK
	var fleet *gameFleetSummary
	if result.Authenticated {
		mapped := toGameFleetSummary(result.Fleet)
		fleet = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameFleetResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Fleet:         fleet,
	})
}

func toGameFleetSummary(fleet domaingame.Fleet) gameFleetSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(fleet.PlanetSwitcher))
	for _, planet := range fleet.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	missions := make([]gameFleetMissionResponse, 0, len(fleet.Missions))
	for _, mission := range fleet.Missions {
		missions = append(missions, toGameFleetMissionResponse(mission))
	}
	ships := make([]gameFleetShipResponse, 0, len(fleet.Ships))
	for _, ship := range fleet.Ships {
		ships = append(ships, toGameFleetShipResponse(ship))
	}
	return gameFleetSummary{
		Commander:       fleet.Commander,
		CommanderActive: fleet.CommanderActive,
		CurrentPlanet:   toGamePlanetOverviewResponse(fleet.CurrentPlanet),
		PlanetSwitcher:  planets,
		Slots: gameFleetSlotsResponse{
			Used:    fleet.Slots.Used,
			Max:     fleet.Slots.Max,
			BaseMax: fleet.Slots.BaseMax,
			Admiral: fleet.Slots.Admiral,
		},
		Expeditions: gameExpeditionSlotsResponse{
			Used: fleet.Expeditions.Used,
			Max:  fleet.Expeditions.Max,
		},
		Missions:  missions,
		Ships:     ships,
		Templates: toGameFleetTemplatesResponse(fleet),
	}
}

func toGameFleetMissionResponse(mission domaingame.FleetMission) gameFleetMissionResponse {
	ships := make([]gameFleetShipCountResponse, 0, len(mission.Ships))
	for _, ship := range mission.Ships {
		ships = append(ships, gameFleetShipCountResponse{
			ID:    ship.ID,
			Name:  ship.Name,
			Count: ship.Count,
		})
	}
	return gameFleetMissionResponse{
		ID:              mission.ID,
		Mission:         mission.Mission,
		MissionName:     mission.MissionName,
		StateTitle:      mission.StateTitle,
		StateShort:      mission.StateShort,
		Ships:           ships,
		TotalShips:      mission.TotalShips,
		Origin:          toGameCoordinatesResponse(mission.Origin),
		Target:          toGameCoordinatesResponse(mission.Target),
		TargetType:      mission.TargetType,
		TargetOwnerName: mission.TargetOwnerName,
		DepartureAt:     mission.DepartureAt,
		ArrivalAt:       mission.ArrivalAt,
		CanRecall:       mission.CanRecall,
		CanCreateUnion:  mission.CanCreateUnion,
	}
}

func toGameFleetShipResponse(ship domaingame.FleetShipSelection) gameFleetShipResponse {
	return gameFleetShipResponse{
		ID:          ship.ID,
		Name:        ship.Name,
		Count:       ship.Count,
		Speed:       ship.Speed,
		Cargo:       ship.Cargo,
		Consumption: ship.Consumption,
		Selectable:  ship.Selectable,
	}
}

func toGameFleetTemplatesResponse(fleet domaingame.Fleet) gameFleetTemplatesResponse {
	items := make([]gameFleetTemplateResponse, 0, len(fleet.Templates))
	for _, template := range fleet.Templates {
		ships := make([]gameFleetTemplateShipResponse, 0, len(template.Ships))
		for _, ship := range template.Ships {
			ships = append(ships, gameFleetTemplateShipResponse{
				ID:    ship.ID,
				Name:  ship.Name,
				Count: ship.Count,
			})
		}
		items = append(items, gameFleetTemplateResponse{
			ID:        template.ID,
			Name:      template.Name,
			UpdatedAt: template.UpdatedAt,
			Ships:     ships,
		})
	}
	return gameFleetTemplatesResponse{
		CommanderActive: fleet.CommanderActive,
		Max:             fleet.TemplateLimit,
		Items:           items,
	}
}

func parseFleetTemplateShips(raw map[string]int) map[int]int {
	ships := make(map[int]int, len(raw))
	for idText, count := range raw {
		id, err := strconv.Atoi(idText)
		if err != nil {
			continue
		}
		ships[id] = count
	}
	return ships
}

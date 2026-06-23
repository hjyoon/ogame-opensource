package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameGalaxyResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Galaxy        *gameGalaxySummary         `json:"galaxy,omitempty"`
	ActionIssue   *gameGalaxyActionIssue     `json:"actionIssue,omitempty"`
}

type gameGalaxyActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameGalaxyMutationRequest struct {
	Action          string `json:"action"`
	TargetPlanetID  int    `json:"targetPlanetId"`
	TargetGalaxy    int    `json:"targetGalaxy"`
	TargetSystem    int    `json:"targetSystem"`
	TargetPosition  int    `json:"targetPosition"`
	TargetType      int    `json:"targetType"`
	Amount          int    `json:"amount"`
	TargetDefenseID int    `json:"targetDefenseId"`
}

type gameGalaxySummary struct {
	Commander           string                      `json:"commander"`
	CurrentPlanet       gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher      []gamePlanetSummaryResponse `json:"planetSwitcher"`
	Coordinates         gameCoordinatesResponse     `json:"coordinates"`
	Bounds              gameGalaxyBoundsResponse    `json:"bounds"`
	Rows                []gameGalaxyRowResponse     `json:"rows"`
	Populated           int                         `json:"populated"`
	Slots               gameFleetSlotsResponse      `json:"slots"`
	Extra               gameGalaxyExtraResponse     `json:"extra"`
	NotEnoughDeuterium  bool                        `json:"notEnoughDeuterium"`
	RemoteSystemCostDue bool                        `json:"remoteSystemCostDue"`
}

type gameGalaxyBoundsResponse struct {
	Galaxies int `json:"galaxies"`
	Systems  int `json:"systems"`
}

type gameGalaxyRowResponse struct {
	Position int               `json:"position"`
	Planet   *gameGalaxyPlanet `json:"planet,omitempty"`
	Moon     *gameGalaxyPlanet `json:"moon,omitempty"`
	Debris   *gameGalaxyDebris `json:"debris,omitempty"`
}

type gameGalaxyPlanet struct {
	ID           int                         `json:"id"`
	Name         string                      `json:"name"`
	DisplayName  string                      `json:"displayName"`
	Type         int                         `json:"type"`
	Coordinates  gameCoordinatesResponse     `json:"coordinates"`
	Diameter     int                         `json:"diameter"`
	Temperature  int                         `json:"temperature"`
	ActivityText string                      `json:"activityText"`
	Destroyed    bool                        `json:"destroyed"`
	Abandoned    bool                        `json:"abandoned"`
	Own          bool                        `json:"own"`
	Player       *gameGalaxyPlayerStatus     `json:"player,omitempty"`
	Alliance     *gameGalaxyAllianceResponse `json:"alliance,omitempty"`
	Actions      gameGalaxyActionsResponse   `json:"actions"`
}

type gameGalaxyPlayerStatus struct {
	ID          int                      `json:"id"`
	Name        string                   `json:"name"`
	Rank        int                      `json:"rank"`
	Status      string                   `json:"status"`
	StatusClass string                   `json:"statusClass"`
	Suffixes    []gameGalaxyStatusSuffix `json:"suffixes"`
	Own         bool                     `json:"own"`
}

type gameGalaxyStatusSuffix struct {
	Text  string `json:"text"`
	Class string `json:"class"`
}

type gameGalaxyAllianceResponse struct {
	ID  int    `json:"id"`
	Tag string `json:"tag"`
}

type gameGalaxyDebris struct {
	ID         int     `json:"id"`
	Metal      float64 `json:"metal"`
	Crystal    float64 `json:"crystal"`
	Harvesters int     `json:"harvesters"`
	Visible    bool    `json:"visible"`
}

type gameGalaxyActionsResponse struct {
	Deploy    bool `json:"deploy"`
	Transport bool `json:"transport"`
	Spy       bool `json:"spy"`
	Message   bool `json:"message"`
	Buddy     bool `json:"buddy"`
	Missile   bool `json:"missile"`
	Attack    bool `json:"attack"`
	Defend    bool `json:"defend"`
	Destroy   bool `json:"destroy"`
	Recycle   bool `json:"recycle"`
}

type gameGalaxyExtraResponse struct {
	Commander bool                   `json:"commander"`
	SpyProbes int                    `json:"spyProbes"`
	Recyclers int                    `json:"recyclers"`
	Missiles  int                    `json:"missiles"`
	MaxSpy    int                    `json:"maxSpy"`
	Slots     gameFleetSlotsResponse `json:"slots"`
}

func (a app) handleGameGalaxy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameGalaxyGet(w, r)
	case http.MethodPost:
		a.handleGameGalaxyPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameGalaxyGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameGalaxy == nil {
		http.Error(w, "game galaxy unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	coordinates, err := selectedGalaxyCoordinates(r)
	if err != nil {
		http.Error(w, "invalid galaxy coordinates", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameGalaxy.GetGalaxy(r.Context(), appgame.GalaxyCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Coordinates:     coordinates,
	})
	if err != nil {
		http.Error(w, "game galaxy unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameGalaxyResponse(w, result)
}

func (a app) handleGameGalaxyPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameGalaxy == nil {
		http.Error(w, "game galaxy unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	coordinates, err := selectedGalaxyCoordinates(r)
	if err != nil {
		http.Error(w, "invalid galaxy coordinates", http.StatusBadRequest)
		return
	}

	var request gameGalaxyMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid galaxy request", http.StatusBadRequest)
		return
	}
	switch request.Action {
	case "launch-missile", "launch-missiles":
		result, err := a.deps.GameGalaxy.LaunchMissiles(r.Context(), appgame.GalaxyMissileLaunchCommand{
			PublicSession:   r.URL.Query().Get("session"),
			PrivateSessions: cookieMap(r),
			RemoteAddr:      remoteIP(r.RemoteAddr),
			PlanetID:        planetID,
			Coordinates:     coordinates,
			TargetPlanetID:  request.TargetPlanetID,
			Amount:          request.Amount,
			TargetDefenseID: request.TargetDefenseID,
		})
		if err != nil {
			http.Error(w, "game galaxy unavailable", http.StatusServiceUnavailable)
			return
		}

		writeGameGalaxyResponse(w, result)
	case "dispatch-spy", "instant-spy", "spy":
		a.handleGameGalaxyInstantDispatch(w, r, planetID, coordinates, request, domaingame.FleetMissionSpy)
	case "dispatch-recycle", "instant-recycle", "recycle":
		a.handleGameGalaxyInstantDispatch(w, r, planetID, coordinates, request, domaingame.FleetMissionRecycle)
	default:
		http.Error(w, "unsupported galaxy action", http.StatusBadRequest)
	}
}

func (a app) handleGameGalaxyInstantDispatch(w http.ResponseWriter, r *http.Request, planetID int, coordinates domaingame.Coordinates, request gameGalaxyMutationRequest, mission int) {
	targetType := request.TargetType
	if targetType <= 0 {
		targetType = domaingame.GamePlanetTypePlanet
	}
	if mission == domaingame.FleetMissionRecycle && request.TargetType <= 0 {
		targetType = domaingame.GamePlanetTypeDebris
	}

	result, err := a.deps.GameGalaxy.DispatchInstantFleet(r.Context(), appgame.GalaxyInstantDispatchCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Coordinates:     coordinates,
		Target: domaingame.Coordinates{
			Galaxy:   request.TargetGalaxy,
			System:   request.TargetSystem,
			Position: request.TargetPosition,
		},
		TargetType: targetType,
		Mission:    mission,
		Amount:     request.Amount,
	})
	if err != nil {
		http.Error(w, "game galaxy unavailable", http.StatusServiceUnavailable)
		return
	}

	writeGameGalaxyResponse(w, result)
}

func writeGameGalaxyResponse(w http.ResponseWriter, result appgame.GalaxyResult) {
	status := http.StatusOK
	var galaxy *gameGalaxySummary
	if result.Authenticated {
		mapped := toGameGalaxySummary(result.Galaxy)
		galaxy = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameGalaxyResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Galaxy:        galaxy,
		ActionIssue:   toGameGalaxyActionIssue(result.ActionIssue),
	})
}

func toGameGalaxyActionIssue(issue *domaingame.GalaxyActionIssue) *gameGalaxyActionIssue {
	if issue == nil {
		return nil
	}
	return &gameGalaxyActionIssue{Code: issue.Code, Message: issue.Message}
}

func selectedGalaxyCoordinates(r *http.Request) (domaingame.Coordinates, error) {
	query := r.URL.Query()
	galaxy, err := selectedOptionalInt(query.Get("galaxy"), query.Get("p1"))
	if err != nil {
		return domaingame.Coordinates{}, err
	}
	system, err := selectedOptionalInt(query.Get("system"), query.Get("p2"))
	if err != nil {
		return domaingame.Coordinates{}, err
	}
	position, err := selectedOptionalInt(query.Get("position"), query.Get("p3"))
	if err != nil {
		return domaingame.Coordinates{}, err
	}
	return domaingame.Coordinates{Galaxy: galaxy, System: system, Position: position}, nil
}

func selectedOptionalInt(primary string, alias string) (int, error) {
	raw := primary
	if raw == "" {
		raw = alias
	}
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func toGameGalaxySummary(galaxy domaingame.Galaxy) gameGalaxySummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(galaxy.PlanetSwitcher))
	for _, planet := range galaxy.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameGalaxyRowResponse, 0, len(galaxy.Rows))
	for _, row := range galaxy.Rows {
		rows = append(rows, toGameGalaxyRow(row))
	}
	return gameGalaxySummary{
		Commander:      galaxy.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(galaxy.CurrentPlanet),
		PlanetSwitcher: planets,
		Coordinates:    toGameCoordinatesResponse(galaxy.Coordinates),
		Bounds: gameGalaxyBoundsResponse{
			Galaxies: galaxy.Bounds.Galaxies,
			Systems:  galaxy.Bounds.Systems,
		},
		Rows:      rows,
		Populated: galaxy.Populated,
		Slots: gameFleetSlotsResponse{
			Used:    galaxy.Slots.Used,
			Max:     galaxy.Slots.Max,
			BaseMax: galaxy.Slots.BaseMax,
			Admiral: galaxy.Slots.Admiral,
		},
		Extra: gameGalaxyExtraResponse{
			Commander: galaxy.Extra.Commander,
			SpyProbes: galaxy.Extra.SpyProbes,
			Recyclers: galaxy.Extra.Recyclers,
			Missiles:  galaxy.Extra.Missiles,
			MaxSpy:    galaxy.Extra.MaxSpy,
			Slots: gameFleetSlotsResponse{
				Used:    galaxy.Extra.Slots.Used,
				Max:     galaxy.Extra.Slots.Max,
				BaseMax: galaxy.Extra.Slots.BaseMax,
				Admiral: galaxy.Extra.Slots.Admiral,
			},
		},
		NotEnoughDeuterium:  galaxy.NotEnoughDeuterium,
		RemoteSystemCostDue: galaxy.RemoteSystemCostDue,
	}
}

func toGameGalaxyRow(row domaingame.GalaxyRow) gameGalaxyRowResponse {
	return gameGalaxyRowResponse{
		Position: row.Position,
		Planet:   toGameGalaxyPlanet(row.Planet),
		Moon:     toGameGalaxyPlanet(row.Moon),
		Debris:   toGameGalaxyDebris(row.Debris),
	}
}

func toGameGalaxyPlanet(planet *domaingame.GalaxyPlanet) *gameGalaxyPlanet {
	if planet == nil {
		return nil
	}
	status := toGameGalaxyPlayerStatus(planet.Player)
	alliance := toGameGalaxyAlliance(planet.Alliance)
	return &gameGalaxyPlanet{
		ID:           planet.ID,
		Name:         planet.Name,
		DisplayName:  planet.DisplayName,
		Type:         planet.Type,
		Coordinates:  toGameCoordinatesResponse(planet.Coordinates),
		Diameter:     planet.Diameter,
		Temperature:  planet.Temperature,
		ActivityText: planet.ActivityText,
		Destroyed:    planet.Destroyed,
		Abandoned:    planet.Abandoned,
		Own:          planet.Own,
		Player:       status,
		Alliance:     alliance,
		Actions: gameGalaxyActionsResponse{
			Deploy:    planet.Actions.Deploy,
			Transport: planet.Actions.Transport,
			Spy:       planet.Actions.Spy,
			Message:   planet.Actions.Message,
			Buddy:     planet.Actions.Buddy,
			Missile:   planet.Actions.Missile,
			Attack:    planet.Actions.Attack,
			Defend:    planet.Actions.Defend,
			Destroy:   planet.Actions.Destroy,
			Recycle:   planet.Actions.Recycle,
		},
	}
}

func toGameGalaxyPlayerStatus(player *domaingame.GalaxyPlayerStatus) *gameGalaxyPlayerStatus {
	if player == nil {
		return nil
	}
	suffixes := make([]gameGalaxyStatusSuffix, 0, len(player.Suffixes))
	for _, suffix := range player.Suffixes {
		suffixes = append(suffixes, gameGalaxyStatusSuffix{Text: suffix.Text, Class: suffix.Class})
	}
	return &gameGalaxyPlayerStatus{
		ID:          player.ID,
		Name:        player.Name,
		Rank:        player.Rank,
		Status:      player.Status,
		StatusClass: player.StatusClass,
		Suffixes:    suffixes,
		Own:         player.Own,
	}
}

func toGameGalaxyAlliance(alliance *domaingame.GalaxyAlliance) *gameGalaxyAllianceResponse {
	if alliance == nil {
		return nil
	}
	return &gameGalaxyAllianceResponse{ID: alliance.ID, Tag: alliance.Tag}
}

func toGameGalaxyDebris(debris *domaingame.GalaxyDebris) *gameGalaxyDebris {
	if debris == nil {
		return nil
	}
	return &gameGalaxyDebris{
		ID:         debris.ID,
		Metal:      debris.Metal,
		Crystal:    debris.Crystal,
		Harvesters: debris.Harvesters,
		Visible:    debris.Visible,
	}
}

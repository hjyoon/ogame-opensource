package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameTechnologyResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Technology    *gameTechnologySummary     `json:"technology,omitempty"`
}

type gameTechnologySummary struct {
	Commander      string                         `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse     `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse    `json:"planetSwitcher"`
	Groups         []gameTechnologyGroupResponse  `json:"groups"`
	Details        *gameTechnologyDetailsResponse `json:"details,omitempty"`
	Info           *gameTechnologyInfoResponse    `json:"info,omitempty"`
}

type gameTechnologyGroupResponse struct {
	Key   string                       `json:"key"`
	Name  string                       `json:"name"`
	Items []gameTechnologyItemResponse `json:"items"`
}

type gameTechnologyItemResponse struct {
	ID               int                                 `json:"id"`
	Name             string                              `json:"name"`
	Requirements     []gameTechnologyRequirementResponse `json:"requirements"`
	DetailsAvailable bool                                `json:"detailsAvailable"`
}

type gameTechnologyRequirementResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	CurrentLevel int    `json:"currentLevel"`
	Met          bool   `json:"met"`
}

type gameTechnologyDetailsResponse struct {
	Target   gameTechnologyItemResponse           `json:"target"`
	Levels   []gameTechnologyDetailsLevelResponse `json:"levels"`
	Demolish *gameTechnologyDemolishResponse      `json:"demolish,omitempty"`
}

type gameTechnologyDetailsLevelResponse struct {
	Step         int                                 `json:"step"`
	Requirements []gameTechnologyRequirementResponse `json:"requirements"`
}

type gameTechnologyDemolishResponse struct {
	Level           int                      `json:"level"`
	Cost            gameBuildingCostResponse `json:"cost"`
	DurationSeconds int                      `json:"durationSeconds"`
}

type gameTechnologyInfoResponse struct {
	ID          int                         `json:"id"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Level       int                         `json:"level"`
	Kind        string                      `json:"kind"`
	Rows        []gameTechnologyInfoRowItem `json:"rows"`
}

type gameTechnologyInfoRowItem struct {
	Level                int  `json:"level"`
	Current              bool `json:"current"`
	Production           int  `json:"production"`
	ProductionDifference int  `json:"productionDifference"`
	Energy               int  `json:"energy"`
	EnergyDifference     int  `json:"energyDifference"`
	Storage              int  `json:"storage"`
	StorageDifference    int  `json:"storageDifference"`
	DeuteriumConsumption int  `json:"deuteriumConsumption"`
	DeuteriumDifference  int  `json:"deuteriumDifference"`
}

func (a app) handleGameTechnology(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameTechnology == nil {
		http.Error(w, "game technology unavailable", http.StatusServiceUnavailable)
		return
	}

	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	technologyDetailsID, err := selectedTechnologyDetailsID(r)
	if err != nil {
		http.Error(w, "invalid selected technology", http.StatusBadRequest)
		return
	}
	technologyInfoID, err := selectedTechnologyInfoID(r)
	if err != nil {
		http.Error(w, "invalid selected technology", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameTechnology.GetTechnology(r.Context(), appgame.TechnologyCommand{
		PublicSession:       r.URL.Query().Get("session"),
		PrivateSessions:     cookieMap(r),
		RemoteAddr:          remoteIP(r.RemoteAddr),
		PlanetID:            planetID,
		TechnologyDetailsID: technologyDetailsID,
		TechnologyInfoID:    technologyInfoID,
	})
	if err != nil {
		http.Error(w, "game technology unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var technology *gameTechnologySummary
	if result.Authenticated {
		mapped := toGameTechnologySummary(result.Technology)
		technology = &mapped
	} else {
		status = http.StatusUnauthorized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameTechnologyResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Technology:    technology,
	})
}

func toGameTechnologySummary(technology domaingame.Technology) gameTechnologySummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(technology.PlanetSwitcher))
	for _, planet := range technology.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	groups := make([]gameTechnologyGroupResponse, 0, len(technology.Groups))
	for _, group := range technology.Groups {
		groups = append(groups, toGameTechnologyGroupResponse(group))
	}
	return gameTechnologySummary{
		Commander:      technology.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(technology.CurrentPlanet),
		PlanetSwitcher: planets,
		Groups:         groups,
		Details:        toGameTechnologyDetailsResponse(technology.Details),
		Info:           toGameTechnologyInfoResponse(technology.Info),
	}
}

func toGameTechnologyGroupResponse(group domaingame.TechnologyGroup) gameTechnologyGroupResponse {
	items := make([]gameTechnologyItemResponse, 0, len(group.Items))
	for _, item := range group.Items {
		items = append(items, toGameTechnologyItemResponse(item))
	}
	return gameTechnologyGroupResponse{
		Key:   group.Key,
		Name:  group.Name,
		Items: items,
	}
}

func toGameTechnologyItemResponse(item domaingame.TechnologyItem) gameTechnologyItemResponse {
	requirements := make([]gameTechnologyRequirementResponse, 0, len(item.Requirements))
	for _, requirement := range item.Requirements {
		requirements = append(requirements, gameTechnologyRequirementResponse{
			ID:           requirement.ID,
			Name:         requirement.Name,
			Level:        requirement.Level,
			CurrentLevel: requirement.CurrentLevel,
			Met:          requirement.Met,
		})
	}
	return gameTechnologyItemResponse{
		ID:               item.ID,
		Name:             item.Name,
		Requirements:     requirements,
		DetailsAvailable: item.DetailsAvailable,
	}
}

func toGameTechnologyDetailsResponse(details *domaingame.TechnologyDetails) *gameTechnologyDetailsResponse {
	if details == nil {
		return nil
	}
	levels := make([]gameTechnologyDetailsLevelResponse, 0, len(details.Levels))
	for _, level := range details.Levels {
		requirements := make([]gameTechnologyRequirementResponse, 0, len(level.Requirements))
		for _, requirement := range level.Requirements {
			requirements = append(requirements, gameTechnologyRequirementResponse{
				ID:           requirement.ID,
				Name:         requirement.Name,
				Level:        requirement.Level,
				CurrentLevel: requirement.CurrentLevel,
				Met:          requirement.Met,
			})
		}
		levels = append(levels, gameTechnologyDetailsLevelResponse{
			Step:         level.Step,
			Requirements: requirements,
		})
	}
	return &gameTechnologyDetailsResponse{
		Target:   toGameTechnologyItemResponse(details.Target),
		Levels:   levels,
		Demolish: toGameTechnologyDemolishResponse(details.Demolish),
	}
}

func toGameTechnologyDemolishResponse(demolish *domaingame.TechnologyDemolish) *gameTechnologyDemolishResponse {
	if demolish == nil {
		return nil
	}
	return &gameTechnologyDemolishResponse{
		Level:           demolish.Level,
		Cost:            toGameBuildingCostResponse(demolish.Cost),
		DurationSeconds: demolish.DurationSeconds,
	}
}

func toGameTechnologyInfoResponse(info *domaingame.TechnologyInfo) *gameTechnologyInfoResponse {
	if info == nil {
		return nil
	}
	rows := make([]gameTechnologyInfoRowItem, 0, len(info.Rows))
	for _, row := range info.Rows {
		rows = append(rows, gameTechnologyInfoRowItem{
			Level:                row.Level,
			Current:              row.Current,
			Production:           row.Production,
			ProductionDifference: row.ProductionDifference,
			Energy:               row.Energy,
			EnergyDifference:     row.EnergyDifference,
			Storage:              row.Storage,
			StorageDifference:    row.StorageDifference,
			DeuteriumConsumption: row.DeuteriumConsumption,
			DeuteriumDifference:  row.DeuteriumDifference,
		})
	}
	return &gameTechnologyInfoResponse{
		ID:          info.ID,
		Name:        info.Name,
		Description: info.Description,
		Level:       info.Level,
		Kind:        info.Kind,
		Rows:        rows,
	}
}

func selectedTechnologyDetailsID(r *http.Request) (int, error) {
	return selectedTechnologyQueryID(r, "tid")
}

func selectedTechnologyInfoID(r *http.Request) (int, error) {
	return selectedTechnologyQueryID(r, "gid")
}

func selectedTechnologyQueryID(r *http.Request, key string) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id < 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

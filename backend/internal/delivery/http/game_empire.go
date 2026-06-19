package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameEmpireResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Empire        *gameEmpireSummary         `json:"empire,omitempty"`
	ActionIssue   *gameEmpireActionIssue     `json:"actionIssue,omitempty"`
}

type gameEmpireActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameEmpireSummary struct {
	Commander       string                      `json:"commander"`
	CommanderActive bool                        `json:"commanderActive"`
	CurrentPlanet   gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher  []gamePlanetSummaryResponse `json:"planetSwitcher"`
	PlanetType      int                         `json:"planetType"`
	MoonEnabled     bool                        `json:"moonEnabled"`
	HasMoons        bool                        `json:"hasMoons"`
	Planets         []gameEmpirePlanet          `json:"planets"`
	Resources       []gameEmpireResourceRow     `json:"resources"`
	Buildings       []gameEmpireLevelRow        `json:"buildings"`
	Research        []gameEmpireLevelRow        `json:"research"`
	Fleet           []gameEmpireCountRow        `json:"fleet"`
	Defense         []gameEmpireCountRow        `json:"defense"`
}

type gameEmpirePlanet struct {
	ID          int                     `json:"id"`
	Name        string                  `json:"name"`
	Type        int                     `json:"type"`
	Coordinates gameCoordinatesResponse `json:"coordinates"`
	Fields      int                     `json:"fields"`
	MaxFields   int                     `json:"maxFields"`
}

type gameEmpireResourceRow struct {
	ID         int                       `json:"id"`
	Name       string                    `json:"name"`
	Values     []gameEmpireResourceValue `json:"values"`
	Total      int                       `json:"total"`
	Production int                       `json:"production"`
}

type gameEmpireResourceValue struct {
	PlanetID   int `json:"planetId"`
	Amount     int `json:"amount"`
	Production int `json:"production"`
}

type gameEmpireLevelRow struct {
	ID      int                    `json:"id"`
	Name    string                 `json:"name"`
	Values  []gameEmpireLevelValue `json:"values"`
	Total   int                    `json:"total"`
	Average float64                `json:"average"`
}

type gameEmpireLevelValue struct {
	PlanetID int `json:"planetId"`
	Level    int `json:"level"`
}

type gameEmpireCountRow struct {
	ID     int                    `json:"id"`
	Name   string                 `json:"name"`
	Values []gameEmpireCountValue `json:"values"`
	Total  int                    `json:"total"`
}

type gameEmpireCountValue struct {
	PlanetID int `json:"planetId"`
	Count    int `json:"count"`
}

func (a app) handleGameEmpire(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameEmpire == nil {
		http.Error(w, "game empire unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	planetType, err := selectedEmpirePlanetType(r)
	if err != nil {
		http.Error(w, "invalid planet type", http.StatusBadRequest)
		return
	}
	shortcut := selectedEmpireShortcut(r, planetID)
	var result appgame.EmpireResult
	if shortcut.Action != "" && r.Method == http.MethodGet {
		result, err = a.deps.GameEmpire.MutateEmpire(r.Context(), appgame.EmpireMutationCommand{
			PublicSession:   r.URL.Query().Get("session"),
			PrivateSessions: cookieMap(r),
			RemoteAddr:      remoteIP(r.RemoteAddr),
			PlanetID:        planetID,
			PlanetType:      planetType,
			Action:          shortcut.Action,
			TargetPlanetID:  shortcut.TargetPlanetID,
			TechID:          shortcut.TechID,
			ListID:          shortcut.ListID,
		})
	} else {
		result, err = a.deps.GameEmpire.GetEmpire(r.Context(), appgame.EmpireCommand{
			PublicSession:   r.URL.Query().Get("session"),
			PrivateSessions: cookieMap(r),
			RemoteAddr:      remoteIP(r.RemoteAddr),
			PlanetID:        planetID,
			PlanetType:      planetType,
		})
	}
	if err != nil {
		http.Error(w, "game empire unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameEmpireResponse(w, result)
}

type empireShortcut struct {
	Action         string
	TargetPlanetID int
	TechID         int
	ListID         int
}

func selectedEmpireShortcut(r *http.Request, fallbackPlanetID int) empireShortcut {
	action := domaingame.NormalizeBuildingsMutationAction(r.URL.Query().Get("modus"))
	if action == "" {
		return empireShortcut{}
	}
	targetPlanetID := legacyQueryInt(r, "planet")
	if targetPlanetID == 0 {
		targetPlanetID = fallbackPlanetID
	}
	return empireShortcut{
		Action:         action,
		TargetPlanetID: targetPlanetID,
		TechID:         legacyQueryInt(r, "techid"),
		ListID:         legacyQueryInt(r, "listid"),
	}
}

func legacyQueryInt(r *http.Request, key string) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func selectedEmpirePlanetType(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("planettype")
	if raw == "" {
		return domaingame.EmpirePlanetTypePlanets, nil
	}
	planetType, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if planetType != domaingame.EmpirePlanetTypePlanets && planetType != domaingame.EmpirePlanetTypeMoons {
		return domaingame.EmpirePlanetTypePlanets, nil
	}
	return planetType, nil
}

func writeGameEmpireResponse(w http.ResponseWriter, result appgame.EmpireResult) {
	status := http.StatusOK
	var empire *gameEmpireSummary
	if result.Authenticated {
		mapped := toGameEmpireSummary(result.Empire)
		empire = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameEmpireResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Empire:        empire,
		ActionIssue:   toGameEmpireActionIssue(result.ActionIssue),
	})
}

func toGameEmpireSummary(empire domaingame.Empire) gameEmpireSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(empire.PlanetSwitcher))
	for _, planet := range empire.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	return gameEmpireSummary{
		Commander:       empire.Commander,
		CommanderActive: empire.CommanderActive,
		CurrentPlanet:   toGamePlanetOverviewResponse(empire.CurrentPlanet),
		PlanetSwitcher:  planets,
		PlanetType:      empire.PlanetType,
		MoonEnabled:     empire.MoonEnabled,
		HasMoons:        empire.HasMoons,
		Planets:         toGameEmpirePlanets(empire.Planets),
		Resources:       toGameEmpireResourceRows(empire.Resources),
		Buildings:       toGameEmpireLevelRows(empire.Buildings),
		Research:        toGameEmpireLevelRows(empire.Research),
		Fleet:           toGameEmpireCountRows(empire.Fleet),
		Defense:         toGameEmpireCountRows(empire.Defense),
	}
}

func toGameEmpirePlanets(planets []domaingame.EmpirePlanet) []gameEmpirePlanet {
	mapped := make([]gameEmpirePlanet, 0, len(planets))
	for _, planet := range planets {
		mapped = append(mapped, gameEmpirePlanet{
			ID:          planet.ID,
			Name:        planet.Name,
			Type:        planet.Type,
			Coordinates: toGameCoordinatesResponse(planet.Coordinates),
			Fields:      planet.Fields,
			MaxFields:   planet.MaxFields,
		})
	}
	return mapped
}

func toGameEmpireResourceRows(rows []domaingame.EmpireResourceRow) []gameEmpireResourceRow {
	mapped := make([]gameEmpireResourceRow, 0, len(rows))
	for _, row := range rows {
		values := make([]gameEmpireResourceValue, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, gameEmpireResourceValue{PlanetID: value.PlanetID, Amount: value.Amount, Production: value.Production})
		}
		mapped = append(mapped, gameEmpireResourceRow{
			ID:         row.ID,
			Name:       row.Name,
			Values:     values,
			Total:      row.Total,
			Production: row.Production,
		})
	}
	return mapped
}

func toGameEmpireLevelRows(rows []domaingame.EmpireLevelRow) []gameEmpireLevelRow {
	mapped := make([]gameEmpireLevelRow, 0, len(rows))
	for _, row := range rows {
		values := make([]gameEmpireLevelValue, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, gameEmpireLevelValue{PlanetID: value.PlanetID, Level: value.Level})
		}
		mapped = append(mapped, gameEmpireLevelRow{ID: row.ID, Name: row.Name, Values: values, Total: row.Total, Average: row.Average})
	}
	return mapped
}

func toGameEmpireCountRows(rows []domaingame.EmpireCountRow) []gameEmpireCountRow {
	mapped := make([]gameEmpireCountRow, 0, len(rows))
	for _, row := range rows {
		values := make([]gameEmpireCountValue, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, gameEmpireCountValue{PlanetID: value.PlanetID, Count: value.Count})
		}
		mapped = append(mapped, gameEmpireCountRow{ID: row.ID, Name: row.Name, Values: values, Total: row.Total})
	}
	return mapped
}

func toGameEmpireActionIssue(issue *domaingame.EmpireActionIssue) *gameEmpireActionIssue {
	if issue == nil {
		return nil
	}
	return &gameEmpireActionIssue{Code: issue.Code, Message: issue.Message}
}

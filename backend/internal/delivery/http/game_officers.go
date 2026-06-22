package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameOfficersResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Officers      *gameOfficersSummary       `json:"officers,omitempty"`
	ActionIssue   *gameOfficerActionIssue    `json:"actionIssue,omitempty"`
}

type gameOfficerActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameOfficersSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	User           gameOfficersUser            `json:"user"`
	Rows           []gameOfficerRow            `json:"rows"`
}

type gameOfficersUser struct {
	PaidDarkMatter int `json:"paidDarkMatter"`
	FreeDarkMatter int `json:"freeDarkMatter"`
}

type gameOfficerRow struct {
	ID             int    `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Note           string `json:"note"`
	Image          string `json:"image"`
	Icon           string `json:"icon"`
	Active         bool   `json:"active"`
	Until          int64  `json:"until"`
	DaysLeft       int    `json:"daysLeft"`
	WeekCost       int    `json:"weekCost"`
	ThreeMonthCost int    `json:"threeMonthCost"`
}

type gameOfficerMutationRequest struct {
	OfficerID int `json:"officerId"`
	Type      int `json:"type"`
	Days      int `json:"days"`
}

func (a app) handleGameOfficers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameOfficersGet(w, r)
	case http.MethodPost:
		a.handleGameOfficersPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameOfficersGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOfficers == nil {
		http.Error(w, "game officers unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameOfficers.GetOfficers(r.Context(), appgame.OfficersCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game officers unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameOfficersResponse(w, result)
}

func (a app) handleGameOfficersPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOfficers == nil {
		http.Error(w, "game officers unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	mutation, err := decodeGameOfficerMutation(r)
	if err != nil {
		http.Error(w, "invalid officers request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameOfficers.RecruitOfficer(r.Context(), appgame.OfficersMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mutation:        mutation,
	})
	if err != nil {
		http.Error(w, "game officers unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameOfficersResponse(w, result)
}

func decodeGameOfficerMutation(r *http.Request) (domaingame.OfficerMutation, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var request gameOfficerMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return domaingame.OfficerMutation{}, err
		}
		officerID := request.OfficerID
		if officerID == 0 {
			officerID = request.Type
		}
		return domaingame.OfficerMutation{OfficerID: officerID, Days: request.Days}, nil
	}
	if err := r.ParseForm(); err != nil {
		return domaingame.OfficerMutation{}, err
	}
	return domaingame.OfficerMutation{
		OfficerID: legacyOfficerInt(formLast(r, "type")),
		Days:      legacyOfficerInt(formLast(r, "days")),
	}, nil
}

func writeGameOfficersResponse(w http.ResponseWriter, result appgame.OfficersResult) {
	status := http.StatusOK
	var officers *gameOfficersSummary
	if result.Authenticated {
		mapped := toGameOfficersSummary(result.Officers)
		officers = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameOfficersResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Officers:      officers,
		ActionIssue:   toGameOfficerActionIssue(result.ActionIssue),
	})
}

func toGameOfficersSummary(officers domaingame.Officers) gameOfficersSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(officers.PlanetSwitcher))
	for _, planet := range officers.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameOfficerRow, 0, len(officers.Rows))
	for _, row := range officers.Rows {
		rows = append(rows, gameOfficerRow{
			ID:             row.ID,
			Key:            row.Key,
			Name:           row.Name,
			Description:    row.Description,
			Note:           row.Note,
			Image:          row.Image,
			Icon:           row.Icon,
			Active:         row.Active,
			Until:          row.Until,
			DaysLeft:       row.DaysLeft,
			WeekCost:       row.WeekCost,
			ThreeMonthCost: row.ThreeMonthCost,
		})
	}
	return gameOfficersSummary{
		Commander:      officers.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(officers.CurrentPlanet),
		PlanetSwitcher: planets,
		User: gameOfficersUser{
			PaidDarkMatter: officers.User.PaidDarkMatter,
			FreeDarkMatter: officers.User.FreeDarkMatter,
		},
		Rows: rows,
	}
}

func toGameOfficerActionIssue(issue *domaingame.OfficerActionIssue) *gameOfficerActionIssue {
	if issue == nil {
		return nil
	}
	return &gameOfficerActionIssue{Code: issue.Code, Message: issue.Message}
}

func legacyOfficerInt(value string) int {
	value = strings.TrimSpace(strings.ReplaceAll(value, ".", ""))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	if parsed < 0 {
		return -parsed
	}
	return parsed
}

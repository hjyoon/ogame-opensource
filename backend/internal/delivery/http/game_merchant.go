package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameMerchantResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Merchant      *gameMerchantSummary       `json:"merchant,omitempty"`
	ActionIssue   *gameMerchantActionIssue   `json:"actionIssue,omitempty"`
}

type gameMerchantActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameMerchantSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	User           gameMerchantUser            `json:"user"`
	ActiveOfferID  int                         `json:"activeOfferId"`
	Rates          gameMerchantRates           `json:"rates"`
	Rows           []gameMerchantResourceRow   `json:"rows"`
}

type gameMerchantUser struct {
	PaidDarkMatter int `json:"paidDarkMatter"`
	FreeDarkMatter int `json:"freeDarkMatter"`
}

type gameMerchantRates struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
}

type gameMerchantResourceRow struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Offered     bool    `json:"offered"`
	Value       int     `json:"value"`
	FreeStorage int     `json:"freeStorage"`
	Rate        float64 `json:"rate"`
}

type gameMerchantMutationRequest struct {
	Action  string                         `json:"action"`
	OfferID int                            `json:"offerId"`
	Values  domaingame.MerchantTradeValues `json:"values"`
}

func (a app) handleGameMerchant(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameMerchantGet(w, r)
	case http.MethodPost:
		a.handleGameMerchantPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameMerchantGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameMerchant == nil {
		http.Error(w, "game merchant unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameMerchant.GetMerchant(r.Context(), appgame.MerchantCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game merchant unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameMerchantResponse(w, result)
}

func (a app) handleGameMerchantPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameMerchant == nil {
		http.Error(w, "game merchant unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	mutation, err := decodeGameMerchantMutation(r)
	if err != nil {
		http.Error(w, "invalid merchant request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameMerchant.MutateMerchant(r.Context(), appgame.MerchantMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mutation:        mutation,
	})
	if err != nil {
		http.Error(w, "game merchant unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameMerchantResponse(w, result)
}

func decodeGameMerchantMutation(r *http.Request) (domaingame.MerchantMutation, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var request gameMerchantMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return domaingame.MerchantMutation{}, err
		}
		return domaingame.MerchantMutation{
			Action:  request.Action,
			OfferID: request.OfferID,
			Values:  request.Values,
		}, nil
	}
	if err := r.ParseForm(); err != nil {
		return domaingame.MerchantMutation{}, err
	}
	action := "call"
	if _, ok := r.PostForm["trade"]; ok {
		action = "trade"
	}
	return domaingame.MerchantMutation{
		Action:  action,
		OfferID: legacyMerchantInt(formLast(r, "offer_id")),
		Values: domaingame.MerchantTradeValues{
			Metal:     legacyMerchantInt(formLast(r, "1_value")),
			Crystal:   legacyMerchantInt(formLast(r, "2_value")),
			Deuterium: legacyMerchantInt(formLast(r, "3_value")),
		},
	}, nil
}

func writeGameMerchantResponse(w http.ResponseWriter, result appgame.MerchantResult) {
	status := http.StatusOK
	var merchant *gameMerchantSummary
	if result.Authenticated {
		mapped := toGameMerchantSummary(result.Merchant)
		merchant = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameMerchantResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Merchant:      merchant,
		ActionIssue:   toGameMerchantActionIssue(result.ActionIssue),
	})
}

func toGameMerchantSummary(merchant domaingame.Merchant) gameMerchantSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(merchant.PlanetSwitcher))
	for _, planet := range merchant.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	rows := make([]gameMerchantResourceRow, 0, len(merchant.Rows))
	for _, row := range merchant.Rows {
		rows = append(rows, gameMerchantResourceRow{
			ID:          row.ID,
			Name:        row.Name,
			Offered:     row.Offered,
			Value:       row.Value,
			FreeStorage: row.FreeStorage,
			Rate:        row.Rate,
		})
	}
	return gameMerchantSummary{
		Commander:      merchant.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(merchant.CurrentPlanet),
		PlanetSwitcher: planets,
		User: gameMerchantUser{
			PaidDarkMatter: merchant.User.PaidDarkMatter,
			FreeDarkMatter: merchant.User.FreeDarkMatter,
		},
		ActiveOfferID: merchant.ActiveOfferID,
		Rates: gameMerchantRates{
			Metal:     merchant.Rates.Metal,
			Crystal:   merchant.Rates.Crystal,
			Deuterium: merchant.Rates.Deuterium,
		},
		Rows: rows,
	}
}

func toGameMerchantActionIssue(issue *domaingame.MerchantActionIssue) *gameMerchantActionIssue {
	if issue == nil {
		return nil
	}
	return &gameMerchantActionIssue{Code: issue.Code, Message: issue.Message}
}

func legacyMerchantInt(value string) int {
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

package httpdelivery

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameOptionsResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Options       *gameOptionsSummary        `json:"options,omitempty"`
	ActionIssue   *gameOptionsActionIssue    `json:"actionIssue,omitempty"`
}

type gameOptionsActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameOptionsSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	User           gameOptionsUser             `json:"user"`
	Universe       gameOptionsUniverse         `json:"universe"`
	Settings       gameOptionsSettings         `json:"settings"`
	Account        gameOptionsAccount          `json:"account"`
	Flags          gameOptionsFlags            `json:"flags"`
}

type gameOptionsUser struct {
	Name        string `json:"name"`
	NameLocked  bool   `json:"nameLocked"`
	Email       string `json:"email"`
	PlainEmail  string `json:"plainEmail"`
	Validated   bool   `json:"validated"`
	Admin       int    `json:"admin"`
	FeedID      string `json:"feedId"`
	CommanderOn bool   `json:"commanderOn"`
}

type gameOptionsUniverse struct {
	Language      string `json:"language"`
	ForceLanguage bool   `json:"forceLanguage"`
	FeedAge       int    `json:"feedAge"`
	Speed         int    `json:"speed"`
}

type gameOptionsSettings struct {
	Language         string `json:"language"`
	SkinPath         string `json:"skinPath"`
	UseSkin          bool   `json:"useSkin"`
	DeactivateIP     bool   `json:"deactivateIp"`
	SortBy           int    `json:"sortBy"`
	SortOrder        int    `json:"sortOrder"`
	MaxSpy           int    `json:"maxSpy"`
	MaxFleetMessages int    `json:"maxFleetMessages"`
}

type gameOptionsAccount struct {
	Vacation       bool  `json:"vacation"`
	VacationUntil  int64 `json:"vacationUntil"`
	DeletionQueued bool  `json:"deletionQueued"`
	DeletionAt     int64 `json:"deletionAt"`
}

type gameOptionsFlags struct {
	ShowEspionageButton bool `json:"showEspionageButton"`
	ShowWriteMessage    bool `json:"showWriteMessage"`
	ShowBuddy           bool `json:"showBuddy"`
	ShowRocketAttack    bool `json:"showRocketAttack"`
	ShowViewReport      bool `json:"showViewReport"`
	DoNotUseFolders     bool `json:"doNotUseFolders"`
	FeedEnabled         bool `json:"feedEnabled"`
	FeedAtom            bool `json:"feedAtom"`
	HideGOEmail         bool `json:"hideGoEmail"`
}

type gameOptionsMutationRequest struct {
	Language         string `json:"language"`
	SkinPath         string `json:"skinPath"`
	UseSkin          bool   `json:"useSkin"`
	DeactivateIP     bool   `json:"deactivateIp"`
	SortBy           int    `json:"sortBy"`
	SortOrder        int    `json:"sortOrder"`
	MaxSpy           int    `json:"maxSpy"`
	MaxFleetMessages int    `json:"maxFleetMessages"`
	VacationMode     *bool  `json:"vacationMode"`
	DeleteAccount    bool   `json:"deleteAccount"`
}

func (a app) handleGameOptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameOptionsGet(w, r)
	case http.MethodPost:
		a.handleGameOptionsPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameOptionsGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOptions == nil {
		http.Error(w, "game options unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameOptions.GetOptions(r.Context(), appgame.OptionsCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
	})
	if err != nil {
		http.Error(w, "game options unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameOptionsResponse(w, result)
}

func (a app) handleGameOptionsPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameOptions == nil {
		http.Error(w, "game options unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	mutation, err := decodeGameOptionsMutation(r)
	if err != nil {
		http.Error(w, "invalid options request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameOptions.UpdateOptions(r.Context(), appgame.OptionsUpdateCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mutation:        mutation,
	})
	if err != nil {
		http.Error(w, "game options unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameOptionsResponse(w, result)
}

func decodeGameOptionsMutation(r *http.Request) (domaingame.OptionsMutation, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var request gameOptionsMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return domaingame.OptionsMutation{}, err
		}
		host, port := requestHostPort(r)
		request.SkinPath = domaingame.NormalizeSkinPath(request.SkinPath, host, port)
		return domaingame.OptionsMutation{
			Language:         request.Language,
			SkinPath:         request.SkinPath,
			UseSkin:          request.UseSkin,
			DeactivateIP:     request.DeactivateIP,
			SortBy:           request.SortBy,
			SortOrder:        request.SortOrder,
			MaxSpy:           request.MaxSpy,
			MaxFleetMessages: request.MaxFleetMessages,
			VacationMode:     request.VacationMode != nil && *request.VacationMode,
			VacationModeSet:  request.VacationMode != nil,
			DeleteAccount:    request.DeleteAccount,
		}, nil
	}
	if err := r.ParseForm(); err != nil {
		return domaingame.OptionsMutation{}, err
	}
	host, port := requestHostPort(r)
	vacationModeSet := r.PostForm.Has("urlaubs_modus") || r.PostForm.Has("urlaub_aus")
	return domaingame.OptionsMutation{
		Language:         formLast(r, "lang"),
		SkinPath:         domaingame.NormalizeSkinPath(formLast(r, "dpath"), host, port),
		UseSkin:          formChecked(r, "design"),
		DeactivateIP:     formChecked(r, "noipcheck"),
		SortBy:           legacyInt(r.PostForm["settings_sort"]),
		SortOrder:        legacyInt(r.PostForm["settings_order"]),
		MaxSpy:           legacyInt(r.PostForm["spio_anz"]),
		MaxFleetMessages: legacyInt(r.PostForm["settings_fleetactions"]),
		VacationMode:     formChecked(r, "urlaubs_modus") && !formChecked(r, "urlaub_aus"),
		VacationModeSet:  vacationModeSet,
		DeleteAccount:    formChecked(r, "db_deaktjava"),
	}, nil
}

func writeGameOptionsResponse(w http.ResponseWriter, result appgame.OptionsResult) {
	status := http.StatusOK
	var options *gameOptionsSummary
	if result.Authenticated {
		mapped := toGameOptionsSummary(result.Options)
		options = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameOptionsResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Options:       options,
		ActionIssue:   toGameOptionsActionIssue(result.ActionIssue),
	})
}

func toGameOptionsSummary(options domaingame.Options) gameOptionsSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(options.PlanetSwitcher))
	for _, planet := range options.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	return gameOptionsSummary{
		Commander:      options.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(options.CurrentPlanet),
		PlanetSwitcher: planets,
		User: gameOptionsUser{
			Name:        options.User.Name,
			NameLocked:  options.User.NameLocked,
			Email:       options.User.Email,
			PlainEmail:  options.User.PlainEmail,
			Validated:   options.User.Validated,
			Admin:       options.User.Admin,
			FeedID:      options.User.FeedID,
			CommanderOn: options.User.CommanderOn,
		},
		Universe: gameOptionsUniverse{
			Language:      options.Universe.Language,
			ForceLanguage: options.Universe.ForceLanguage,
			FeedAge:       options.Universe.FeedAge,
			Speed:         options.Universe.Speed,
		},
		Settings: gameOptionsSettings{
			Language:         options.Settings.Language,
			SkinPath:         options.Settings.SkinPath,
			UseSkin:          options.Settings.UseSkin,
			DeactivateIP:     options.Settings.DeactivateIP,
			SortBy:           options.Settings.SortBy,
			SortOrder:        options.Settings.SortOrder,
			MaxSpy:           options.Settings.MaxSpy,
			MaxFleetMessages: options.Settings.MaxFleetMessages,
		},
		Account: gameOptionsAccount{
			Vacation:       options.Account.Vacation,
			VacationUntil:  options.Account.VacationUntil,
			DeletionQueued: options.Account.DeletionQueued,
			DeletionAt:     options.Account.DeletionAt,
		},
		Flags: gameOptionsFlags{
			ShowEspionageButton: options.Flags.ShowEspionageButton,
			ShowWriteMessage:    options.Flags.ShowWriteMessage,
			ShowBuddy:           options.Flags.ShowBuddy,
			ShowRocketAttack:    options.Flags.ShowRocketAttack,
			ShowViewReport:      options.Flags.ShowViewReport,
			DoNotUseFolders:     options.Flags.DoNotUseFolders,
			FeedEnabled:         options.Flags.FeedEnabled,
			FeedAtom:            options.Flags.FeedAtom,
			HideGOEmail:         options.Flags.HideGOEmail,
		},
	}
}

func toGameOptionsActionIssue(issue *domaingame.OptionsActionIssue) *gameOptionsActionIssue {
	if issue == nil {
		return nil
	}
	return &gameOptionsActionIssue{Code: issue.Code, Message: issue.Message}
}

func formChecked(r *http.Request, key string) bool {
	for _, value := range r.PostForm[key] {
		if value == "on" || value == "1" || strings.EqualFold(value, "true") {
			return true
		}
	}
	return false
}

func formLast(r *http.Request, key string) string {
	values := r.PostForm[key]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func requestHostPort(r *http.Request) (string, int) {
	host := r.Host
	if host == "" {
		return "", defaultRequestPort(r)
	}
	hostname, portText, err := net.SplitHostPort(host)
	if err == nil {
		port, parseErr := strconv.Atoi(portText)
		if parseErr != nil {
			port = defaultRequestPort(r)
		}
		return hostname, port
	}
	if strings.Contains(host, ":") && strings.Count(host, ":") > 1 {
		return strings.Trim(host, "[]"), defaultRequestPort(r)
	}
	return strings.Split(host, ":")[0], defaultRequestPort(r)
}

func defaultRequestPort(r *http.Request) int {
	if r.TLS != nil {
		return 443
	}
	return 80
}

package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameAllianceResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Alliance      *gameAllianceSummary       `json:"alliance,omitempty"`
	ActionIssue   *gameAllianceActionIssue   `json:"actionIssue,omitempty"`
}

type gameAllianceActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gameAllianceSummary struct {
	Commander      string                      `json:"commander"`
	CurrentPlanet  gamePlanetOverviewResponse  `json:"currentPlanet"`
	PlanetSwitcher []gamePlanetSummaryResponse `json:"planetSwitcher"`
	View           string                      `json:"view"`
	Viewer         gameAllianceViewer          `json:"viewer"`
	Own            *gameAllianceInfo           `json:"own,omitempty"`
	Target         *gameAllianceInfo           `json:"target,omitempty"`
	Pending        *gameAllianceApplication    `json:"pending,omitempty"`
	SearchText     string                      `json:"searchText"`
	TextKind       int                         `json:"textKind"`
	SearchResults  []gameAllianceSearchResult  `json:"searchResults"`
	Applications   []gameAllianceApplication   `json:"applications"`
	SelectedApp    *gameAllianceApplication    `json:"selectedApp,omitempty"`
	Members        []gameAllianceMember        `json:"members"`
	Ranks          []gameAllianceRank          `json:"ranks"`
	CircularResult *gameAllianceCircularResult `json:"circularResult,omitempty"`
}

type gameAllianceViewer struct {
	PlayerID   int    `json:"playerId"`
	Name       string `json:"name"`
	Validated  bool   `json:"validated"`
	AllianceID int    `json:"allianceId"`
	RankID     int    `json:"rankId"`
	RankName   string `json:"rankName"`
	RankRights int    `json:"rankRights"`
	Founder    bool   `json:"founder"`
}

type gameAllianceInfo struct {
	ID               int    `json:"id"`
	Tag              string `json:"tag"`
	Name             string `json:"name"`
	OwnerID          int    `json:"ownerId"`
	Homepage         string `json:"homepage"`
	ImageLogo        string `json:"imageLogo"`
	Open             bool   `json:"open"`
	InsertApp        bool   `json:"insertApp"`
	ExternalText     string `json:"externalText"`
	InternalText     string `json:"internalText"`
	ApplicationText  string `json:"applicationText"`
	OldTag           string `json:"oldTag"`
	OldName          string `json:"oldName"`
	TagUntil         int64  `json:"tagUntil"`
	NameUntil        int64  `json:"nameUntil"`
	MemberCount      int    `json:"memberCount"`
	ApplicationCount int    `json:"applicationCount"`
}

type gameAllianceSearchResult struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount"`
}

type gameAllianceApplication struct {
	ID         int    `json:"id"`
	AllianceID int    `json:"allianceId"`
	PlayerID   int    `json:"playerId"`
	PlayerName string `json:"playerName"`
	Text       string `json:"text"`
	Date       int64  `json:"date"`
}

type gameAllianceMember struct {
	PlayerID  int    `json:"playerId"`
	Name      string `json:"name"`
	RankID    int    `json:"rankId"`
	RankName  string `json:"rankName"`
	Score     int64  `json:"score"`
	JoinedAt  int64  `json:"joinedAt"`
	LastClick int64  `json:"lastClick"`
	Galaxy    int    `json:"galaxy"`
	System    int    `json:"system"`
	Position  int    `json:"position"`
}

type gameAllianceRank struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Rights int    `json:"rights"`
}

type gameAllianceCircularResult struct {
	Recipients []string `json:"recipients"`
}

type gameAllianceMutationRequest struct {
	Action          string `json:"action"`
	Tag             string `json:"tag"`
	Name            string `json:"name"`
	Text            string `json:"text"`
	TextKind        int    `json:"textKind"`
	Homepage        string `json:"homepage"`
	ImageLogo       string `json:"imageLogo"`
	Open            bool   `json:"open"`
	InsertApp       bool   `json:"insertApp"`
	FounderRankName string `json:"founderRankName"`
	AllianceID      int    `json:"allianceId"`
	ApplicationID   int    `json:"applicationId"`
	RankID          int    `json:"rankId"`
	RankName        string `json:"rankName"`
	RankRights      []struct {
		ID     int `json:"id"`
		Rights int `json:"rights"`
	} `json:"rankRights"`
	TargetPlayerID int `json:"targetPlayerId"`
	TargetRankID   int `json:"targetRankId"`
	CircularRankID int `json:"circularRankId"`
}

func (a app) handleGameAlliance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGameAllianceGet(w, r)
	case http.MethodPost:
		a.handleGameAlliancePost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameAllianceGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAlliance == nil {
		http.Error(w, "game alliance unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	query := selectedAllianceQuery(r)
	result, err := a.deps.GameAlliance.GetAlliance(r.Context(), appgame.AllianceCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		View:            query.View,
		SearchText:      query.SearchText,
		TextKind:        query.TextKind,
		AllianceID:      query.AllianceID,
		ApplicationID:   query.ApplicationID,
	})
	if err != nil {
		http.Error(w, "game alliance unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameAllianceResponse(w, result)
}

func (a app) handleGameAlliancePost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAlliance == nil {
		http.Error(w, "game alliance unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	mutation, err := decodeGameAllianceMutation(r)
	if err != nil {
		http.Error(w, "invalid alliance request", http.StatusBadRequest)
		return
	}
	query := selectedAllianceQuery(r)
	result, err := a.deps.GameAlliance.MutateAlliance(r.Context(), appgame.AllianceMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Query:           query,
		Mutation:        mutation,
	})
	if err != nil {
		http.Error(w, "game alliance unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGameAllianceResponse(w, result)
}

func selectedAllianceQuery(r *http.Request) appgame.AllianceQuery {
	query := r.URL.Query()
	page := query.Get("page")
	action := query.Get("a")
	allianceID := legacyAllianceInt(query.Get("allyid"))
	view := domaingame.AllianceViewHome
	switch {
	case page == "bewerben":
		view = domaingame.AllianceViewApply
	case page == "bewerbungen":
		view = domaingame.AllianceViewApplications
	case page == "ainfo":
		view = domaingame.AllianceViewInfo
	case page == "" && action == "" && allianceID > 0:
		view = domaingame.AllianceViewInfo
	case action == "2" && allianceID > 0:
		view = domaingame.AllianceViewApply
	case action == "1":
		view = domaingame.AllianceViewCreate
	case action == "2":
		view = domaingame.AllianceViewSearch
	case action == "4":
		view = domaingame.AllianceViewMembers
	case action == "7":
		view = domaingame.AllianceViewMembers
	case action == "5", action == "11":
		view = domaingame.AllianceViewManagement
	case action == "6", action == "15":
		view = domaingame.AllianceViewRanks
	case action == "9":
		view = domaingame.AllianceViewRenameTag
	case action == "10":
		view = domaingame.AllianceViewRenameName
	case action == "17":
		view = domaingame.AllianceViewCircular
	default:
		view = domaingame.AllianceViewHome
	}
	applicationID := legacyAllianceInt(query.Get("show"))
	if applicationID == 0 {
		applicationID = legacyAllianceInt(query.Get("u"))
	}
	return appgame.AllianceQuery{
		View:          view,
		SearchText:    strings.TrimSpace(query.Get("suchtext")),
		TextKind:      domaingame.NormalizeAllianceTextKind(legacyAllianceInt(query.Get("t"))),
		AllianceID:    allianceID,
		ApplicationID: applicationID,
	}
}

func decodeGameAllianceMutation(r *http.Request) (domaingame.AllianceMutation, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var request gameAllianceMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return domaingame.AllianceMutation{}, err
		}
		rankRights := make([]domaingame.AllianceRank, 0, len(request.RankRights))
		for _, rank := range request.RankRights {
			rankRights = append(rankRights, domaingame.AllianceRank{ID: rank.ID, Rights: rank.Rights})
		}
		return domaingame.AllianceMutation{
			Action:          strings.TrimSpace(request.Action),
			Tag:             request.Tag,
			Name:            request.Name,
			Text:            request.Text,
			TextKind:        domaingame.NormalizeAllianceTextKind(request.TextKind),
			Homepage:        request.Homepage,
			ImageLogo:       request.ImageLogo,
			Open:            request.Open,
			InsertApp:       request.InsertApp,
			FounderRankName: request.FounderRankName,
			AllianceID:      request.AllianceID,
			ApplicationID:   request.ApplicationID,
			RankID:          request.RankID,
			RankName:        request.RankName,
			RankRights:      rankRights,
			TargetPlayerID:  request.TargetPlayerID,
			TargetRankID:    request.TargetRankID,
			CircularRankID:  request.CircularRankID,
		}, nil
	}
	if err := r.ParseForm(); err != nil {
		return domaingame.AllianceMutation{}, err
	}
	page := r.URL.Query().Get("page")
	action := r.URL.Query().Get("a")
	mutation := domaingame.AllianceMutation{
		Tag:             formLast(r, "tag"),
		Name:            formLast(r, "name"),
		Text:            formLast(r, "text"),
		TextKind:        domaingame.NormalizeAllianceTextKind(legacyAllianceInt(r.URL.Query().Get("t"))),
		Homepage:        formLast(r, "hp"),
		ImageLogo:       formLast(r, "logo"),
		Open:            formLast(r, "bew") != "1",
		InsertApp:       formLast(r, "bewforce") == "1",
		FounderRankName: formLast(r, "fname"),
		AllianceID:      legacyAllianceInt(r.URL.Query().Get("allyid")),
		ApplicationID:   legacyAllianceInt(r.URL.Query().Get("show")),
		RankID:          legacyAllianceInt(r.URL.Query().Get("d")),
		RankName:        formLast(r, "newrangname"),
		RankRights:      formAllianceRankRights(r),
		TargetPlayerID:  legacyAllianceInt(r.URL.Query().Get("u")),
		TargetRankID:    legacyAllianceInt(formLast(r, "newrang")),
		CircularRankID:  legacyAllianceInt(formLast(r, "r")),
	}
	switch {
	case page == "bewerben" || formLast(r, "weiter") == "Submit":
		mutation.Action = "apply"
	case page == "bewerbungen" && formLast(r, "aktion") == "Accept":
		mutation.Action = "accept"
	case page == "bewerbungen" && formLast(r, "aktion") == "Reject":
		mutation.Action = "reject"
	case action == "1" && r.URL.Query().Get("weiter") == "1":
		mutation.Action = "create"
	case action == "3":
		mutation.Action = "leave"
	case action == "15" && formLast(r, "newrangname") != "":
		mutation.Action = "add_rank"
	case action == "15" && r.URL.Query().Get("d") != "":
		mutation.Action = "delete_rank"
	case action == "15":
		mutation.Action = "save_ranks"
	case action == "16":
		mutation.Action = "assign_rank"
	case action == "17" && r.URL.Query().Get("sendmail") == "1":
		mutation.Action = "send_circular"
	case action == "11" && r.URL.Query().Get("d") == "1":
		mutation.Action = "save_text"
	case action == "11" && r.URL.Query().Get("d") == "2":
		mutation.Action = "save_settings"
	case formLast(r, "bcancel") != "":
		mutation.Action = "withdraw"
	default:
		mutation.Action = action
	}
	return mutation, nil
}

func formAllianceRankRights(r *http.Request) []domaingame.AllianceRank {
	rights := map[int]int{}
	for key := range r.PostForm {
		if !strings.HasPrefix(key, "u") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(key, "u"), "r", 2)
		if len(parts) != 2 {
			continue
		}
		rankID := legacyAllianceInt(parts[0])
		rightBit := legacyAllianceInt(parts[1])
		if rankID <= 1 || rightBit < 0 || rightBit > 8 || formLast(r, key) != "on" {
			continue
		}
		rights[rankID] |= 1 << rightBit
	}
	result := make([]domaingame.AllianceRank, 0, len(rights))
	for rankID, mask := range rights {
		result = append(result, domaingame.AllianceRank{ID: rankID, Rights: mask})
	}
	return result
}

func writeGameAllianceResponse(w http.ResponseWriter, result appgame.AllianceResult) {
	status := http.StatusOK
	var alliance *gameAllianceSummary
	if result.Authenticated {
		mapped := toGameAllianceSummary(result.Alliance)
		alliance = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameAllianceResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Alliance:      alliance,
		ActionIssue:   toGameAllianceActionIssue(result.ActionIssue),
	})
}

func toGameAllianceSummary(alliance domaingame.Alliance) gameAllianceSummary {
	planets := make([]gamePlanetSummaryResponse, 0, len(alliance.PlanetSwitcher))
	for _, planet := range alliance.PlanetSwitcher {
		planets = append(planets, toGamePlanetSummaryResponse(planet))
	}
	results := make([]gameAllianceSearchResult, 0, len(alliance.SearchResults))
	for _, row := range alliance.SearchResults {
		results = append(results, gameAllianceSearchResult{ID: row.ID, Tag: row.Tag, Name: row.Name, MemberCount: row.MemberCount})
	}
	applications := make([]gameAllianceApplication, 0, len(alliance.Applications))
	for _, app := range alliance.Applications {
		applications = append(applications, toGameAllianceApplication(app))
	}
	members := make([]gameAllianceMember, 0, len(alliance.Members))
	for _, member := range alliance.Members {
		members = append(members, gameAllianceMember{
			PlayerID:  member.PlayerID,
			Name:      member.Name,
			RankID:    member.RankID,
			RankName:  member.RankName,
			Score:     member.Score,
			JoinedAt:  member.JoinedAt,
			LastClick: member.LastClick,
			Galaxy:    member.Galaxy,
			System:    member.System,
			Position:  member.Position,
		})
	}
	ranks := make([]gameAllianceRank, 0, len(alliance.Ranks))
	for _, rank := range alliance.Ranks {
		ranks = append(ranks, gameAllianceRank{ID: rank.ID, Name: rank.Name, Rights: rank.Rights})
	}
	return gameAllianceSummary{
		Commander:      alliance.Commander,
		CurrentPlanet:  toGamePlanetOverviewResponse(alliance.CurrentPlanet),
		PlanetSwitcher: planets,
		View:           string(alliance.View),
		Viewer: gameAllianceViewer{
			PlayerID:   alliance.Viewer.PlayerID,
			Name:       alliance.Viewer.Name,
			Validated:  alliance.Viewer.Validated,
			AllianceID: alliance.Viewer.AllianceID,
			RankID:     alliance.Viewer.RankID,
			RankName:   alliance.Viewer.RankName,
			RankRights: alliance.Viewer.RankRights,
			Founder:    alliance.Viewer.Founder,
		},
		Own:            toGameAllianceInfo(alliance.Own),
		Target:         toGameAllianceInfo(alliance.Target),
		Pending:        toGameAllianceApplicationPtr(alliance.Pending),
		SearchText:     alliance.SearchText,
		TextKind:       alliance.TextKind,
		SearchResults:  results,
		Applications:   applications,
		SelectedApp:    toGameAllianceApplicationPtr(alliance.SelectedApp),
		Members:        members,
		Ranks:          ranks,
		CircularResult: toGameAllianceCircularResult(alliance.CircularResult),
	}
}

func toGameAllianceCircularResult(result *domaingame.AllianceCircularResult) *gameAllianceCircularResult {
	if result == nil {
		return nil
	}
	return &gameAllianceCircularResult{Recipients: append([]string{}, result.Recipients...)}
}

func toGameAllianceInfo(info *domaingame.AllianceInfo) *gameAllianceInfo {
	if info == nil {
		return nil
	}
	return &gameAllianceInfo{
		ID:               info.ID,
		Tag:              info.Tag,
		Name:             info.Name,
		OwnerID:          info.OwnerID,
		Homepage:         info.Homepage,
		ImageLogo:        info.ImageLogo,
		Open:             info.Open,
		InsertApp:        info.InsertApp,
		ExternalText:     info.ExternalText,
		InternalText:     info.InternalText,
		ApplicationText:  info.ApplicationText,
		OldTag:           info.OldTag,
		OldName:          info.OldName,
		TagUntil:         info.TagUntil,
		NameUntil:        info.NameUntil,
		MemberCount:      info.MemberCount,
		ApplicationCount: info.ApplicationCount,
	}
}

func toGameAllianceApplicationPtr(application *domaingame.AllianceApplication) *gameAllianceApplication {
	if application == nil {
		return nil
	}
	mapped := toGameAllianceApplication(*application)
	return &mapped
}

func toGameAllianceApplication(application domaingame.AllianceApplication) gameAllianceApplication {
	return gameAllianceApplication{
		ID:         application.ID,
		AllianceID: application.AllianceID,
		PlayerID:   application.PlayerID,
		PlayerName: application.PlayerName,
		Text:       application.Text,
		Date:       application.Date,
	}
}

func toGameAllianceActionIssue(issue *domaingame.AllianceActionIssue) *gameAllianceActionIssue {
	if issue == nil {
		return nil
	}
	return &gameAllianceActionIssue{Code: issue.Code, Message: issue.Message}
}

func legacyAllianceInt(value string) int {
	value = strings.TrimSpace(value)
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

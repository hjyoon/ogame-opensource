package httpdelivery

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
)

type gameAdminBotEditUseCase interface {
	MutateAdminBotEdit(context.Context, appgame.AdminBotEditMutationCommand) (appgame.AdminBotEditMutationResult, error)
}

func (a app) handleLegacyGameIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		a.handleFrontend(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "BotEdit") {
		a.handleLegacyBotEditPost(w, r)
		return
	}
	w.Header().Set("Allow", "GET, HEAD, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (a app) handleLegacyBotEditPost(w http.ResponseWriter, r *http.Request) {
	usecase, ok := a.deps.GameAdmin.(gameAdminBotEditUseCase)
	if !ok {
		http.Error(w, "game admin botedit unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid botedit request", http.StatusBadRequest)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	source := formLast(r, "source")
	if decoded, err := url.QueryUnescape(source); err == nil {
		source = decoded
	}
	result, err := usecase.MutateAdminBotEdit(r.Context(), appgame.AdminBotEditMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          formLast(r, "action"),
		StrategyID:      legacyBotEditInt(formLast(r, "strat")),
		Name:            formLast(r, "name"),
		Source:          source,
	})
	if err != nil {
		http.Error(w, "game admin botedit unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Authenticated {
		http.Error(w, "unauthenticated", http.StatusForbidden)
		return
	}
	if result.ActionIssue != nil {
		http.Error(w, result.ActionIssue.Message, http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch formLast(r, "action") {
	case "load":
		_, _ = w.Write([]byte(result.Source))
	case "rename":
		_, _ = w.Write([]byte(legacyBotEditOptionsHTML(result)))
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func legacyBotEditOptionsHTML(result appgame.AdminBotEditMutationResult) string {
	var builder strings.Builder
	builder.WriteString(`<option value="0">-- Choose a strategy --</option>` + "\n")
	for _, strategy := range result.Strategies {
		builder.WriteString(`<option value="`)
		builder.WriteString(strconv.Itoa(strategy.ID))
		builder.WriteString(`"`)
		if strategy.ID == result.SelectedStrategyID {
			builder.WriteString(` selected`)
		}
		builder.WriteString(`>`)
		builder.WriteString(html.EscapeString(strategy.Name))
		builder.WriteString(`</option>` + "\n")
	}
	return builder.String()
}

func legacyBotEditInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return number
}

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
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "BotEdit") && r.URL.Query().Get("action") != "" {
			a.handleLegacyBotEditGet(w, r)
			return
		}
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

func (a app) handleLegacyBotEditGet(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	if action != "preview" && action != "export" {
		a.handleFrontend(w, r)
		return
	}
	usecase, ok := a.deps.GameAdmin.(gameAdminBotEditUseCase)
	if !ok {
		http.Error(w, "game admin botedit unavailable", http.StatusServiceUnavailable)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := usecase.MutateAdminBotEdit(r.Context(), appgame.AdminBotEditMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Action:          "load",
		StrategyID:      legacyBotEditInt(r.URL.Query().Get("strat")),
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
	if action == "export" {
		_, _ = w.Write([]byte(result.Source))
		return
	}
	_, _ = w.Write([]byte(legacyBotEditPreviewHTML(r.URL.Query().Get("session"), result)))
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

func legacyBotEditPreviewHTML(session string, result appgame.AdminBotEditMutationResult) string {
	title := html.EscapeString(result.Name)
	if title == "" {
		title = strconv.Itoa(result.SelectedStrategyID)
	}
	source := html.EscapeString(result.Source)
	strategyID := strconv.Itoa(result.SelectedStrategyID)
	sessionScript := html.EscapeString(session)
	return `<!doctype html>
<html>
 <head>
  <link rel="stylesheet" type="text/css" href="/public-assets/game/css/default.css" />
  <link rel="stylesheet" type="text/css" href="/public-assets/game/css/formate.css" />
  <link rel="stylesheet" type="text/css" href="/public-assets/game/css/combox.css" />
  <script>var session="` + sessionScript + `";</script>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <title>` + title + `</title>
  <script src="/public-assets/game/js/utilities.js" type="text/javascript"></script>
 </head>
 <body>
  <script type="text/javascript" src="/public-assets/game/js/tw-sack.js"></script>
  <script type="text/javascript" src="/public-assets/game/js/go.js"></script>
  <script type="text/javascript" src="/public-assets/game/js/go-game.js"></script>
  <div id="sample">
   <div style="width:100%; white-space:nowrap; display:none;">
    <span style="display: inline-block; vertical-align: top; padding: 5px; width:100px">
     <div id="myPalette" style="background-color: #344566; border: solid 1px black; height: 500px"></div>
    </span>
    <span style="display: inline-block; vertical-align: top; padding: 5px; width:88%">
     <div id="myDiagram" style="background-color: #344566; border: solid 1px black; height: 500px"></div>
    </span>
   </div>
   <input type="hidden" id="strategyId_ForImport" name="strategyId_ForImport" value="0" >
   <input type="text" size="50" id="strategyName" style="display:none;">
   <select id="strategyId" style="display:none;">
    <option value="` + strategyID + `" selected>` + strategyID + `</option>
   </select>
   <textarea id="mySavedModel" style="width:100%;height:300px; display:none;">` + source + `  </textarea>
  </div>
  <img src="" id="preview_img">
  <script type="text/javascript">init();</script>
 </body>
</html>
`
}

func legacyBotEditInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return number
}

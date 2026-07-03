package httpdelivery

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
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
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "Mods") && r.URL.Query().Get("action") != "" {
			a.handleLegacyAdminModsGet(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "Bots") && r.URL.Query().Get("action") != "" {
			a.handleLegacyAdminBotsGet(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "infos" && r.URL.Query().Get("gid") == "43" {
			a.handleLegacyJumpGateInfoGet(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "pranger" {
			a.handleLegacyPranger(w, r)
			return
		}
		a.handleFrontend(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "BotEdit") {
		a.handleLegacyBotEditPost(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "Bots") {
		a.handleLegacyAdminBotsPost(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "Logins") {
		a.handleLegacyAdminLoginsPost(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "admin" && strings.EqualFold(r.URL.Query().Get("mode"), "Loca") {
		a.handleLegacyAdminLocaPost(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("page") == "sprungtor" {
		a.handleLegacyJumpGatePost(w, r)
		return
	}
	w.Header().Set("Allow", "GET, HEAD, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (a app) handleLegacyAdminBotsPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid admin bots request", http.StatusBadRequest)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.MutateAdmin(r.Context(), appgame.AdminMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            "Bots",
		Action:          domaingame.AdminActionBotAdd,
		Name:            formLast(r, "name"),
	})
	if err != nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Authenticated {
		http.Error(w, "unauthenticated", http.StatusForbidden)
		return
	}
	if result.ActionIssue != nil && result.ActionIssue.Code == domaingame.AdminIssueAccessDenied {
		http.Error(w, result.ActionIssue.Message, http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyAdminBotsHTML(r.URL.Query().Get("session"), result.Admin.BotRows, result.ActionIssue)))
}

func legacyAdminBotsHTML(session string, rows []domaingame.AdminBotRow, issue *domaingame.AdminActionIssue) string {
	var builder strings.Builder
	if issue != nil {
		color := "lime"
		if issue.Code == domaingame.AdminIssueBotExists || issue.Code == domaingame.AdminIssueBotNoStart || issue.Code == domaingame.AdminIssueAccessDenied {
			color = "red"
		}
		builder.WriteString(`<center><font color=`)
		builder.WriteString(color)
		builder.WriteString(`>`)
		builder.WriteString(html.EscapeString(issue.Message))
		builder.WriteString(`</font></center>`)
	}
	builder.WriteString("<h2>Bot List:</h2>")
	if len(rows) == 0 {
		builder.WriteString("No bots found<br>")
	} else {
		builder.WriteString(`<table><tr><td class=c>ID</td><td class=c>Name</td><td class=c>Home Planet</td><td class=c>Action</td></tr>`)
		for _, row := range rows {
			builder.WriteString("<tr><td>")
			builder.WriteString(strconv.Itoa(row.PlayerID))
			builder.WriteString(`</td><td><a href="index.php?page=admin&amp;session=`)
			builder.WriteString(html.EscapeString(url.QueryEscape(session)))
			builder.WriteString(`&amp;mode=Users&amp;player_id=`)
			builder.WriteString(strconv.Itoa(row.PlayerID))
			builder.WriteString(`">`)
			builder.WriteString(html.EscapeString(row.Name))
			builder.WriteString("</a></td><td>")
			if row.HomePlanet != nil {
				builder.WriteString(`<a href="index.php?page=admin&amp;session=`)
				builder.WriteString(html.EscapeString(url.QueryEscape(session)))
				builder.WriteString(`&amp;mode=Planets&amp;cp=`)
				builder.WriteString(strconv.Itoa(row.HomePlanet.ID))
				builder.WriteString(`">`)
				builder.WriteString(html.EscapeString(row.HomePlanet.Name))
				builder.WriteString(`</a> [<a href="index.php?page=galaxy&amp;session=`)
				builder.WriteString(html.EscapeString(url.QueryEscape(session)))
				builder.WriteString(`&amp;galaxy=`)
				builder.WriteString(strconv.Itoa(row.HomePlanet.Coordinates.Galaxy))
				builder.WriteString(`&amp;system=`)
				builder.WriteString(strconv.Itoa(row.HomePlanet.Coordinates.System))
				builder.WriteString(`">`)
				builder.WriteString(strconv.Itoa(row.HomePlanet.Coordinates.Galaxy))
				builder.WriteString(":")
				builder.WriteString(strconv.Itoa(row.HomePlanet.Coordinates.System))
				builder.WriteString(":")
				builder.WriteString(strconv.Itoa(row.HomePlanet.Coordinates.Position))
				builder.WriteString(`</a>]`)
			}
			builder.WriteString(`</td><td><a href="index.php?page=admin&amp;session=`)
			builder.WriteString(html.EscapeString(url.QueryEscape(session)))
			builder.WriteString(`&amp;mode=Bots&amp;action=stop&amp;id=`)
			builder.WriteString(strconv.Itoa(row.PlayerID))
			builder.WriteString(`">Stop</a></td></tr>`)
		}
		builder.WriteString("</table>")
	}
	builder.WriteString(`<h2>Add bot:</h2><form action="index.php?page=admin&amp;session=`)
	builder.WriteString(html.EscapeString(url.QueryEscape(session)))
	builder.WriteString(`&amp;mode=Bots" method="POST"><table><tr><td>Name <input type=text size=10 name="name" /> <input type=submit value="Submit" /></td></tr></table></form>`)
	return builder.String()
}

func (a app) handleLegacyAdminLoginsPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid admin logins request", http.StatusBadRequest)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.GetAdmin(r.Context(), appgame.AdminCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            "Logins",
		LoginName:       formLast(r, "name"),
		LoginUserID:     legacyBotEditInt(formLast(r, "id")),
		LoginIP:         formLast(r, "ip"),
	})
	if err != nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Authenticated {
		http.Error(w, "unauthenticated", http.StatusForbidden)
		return
	}
	if result.ActionIssue != nil && result.ActionIssue.Code == domaingame.AdminIssueAccessDenied {
		http.Error(w, result.ActionIssue.Message, http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyAdminLoginsHTML(r.URL.Query().Get("session"), result.Admin.LoginRows)))
}

func legacyAdminLoginsHTML(session string, rows []domaingame.AdminLoginRow) string {
	var builder strings.Builder
	if len(rows) > 0 {
		builder.WriteString("<table>")
		for _, row := range rows {
			builder.WriteString("<tr><td>")
			builder.WriteString(html.EscapeString(legacyAdminDateTime(row.Date)))
			builder.WriteString(" ")
			builder.WriteString(html.EscapeString(row.IP))
			builder.WriteString(" ")
			if row.UserID > 0 {
				builder.WriteString(`<a href="index.php?page=admin&amp;session=`)
				builder.WriteString(html.EscapeString(url.QueryEscape(session)))
				builder.WriteString(`&amp;mode=Users&amp;player_id=`)
				builder.WriteString(strconv.Itoa(row.UserID))
				builder.WriteString(`">`)
				builder.WriteString(html.EscapeString(row.UserName))
				builder.WriteString("</a>")
			} else {
				builder.WriteString(html.EscapeString(row.UserName))
			}
			builder.WriteString("</td></tr>")
		}
		builder.WriteString("</table>")
	}
	builder.WriteString(`<form action="index.php?page=admin&amp;session=`)
	builder.WriteString(html.EscapeString(url.QueryEscape(session)))
	builder.WriteString(`&amp;mode=Logins" method="POST"><table>`)
	builder.WriteString(`<tr><td class=d>By user name:</td><td><input type=text size=20 name=name></td></tr>`)
	builder.WriteString(`<tr><td class=d>By User ID:</td><td><input type=text size=20 name=id></td></tr>`)
	builder.WriteString(`<tr><td class=d>By IP address:</td><td><input type=text size=20 name=ip></td></tr>`)
	builder.WriteString(`<tr><td colspan=2 class=d><center><input type="submit" value="Search"></center></td></tr>`)
	builder.WriteString(`</table></form>`)
	return builder.String()
}

func legacyAdminDateTime(timestamp int64) string {
	return time.Unix(timestamp+3*60*60, 0).UTC().Format("2006-01-02 15:04:05")
}

func (a app) handleLegacyAdminLocaPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameAdmin == nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid admin localization request", http.StatusBadRequest)
		return
	}
	planetID, err := selectedPlanetID(r)
	if err != nil {
		http.Error(w, "invalid selected planet", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GameAdmin.GetAdmin(r.Context(), appgame.AdminCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		PlanetID:        planetID,
		Mode:            "Loca",
		LocaSource:      formLast(r, "loca_src"),
		LocaTarget:      formLast(r, "loca_dst"),
	})
	if err != nil {
		http.Error(w, "game admin unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Authenticated {
		http.Error(w, "unauthenticated", http.StatusForbidden)
		return
	}
	if result.ActionIssue != nil && result.ActionIssue.Code == domaingame.AdminIssueAccessDenied {
		http.Error(w, result.ActionIssue.Message, http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyAdminLocaHTML(r.URL.Query().Get("session"), result.Admin.Localization)))
}

func legacyAdminLocaHTML(session string, localization *domaingame.AdminLocalization) string {
	var builder strings.Builder
	builder.WriteString(`<table><form action="index.php?page=admin&amp;session=`)
	builder.WriteString(html.EscapeString(url.QueryEscape(session)))
	builder.WriteString(`&amp;mode=Loca&amp;action=search" method="POST"><tr><td class="c" colspan=2>Compare localization between the specified languages</td></tr>`)
	builder.WriteString(`<tr><td>Source language:</td><td><select name="loca_src">`)
	builder.WriteString(legacyAdminLocaOptions(localization, true))
	builder.WriteString(`</select></td></tr><tr/><tr><td>Target language:</td><td><select name="loca_dst">`)
	builder.WriteString(legacyAdminLocaOptions(localization, false))
	builder.WriteString(`</select></td></tr><tr><td class="c" colspan=2><input type="submit" value="Compare" /></td></tr></form></table><br/>`)
	if localization == nil {
		return builder.String()
	}
	for _, file := range localization.Files {
		builder.WriteString("<h2>")
		builder.WriteString(html.EscapeString(file.Name))
		builder.WriteString("</h2>\n\n")
		if file.TargetMissing {
			builder.WriteString(`<font color=red>The file is not localized!</font><br/>`)
			continue
		}
		builder.WriteString("<table>\n")
		for _, row := range file.Rows {
			color := "green"
			if row.Status == "same" {
				color = "orange"
			}
			if row.Status == "missing" {
				color = "red"
			}
			builder.WriteString(`<tr><td style="background-color: `)
			builder.WriteString(color)
			builder.WriteString(`;">`)
			builder.WriteString(html.EscapeString(row.Key))
			builder.WriteString(`</td><td style="background-color: `)
			builder.WriteString(color)
			builder.WriteString(`;"><pre>`)
			builder.WriteString(html.EscapeString(row.Source))
			builder.WriteString(`</pre></td><td style="background-color: `)
			builder.WriteString(color)
			builder.WriteString(`;"><pre>`)
			builder.WriteString(html.EscapeString(row.Target))
			builder.WriteString("</pre></td></tr>")
		}
		builder.WriteString("</table>\n")
	}
	return builder.String()
}

func legacyAdminLocaOptions(localization *domaingame.AdminLocalization, source bool) string {
	if localization == nil {
		return ""
	}
	selected := localization.Target
	if source {
		selected = localization.Source
	}
	var builder strings.Builder
	for _, language := range localization.Languages {
		builder.WriteString(`<option value="`)
		builder.WriteString(html.EscapeString(language))
		builder.WriteString(`"`)
		if language == selected {
			builder.WriteString(" selected")
		}
		builder.WriteString(">")
		builder.WriteString(html.EscapeString(language))
		builder.WriteString("</option>\n")
	}
	return builder.String()
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

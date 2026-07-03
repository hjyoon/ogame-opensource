package httpdelivery

import (
	"context"
	"html"
	"net/http"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameMaintenanceUseCase interface {
	GetMaintenance(context.Context) (domaingame.Maintenance, error)
}

func (a app) handleLegacyMaintenance(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameMaintenance == nil {
		http.Error(w, "game maintenance unavailable", http.StatusServiceUnavailable)
		return
	}
	maintenance, err := a.deps.GameMaintenance.GetMaintenance(r.Context())
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("legacy maintenance unavailable", "error", err.Error())
		}
		http.Error(w, "game maintenance unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !maintenance.Frozen {
		_, _ = w.Write([]byte(legacyMaintenanceRedirectHTML(a.deps.CurrentMaintenanceStartPage())))
		return
	}
	_, _ = w.Write([]byte(legacyMaintenanceHTML(maintenance)))
}

func legacyMaintenanceRedirectHTML(startPage string) string {
	return "<html><head><meta http-equiv='refresh' content='0;url=" + html.EscapeString(startPage) + "' /></head><body></body>"
}

func legacyMaintenanceHTML(maintenance domaingame.Maintenance) string {
	text := legacyMaintenanceText(maintenance.Language)
	var builder strings.Builder
	builder.WriteString(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">` + "\n")
	builder.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml">` + "\n")
	builder.WriteString("<head>\n")
	builder.WriteString(`	<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />` + "\n")
	builder.WriteString("	<title>")
	builder.WriteString(html.EscapeString(text.title))
	builder.WriteString("</title>\n")
	builder.WriteString(`    <style type="text/css" >` + "\n")
	builder.WriteString(`        <!--
html, body, div, span, applet, object, iframe,
h1, h2, h3, h4, h5, h6, p, blockquote, pre,
a, abbr, acronym, address, big, cite, code,
del, dfn, em, font, img, ins, kbd, q, s, samp,
small, strike, strong, sub, sup, tt, var,
dl, dt, dd, ol, ul, li,
fieldset, form, label, legend,
table, caption, tbody, tfoot, thead, tr, th, td {
	margin: 0;
	padding: 0;
	border: 0;
	outline: 0;
	font-weight: inherit;
	font-style: inherit;
	font-size: 100%;
	font-family: inherit;
	vertical-align: baseline;
}

body#maintenance {
  	background:#000000 url(img/maintenance-background.jpg) no-repeat;
	color: #848484;
  	font-size: 12px;
  	font-family: Verdana, Arial, SunSans-Regular, Sans-Serif;
	padding:0px 0 0;
	margin:0px 0 0;
}

#maintenance #infowrapper {
	position:absolute;
	width:315px;
	height:180px;
	left:349px;
	top:242px;
    padding-left: 20px;
    padding-right: 20px;
}

#maintenance #infowrapper h2 {
	font-weight:700;
	margin:0;
    padding-top: 3px;
	text-align: center;
	margin:0 0 40px;
}
a,
a:link,
a:visited,
a:active {
	color:#6F9FC8;;
    text-decoration: none;
}

a:hover {
    text-decoration: underline;
}
        -->
    </style>
</head>

<body id="maintenance">
    <div id="infowrapper">
        <h2>`)
	builder.WriteString(html.EscapeString(text.head))
	builder.WriteString(`</h2>
        <p>`)
	builder.WriteString(html.EscapeString(strings.ReplaceAll(text.info1, "#1", "OGame")))
	builder.WriteString(`</p>
        <p>`)
	builder.WriteString(html.EscapeString(text.info2))
	builder.WriteString(`</p>
        <br/>
        <br/>
        <br/>
        <p>
`)
	if maintenance.BoardURL != "" {
		builder.WriteString("        ")
		builder.WriteString(strings.ReplaceAll(text.boardLink, "#1", html.EscapeString(maintenance.BoardURL)))
		builder.WriteString("\n")
	}
	builder.WriteString(`        </p>
    </div>
</body>
</html>`)
	return builder.String()
}

type legacyMaintenanceCopy struct {
	title     string
	head      string
	info1     string
	info2     string
	boardLink string
}

func legacyMaintenanceText(language string) legacyMaintenanceCopy {
	if language == "fr" {
		return legacyMaintenanceCopy{
			title:     "Maintenance",
			head:      "Maintenance",
			info1:     "#1 est en maintenance.",
			info2:     "Nous serons bientot de nouveau en ligne.",
			boardLink: `Merci de verifier les <a href="#1">annonces</a> pour les mises a jour.`,
		}
	}
	return legacyMaintenanceCopy{
		title:     "Maintenance",
		head:      "Maintenance",
		info1:     "#1 is currently under maintenance.",
		info2:     "We should be back online shortly.",
		boardLink: `Please check out the <a href="#1">Board</a> for updates.`,
	}
}

var _ gameMaintenanceUseCase = appgame.MaintenanceService{}

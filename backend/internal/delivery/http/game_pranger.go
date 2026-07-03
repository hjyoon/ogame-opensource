package httpdelivery

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (a app) handleLegacyPranger(w http.ResponseWriter, r *http.Request) {
	if a.deps.GamePranger == nil {
		http.Error(w, "game pranger unavailable", http.StatusServiceUnavailable)
		return
	}
	from := legacyPrangerInt(r.URL.Query().Get("from"))
	pranger, err := a.deps.GamePranger.GetPranger(r.Context(), appgame.PrangerCommand{
		Universe: a.deps.CurrentUniverseNumber(),
		From:     from,
		Internal: r.URL.Query().Has("session"),
	})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("legacy pranger unavailable", "error", err.Error())
		}
		http.Error(w, "game pranger unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyPrangerHTML(r.URL.Query().Get("session"), pranger)))
}

func legacyPrangerHTML(session string, pranger domaingame.Pranger) string {
	var builder strings.Builder
	builder.WriteString("   <h1>")
	builder.WriteString(html.EscapeString(fmt.Sprintf("OGame Pillory Universe %d", pranger.Universe)))
	builder.WriteString("</h1>\n")
	builder.WriteString("   <p>Here is a list of players who have been banned, until when and for what reason. \n")
	builder.WriteString("<br />Blocking by the Admin Council and the system is NOT negotiable. \n")
	builder.WriteString("<br />Attention! Your message will be processed faster with an automatic subject line.</p>\n\n")
	builder.WriteString("   <table border=\"0\" cellpadding=\"2\" cellspacing=\"1\">\n")
	builder.WriteString("    <tr height=\"20\">\n")
	builder.WriteString("     <td class=\"c\">Ban Date</td>\n")
	builder.WriteString("     <td class=\"c\">Admin Name</td>\n")
	builder.WriteString("     <td class=\"c\">Player Name</td>\n")
	builder.WriteString("     <td class=\"c\">Blocked Until</td>\n")
	builder.WriteString("     <td class=\"c\">Reason</td>\n")
	builder.WriteString("    </tr>\n\n")
	for _, entry := range pranger.Entries {
		builder.WriteString("        <tr height=\"20\">\n")
		builder.WriteString("     <th>")
		builder.WriteString(legacyPrangerDate(entry.BanWhen))
		builder.WriteString(" </th>\n\n")
		builder.WriteString("          <th>\n")
		builder.WriteString("       ")
		builder.WriteString(html.EscapeString(entry.AdminName))
		builder.WriteString("     </th>\n\n")
		builder.WriteString("     <th>")
		builder.WriteString(html.EscapeString(entry.UserName))
		builder.WriteString("</th>\n")
		builder.WriteString("     <th>")
		builder.WriteString(legacyPrangerDate(entry.BanUntil))
		builder.WriteString("</th>\n")
		builder.WriteString("     <th>")
		builder.WriteString(entry.Reason)
		builder.WriteString("</th>\n")
		builder.WriteString("    </tr>\n")
	}
	builder.WriteString("       <tr>\n")
	builder.WriteString("   <th colspan=\"5\">\n")
	if pranger.HasPrevious() {
		builder.WriteString("     <a href=\"")
		builder.WriteString(legacyPrangerPageURL(session, pranger.Internal, pranger.PreviousFrom()))
		builder.WriteString("\">&lt;&lt; Previous 50</a>&nbsp;&nbsp;&nbsp;&nbsp;\n")
	}
	if pranger.HasNext() {
		builder.WriteString("        <a href=\"")
		builder.WriteString(legacyPrangerPageURL(session, pranger.Internal, pranger.NextFrom()))
		builder.WriteString("\">Next 50 &gt;&gt;</a>\n")
	}
	builder.WriteString("      </th>\n")
	builder.WriteString("   </tr>\n")
	builder.WriteString("   </table>\n")
	return builder.String()
}

func legacyPrangerPageURL(session string, internal bool, from int) string {
	if internal {
		return "index.php?page=pranger&session=" + html.EscapeString(session) + "&from=" + strconv.Itoa(from)
	}
	return "pranger.php?from=" + strconv.Itoa(from)
}

func legacyPrangerDate(timestamp int64) string {
	date := time.Unix(timestamp, 0).UTC()
	return fmt.Sprintf("%s %s %d %d %d:%02d:%02d",
		date.Format("Mon"),
		date.Format("Jan"),
		date.Day(),
		date.Year(),
		date.Hour(),
		date.Minute(),
		date.Second(),
	)
}

func legacyPrangerInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return domaingame.NormalizePrangerFrom(number)
}

type gamePrangerUseCase interface {
	GetPranger(context.Context, appgame.PrangerCommand) (domaingame.Pranger, error)
}

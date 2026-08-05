package httpdelivery

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gamePrangerResponse struct {
	Pranger *gamePrangerSummary `json:"pranger,omitempty"`
	Error   string              `json:"error,omitempty"`
}

type gamePrangerSummary struct {
	Universe     int                        `json:"universe"`
	From         int                        `json:"from"`
	HasPrevious  bool                       `json:"hasPrevious"`
	PreviousFrom int                        `json:"previousFrom"`
	HasNext      bool                       `json:"hasNext"`
	NextFrom     int                        `json:"nextFrom"`
	Entries      []gamePrangerEntryResponse `json:"entries"`
}

type gamePrangerEntryResponse struct {
	BanWhen      string `json:"banWhen"`
	BanWhenUnix  int64  `json:"banWhenUnix"`
	AdminName    string `json:"adminName"`
	UserName     string `json:"userName"`
	BanUntil     string `json:"banUntil"`
	BanUntilUnix int64  `json:"banUntilUnix"`
	Reason       string `json:"reason"`
}

func (a app) handleGamePranger(w http.ResponseWriter, r *http.Request) {
	if a.deps.GamePranger == nil {
		writeGamePrangerError(w, http.StatusServiceUnavailable, "game pranger unavailable")
		return
	}
	pranger, err := a.deps.GamePranger.GetPranger(r.Context(), appgame.PrangerCommand{
		Universe: a.deps.CurrentUniverseNumber(),
		From:     legacyPrangerInt(r.URL.Query().Get("from")),
		Internal: false,
	})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("game pranger unavailable", "error", err.Error())
		}
		writeGamePrangerError(w, http.StatusServiceUnavailable, "game pranger unavailable")
		return
	}
	summary := toGamePrangerSummary(pranger)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(gamePrangerResponse{Pranger: &summary})
}

func writeGamePrangerError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gamePrangerResponse{Error: message})
}

func toGamePrangerSummary(pranger domaingame.Pranger) gamePrangerSummary {
	entries := make([]gamePrangerEntryResponse, 0, len(pranger.Entries))
	for _, entry := range pranger.Entries {
		entries = append(entries, gamePrangerEntryResponse{
			BanWhen:      legacyPrangerBanDate(entry.BanWhen),
			BanWhenUnix:  entry.BanWhen,
			AdminName:    entry.AdminName,
			UserName:     entry.UserName,
			BanUntil:     legacyPrangerDate(entry.BanUntil),
			BanUntilUnix: entry.BanUntil,
			Reason:       entry.Reason,
		})
	}
	return gamePrangerSummary{
		Universe:     pranger.Universe,
		From:         pranger.From,
		HasPrevious:  pranger.HasPrevious(),
		PreviousFrom: pranger.PreviousFrom(),
		HasNext:      pranger.HasNext(),
		NextFrom:     pranger.NextFrom(),
		Entries:      entries,
	}
}

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
		builder.WriteString(legacyPrangerBrowserTime(entry.BanWhen))
		builder.WriteString(" </th>\n\n")
		builder.WriteString("          <th>\n")
		builder.WriteString("       ")
		builder.WriteString(html.EscapeString(entry.AdminName))
		builder.WriteString("     </th>\n\n")
		builder.WriteString("     <th>")
		builder.WriteString(html.EscapeString(entry.UserName))
		builder.WriteString("</th>\n")
		builder.WriteString("     <th>")
		builder.WriteString(legacyPrangerBrowserTime(entry.BanUntil))
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
	builder.WriteString(legacyPrangerLocalTimeScript)
	return builder.String()
}

func legacyPrangerBrowserTime(timestamp int64) string {
	utc := time.Unix(timestamp, 0).UTC()
	return fmt.Sprintf(
		`<time datetime="%s" data-ogame-unix="%d">%s</time>`,
		utc.Format(time.RFC3339),
		timestamp,
		formatLegacyPrangerDate(utc),
	)
}

const legacyPrangerLocalTimeScript = `<script>
(() => {
  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  const pad = (value) => String(value).padStart(2, "0");
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  document.querySelectorAll("time[data-ogame-unix]").forEach((element) => {
    const date = new Date(Number(element.dataset.ogameUnix) * 1000);
    if (Number.isNaN(date.getTime())) return;
    element.textContent = weekdays[date.getDay()] + " " + months[date.getMonth()] + " " + date.getDate() + " " + date.getFullYear() + " " + date.getHours() + ":" + pad(date.getMinutes()) + ":" + pad(date.getSeconds());
    element.title = "Displayed in browser timezone " + timezone;
  });
})();
</script>`

func legacyPrangerPageURL(session string, internal bool, from int) string {
	if internal {
		return "index.php?page=pranger&session=" + html.EscapeString(session) + "&from=" + strconv.Itoa(from)
	}
	return "pranger.php?from=" + strconv.Itoa(from)
}

func legacyPrangerDate(timestamp int64) string {
	return formatLegacyPrangerDate(time.Unix(timestamp, 0).UTC())
}

func legacyPrangerBanDate(timestamp int64) string {
	return formatLegacyPrangerDate(time.Unix(timestamp, 0).UTC())
}

func formatLegacyPrangerDate(date time.Time) string {
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

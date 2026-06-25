package httpdelivery

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type GameFeedUseCase interface {
	GetFeed(context.Context, appgame.FeedQuery) (domaingame.Feed, error)
	GetFeedItem(context.Context, appgame.FeedItemQuery) (domaingame.FeedItem, error)
}

var legacyFeedIDPattern = regexp.MustCompile(`^[A-Fa-f0-9]{1,32}$`)

const legacyFeedValidationError = "Error validating request parameters: feedid"

func (a app) handleGameFeedShow(w http.ResponseWriter, r *http.Request) {
	feedID := r.URL.Query().Get("feedid")
	if feedID == "" {
		writeLegacyFeedText(w, "No feed specified")
		return
	}
	if !legacyFeedIDPattern.MatchString(feedID) {
		writeLegacyFeedText(w, legacyFeedValidationError)
		return
	}
	if a.deps.GameFeed == nil {
		http.Error(w, "game feed unavailable", http.StatusServiceUnavailable)
		return
	}
	feed, err := a.deps.GameFeed.GetFeed(r.Context(), appgame.FeedQuery{FeedID: feedID})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("game feed unavailable", "error", err.Error())
		}
		http.Error(w, "game feed unavailable", http.StatusServiceUnavailable)
		return
	}
	if feed.FeedID == "" {
		writeLegacyFeedText(w, "Authentifizierung fehlgeschlagen")
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if feed.Atom {
		_, _ = w.Write([]byte(renderLegacyAtomFeed(r, feed)))
		return
	}
	_, _ = w.Write([]byte(renderLegacyRSSFeed(r, feed)))
}

func (a app) handleGameFeedItem(w http.ResponseWriter, r *http.Request) {
	feedID := r.URL.Query().Get("feedid")
	if feedID == "" {
		writeLegacyFeedText(w, "No feed specified")
		return
	}
	if !legacyFeedIDPattern.MatchString(feedID) {
		writeLegacyFeedText(w, legacyFeedValidationError)
		return
	}
	messageIDRaw := r.URL.Query().Get("mid")
	if messageIDRaw == "" {
		writeLegacyFeedText(w, "No message specified")
		return
	}
	messageID, err := strconv.Atoi(messageIDRaw)
	if err != nil || messageID <= 0 {
		writeLegacyFeedText(w, "Error validating request parameters: mid")
		return
	}
	if a.deps.GameFeed == nil {
		http.Error(w, "game feed unavailable", http.StatusServiceUnavailable)
		return
	}
	item, err := a.deps.GameFeed.GetFeedItem(r.Context(), appgame.FeedItemQuery{FeedID: feedID, MessageID: messageID})
	if err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Error("game feed item unavailable", "error", err.Error())
		}
		http.Error(w, "game feed item unavailable", http.StatusServiceUnavailable)
		return
	}
	if item.Subject == "" && item.Text == "" {
		writeLegacyFeedText(w, "")
		return
	}

	subject := html.EscapeString(domaingame.FeedText(item.Subject))
	text := html.EscapeString(domaingame.FeedText(item.Text))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p><body></html>", subject, subject, text)
}

func writeLegacyFeedText(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func renderLegacyRSSFeed(r *http.Request, feed domaingame.Feed) string {
	var builder strings.Builder
	feedURL := xmlText(publicURL(r, "/game/feed/show.php?feedid="+url.QueryEscape(feed.FeedID)))
	feedTitle := xmlText("OGame-Nachrichten von " + feed.Owner)
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString("\n<rss version=\"2.0\">\n<channel>\n")
	builder.WriteString("<title>")
	builder.WriteString(feedTitle)
	builder.WriteString("</title>\n<link>")
	builder.WriteString(feedURL)
	builder.WriteString("</link>\n<description>")
	builder.WriteString(feedTitle)
	builder.WriteString("</description>\n")
	for _, message := range feed.Messages {
		itemURL := xmlText(publicURL(r, fmt.Sprintf("/game/feed/viewitem.php?mid=%d&feedid=%s&type=i", message.ID, url.QueryEscape(feed.FeedID))))
		title := xmlText(domaingame.FeedText(message.Subject))
		text := domaingame.FeedCData(domaingame.FeedText(message.Text))
		builder.WriteString("<item>\n<title>")
		builder.WriteString(title)
		builder.WriteString("</title>\n<link>")
		builder.WriteString(itemURL)
		builder.WriteString("</link>\n<guid>")
		builder.WriteString(itemURL)
		builder.WriteString("</guid>\n<pubDate>")
		builder.WriteString(time.Unix(message.Date, 0).UTC().Format(time.RFC1123Z))
		builder.WriteString("</pubDate>\n<description><![CDATA[")
		builder.WriteString(text)
		builder.WriteString("]]></description>\n</item>\n")
	}
	builder.WriteString("</channel>\n</rss>\n")
	return builder.String()
}

func renderLegacyAtomFeed(r *http.Request, feed domaingame.Feed) string {
	var builder strings.Builder
	feedURL := publicURL(r, "/game/feed/show.php?feedid="+url.QueryEscape(feed.FeedID))
	feedTitle := xmlText("OGame-Nachrichten von " + feed.Owner)
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString("\n<feed xmlns=\"http://www.w3.org/2005/Atom\">\n<title>")
	builder.WriteString(feedTitle)
	builder.WriteString("</title>\n<link href=\"")
	builder.WriteString(xmlText(feedURL))
	builder.WriteString("\" />\n<id>")
	builder.WriteString(xmlText(feedURL))
	builder.WriteString("</id>\n<updated>")
	builder.WriteString(time.Unix(feed.LastFeed, 0).UTC().Format(time.RFC3339))
	builder.WriteString("</updated>\n")
	for _, message := range feed.Messages {
		itemURL := publicURL(r, fmt.Sprintf("/game/feed/viewitem.php?mid=%d&feedid=%s&type=i", message.ID, url.QueryEscape(feed.FeedID)))
		title := xmlText(domaingame.FeedText(message.Subject))
		text := domaingame.FeedCData(domaingame.FeedText(message.Text))
		builder.WriteString("<entry>\n<title>")
		builder.WriteString(title)
		builder.WriteString("</title>\n<link href=\"")
		builder.WriteString(xmlText(itemURL))
		builder.WriteString("\" />\n<id>")
		builder.WriteString(xmlText(itemURL))
		builder.WriteString("</id>\n<updated>")
		builder.WriteString(time.Unix(message.Date, 0).UTC().Format(time.RFC3339))
		builder.WriteString("</updated>\n<content type=\"html\"><![CDATA[")
		builder.WriteString(text)
		builder.WriteString("]]></content>\n</entry>\n")
	}
	builder.WriteString("</feed>\n")
	return builder.String()
}

func publicURL(r *http.Request, path string) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + path
}

func xmlText(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

package httpdelivery

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestGameFeedShowRendersRSS(t *testing.T) {
	usecase := &fakeGameFeedUseCase{feed: domaingame.Feed{
		FeedID:   "abcdef",
		Owner:    "Legor",
		LastFeed: 1000,
		Messages: []domaingame.FeedMessage{{
			ID:      11,
			Subject: `<b>Hello&nbsp;World</b>`,
			Text:    `<a href="x">Secret&nbsp;Text</a><script>alert(1)</script>`,
			Date:    900,
		}},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/feed/show.php?feedid=abcdef", nil)
	req.Host = "example.test"

	New(Dependencies{GameFeed: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "<rss version=\"2.0\">") {
		t.Fatalf("unexpected RSS response: status=%d body=%s", recorder.Code, body)
	}
	if !strings.Contains(body, "OGame-Nachrichten von Legor") || !strings.Contains(body, "Secret Textalert(1)") {
		t.Fatalf("RSS response missing feed content: %s", body)
	}
	if strings.Contains(body, "<script") || strings.Contains(body, "<a href") || strings.Contains(body, "href=\"http://127.0.0.1") {
		t.Fatalf("RSS response leaked unsafe or loopback markup: %s", body)
	}
	if usecase.feedQuery.FeedID != "abcdef" {
		t.Fatalf("unexpected feed query: %+v", usecase.feedQuery)
	}
}

func TestGameFeedShowRendersAtom(t *testing.T) {
	usecase := &fakeGameFeedUseCase{feed: domaingame.Feed{
		FeedID:   "abcdef",
		Owner:    "Legor",
		LastFeed: 1000,
		Atom:     true,
		Messages: []domaingame.FeedMessage{{ID: 11, Subject: "Subject", Text: "Body", Date: 900}},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/feed/show.php?feedid=abcdef", nil)

	New(Dependencies{GameFeed: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `<feed xmlns="http://www.w3.org/2005/Atom">`) || !strings.Contains(body, "<entry>") {
		t.Fatalf("unexpected Atom response: status=%d body=%s", recorder.Code, body)
	}
}

func TestGameFeedItemRendersEscapedHTML(t *testing.T) {
	usecase := &fakeGameFeedUseCase{item: domaingame.FeedItem{
		Subject: `<b>Subject&nbsp;One</b>`,
		Text:    `<img src=x onerror=alert(1)>Body`,
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/feed/viewitem.php?feedid=abcdef&mid=11", nil)

	New(Dependencies{GameFeed: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "<h1>Subject One</h1>") || !strings.Contains(body, "<p>Body</p>") {
		t.Fatalf("unexpected item response: status=%d body=%s", recorder.Code, body)
	}
	if strings.Contains(body, "<img") || strings.Contains(body, "<b>") {
		t.Fatalf("item response leaked raw markup: %s", body)
	}
	if usecase.itemQuery.MessageID != 11 {
		t.Fatalf("unexpected item query: %+v", usecase.itemQuery)
	}
}

func TestGameFeedInputGuards(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/game/feed/show.php", want: "No feed specified"},
		{path: "/game/feed/show.php?feedid=bad-token", want: legacyFeedValidationError},
		{path: "/game/feed/viewitem.php?feedid=bad-token&mid=11", want: legacyFeedValidationError},
		{path: "/game/feed/viewitem.php?feedid=abcdef", want: "No message specified"},
		{path: "/game/feed/viewitem.php?feedid=abcdef&mid=abc", want: "Error validating request parameters: mid"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			New(Dependencies{GameFeed: &fakeGameFeedUseCase{}}).ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK || recorder.Body.String() != tt.want {
				t.Fatalf("unexpected guard response: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestGameFeedRequiresGET(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/game/feed/show.php?feedid=abcdef", nil)
	New(Dependencies{GameFeed: &fakeGameFeedUseCase{}}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", recorder.Code)
	}
}

func TestGameFeedUnavailableAndErrorBranches(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.Handler
		path     string
		wantCode int
		wantBody string
	}{
		{
			name:     "show dependency missing",
			handler:  New(Dependencies{}),
			path:     "/game/feed/show.php?feedid=abcdef",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game feed unavailable",
		},
		{
			name:     "show usecase error",
			handler:  New(Dependencies{GameFeed: &fakeGameFeedUseCase{err: errors.New("boom")}}),
			path:     "/game/feed/show.php?feedid=abcdef",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game feed unavailable",
		},
		{
			name:     "show empty feed",
			handler:  New(Dependencies{GameFeed: &fakeGameFeedUseCase{}}),
			path:     "/game/feed/show.php?feedid=abcdef",
			wantCode: http.StatusOK,
			wantBody: "Authentifizierung fehlgeschlagen",
		},
		{
			name:     "item dependency missing",
			handler:  New(Dependencies{}),
			path:     "/game/feed/viewitem.php?feedid=abcdef&mid=11",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game feed unavailable",
		},
		{
			name:     "item usecase error",
			handler:  New(Dependencies{GameFeed: &fakeGameFeedUseCase{err: errors.New("boom")}}),
			path:     "/game/feed/viewitem.php?feedid=abcdef&mid=11",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game feed item unavailable",
		},
		{
			name:     "item empty result",
			handler:  New(Dependencies{GameFeed: &fakeGameFeedUseCase{}}),
			path:     "/game/feed/viewitem.php?feedid=abcdef&mid=11",
			wantCode: http.StatusOK,
			wantBody: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			tt.handler.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantCode || !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("unexpected response: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestGameFeedPublicURLUsesForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/game/feed/show.php?feedid=abcdef", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "game.example.test")
	if got := publicURL(req, "/game/feed/show.php?feedid=abcdef"); got != "https://game.example.test/game/feed/show.php?feedid=abcdef" {
		t.Fatalf("unexpected public URL: %s", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/game/feed/show.php?feedid=abcdef", nil)
	req.TLS = &tls.ConnectionState{}
	req.Host = "secure.example.test"
	if got := publicURL(req, "/game/feed/show.php?feedid=abcdef"); got != "https://secure.example.test/game/feed/show.php?feedid=abcdef" {
		t.Fatalf("unexpected TLS public URL: %s", got)
	}
}

type fakeGameFeedUseCase struct {
	feed      domaingame.Feed
	item      domaingame.FeedItem
	feedQuery appgame.FeedQuery
	itemQuery appgame.FeedItemQuery
	err       error
}

func (f *fakeGameFeedUseCase) GetFeed(_ context.Context, query appgame.FeedQuery) (domaingame.Feed, error) {
	f.feedQuery = query
	return f.feed, f.err
}

func (f *fakeGameFeedUseCase) GetFeedItem(_ context.Context, query appgame.FeedItemQuery) (domaingame.FeedItem, error) {
	f.itemQuery = query
	return f.item, f.err
}

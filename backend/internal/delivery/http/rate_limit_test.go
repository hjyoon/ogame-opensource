package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
)

func TestMCPRateLimitRejectsExcessRPCRequests(t *testing.T) {
	server := New(Dependencies{
		MCP: fakeMCPUseCase{},
		MCPRateLimit: RateLimitConfig{
			RequestsPerMinute: 1,
			Burst:             1,
		},
	})

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d: expected status %d, got %d body=%q", i+1, want, rec.Code, rec.Body.String())
		}
		if want == http.StatusTooManyRequests && rec.Header().Get("Retry-After") == "" {
			t.Fatalf("expected retry-after header on rate limited response")
		}
	}
}

func TestMCPRateLimitRejectsExcessOAuthRequests(t *testing.T) {
	oauth := &fakeMCPOAuthUseCase{exchangeResult: appmcp.OAuthTokenResult{AccessToken: "token", TokenType: "Bearer"}}
	server := New(Dependencies{
		MCPOAuth: oauth,
		MCPRateLimit: RateLimitConfig{
			RequestsPerMinute: 1,
			Burst:             1,
		},
	})

	body := "grant_type=authorization_code&code=abc&redirect_uri=http%3A%2F%2F127.0.0.1%3A9000%2Fcallback&resource=http%3A%2F%2Fgame.local%2Fmcp&client_id=desktop&code_verifier=" + strings.Repeat("b", 43)
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d: expected status %d, got %d body=%q", i+1, want, rec.Code, rec.Body.String())
		}
	}
}

func TestMCPRateLimitUsesForwardedClientKey(t *testing.T) {
	server := New(Dependencies{
		MCP: fakeMCPUseCase{},
		MCPRateLimit: RateLimitConfig{
			RequestsPerMinute: 1,
			Burst:             1,
		},
	})

	for _, forwardedFor := range []string{"198.51.100.1", "198.51.100.2"} {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("X-Forwarded-For", forwardedFor)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected separate forwarded clients to pass, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
}

func TestMCPRateLimiterConfigAndRefill(t *testing.T) {
	if newMCPRateLimiter(RateLimitConfig{Disabled: true}) != nil {
		t.Fatalf("expected disabled rate limiter to be nil")
	}
	limiter := newMCPRateLimiter(RateLimitConfig{RequestsPerMinute: 1, Burst: 5})
	if limiter.requestsPerMinute != 1 || limiter.burst != 1 {
		t.Fatalf("expected burst to clamp to request rate, got %+v", limiter)
	}
	limiter = newMCPRateLimiter(RateLimitConfig{})
	if limiter.requestsPerMinute != defaultMCPRateLimitPerMinute || limiter.burst != defaultMCPRateLimitBurst {
		t.Fatalf("expected default rate limit config, got %+v", limiter)
	}

	now := time.Unix(1700, 0)
	limiter = newMCPRateLimiter(RateLimitConfig{RequestsPerMinute: 60, Burst: 1})
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.allow("client"); !allowed {
		t.Fatalf("expected first request to pass")
	}
	if allowed, retryAfter := limiter.allow("client"); allowed || retryAfter <= 0 {
		t.Fatalf("expected second request to be limited, allowed=%v retryAfter=%d", allowed, retryAfter)
	}
	now = now.Add(time.Second)
	if allowed, _ := limiter.allow("client"); !allowed {
		t.Fatalf("expected request to pass after refill")
	}
}

func TestMCPRateLimiterNilWrapAndClientKeys(t *testing.T) {
	called := false
	var limiter *rateLimiter
	handler := limiter.wrap("scope", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/mcp", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("expected nil limiter wrapper to call next, called=%v status=%d", called, rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://game.local/mcp", nil)
	req.Header.Set("X-Forwarded-For", " 198.51.100.10, 198.51.100.11 ")
	if got := requestClientKey(req); got != "198.51.100.10" {
		t.Fatalf("expected forwarded client key, got %q", got)
	}
	req.Header.Del("X-Forwarded-For")
	req.RemoteAddr = "203.0.113.7:4321"
	if got := requestClientKey(req); got != "203.0.113.7" {
		t.Fatalf("expected remote host client key, got %q", got)
	}
	req.RemoteAddr = "not-a-host-port"
	if got := requestClientKey(req); got != "not-a-host-port" {
		t.Fatalf("expected raw remote addr client key, got %q", got)
	}
	req.RemoteAddr = ""
	if got := requestClientKey(req); got != "unknown" {
		t.Fatalf("expected unknown client key, got %q", got)
	}
}

package httpdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMCPOAuthAuthorizationServerMetadata(t *testing.T) {
	server := New(Dependencies{MCPOAuth: fakeMCPOAuthUseCase{}})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/oauth-authorization-server", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected metadata success, got status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body domainmcp.OAuthAuthorizationServerMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if body.Issuer != "https://game.local" || body.AuthorizationEndpoint != "https://game.local/oauth/authorize" || body.TokenEndpoint != "https://game.local/oauth/token" {
		t.Fatalf("unexpected metadata: %+v", body)
	}
	if body.ResponseTypesSupported[0] != "code" || body.CodeChallengeMethodsSupported[0] != "S256" || body.TokenEndpointAuthMethodsSupported[0] != "none" {
		t.Fatalf("unexpected OAuth 2.1 metadata: %+v", body)
	}
}

func TestMCPOAuthUnavailableEndpoints(t *testing.T) {
	server := New(Dependencies{MCPOAuth: fakeMCPOAuthUseCase{}})
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "authorize", method: http.MethodGet, path: "/oauth/authorize"},
		{name: "token", method: http.MethodPost, path: "/oauth/token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://game.local"+tt.path, nil)
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected OAuth unavailable response status=%d headers=%v body=%q", rec.Code, rec.Header(), rec.Body.String())
			}
			var body oauthUnavailableResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode OAuth error: %v", err)
			}
			if body.Error != "temporarily_unavailable" {
				t.Fatalf("unexpected OAuth error body: %+v", body)
			}
		})
	}
}

func TestMCPOAuthMetadataUnavailableAndMethodGuards(t *testing.T) {
	server := New(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected metadata unavailable, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: fakeMCPOAuthUseCase{}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/.well-known/oauth-authorization-server", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected metadata method guard, got status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
}

type fakeMCPOAuthUseCase struct{}

func (fakeMCPOAuthUseCase) OAuthAuthorizationServerMetadata(_ context.Context, issuer string) domainmcp.OAuthAuthorizationServerMetadata {
	return (appmcp.Service{}).OAuthAuthorizationServerMetadata(context.Background(), issuer)
}

package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestGameMCPTokensListCreateAndRevoke(t *testing.T) {
	manager := &recordingMCPTokenUseCase{
		listResult: appmcp.TokenListResult{
			Authenticated:   true,
			Tokens:          []domainmcp.Token{{ID: 3, Name: "Desktop", Scopes: []string{domainmcp.ScopeRead}}},
			UserType:        1,
			Role:            "operator",
			AvailableScopes: []string{domainmcp.ScopeRead, domainmcp.ScopeOperator},
		},
		createResult: appmcp.TokenCreationResult{
			Authenticated: true,
			Creation: domainmcp.TokenCreation{
				Token:  domainmcp.Token{ID: 4, Name: "Agent", Scopes: []string{domainmcp.ScopeRead}},
				Secret: "ogmcp_secret",
			},
		},
		revokeResult: appmcp.TokenRevokeResult{Authenticated: true, Revoked: true},
	}
	server := New(Dependencies{MCPTokens: manager})

	req := httptest.NewRequest(http.MethodGet, "http://game.local/api/game/mcp-tokens?session=public", nil)
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || manager.listCommand.PublicSession != "public" || manager.listCommand.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected list status=%d body=%q command=%+v", rec.Code, rec.Body.String(), manager.listCommand)
	}
	var listBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil || listBody["role"] != "operator" || listBody["userType"] != float64(1) {
		t.Fatalf("unexpected role-aware token list: body=%q parsed=%+v err=%v", rec.Body.String(), listBody, err)
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens?session=public", strings.NewReader(`{"name":"Agent","scopes":["mcp:read"]}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || manager.createCommand.Name != "Agent" || manager.createCommand.Scopes[0] != domainmcp.ScopeRead {
		t.Fatalf("unexpected create status=%d body=%q command=%+v", rec.Code, rec.Body.String(), manager.createCommand)
	}
	var createBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("invalid create response: %v", err)
	}
	if createBody["secret"] != "ogmcp_secret" {
		t.Fatalf("expected one-time secret in create response, got %+v", createBody)
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens/revoke?session=public", strings.NewReader(`{"tokenId":4}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || manager.revokeCommand.TokenID != 4 {
		t.Fatalf("unexpected revoke status=%d body=%q command=%+v", rec.Code, rec.Body.String(), manager.revokeCommand)
	}
}

func TestGameMCPTokensErrors(t *testing.T) {
	server := New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{
		listResult: appmcp.TokenListResult{
			Authenticated: false,
			Issues:        []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssueInvalid, Message: "Session is invalid."}},
		},
	}})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/api/game/mcp-tokens", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized token list, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{createErr: appmcp.ErrInvalidTokenRequest}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens", strings.NewReader(`{"name":"bad"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad token create request, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid json request, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "http://game.local/api/game/mcp-tokens", nil)
	rec = httptest.NewRecorder()
	New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{}}).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("unexpected method guard status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}

	req = httptest.NewRequest(http.MethodGet, "http://game.local/api/game/mcp-tokens", nil)
	rec = httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token manager, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens", strings.NewReader(`{"name":"x"}`))
	rec = httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token create, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens/revoke", strings.NewReader(`{"id":1}`))
	rec = httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token revoke, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{listErr: errors.New("down")}})
	req = httptest.NewRequest(http.MethodGet, "http://game.local/api/game/mcp-tokens", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token list, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{createErr: errors.New("down")}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens", strings.NewReader(`{"name":"x"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token create dependency, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens/revoke", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad revoke json, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPTokens: &recordingMCPTokenUseCase{revokeErr: errors.New("down")}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/api/game/mcp-tokens/revoke", strings.NewReader(`{"id":1}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable revoke, got status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type recordingMCPTokenUseCase struct {
	listResult   appmcp.TokenListResult
	createResult appmcp.TokenCreationResult
	revokeResult appmcp.TokenRevokeResult
	listErr      error
	createErr    error
	revokeErr    error

	listCommand   appmcp.TokenManagementCommand
	createCommand appmcp.CreateTokenCommand
	revokeCommand appmcp.RevokeTokenCommand
}

func (r *recordingMCPTokenUseCase) ListTokens(ctx context.Context, command appmcp.TokenManagementCommand) (appmcp.TokenListResult, error) {
	_ = ctx
	r.listCommand = command
	if r.listErr != nil {
		return appmcp.TokenListResult{}, r.listErr
	}
	return r.listResult, nil
}

func (r *recordingMCPTokenUseCase) CreateToken(ctx context.Context, command appmcp.CreateTokenCommand) (appmcp.TokenCreationResult, error) {
	_ = ctx
	r.createCommand = command
	if r.createErr != nil {
		return appmcp.TokenCreationResult{}, r.createErr
	}
	return r.createResult, nil
}

func (r *recordingMCPTokenUseCase) RevokeToken(ctx context.Context, command appmcp.RevokeTokenCommand) (appmcp.TokenRevokeResult, error) {
	_ = ctx
	r.revokeCommand = command
	if r.revokeErr != nil {
		return appmcp.TokenRevokeResult{}, r.revokeErr
	}
	return r.revokeResult, nil
}

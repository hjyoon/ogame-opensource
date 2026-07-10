package httpdelivery

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"strings"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
)

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (a app) handleMCPOAuthAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		http.Error(w, "mcp oauth unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(a.deps.MCPOAuth.OAuthAuthorizationServerMetadata(r.Context(), requestIssuer(r)))
}

func (a app) handleMCPOAuthProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		http.Error(w, "mcp protected resource metadata unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	issuer := requestIssuer(r)
	_ = json.NewEncoder(w).Encode(a.deps.MCPOAuth.OAuthProtectedResourceMetadata(r.Context(), issuer+"/mcp", issuer))
}

func (a app) handleMCPOAuthJWKS(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		http.Error(w, "mcp oidc unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(a.deps.MCPOAuth.OAuthJWKS(r.Context()))
}

func (a app) handleMCPOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "OAuth consent flow is unavailable.")
		return
	}
	result, err := a.deps.MCPOAuth.AuthorizeOAuth(r.Context(), oauthAuthorizeCommand(r))
	if err != nil {
		writeOAuthApplicationError(w, err)
		return
	}
	if !result.Authenticated {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("<!doctype html><title>Login required</title><p>Login is required before authorizing MCP access.</p>"))
		return
	}
	if result.RequiresConsent {
		writeOAuthConsent(w, r, result)
		return
	}
	http.Redirect(w, r, result.RedirectTo, http.StatusFound)
}

func (a app) handleMCPOAuthToken(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "OAuth token flow is unavailable.")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "OAuth token request is invalid.")
		return
	}
	result, err := a.deps.MCPOAuth.ExchangeOAuthCode(r.Context(), appmcp.OAuthTokenCommand{
		GrantType:    r.Form.Get("grant_type"),
		Code:         r.Form.Get("code"),
		RedirectURI:  r.Form.Get("redirect_uri"),
		Resource:     r.Form.Get("resource"),
		ClientID:     r.Form.Get("client_id"),
		CodeVerifier: r.Form.Get("code_verifier"),
		Issuer:       requestIssuer(r),
	})
	if err != nil {
		writeOAuthApplicationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	body := map[string]any{
		"access_token": result.AccessToken,
		"token_type":   result.TokenType,
		"scope":        result.Scope,
	}
	if result.IDToken != "" {
		body["id_token"] = result.IDToken
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (a app) handleMCPOAuthRevoke(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "OAuth token revocation is unavailable.")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "OAuth revocation request is invalid.")
		return
	}
	result, err := a.deps.MCPOAuth.RevokeOAuthToken(r.Context(), appmcp.OAuthRevokeCommand{
		Token:         r.Form.Get("token"),
		TokenTypeHint: r.Form.Get("token_type_hint"),
	})
	if err != nil {
		writeOAuthApplicationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"revoked": result.Revoked})
}

func requestIssuer(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func oauthAuthorizeCommand(r *http.Request) appmcp.OAuthAuthorizeCommand {
	query := r.URL.Query()
	return appmcp.OAuthAuthorizeCommand{
		TokenManagementCommand: mcpTokenCommand(r),
		ResponseType:           query.Get("response_type"),
		ClientID:               query.Get("client_id"),
		RedirectURI:            query.Get("redirect_uri"),
		Resource:               query.Get("resource"),
		Scope:                  query.Get("scope"),
		State:                  query.Get("state"),
		CodeChallenge:          query.Get("code_challenge"),
		CodeChallengeMethod:    query.Get("code_challenge_method"),
		ConsentApproved:        query.Get("consent") == "approve",
		Issuer:                 requestIssuer(r),
	}
}

func writeOAuthConsent(w http.ResponseWriter, r *http.Request, result appmcp.OAuthAuthorizeResult) {
	query := r.URL.Query()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><html><head><title>Authorize MCP access</title></head><body>"))
	_, _ = w.Write([]byte("<h1>Authorize MCP access</h1>"))
	_, _ = w.Write([]byte("<p>Client <strong>" + html.EscapeString(result.ClientID) + "</strong> requests access to this game account.</p>"))
	_, _ = w.Write([]byte("<p>Scopes: " + html.EscapeString(strings.Join(result.Scopes, " ")) + "</p>"))
	_, _ = w.Write([]byte("<form method=\"get\" action=\"/oauth/authorize\">"))
	for _, key := range []string{"response_type", "client_id", "redirect_uri", "resource", "scope", "state", "code_challenge", "code_challenge_method", "session"} {
		if value := query.Get(key); value != "" {
			_, _ = w.Write([]byte("<input type=\"hidden\" name=\"" + html.EscapeString(key) + "\" value=\"" + html.EscapeString(value) + "\">"))
		}
	}
	_, _ = w.Write([]byte("<input type=\"hidden\" name=\"consent\" value=\"approve\">"))
	_, _ = w.Write([]byte("<button type=\"submit\">Authorize</button></form>"))
	_, _ = w.Write([]byte("</body></html>"))
}

func writeOAuthApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appmcp.ErrInvalidOAuthRequest):
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, appmcp.ErrInvalidOAuthGrant):
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "OAuth authorization code is invalid.")
	default:
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "OAuth flow is unavailable.")
	}
}

func writeOAuthError(w http.ResponseWriter, status int, errorCode string, description string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(oauthErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	})
}

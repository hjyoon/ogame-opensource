package httpdelivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strings"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
)

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type oauthClientRegistrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
}

const (
	defaultMCPOAuthConsentSecret = "ogame-mcp-development-consent-secret"
	oauthConsentTokenParam       = "consent_token"
)

var oauthConsentSignedFields = []string{"response_type", "client_id", "redirect_uri", "resource", "scope", "state", "code_challenge", "code_challenge_method", "session"}

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

func (a app) handleMCPOAuthRegister(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPOAuth == nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "OAuth client registration is unavailable.")
		return
	}
	var body oauthClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "OAuth client registration request is invalid.")
		return
	}
	result, err := a.deps.MCPOAuth.RegisterOAuthClient(r.Context(), appmcp.OAuthClientRegistrationCommand{
		RedirectURIs:            body.RedirectURIs,
		ClientName:              body.ClientName,
		GrantTypes:              body.GrantTypes,
		ResponseTypes:           body.ResponseTypes,
		TokenEndpointAuthMethod: body.TokenEndpointAuthMethod,
		Scope:                   body.Scope,
	})
	if err != nil {
		writeOAuthApplicationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"client_id":                  result.ClientID,
		"client_id_issued_at":        result.ClientIDIssuedAt,
		"client_name":                result.ClientName,
		"redirect_uris":              result.RedirectURIs,
		"grant_types":                result.GrantTypes,
		"response_types":             result.ResponseTypes,
		"token_endpoint_auth_method": result.TokenEndpointAuthMethod,
		"scope":                      result.Scope,
	})
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
	command, err := a.oauthAuthorizeCommand(r)
	if err != nil {
		writeOAuthApplicationError(w, err)
		return
	}
	result, err := a.deps.MCPOAuth.AuthorizeOAuth(r.Context(), command)
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
		a.writeOAuthConsent(w, r, result)
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
	if result.ExpiresIn > 0 {
		body["expires_in"] = result.ExpiresIn
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

func (a app) oauthAuthorizeCommand(r *http.Request) (appmcp.OAuthAuthorizeCommand, error) {
	query := r.URL.Query()
	consentApproved := query.Get("consent") == "approve"
	if consentApproved && !validOAuthConsentToken(a.currentOAuthConsentSecret(), query) {
		return appmcp.OAuthAuthorizeCommand{}, appmcp.ErrInvalidOAuthRequest
	}
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
		ConsentApproved:        consentApproved,
		Issuer:                 requestIssuer(r),
	}, nil
}

func (a app) writeOAuthConsent(w http.ResponseWriter, r *http.Request, result appmcp.OAuthAuthorizeResult) {
	query := r.URL.Query()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><html><head><title>Authorize MCP access</title></head><body>"))
	_, _ = w.Write([]byte("<h1>Authorize MCP access</h1>"))
	_, _ = w.Write([]byte("<p>Client <strong>" + html.EscapeString(result.ClientID) + "</strong> requests access to this game account.</p>"))
	_, _ = w.Write([]byte("<dl>"))
	_, _ = w.Write([]byte("<dt>Resource</dt><dd>" + html.EscapeString(query.Get("resource")) + "</dd>"))
	_, _ = w.Write([]byte("<dt>Redirect</dt><dd>" + html.EscapeString(result.RedirectURI) + "</dd>"))
	_, _ = w.Write([]byte("</dl>"))
	_, _ = w.Write([]byte("<h2>Requested permissions</h2><ul>"))
	for _, scope := range result.Scopes {
		_, _ = w.Write([]byte("<li><strong>" + html.EscapeString(scope) + "</strong>: " + html.EscapeString(oauthScopeDescription(scope)) + "</li>"))
	}
	_, _ = w.Write([]byte("</ul>"))
	_, _ = w.Write([]byte("<form method=\"get\" action=\"/oauth/authorize\">"))
	for _, key := range []string{"response_type", "client_id", "redirect_uri", "resource", "scope", "state", "code_challenge", "code_challenge_method", "session"} {
		if value := query.Get(key); value != "" {
			_, _ = w.Write([]byte("<input type=\"hidden\" name=\"" + html.EscapeString(key) + "\" value=\"" + html.EscapeString(value) + "\">"))
		}
	}
	_, _ = w.Write([]byte("<input type=\"hidden\" name=\"" + oauthConsentTokenParam + "\" value=\"" + oauthConsentToken(a.currentOAuthConsentSecret(), query) + "\">"))
	_, _ = w.Write([]byte("<input type=\"hidden\" name=\"consent\" value=\"approve\">"))
	_, _ = w.Write([]byte("<button type=\"submit\">Authorize</button></form>"))
	if denyTo := oauthAccessDeniedRedirect(result.RedirectURI, query.Get("state")); denyTo != "" {
		_, _ = w.Write([]byte("<p><a href=\"" + html.EscapeString(denyTo) + "\">Deny</a></p>"))
	}
	_, _ = w.Write([]byte("</body></html>"))
}

func (a app) currentOAuthConsentSecret() string {
	if strings.TrimSpace(a.deps.MCPOAuthConsentSecret) != "" {
		return a.deps.MCPOAuthConsentSecret
	}
	return defaultMCPOAuthConsentSecret
}

func oauthConsentToken(secret string, query url.Values) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(oauthConsentPayload(query)))
	return hex.EncodeToString(mac.Sum(nil))
}

func validOAuthConsentToken(secret string, query url.Values) bool {
	got := strings.TrimSpace(query.Get(oauthConsentTokenParam))
	if got == "" {
		return false
	}
	want := oauthConsentToken(secret, query)
	return hmac.Equal([]byte(got), []byte(want))
}

func oauthConsentPayload(query url.Values) string {
	values := url.Values{}
	for _, key := range oauthConsentSignedFields {
		if value := query.Get(key); value != "" {
			values.Set(key, value)
		}
	}
	return values.Encode()
}

func oauthScopeDescription(scope string) string {
	switch scope {
	case "openid":
		return "issue an ID token for this account"
	case "profile":
		return "identify the current player profile"
	case "mcp:read":
		return "read account, planet, resource, building queue, and fleet movement data"
	case "mcp:messages":
		return "read message-related MCP data when message tools are available"
	case "mcp:message_write":
		return "send in-game private messages only after dry-run and explicit confirmation"
	case "mcp:fleet":
		return "read fleet-related MCP data when fleet tools are available"
	default:
		return "access requested MCP capability"
	}
}

func oauthAccessDeniedRedirect(rawRedirect string, state string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawRedirect))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	query := parsed.Query()
	query.Set("error", "access_denied")
	query.Set("error_description", "MCP access denied by user")
	if strings.TrimSpace(state) != "" {
		query.Set("state", strings.TrimSpace(state))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
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

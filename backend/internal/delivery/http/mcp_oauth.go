package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strings"
)

type oauthUnavailableResponse struct {
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

func (a app) handleMCPOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	writeOAuthUnavailable(w)
}

func (a app) handleMCPOAuthToken(w http.ResponseWriter, r *http.Request) {
	writeOAuthUnavailable(w)
}

func requestIssuer(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func writeOAuthUnavailable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(oauthUnavailableResponse{
		Error:            "temporarily_unavailable",
		ErrorDescription: "OAuth consent flow is not enabled yet.",
	})
}

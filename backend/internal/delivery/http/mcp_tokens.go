package httpdelivery

import (
	"encoding/json"
	"errors"
	"net/http"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type mcpTokenCreateRequest struct {
	Name             string   `json:"name"`
	Scopes           []string `json:"scopes"`
	ExpiresInSeconds *int64   `json:"expiresInSeconds"`
}

type mcpTokenRevokeRequest struct {
	ID      int `json:"id"`
	TokenID int `json:"tokenId"`
}

func (a app) handleGameMCPTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleGameMCPTokenList(w, r)
	case http.MethodPost:
		a.handleGameMCPTokenCreate(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGameMCPTokenList(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPTokens == nil {
		http.Error(w, "mcp token management unavailable", http.StatusServiceUnavailable)
		return
	}
	result, err := a.deps.MCPTokens.ListTokens(r.Context(), mcpTokenCommand(r))
	if err != nil {
		http.Error(w, "mcp token management unavailable", http.StatusServiceUnavailable)
		return
	}
	writeMCPTokenResult(w, result.Authenticated, result.Issues, map[string]any{
		"tokens":          result.Tokens,
		"userType":        result.UserType,
		"role":            result.Role,
		"availableScopes": result.AvailableScopes,
		"maxActiveTokens": result.MaxActiveTokens,
		"expiryOptions":   result.ExpiryOptions,
	})
}

func (a app) handleGameMCPTokenCreate(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPTokens == nil {
		http.Error(w, "mcp token management unavailable", http.StatusServiceUnavailable)
		return
	}
	var request mcpTokenCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid mcp token request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.MCPTokens.CreateToken(r.Context(), appmcp.CreateTokenCommand{
		TokenManagementCommand: mcpTokenCommand(r),
		Name:                   request.Name,
		Scopes:                 request.Scopes,
		ExpiresInSeconds:       request.ExpiresInSeconds,
	})
	if err != nil {
		writeMCPTokenError(w, err)
		return
	}
	writeMCPTokenResult(w, result.Authenticated, result.Issues, map[string]any{
		"token":  result.Creation.Token,
		"secret": result.Creation.Secret,
	})
}

func (a app) handleGameMCPTokenRevoke(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCPTokens == nil {
		http.Error(w, "mcp token management unavailable", http.StatusServiceUnavailable)
		return
	}
	var request mcpTokenRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid mcp token revoke request", http.StatusBadRequest)
		return
	}
	tokenID := request.TokenID
	if tokenID == 0 {
		tokenID = request.ID
	}
	result, err := a.deps.MCPTokens.RevokeToken(r.Context(), appmcp.RevokeTokenCommand{
		TokenManagementCommand: mcpTokenCommand(r),
		TokenID:                tokenID,
	})
	if err != nil {
		writeMCPTokenError(w, err)
		return
	}
	writeMCPTokenResult(w, result.Authenticated, result.Issues, map[string]any{"revoked": result.Revoked})
}

func mcpTokenCommand(r *http.Request) appmcp.TokenManagementCommand {
	return appmcp.TokenManagementCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
	}
}

func writeMCPTokenError(w http.ResponseWriter, err error) {
	if errors.Is(err, appmcp.ErrInvalidTokenRequest) {
		http.Error(w, "invalid mcp token request", http.StatusBadRequest)
		return
	}
	if errors.Is(err, appmcp.ErrTokenLimitReached) {
		http.Error(w, "maximum of 5 active mcp tokens reached", http.StatusConflict)
		return
	}
	http.Error(w, "mcp token management unavailable", http.StatusServiceUnavailable)
}

func writeMCPTokenResult(w http.ResponseWriter, authenticated bool, issues []domainpublicsite.SessionIssue, body map[string]any) {
	status := http.StatusOK
	if !authenticated {
		status = http.StatusUnauthorized
	}
	body["authenticated"] = authenticated
	body["issues"] = toGameSessionIssueResponses(issues)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

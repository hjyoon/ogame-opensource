package httpdelivery

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type loginValidationRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Pass     string `json:"pass"`
	Universe string `json:"universe"`
}

type loginValidationResponse struct {
	Valid  bool                 `json:"valid"`
	Issues []loginIssueResponse `json:"issues"`
	Draft  loginDraftResponse   `json:"draft"`
}

type loginResponse struct {
	Valid   bool                  `json:"valid"`
	Issues  []loginIssueResponse  `json:"issues"`
	Draft   loginDraftResponse    `json:"draft"`
	Session *loginSessionResponse `json:"session,omitempty"`
}

type loginIssueResponse struct {
	Field           string `json:"field"`
	Code            string `json:"code"`
	Message         string `json:"message"`
	LegacyErrorCode int    `json:"legacyErrorCode"`
}

type loginDraftResponse struct {
	Login    string `json:"login"`
	Universe string `json:"universe"`
}

type loginSessionResponse struct {
	RedirectTo     string `json:"redirectTo"`
	UniverseNumber int    `json:"universeNumber"`
}

func (a app) handleLoginValidation(w http.ResponseWriter, r *http.Request) {
	var request loginValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid login validation request", http.StatusBadRequest)
		return
	}

	password := request.Password
	if password == "" {
		password = request.Pass
	}
	result, err := a.deps.LoginDrafts.ValidateLoginDraft(r.Context(), apppublicsite.LoginDraftCommand{
		Login:    request.Login,
		Password: password,
		Universe: request.Universe,
	})
	if err != nil {
		http.Error(w, "login validation unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(loginValidationResponse{
		Valid:  result.Valid,
		Issues: toLoginIssueResponses(result.Issues),
		Draft: loginDraftResponse{
			Login:    request.Login,
			Universe: request.Universe,
		},
	})
}

func (a app) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.deps.Login == nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}

	var request loginValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid login request", http.StatusBadRequest)
		return
	}

	password := request.Password
	if password == "" {
		password = request.Pass
	}
	result, err := a.deps.Login.AuthenticateLogin(r.Context(), apppublicsite.LoginCommand{
		Login:      request.Login,
		Password:   password,
		Universe:   request.Universe,
		RemoteAddr: remoteIP(r.RemoteAddr),
	})
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}

	var session *loginSessionResponse
	if result.Valid {
		setLoginSessionCookie(w, result.Session)
		session = &loginSessionResponse{
			RedirectTo:     result.Session.RedirectTarget(),
			UniverseNumber: result.Session.UniverseNumber,
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(loginResponse{
		Valid:  result.Valid,
		Issues: toLoginIssueResponses(result.Issues),
		Draft: loginDraftResponse{
			Login:    request.Login,
			Universe: request.Universe,
		},
		Session: session,
	})
}

func setLoginSessionCookie(w http.ResponseWriter, session domain.LoginSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     session.PrivateCookieName(),
		Value:    session.PrivateID,
		Path:     "/",
		Expires:  time.Unix(session.LastLogin, 0).Add(24 * time.Hour),
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearLoginSessionCookie(w http.ResponseWriter, name string) {
	if name == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func toLoginIssueResponses(issues []domain.LoginIssue) []loginIssueResponse {
	responses := make([]loginIssueResponse, 0, len(issues))
	for _, issue := range issues {
		responses = append(responses, loginIssueResponse{
			Field:           issue.Field,
			Code:            issue.Code,
			Message:         issue.Message,
			LegacyErrorCode: issue.LegacyErrorCode,
		})
	}
	return responses
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}

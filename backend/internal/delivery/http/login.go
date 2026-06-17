package httpdelivery

import (
	"encoding/json"
	"net/http"

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

package httpdelivery

import (
	"encoding/json"
	"net/http"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type registrationValidationRequest struct {
	Character     string `json:"character"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	Universe      string `json:"universe"`
	AGB           bool   `json:"agb"`
	TermsAccepted bool   `json:"termsAccepted"`
}

type registrationValidationResponse struct {
	Valid  bool                        `json:"valid"`
	Issues []registrationIssueResponse `json:"issues"`
	Draft  registrationDraftResponse   `json:"draft"`
}

type registrationResponse struct {
	Valid   bool                         `json:"valid"`
	Created bool                         `json:"created"`
	Issues  []registrationIssueResponse  `json:"issues"`
	Draft   registrationDraftResponse    `json:"draft"`
	Account *registrationAccountResponse `json:"account,omitempty"`
	Session *loginSessionResponse        `json:"session,omitempty"`
}

type registrationIssueResponse struct {
	Field           string `json:"field"`
	Code            string `json:"code"`
	Message         string `json:"message"`
	LegacyErrorCode int    `json:"legacyErrorCode"`
}

type registrationDraftResponse struct {
	Character string `json:"character"`
	Email     string `json:"email"`
	Universe  string `json:"universe"`
	AGB       bool   `json:"agb"`
}

type registrationAccountResponse struct {
	PlayerID           int  `json:"playerId"`
	HomePlanetID       int  `json:"homePlanetId"`
	ActivationRequired bool `json:"activationRequired"`
}

func (a app) handleRegistrationValidation(w http.ResponseWriter, r *http.Request) {
	var request registrationValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid registration validation request", http.StatusBadRequest)
		return
	}

	termsAccepted := request.AGB || request.TermsAccepted
	result, err := a.deps.RegistrationDrafts.ValidateRegistrationDraft(r.Context(), apppublicsite.RegistrationDraftCommand{
		Character:     request.Character,
		Password:      request.Password,
		Email:         request.Email,
		Universe:      request.Universe,
		TermsAccepted: termsAccepted,
	})
	if err != nil {
		http.Error(w, "registration validation unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(registrationValidationResponse{
		Valid:  result.Valid,
		Issues: toRegistrationIssueResponses(result.Issues),
		Draft: registrationDraftResponse{
			Character: request.Character,
			Email:     request.Email,
			Universe:  request.Universe,
			AGB:       termsAccepted,
		},
	})
}

func (a app) handleRegistration(w http.ResponseWriter, r *http.Request) {
	if a.deps.Registration == nil {
		http.Error(w, "registration unavailable", http.StatusServiceUnavailable)
		return
	}

	var request registrationValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid registration request", http.StatusBadRequest)
		return
	}

	termsAccepted := request.AGB || request.TermsAccepted
	result, err := a.deps.Registration.RegisterAccount(r.Context(), apppublicsite.RegistrationCommand{
		Character:     request.Character,
		Password:      request.Password,
		Email:         request.Email,
		Universe:      request.Universe,
		TermsAccepted: termsAccepted,
		RemoteAddr:    remoteIP(r.RemoteAddr),
	})
	if err != nil {
		http.Error(w, "registration unavailable", http.StatusServiceUnavailable)
		return
	}

	var account *registrationAccountResponse
	var session *loginSessionResponse
	if result.Valid {
		setLoginSessionCookie(w, result.Session)
		account = &registrationAccountResponse{
			PlayerID:           result.Account.PlayerID,
			HomePlanetID:       result.Account.HomePlanetID,
			ActivationRequired: !result.Account.Validated,
		}
		session = &loginSessionResponse{
			RedirectTo:     result.Session.RedirectTarget(),
			UniverseNumber: result.Session.UniverseNumber,
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(registrationResponse{
		Valid:   result.Valid,
		Created: result.Valid,
		Issues:  toRegistrationIssueResponses(result.Issues),
		Draft: registrationDraftResponse{
			Character: request.Character,
			Email:     request.Email,
			Universe:  request.Universe,
			AGB:       termsAccepted,
		},
		Account: account,
		Session: session,
	})
}

func toRegistrationIssueResponses(issues []domain.RegistrationIssue) []registrationIssueResponse {
	responses := make([]registrationIssueResponse, 0, len(issues))
	for _, issue := range issues {
		responses = append(responses, registrationIssueResponse{
			Field:           issue.Field,
			Code:            issue.Code,
			Message:         issue.Message,
			LegacyErrorCode: issue.LegacyErrorCode,
		})
	}
	return responses
}

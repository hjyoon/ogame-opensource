package publicsite

import (
	"context"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestRegistrationDraftValidatorReturnsDomainValidation(t *testing.T) {
	validator := NewRegistrationDraftValidator()

	result, err := validator.ValidateRegistrationDraft(context.Background(), RegistrationDraftCommand{
		Character:     "ab",
		Password:      "short",
		Email:         "invalid",
		Universe:      "",
		TermsAccepted: false,
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected invalid draft, got %+v", result)
	}
	if !hasIssue(result, domain.RegistrationIssueCharacterInvalid) || !hasIssue(result, domain.RegistrationIssuePasswordTooShort) {
		t.Fatalf("expected domain validation issues, got %+v", result)
	}
}

func hasIssue(result domain.RegistrationValidation, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

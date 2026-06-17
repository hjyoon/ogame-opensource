package publicsite

import (
	"context"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestLoginDraftValidatorReturnsDomainValidation(t *testing.T) {
	validator := NewLoginDraftValidator()

	result, err := validator.ValidateLoginDraft(context.Background(), LoginDraftCommand{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected invalid draft, got %+v", result)
	}
	if !hasLoginIssue(result, domain.LoginIssueLoginRequired) || !hasLoginIssue(result, domain.LoginIssuePasswordRequired) {
		t.Fatalf("expected domain validation issues, got %+v", result)
	}
}

func hasLoginIssue(result domain.LoginValidation, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

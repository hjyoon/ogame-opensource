package publicsite

import (
	"context"
	"errors"
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

func TestLoginDraftValidatorAddsCredentialIssues(t *testing.T) {
	validator := NewLoginDraftValidatorWithCredentials(fakeCredentialChecker{
		credentials: domain.LoginCredentials{Authenticated: false},
	})

	result, err := validator.ValidateLoginDraft(context.Background(), LoginDraftCommand{
		Login:    "Commander01",
		Password: "wrong",
		Universe: "http://localhost:8888",
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || !hasLoginIssue(result, domain.LoginIssueCredentialsInvalid) {
		t.Fatalf("expected credential validation issue, got %+v", result)
	}
}

func TestLoginDraftValidatorAcceptsAuthenticatedCredentials(t *testing.T) {
	validator := NewLoginDraftValidatorWithCredentials(fakeCredentialChecker{
		credentials: domain.LoginCredentials{Authenticated: true, PlayerID: 1},
	})

	result, err := validator.ValidateLoginDraft(context.Background(), LoginDraftCommand{
		Login:    "Commander01",
		Password: "E2E_http123",
		Universe: "http://localhost:8888",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatalf("expected authenticated login draft, got %+v", result)
	}
}

func TestLoginDraftValidatorSkipsCredentialsWhenLocalDraftIsInvalid(t *testing.T) {
	checker := &recordingCredentialChecker{}
	validator := NewLoginDraftValidatorWithCredentials(checker)

	result, err := validator.ValidateLoginDraft(context.Background(), LoginDraftCommand{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected invalid local draft, got %+v", result)
	}
	if checker.called {
		t.Fatal("expected credential checker to be skipped for locally invalid draft")
	}
}

func TestLoginDraftValidatorReturnsCredentialError(t *testing.T) {
	wantErr := errors.New("credential check failed")
	validator := NewLoginDraftValidatorWithCredentials(fakeCredentialChecker{err: wantErr})

	_, err := validator.ValidateLoginDraft(context.Background(), LoginDraftCommand{
		Login:    "Commander01",
		Password: "E2E_http123",
		Universe: "http://localhost:8888",
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected credential error, got %v", err)
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

type fakeCredentialChecker struct {
	credentials domain.LoginCredentials
	err         error
}

func (f fakeCredentialChecker) CheckLoginCredentials(context.Context, domain.LoginDraft) (domain.LoginCredentials, error) {
	return f.credentials, f.err
}

type recordingCredentialChecker struct {
	called bool
}

func (r *recordingCredentialChecker) CheckLoginCredentials(context.Context, domain.LoginDraft) (domain.LoginCredentials, error) {
	r.called = true
	return domain.LoginCredentials{}, nil
}

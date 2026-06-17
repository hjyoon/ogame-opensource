package publicsite

import (
	"context"
	"errors"
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

func TestRegistrationDraftValidatorAddsAvailabilityIssues(t *testing.T) {
	validator := NewRegistrationDraftValidatorWithAvailability(fakeAvailabilityChecker{
		availability: domain.RegistrationAvailability{
			CharacterExists: true,
			EmailExists:     true,
			UserCount:       5,
			MaxUsers:        5,
		},
	})

	result, err := validator.ValidateRegistrationDraft(context.Background(), RegistrationDraftCommand{
		Character:     "Commander01",
		Password:      "E2E_http123",
		Email:         "commander@example.local",
		Universe:      "http://localhost:8888",
		TermsAccepted: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected availability issues, got %+v", result)
	}
	if !hasIssue(result, domain.RegistrationIssueCharacterExists) || !hasIssue(result, domain.RegistrationIssueUniverseFull) {
		t.Fatalf("expected availability validation issues, got %+v", result)
	}
}

func TestRegistrationDraftValidatorSkipsAvailabilityWhenLocalDraftIsInvalid(t *testing.T) {
	checker := &recordingAvailabilityChecker{}
	validator := NewRegistrationDraftValidatorWithAvailability(checker)

	result, err := validator.ValidateRegistrationDraft(context.Background(), RegistrationDraftCommand{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected invalid local draft, got %+v", result)
	}
	if checker.called {
		t.Fatal("expected availability checker to be skipped for locally invalid draft")
	}
}

func TestRegistrationDraftValidatorReturnsAvailabilityError(t *testing.T) {
	wantErr := errors.New("availability failed")
	validator := NewRegistrationDraftValidatorWithAvailability(fakeAvailabilityChecker{err: wantErr})

	_, err := validator.ValidateRegistrationDraft(context.Background(), RegistrationDraftCommand{
		Character:     "Commander01",
		Password:      "E2E_http123",
		Email:         "commander@example.local",
		Universe:      "http://localhost:8888",
		TermsAccepted: true,
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected availability error, got %v", err)
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

type fakeAvailabilityChecker struct {
	availability domain.RegistrationAvailability
	err          error
}

func (f fakeAvailabilityChecker) CheckRegistrationAvailability(context.Context, domain.RegistrationDraft) (domain.RegistrationAvailability, error) {
	return f.availability, f.err
}

type recordingAvailabilityChecker struct {
	called bool
}

func (r *recordingAvailabilityChecker) CheckRegistrationAvailability(context.Context, domain.RegistrationDraft) (domain.RegistrationAvailability, error) {
	r.called = true
	return domain.RegistrationAvailability{}, nil
}

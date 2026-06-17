package publicsite

import (
	"context"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type RegistrationDraftCommand struct {
	Character     string
	Password      string
	Email         string
	Universe      string
	TermsAccepted bool
}

type RegistrationAvailabilityChecker interface {
	CheckRegistrationAvailability(context.Context, domain.RegistrationDraft) (domain.RegistrationAvailability, error)
}

type RegistrationDraftValidator struct {
	availability RegistrationAvailabilityChecker
}

func NewRegistrationDraftValidator() RegistrationDraftValidator {
	return RegistrationDraftValidator{}
}

func NewRegistrationDraftValidatorWithAvailability(availability RegistrationAvailabilityChecker) RegistrationDraftValidator {
	return RegistrationDraftValidator{availability: availability}
}

func (v RegistrationDraftValidator) ValidateRegistrationDraft(ctx context.Context, command RegistrationDraftCommand) (domain.RegistrationValidation, error) {
	draft := domain.RegistrationDraft{
		Character:     command.Character,
		Password:      command.Password,
		Email:         command.Email,
		Universe:      command.Universe,
		TermsAccepted: command.TermsAccepted,
	}
	result := draft.Validate()
	if !result.Valid || v.availability == nil {
		return result, nil
	}

	availability, err := v.availability.CheckRegistrationAvailability(ctx, draft)
	if err != nil {
		return domain.RegistrationValidation{}, err
	}
	result.Issues = append(result.Issues, availability.Validate()...)
	result.Valid = len(result.Issues) == 0
	return result, nil
}

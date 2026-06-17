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

type RegistrationDraftValidator struct{}

func NewRegistrationDraftValidator() RegistrationDraftValidator {
	return RegistrationDraftValidator{}
}

func (v RegistrationDraftValidator) ValidateRegistrationDraft(ctx context.Context, command RegistrationDraftCommand) (domain.RegistrationValidation, error) {
	_ = ctx
	return domain.RegistrationDraft{
		Character:     command.Character,
		Password:      command.Password,
		Email:         command.Email,
		Universe:      command.Universe,
		TermsAccepted: command.TermsAccepted,
	}.Validate(), nil
}

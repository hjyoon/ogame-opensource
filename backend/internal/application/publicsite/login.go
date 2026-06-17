package publicsite

import (
	"context"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type LoginDraftCommand struct {
	Login    string
	Password string
	Universe string
}

type LoginCredentialChecker interface {
	CheckLoginCredentials(context.Context, domain.LoginDraft) (domain.LoginCredentials, error)
}

type LoginDraftValidator struct {
	credentials LoginCredentialChecker
}

func NewLoginDraftValidator() LoginDraftValidator {
	return LoginDraftValidator{}
}

func NewLoginDraftValidatorWithCredentials(credentials LoginCredentialChecker) LoginDraftValidator {
	return LoginDraftValidator{credentials: credentials}
}

func (v LoginDraftValidator) ValidateLoginDraft(ctx context.Context, command LoginDraftCommand) (domain.LoginValidation, error) {
	draft := domain.LoginDraft{
		Login:    command.Login,
		Password: command.Password,
		Universe: command.Universe,
	}
	result := draft.Validate()
	if !result.Valid || v.credentials == nil {
		return result, nil
	}

	credentials, err := v.credentials.CheckLoginCredentials(ctx, draft)
	if err != nil {
		return domain.LoginValidation{}, err
	}
	result.Issues = append(result.Issues, credentials.Validate()...)
	result.Valid = len(result.Issues) == 0
	return result, nil
}

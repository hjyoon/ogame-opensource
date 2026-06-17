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

type LoginDraftValidator struct{}

func NewLoginDraftValidator() LoginDraftValidator {
	return LoginDraftValidator{}
}

func (v LoginDraftValidator) ValidateLoginDraft(ctx context.Context, command LoginDraftCommand) (domain.LoginValidation, error) {
	_ = ctx
	return domain.LoginDraft{
		Login:    command.Login,
		Password: command.Password,
		Universe: command.Universe,
	}.Validate(), nil
}

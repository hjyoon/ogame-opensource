package publicsite

import (
	"context"
	"errors"
	"strings"
	"time"

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

type RegistrationAccountCreator interface {
	CreateRegistrationAccount(context.Context, domain.RegistrationDraft, string) (domain.RegisteredAccount, error)
}

type RegistrationAccountActivator interface {
	ActivateRegistrationAccount(context.Context, string) (domain.ActivatedAccount, error)
}

type RegistrationDraftValidator struct {
	availability RegistrationAvailabilityChecker
}

type RegistrationCommand struct {
	Character     string
	Password      string
	Email         string
	Universe      string
	TermsAccepted bool
	RemoteAddr    string
}

type RegistrationRegistrar struct {
	availability   RegistrationAvailabilityChecker
	accounts       RegistrationAccountCreator
	sessions       LoginSessionWriter
	tokens         LoginTokenGenerator
	now            func() time.Time
	universeNumber int
}

type RegistrationActivationCommand struct {
	ActivationCode string
	RemoteAddr     string
}

type RegistrationActivationService struct {
	accounts       RegistrationAccountActivator
	sessions       LoginSessionWriter
	tokens         LoginTokenGenerator
	now            func() time.Time
	universeNumber int
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

func NewRegistrationRegistrar(
	availability RegistrationAvailabilityChecker,
	accounts RegistrationAccountCreator,
	sessions LoginSessionWriter,
	tokens LoginTokenGenerator,
	universeNumber int,
) RegistrationRegistrar {
	return NewRegistrationRegistrarWithClock(availability, accounts, sessions, tokens, universeNumber, time.Now)
}

func NewRegistrationRegistrarWithClock(
	availability RegistrationAvailabilityChecker,
	accounts RegistrationAccountCreator,
	sessions LoginSessionWriter,
	tokens LoginTokenGenerator,
	universeNumber int,
	now func() time.Time,
) RegistrationRegistrar {
	if now == nil {
		now = time.Now
	}
	return RegistrationRegistrar{
		availability:   availability,
		accounts:       accounts,
		sessions:       sessions,
		tokens:         tokens,
		now:            now,
		universeNumber: universeNumber,
	}
}

func NewRegistrationActivationService(
	accounts RegistrationAccountActivator,
	sessions LoginSessionWriter,
	tokens LoginTokenGenerator,
	universeNumber int,
) RegistrationActivationService {
	return NewRegistrationActivationServiceWithClock(accounts, sessions, tokens, universeNumber, time.Now)
}

func NewRegistrationActivationServiceWithClock(
	accounts RegistrationAccountActivator,
	sessions LoginSessionWriter,
	tokens LoginTokenGenerator,
	universeNumber int,
	now func() time.Time,
) RegistrationActivationService {
	if now == nil {
		now = time.Now
	}
	return RegistrationActivationService{
		accounts:       accounts,
		sessions:       sessions,
		tokens:         tokens,
		now:            now,
		universeNumber: universeNumber,
	}
}

func (r RegistrationRegistrar) RegisterAccount(ctx context.Context, command RegistrationCommand) (domain.RegistrationCreation, error) {
	draft := domain.RegistrationDraft{
		Character:     command.Character,
		Password:      command.Password,
		Email:         command.Email,
		Universe:      command.Universe,
		TermsAccepted: command.TermsAccepted,
	}
	validation := draft.Validate()
	if !validation.Valid {
		return domain.RegistrationCreation{Valid: false, Issues: validation.Issues}, nil
	}
	if r.accounts == nil || r.sessions == nil || r.tokens == nil {
		return domain.RegistrationCreation{}, errors.New("registration registrar dependencies unavailable")
	}
	if r.availability != nil {
		availability, err := r.availability.CheckRegistrationAvailability(ctx, draft)
		if err != nil {
			return domain.RegistrationCreation{}, err
		}
		if issues := availability.Validate(); len(issues) > 0 {
			return domain.RegistrationCreation{Valid: false, Issues: issues}, nil
		}
	}

	account, err := r.accounts.CreateRegistrationAccount(ctx, draft, command.RemoteAddr)
	if err != nil {
		return domain.RegistrationCreation{}, err
	}
	publicSession, err := r.tokens.NewPublicSession()
	if err != nil {
		return domain.RegistrationCreation{}, err
	}
	privateSession, err := r.tokens.NewPrivateSession()
	if err != nil {
		return domain.RegistrationCreation{}, err
	}
	session := domain.LoginSession{
		PlayerID:       account.PlayerID,
		PublicID:       publicSession,
		PrivateID:      privateSession,
		UniverseNumber: r.universeNumber,
		LastLogin:      r.now().Unix(),
		RedirectPath:   "/game/overview",
	}
	if err := r.sessions.SaveLoginSession(ctx, session, command.RemoteAddr); err != nil {
		return domain.RegistrationCreation{}, err
	}

	return domain.RegistrationCreation{
		Valid:   true,
		Account: account,
		Session: session,
	}, nil
}

func (s RegistrationActivationService) ActivateAccount(ctx context.Context, command RegistrationActivationCommand) (domain.RegistrationActivation, error) {
	code := strings.TrimSpace(command.ActivationCode)
	if code == "" {
		return domain.RegistrationActivation{}, nil
	}
	if s.accounts == nil || s.sessions == nil || s.tokens == nil {
		return domain.RegistrationActivation{}, errors.New("registration activation dependencies unavailable")
	}

	account, err := s.accounts.ActivateRegistrationAccount(ctx, code)
	if err != nil {
		return domain.RegistrationActivation{}, err
	}
	if !account.Found {
		return domain.RegistrationActivation{Account: account}, nil
	}

	publicSession, err := s.tokens.NewPublicSession()
	if err != nil {
		return domain.RegistrationActivation{}, err
	}
	privateSession, err := s.tokens.NewPrivateSession()
	if err != nil {
		return domain.RegistrationActivation{}, err
	}
	session := domain.LoginSession{
		PlayerID:       account.PlayerID,
		PublicID:       publicSession,
		PrivateID:      privateSession,
		UniverseNumber: s.universeNumber,
		LastLogin:      s.now().Unix(),
		RedirectPath:   "/game/overview",
	}
	if err := s.sessions.SaveLoginSession(ctx, session, command.RemoteAddr); err != nil {
		return domain.RegistrationActivation{}, err
	}

	return domain.RegistrationActivation{
		Activated: true,
		Account:   account,
		Session:   session,
	}, nil
}

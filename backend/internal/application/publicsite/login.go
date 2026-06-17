package publicsite

import (
	"context"
	"errors"
	"strings"
	"time"

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

type LoginSessionWriter interface {
	SaveLoginSession(context.Context, domain.LoginSession, string) error
}

type GameSessionReader interface {
	FindGameSession(context.Context, string) (domain.GameSession, error)
}

type LoginTokenGenerator interface {
	NewPublicSession() (string, error)
	NewPrivateSession() (string, error)
}

type LoginDraftValidator struct {
	credentials LoginCredentialChecker
}

type LoginCommand struct {
	Login      string
	Password   string
	Universe   string
	RemoteAddr string
}

type GameSessionCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
}

type LoginAuthenticator struct {
	credentials    LoginCredentialChecker
	sessions       LoginSessionWriter
	tokens         LoginTokenGenerator
	now            func() time.Time
	universeNumber int
}

type GameSessionLookup struct {
	sessions       GameSessionReader
	universeNumber int
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

func NewLoginAuthenticator(credentials LoginCredentialChecker, sessions LoginSessionWriter, tokens LoginTokenGenerator, universeNumber int) LoginAuthenticator {
	return NewLoginAuthenticatorWithClock(credentials, sessions, tokens, universeNumber, time.Now)
}

func NewLoginAuthenticatorWithClock(credentials LoginCredentialChecker, sessions LoginSessionWriter, tokens LoginTokenGenerator, universeNumber int, now func() time.Time) LoginAuthenticator {
	if now == nil {
		now = time.Now
	}
	return LoginAuthenticator{
		credentials:    credentials,
		sessions:       sessions,
		tokens:         tokens,
		now:            now,
		universeNumber: universeNumber,
	}
}

func (a LoginAuthenticator) AuthenticateLogin(ctx context.Context, command LoginCommand) (domain.LoginAuthentication, error) {
	draft := domain.LoginDraft{
		Login:    command.Login,
		Password: command.Password,
		Universe: command.Universe,
	}
	validation := draft.Validate()
	if !validation.Valid {
		return domain.LoginAuthentication{Valid: false, Issues: validation.Issues}, nil
	}
	if a.credentials == nil || a.sessions == nil || a.tokens == nil {
		return domain.LoginAuthentication{}, errors.New("login authenticator dependencies unavailable")
	}

	credentials, err := a.credentials.CheckLoginCredentials(ctx, draft)
	if err != nil {
		return domain.LoginAuthentication{}, err
	}
	if issues := credentials.Validate(); len(issues) > 0 {
		return domain.LoginAuthentication{Valid: false, Issues: issues}, nil
	}

	publicSession, err := a.tokens.NewPublicSession()
	if err != nil {
		return domain.LoginAuthentication{}, err
	}
	privateSession, err := a.tokens.NewPrivateSession()
	if err != nil {
		return domain.LoginAuthentication{}, err
	}

	session := domain.LoginSession{
		PlayerID:       credentials.PlayerID,
		PublicID:       publicSession,
		PrivateID:      privateSession,
		UniverseNumber: a.universeNumber,
		LastLogin:      a.now().Unix(),
		RedirectPath:   "/game/overview",
	}
	if err := a.sessions.SaveLoginSession(ctx, session, command.RemoteAddr); err != nil {
		return domain.LoginAuthentication{}, err
	}
	return domain.LoginAuthentication{Valid: true, Session: session}, nil
}

func NewGameSessionLookup(sessions GameSessionReader, universeNumber int) GameSessionLookup {
	return GameSessionLookup{sessions: sessions, universeNumber: universeNumber}
}

func (l GameSessionLookup) GetGameSession(ctx context.Context, command GameSessionCommand) (domain.SessionAuthentication, error) {
	publicSession := strings.TrimSpace(command.PublicSession)
	if publicSession == "" {
		return domain.SessionAuthentication{
			Authenticated: false,
			Issues: []domain.SessionIssue{{
				Code:    domain.SessionIssueRequired,
				Message: "Session is required.",
			}},
		}, nil
	}
	if l.sessions == nil {
		return domain.SessionAuthentication{}, errors.New("game session lookup dependency unavailable")
	}

	session, err := l.sessions.FindGameSession(ctx, publicSession)
	if err != nil {
		return domain.SessionAuthentication{}, err
	}
	session.UniverseNumber = l.universeNumber
	privateSession := command.PrivateSessions[session.PrivateCookieName()]
	issues := session.Validate(privateSession, command.RemoteAddr)
	return domain.SessionAuthentication{
		Authenticated: len(issues) == 0,
		Issues:        issues,
		Session:       session,
	}, nil
}

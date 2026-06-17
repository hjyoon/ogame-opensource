package publicsite

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestLoginAuthenticatorCreatesSession(t *testing.T) {
	writer := &recordingSessionWriter{}
	tokens := &fakeTokenGenerator{
		public:  []string{"public123456"},
		private: []string{"private1234567890private1234567890"},
	}
	authenticator := NewLoginAuthenticatorWithClock(
		fakeCredentialChecker{credentials: domain.LoginCredentials{Authenticated: true, PlayerID: 42}},
		writer,
		tokens,
		3,
		func() time.Time { return time.Unix(1700000000, 0) },
	)

	result, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{
		Login:      "legor",
		Password:   "admin",
		Universe:   "http://localhost:8888",
		RemoteAddr: "203.0.113.10",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Session.PublicID != "public123456" || result.Session.PrivateID != "private1234567890private1234567890" {
		t.Fatalf("expected valid login session, got %+v", result)
	}
	if writer.session.PlayerID != 42 || writer.session.UniverseNumber != 3 || writer.session.LastLogin != 1700000000 {
		t.Fatalf("unexpected stored session: %+v", writer.session)
	}
	if writer.remoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected stored remote address: %q", writer.remoteAddr)
	}
}

func TestLoginAuthenticatorReturnsLocalValidationIssues(t *testing.T) {
	writer := &recordingSessionWriter{}
	authenticator := NewLoginAuthenticator(
		fakeCredentialChecker{credentials: domain.LoginCredentials{Authenticated: true, PlayerID: 1}},
		writer,
		&fakeTokenGenerator{},
		1,
	)

	result, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || !hasLoginIssue(domain.LoginValidation{Issues: result.Issues}, domain.LoginIssueLoginRequired) {
		t.Fatalf("expected local validation issues, got %+v", result)
	}
	if writer.called {
		t.Fatal("expected session writer to be skipped for invalid local draft")
	}
}

func TestLoginAuthenticatorReturnsCredentialIssues(t *testing.T) {
	writer := &recordingSessionWriter{}
	authenticator := NewLoginAuthenticator(
		fakeCredentialChecker{credentials: domain.LoginCredentials{Authenticated: false}},
		writer,
		&fakeTokenGenerator{},
		1,
	)

	result, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{
		Login:    "legor",
		Password: "wrong",
		Universe: "http://localhost:8888",
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || !hasLoginIssue(domain.LoginValidation{Issues: result.Issues}, domain.LoginIssueCredentialsInvalid) {
		t.Fatalf("expected credential issues, got %+v", result)
	}
	if writer.called {
		t.Fatal("expected session writer to be skipped for invalid credentials")
	}
}

func TestLoginAuthenticatorReturnsCredentialError(t *testing.T) {
	wantErr := errors.New("credentials failed")
	authenticator := NewLoginAuthenticator(fakeCredentialChecker{err: wantErr}, &recordingSessionWriter{}, &fakeTokenGenerator{}, 1)

	_, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{
		Login:    "legor",
		Password: "admin",
		Universe: "http://localhost:8888",
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestLoginAuthenticatorReturnsTokenError(t *testing.T) {
	wantErr := errors.New("token failed")
	authenticator := NewLoginAuthenticator(
		fakeCredentialChecker{credentials: domain.LoginCredentials{Authenticated: true, PlayerID: 1}},
		&recordingSessionWriter{},
		&fakeTokenGenerator{err: wantErr},
		1,
	)

	_, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{
		Login:    "legor",
		Password: "admin",
		Universe: "http://localhost:8888",
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestLoginAuthenticatorReturnsSessionWriterError(t *testing.T) {
	wantErr := errors.New("session write failed")
	authenticator := NewLoginAuthenticator(
		fakeCredentialChecker{credentials: domain.LoginCredentials{Authenticated: true, PlayerID: 1}},
		&recordingSessionWriter{err: wantErr},
		&fakeTokenGenerator{public: []string{"public"}, private: []string{"private"}},
		1,
	)

	_, err := authenticator.AuthenticateLogin(context.Background(), LoginCommand{
		Login:    "legor",
		Password: "admin",
		Universe: "http://localhost:8888",
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected session writer error, got %v", err)
	}
}

func TestLoginAuthenticatorRequiresDependencies(t *testing.T) {
	_, err := (LoginAuthenticator{}).AuthenticateLogin(context.Background(), LoginCommand{
		Login:    "legor",
		Password: "admin",
		Universe: "http://localhost:8888",
	})

	if err == nil {
		t.Fatal("expected missing dependencies error")
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

type recordingSessionWriter struct {
	called     bool
	session    domain.LoginSession
	remoteAddr string
	err        error
}

func (r *recordingSessionWriter) SaveLoginSession(_ context.Context, session domain.LoginSession, remoteAddr string) error {
	r.called = true
	r.session = session
	r.remoteAddr = remoteAddr
	return r.err
}

type fakeTokenGenerator struct {
	public  []string
	private []string
	err     error
}

func (f *fakeTokenGenerator) NewPublicSession() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if len(f.public) == 0 {
		return "public", nil
	}
	value := f.public[0]
	f.public = f.public[1:]
	return value, nil
}

func (f *fakeTokenGenerator) NewPrivateSession() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if len(f.private) == 0 {
		return "private", nil
	}
	value := f.private[0]
	f.private = f.private[1:]
	return value, nil
}

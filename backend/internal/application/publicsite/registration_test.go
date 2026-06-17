package publicsite

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestRegistrationRegistrarSkipsAccountCreationWhenDraftIsInvalid(t *testing.T) {
	accounts := &recordingAccountCreator{}
	registrar := NewRegistrationRegistrarWithClock(nil, accounts, fakeSessionWriter{}, fakeLoginTokens{}, 1, fixedRegistrationTime)

	result, err := registrar.RegisterAccount(context.Background(), RegistrationCommand{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected invalid registration result, got %+v", result)
	}
	if accounts.called {
		t.Fatal("expected account creator to be skipped for invalid draft")
	}
}

func TestRegistrationRegistrarSkipsAccountCreationWhenAvailabilityFails(t *testing.T) {
	accounts := &recordingAccountCreator{}
	registrar := NewRegistrationRegistrarWithClock(fakeAvailabilityChecker{
		availability: domain.RegistrationAvailability{CharacterExists: true},
	}, accounts, fakeSessionWriter{}, fakeLoginTokens{}, 1, fixedRegistrationTime)

	result, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("expected unavailable registration result, got %+v", result)
	}
	if accounts.called {
		t.Fatal("expected account creator to be skipped for unavailable draft")
	}
	if !hasCreationIssue(result, domain.RegistrationIssueCharacterExists) {
		t.Fatalf("expected character availability issue, got %+v", result.Issues)
	}
}

func TestRegistrationRegistrarCreatesAccountAndSavesLoginSession(t *testing.T) {
	accounts := &recordingAccountCreator{
		account: domain.RegisteredAccount{PlayerID: 42, HomePlanetID: 99, ActivationCode: "activation", Validated: false},
	}
	sessions := &recordingRegistrationSessionWriter{}
	registrar := NewRegistrationRegistrarWithClock(
		fakeAvailabilityChecker{},
		accounts,
		sessions,
		fakeLoginTokens{publicID: "public123456", privateID: "private1234567890private1234567890"},
		7,
		fixedRegistrationTime,
	)

	result, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Account.PlayerID != 42 || result.Account.HomePlanetID != 99 {
		t.Fatalf("unexpected registration creation: %+v", result)
	}
	if accounts.remoteAddr != "203.0.113.10" || accounts.draft.Character != "Commander01" {
		t.Fatalf("unexpected account creator call: %+v remote=%q", accounts.draft, accounts.remoteAddr)
	}
	if !sessions.called {
		t.Fatal("expected session writer to be called")
	}
	if sessions.session.PlayerID != 42 || sessions.session.PublicID != "public123456" || sessions.session.UniverseNumber != 7 {
		t.Fatalf("unexpected saved session: %+v", sessions.session)
	}
	if sessions.session.LastLogin != fixedRegistrationTime().Unix() || sessions.session.RedirectPath != "/game/overview" {
		t.Fatalf("unexpected session metadata: %+v", sessions.session)
	}
}

func TestRegistrationRegistrarReturnsDependencyError(t *testing.T) {
	registrar := NewRegistrationRegistrar(nil, nil, nil, nil, 1)

	_, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if err == nil {
		t.Fatal("expected dependency error")
	}
}

func TestRegistrationRegistrarReturnsAccountCreationError(t *testing.T) {
	wantErr := errors.New("insert failed")
	registrar := NewRegistrationRegistrarWithClock(nil, fakeAccountCreator{err: wantErr}, fakeSessionWriter{}, fakeLoginTokens{}, 1, fixedRegistrationTime)

	_, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected account creation error, got %v", err)
	}
}

func TestRegistrationRegistrarReturnsAvailabilityError(t *testing.T) {
	wantErr := errors.New("availability failed")
	registrar := NewRegistrationRegistrarWithClock(fakeAvailabilityChecker{err: wantErr}, fakeAccountCreator{}, fakeSessionWriter{}, fakeLoginTokens{}, 1, fixedRegistrationTime)

	_, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected availability error, got %v", err)
	}
}

func TestRegistrationRegistrarReturnsTokenAndSessionErrors(t *testing.T) {
	account := domain.RegisteredAccount{PlayerID: 42, HomePlanetID: 99}
	cases := map[string]RegistrationRegistrar{
		"public token":  NewRegistrationRegistrarWithClock(nil, fakeAccountCreator{account: account}, fakeSessionWriter{}, fakeLoginTokens{publicErr: errors.New("public token failed")}, 1, fixedRegistrationTime),
		"private token": NewRegistrationRegistrarWithClock(nil, fakeAccountCreator{account: account}, fakeSessionWriter{}, fakeLoginTokens{privateErr: errors.New("private token failed")}, 1, fixedRegistrationTime),
		"session":       NewRegistrationRegistrarWithClock(nil, fakeAccountCreator{account: account}, fakeSessionWriter{err: errors.New("session failed")}, fakeLoginTokens{}, 1, fixedRegistrationTime),
	}
	for name, registrar := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand()); err == nil {
				t.Fatal("expected registration error")
			}
		})
	}
}

func TestRegistrationRegistrarUsesDefaultClock(t *testing.T) {
	registrar := NewRegistrationRegistrarWithClock(nil, nil, nil, nil, 1, nil)

	if registrar.now == nil {
		t.Fatal("expected default clock")
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

func hasCreationIssue(result domain.RegistrationCreation, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func validRegistrationCommand() RegistrationCommand {
	return RegistrationCommand{
		Character:     "Commander01",
		Password:      "E2E_http123",
		Email:         "commander@example.local",
		Universe:      "http://localhost:8888",
		TermsAccepted: true,
		RemoteAddr:    "203.0.113.10",
	}
}

func fixedRegistrationTime() time.Time {
	return time.Unix(1700000000, 0)
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

type fakeAccountCreator struct {
	account domain.RegisteredAccount
	err     error
}

func (f fakeAccountCreator) CreateRegistrationAccount(context.Context, domain.RegistrationDraft, string) (domain.RegisteredAccount, error) {
	return f.account, f.err
}

type recordingAccountCreator struct {
	called     bool
	draft      domain.RegistrationDraft
	remoteAddr string
	account    domain.RegisteredAccount
	err        error
}

func (r *recordingAccountCreator) CreateRegistrationAccount(_ context.Context, draft domain.RegistrationDraft, remoteAddr string) (domain.RegisteredAccount, error) {
	r.called = true
	r.draft = draft
	r.remoteAddr = remoteAddr
	return r.account, r.err
}

type fakeSessionWriter struct {
	err error
}

func (f fakeSessionWriter) SaveLoginSession(context.Context, domain.LoginSession, string) error {
	return f.err
}

type recordingRegistrationSessionWriter struct {
	called     bool
	session    domain.LoginSession
	remoteAddr string
	err        error
}

func (r *recordingRegistrationSessionWriter) SaveLoginSession(_ context.Context, session domain.LoginSession, remoteAddr string) error {
	r.called = true
	r.session = session
	r.remoteAddr = remoteAddr
	return r.err
}

type fakeLoginTokens struct {
	publicID   string
	privateID  string
	publicErr  error
	privateErr error
}

func (f fakeLoginTokens) NewPublicSession() (string, error) {
	if f.publicErr != nil {
		return "", f.publicErr
	}
	if f.publicID == "" {
		return "public", nil
	}
	return f.publicID, nil
}

func (f fakeLoginTokens) NewPrivateSession() (string, error) {
	if f.privateErr != nil {
		return "", f.privateErr
	}
	if f.privateID == "" {
		return "private", nil
	}
	return f.privateID, nil
}

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

func TestRegistrationRegistrarSendsWelcomeMail(t *testing.T) {
	accounts := &recordingAccountCreator{
		account: domain.RegisteredAccount{PlayerID: 42, HomePlanetID: 99, ActivationCode: "activation"},
	}
	mailer := &recordingRegistrationMailer{}
	registrar := NewRegistrationRegistrarWithClockAndMailer(
		nil,
		accounts,
		fakeSessionWriter{},
		fakeLoginTokens{},
		7,
		fixedRegistrationTime,
		mailer,
	)

	_, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if err != nil {
		t.Fatal(err)
	}
	if !mailer.called {
		t.Fatal("expected welcome mailer to be called")
	}
	if mailer.message.Character != "Commander01" || mailer.message.Password != "E2E_http123" || mailer.message.Email != "commander@example.local" {
		t.Fatalf("unexpected welcome mail payload: %+v", mailer.message)
	}
	if mailer.message.ActivationCode != "activation" || mailer.message.UniverseNumber != 7 {
		t.Fatalf("unexpected welcome mail activation payload: %+v", mailer.message)
	}
}

func TestRegistrationRegistrarReturnsWelcomeMailError(t *testing.T) {
	wantErr := errors.New("mail failed")
	registrar := NewRegistrationRegistrarWithMailer(
		nil,
		fakeAccountCreator{account: domain.RegisteredAccount{PlayerID: 42, HomePlanetID: 99, ActivationCode: "activation"}},
		fakeSessionWriter{},
		fakeLoginTokens{},
		1,
		&recordingRegistrationMailer{err: wantErr},
	)

	_, err := registrar.RegisterAccount(context.Background(), validRegistrationCommand())

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected welcome mail error, got %v", err)
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

func TestRegistrationActivationServiceActivatesAndSavesLoginSession(t *testing.T) {
	accounts := &recordingAccountActivator{account: domain.ActivatedAccount{Found: true, PlayerID: 42}}
	sessions := &recordingRegistrationSessionWriter{}
	service := NewRegistrationActivationServiceWithClock(
		accounts,
		sessions,
		fakeLoginTokens{publicID: "public123456", privateID: "private1234567890private1234567890"},
		7,
		fixedRegistrationTime,
	)

	result, err := service.ActivateAccount(context.Background(), RegistrationActivationCommand{
		ActivationCode: " activation-code ",
		RemoteAddr:     "203.0.113.10",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Activated || result.Account.PlayerID != 42 || result.Session.PublicID != "public123456" {
		t.Fatalf("unexpected activation result: %+v", result)
	}
	if accounts.code != "activation-code" {
		t.Fatalf("expected trimmed activation code, got %q", accounts.code)
	}
	if sessions.session.PlayerID != 42 || sessions.session.UniverseNumber != 7 || sessions.remoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected saved activation session: session=%+v remote=%q", sessions.session, sessions.remoteAddr)
	}
}

func TestRegistrationActivationServiceSkipsBlankOrMissingCode(t *testing.T) {
	accounts := &recordingAccountActivator{}
	service := NewRegistrationActivationService(accounts, fakeSessionWriter{}, fakeLoginTokens{}, 1)

	result, err := service.ActivateAccount(context.Background(), RegistrationActivationCommand{ActivationCode: " "})

	if err != nil {
		t.Fatal(err)
	}
	if result.Activated || accounts.called {
		t.Fatalf("expected blank activation to be ignored, result=%+v called=%v", result, accounts.called)
	}
}

func TestRegistrationActivationServiceReturnsNotFoundWithoutSession(t *testing.T) {
	accounts := &recordingAccountActivator{account: domain.ActivatedAccount{Found: false}}
	sessions := &recordingRegistrationSessionWriter{}
	service := NewRegistrationActivationService(accounts, sessions, fakeLoginTokens{}, 1)

	result, err := service.ActivateAccount(context.Background(), RegistrationActivationCommand{ActivationCode: "missing"})

	if err != nil {
		t.Fatal(err)
	}
	if result.Activated || sessions.called {
		t.Fatalf("expected missing activation without session, result=%+v sessions=%v", result, sessions.called)
	}
}

func TestRegistrationActivationServiceReturnsDependencyAndStageErrors(t *testing.T) {
	if _, err := (RegistrationActivationService{}).ActivateAccount(context.Background(), RegistrationActivationCommand{ActivationCode: "ack"}); err == nil {
		t.Fatal("expected dependency error")
	}

	account := domain.ActivatedAccount{Found: true, PlayerID: 42}
	cases := map[string]RegistrationActivationService{
		"account": NewRegistrationActivationService(fakeAccountActivator{err: errors.New("account failed")}, fakeSessionWriter{}, fakeLoginTokens{}, 1),
		"public":  NewRegistrationActivationService(fakeAccountActivator{account: account}, fakeSessionWriter{}, fakeLoginTokens{publicErr: errors.New("public failed")}, 1),
		"private": NewRegistrationActivationService(fakeAccountActivator{account: account}, fakeSessionWriter{}, fakeLoginTokens{privateErr: errors.New("private failed")}, 1),
		"session": NewRegistrationActivationService(fakeAccountActivator{account: account}, fakeSessionWriter{err: errors.New("session failed")}, fakeLoginTokens{}, 1),
	}
	for name, service := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := service.ActivateAccount(context.Background(), RegistrationActivationCommand{ActivationCode: "ack"}); err == nil {
				t.Fatal("expected activation error")
			}
		})
	}
}

func TestRegistrationActivationServiceUsesDefaultClock(t *testing.T) {
	service := NewRegistrationActivationServiceWithClock(nil, nil, nil, 1, nil)

	if service.now == nil {
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

type fakeAccountActivator struct {
	account domain.ActivatedAccount
	err     error
}

func (f fakeAccountActivator) ActivateRegistrationAccount(context.Context, string) (domain.ActivatedAccount, error) {
	return f.account, f.err
}

type recordingAccountActivator struct {
	called  bool
	code    string
	account domain.ActivatedAccount
	err     error
}

func (r *recordingAccountActivator) ActivateRegistrationAccount(_ context.Context, code string) (domain.ActivatedAccount, error) {
	r.called = true
	r.code = code
	return r.account, r.err
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

type recordingRegistrationMailer struct {
	called  bool
	message domain.RegistrationWelcomeMail
	err     error
}

func (r *recordingRegistrationMailer) SendRegistrationWelcome(_ context.Context, message domain.RegistrationWelcomeMail) error {
	r.called = true
	r.message = message
	return r.err
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

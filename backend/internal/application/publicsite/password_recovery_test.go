package publicsite

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type fakePasswordRecoveryRepository struct {
	email    string
	password string
	account  domain.PasswordRecoveryAccount
	err      error
	calls    int
}

func (f *fakePasswordRecoveryRepository) RecoverPassword(_ context.Context, email string, password string) (domain.PasswordRecoveryAccount, error) {
	f.calls++
	f.email = email
	f.password = password
	return f.account, f.err
}

type fakePasswordRecoveryMailer struct {
	message domain.PasswordRecoveryMail
	err     error
	calls   int
}

func (f *fakePasswordRecoveryMailer) SendPasswordRecovery(_ context.Context, message domain.PasswordRecoveryMail) error {
	f.calls++
	f.message = message
	return f.err
}

type fixedPasswordGenerator struct {
	password string
	err      error
}

func (f fixedPasswordGenerator) NewPassword() (string, error) {
	return f.password, f.err
}

func TestPasswordRecoveryServiceSendsRecoveryMail(t *testing.T) {
	repository := &fakePasswordRecoveryRepository{account: domain.PasswordRecoveryAccount{
		Found:          true,
		PlayerID:       42,
		Character:      "Legor",
		PermanentEmail: "legor@example.local",
	}}
	mailer := &fakePasswordRecoveryMailer{}
	service := NewPasswordRecoveryServiceWithGenerator(repository, mailer, fixedPasswordGenerator{password: "abc123xy"}, 7, "http://game.test")

	result, err := service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: " LEGOR@example.local "})
	if err != nil {
		t.Fatalf("RecoverPassword returned error: %v", err)
	}
	if !result.Submitted || !result.Sent || result.Account.PlayerID != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if repository.email != "legor@example.local" || repository.password != "abc123xy" {
		t.Fatalf("repository received wrong data: %+v", repository)
	}
	if mailer.message.Email != "legor@example.local" || mailer.message.Password != "abc123xy" || mailer.message.UniverseNumber != 7 {
		t.Fatalf("unexpected mail: %+v", mailer.message)
	}
}

func TestNewPasswordRecoveryServiceUsesRandomGenerator(t *testing.T) {
	service := NewPasswordRecoveryService(&fakePasswordRecoveryRepository{}, &fakePasswordRecoveryMailer{}, 1, "http://game.test")
	if service.passwords == nil || service.universeNumber != 1 || service.publicBaseURL != "http://game.test" {
		t.Fatalf("unexpected service: %+v", service)
	}
	service = NewPasswordRecoveryServiceWithGenerator(&fakePasswordRecoveryRepository{}, &fakePasswordRecoveryMailer{}, nil, 2, "")
	if service.passwords == nil || service.universeNumber != 2 {
		t.Fatalf("nil generator should be replaced: %+v", service)
	}
}

func TestPasswordRecoveryServiceRejectsInvalidOrUnknownEmailSilently(t *testing.T) {
	repository := &fakePasswordRecoveryRepository{}
	mailer := &fakePasswordRecoveryMailer{}
	service := NewPasswordRecoveryServiceWithGenerator(repository, mailer, fixedPasswordGenerator{password: "abc123xy"}, 1, "")

	result, err := service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "bad"})
	if err != nil || !result.Submitted || result.Sent || repository.calls != 0 || mailer.calls != 0 {
		t.Fatalf("invalid email should be silent no-op: result=%+v repo=%d mail=%d err=%v", result, repository.calls, mailer.calls, err)
	}

	result, err = service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "missing@example.local"})
	if err != nil || !result.Submitted || result.Sent || repository.calls != 1 || mailer.calls != 0 {
		t.Fatalf("unknown email should be silent no-op: result=%+v repo=%d mail=%d err=%v", result, repository.calls, mailer.calls, err)
	}
}

func TestPasswordRecoveryServiceErrors(t *testing.T) {
	_, err := NewPasswordRecoveryServiceWithGenerator(nil, nil, fixedPasswordGenerator{password: "abc123xy"}, 1, "").RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "user@example.local"})
	if err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}

	repository := &fakePasswordRecoveryRepository{account: domain.PasswordRecoveryAccount{Found: true, PermanentEmail: "user@example.local"}}
	service := NewPasswordRecoveryServiceWithGenerator(repository, &fakePasswordRecoveryMailer{}, fixedPasswordGenerator{err: errors.New("password failed")}, 1, "")
	if _, err := service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "user@example.local"}); err == nil || !strings.Contains(err.Error(), "password failed") {
		t.Fatalf("expected password error, got %v", err)
	}

	service = NewPasswordRecoveryServiceWithGenerator(&fakePasswordRecoveryRepository{err: errors.New("repo failed")}, &fakePasswordRecoveryMailer{}, fixedPasswordGenerator{password: "abc123xy"}, 1, "")
	if _, err := service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "user@example.local"}); err == nil || !strings.Contains(err.Error(), "repo failed") {
		t.Fatalf("expected repository error, got %v", err)
	}

	service = NewPasswordRecoveryServiceWithGenerator(repository, &fakePasswordRecoveryMailer{err: errors.New("mail failed")}, fixedPasswordGenerator{password: "abc123xy"}, 1, "")
	if _, err := service.RecoverPassword(context.Background(), domain.PasswordRecoveryCommand{Email: "user@example.local"}); err == nil || !strings.Contains(err.Error(), "mail failed") {
		t.Fatalf("expected mail error, got %v", err)
	}
}

func TestRandomPasswordGenerator(t *testing.T) {
	password, err := RandomPasswordGenerator{}.NewPassword()
	if err != nil {
		t.Fatalf("NewPassword returned error: %v", err)
	}
	if len(password) != 8 {
		t.Fatalf("unexpected password length: %q", password)
	}
	for _, ch := range password {
		if !strings.ContainsRune(passwordAlphabet, ch) {
			t.Fatalf("password contains unexpected character %q in %q", ch, password)
		}
	}
}

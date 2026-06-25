package mysqlregistration

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPasswordRecoveryRepositoryUpdatesPasswordAndClearsSession(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{{
		rows: &fakeRows{items: []fakeRow{{values: []any{42, "Legor", "legor@example.local"}}}},
	}}}
	execer := &fakeExecer{}
	repository := NewPasswordRecoveryRepositoryWithRunner(queryer, execer, "uni1_", "secret")

	account, err := repository.RecoverPassword(context.Background(), "legor@example.local", "abc123xy")
	if err != nil {
		t.Fatalf("RecoverPassword returned error: %v", err)
	}
	if !account.Found || account.PlayerID != 42 || account.Character != "Legor" || account.PermanentEmail != "legor@example.local" {
		t.Fatalf("unexpected account: %+v", account)
	}
	if !strings.Contains(queryer.calls[0].query, "email = ? OR pemail = ?") || queryer.calls[0].args[0] != "legor@example.local" || queryer.calls[0].args[1] != "legor@example.local" {
		t.Fatalf("unexpected lookup query: %+v", queryer.calls[0])
	}
	if !strings.Contains(execer.query, "session = ''") || !strings.Contains(execer.query, "password = ?") || execer.args[1] != 42 {
		t.Fatalf("unexpected update: query=%s args=%+v", execer.query, execer.args)
	}
	if got, want := execer.args[0], hashLegacyPassword("abc123xy", "secret"); got != want {
		t.Fatalf("unexpected password hash: got=%v want=%s", got, want)
	}
}

func TestNewPasswordRecoveryRepository(t *testing.T) {
	repository := NewPasswordRecoveryRepository(nil, "uni1_", "secret")
	if repository.queryer == nil || repository.execer == nil || repository.prefix != "uni1_" || repository.secret != "secret" {
		t.Fatalf("unexpected repository: %+v", repository)
	}
}

func TestPasswordRecoveryRepositoryReturnsMissingAccount(t *testing.T) {
	repository := NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{}, &fakeExecer{}, "uni1_", "secret")
	account, err := repository.RecoverPassword(context.Background(), "missing@example.local", "abc123xy")
	if err != nil {
		t.Fatalf("RecoverPassword returned error: %v", err)
	}
	if account.Found {
		t.Fatalf("missing account should not be found: %+v", account)
	}
}

func TestPasswordRecoveryRepositoryErrors(t *testing.T) {
	if _, err := NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{}, &fakeExecer{}, "bad-prefix_", "secret").RecoverPassword(context.Background(), "user@example.local", "abc123xy"); err == nil {
		t.Fatalf("expected bad prefix error")
	}

	repository := NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{responses: []fakeResponse{{err: errors.New("query failed")}}}, &fakeExecer{}, "uni1_", "secret")
	if _, err := repository.RecoverPassword(context.Background(), "user@example.local", "abc123xy"); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected query error, got %v", err)
	}

	repository = NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{scanErr: errors.New("scan failed"), items: []fakeRow{{values: []any{42, "Legor", "legor@example.local"}}}}}}}, &fakeExecer{}, "uni1_", "secret")
	if _, err := repository.RecoverPassword(context.Background(), "user@example.local", "abc123xy"); err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("expected scan error, got %v", err)
	}

	repository = NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{err: errors.New("rows failed")}}}}, &fakeExecer{}, "uni1_", "secret")
	if _, err := repository.RecoverPassword(context.Background(), "user@example.local", "abc123xy"); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}

	repository = NewPasswordRecoveryRepositoryWithRunner(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "Legor", "legor@example.local"}}}}}}}, &fakeExecer{err: errors.New("update failed")}, "uni1_", "secret")
	if _, err := repository.RecoverPassword(context.Background(), "user@example.local", "abc123xy"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}
}

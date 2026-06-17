package mysqlregistration

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestCredentialCheckerAuthenticatesLegacyPassword(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, 0, 0}}}}},
	}}
	checker := NewCredentialCheckerWithQueryer(queryer, "uni1_", "docker-secret")

	credentials, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{
		Login:    "Legor",
		Password: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !credentials.Authenticated || credentials.PlayerID != 42 || credentials.Banned {
		t.Fatalf("unexpected credentials: %+v", credentials)
	}
	if !strings.Contains(queryer.calls[0].query, "`uni1_users`") {
		t.Fatalf("expected prefixed users table, got %+v", queryer.calls[0])
	}
	if queryer.calls[0].args[0] != "legor" {
		t.Fatalf("expected lower-cased login arg, got %+v", queryer.calls[0].args)
	}
	if queryer.calls[0].args[1] != "6f5ed64b0c998f74693d43dfcafca507" {
		t.Fatalf("unexpected legacy password hash: %+v", queryer.calls[0].args)
	}
}

func TestCredentialCheckerReportsMissingCredentials(t *testing.T) {
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{}}}}, "uni1_", "secret")

	credentials, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{
		Login:    "missing",
		Password: "wrong",
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Authenticated {
		t.Fatalf("expected unauthenticated credentials, got %+v", credentials)
	}
}

func TestCredentialCheckerReportsBannedUser(t *testing.T) {
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, 1, 12345}}}}},
	}}, "uni1_", "secret")

	credentials, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{
		Login:    "legor",
		Password: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !credentials.Authenticated || !credentials.Banned || credentials.BannedUntil != 12345 {
		t.Fatalf("expected banned credentials, got %+v", credentials)
	}
}

func TestCredentialCheckerReturnsQueryError(t *testing.T) {
	wantErr := errors.New("credential query failed")
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{{err: wantErr}}}, "uni1_", "secret")

	if _, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestCredentialCheckerReturnsRowsError(t *testing.T) {
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{err: errors.New("rows failed")}}}}, "uni1_", "secret")

	if _, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{}); err == nil {
		t.Fatal("expected rows error")
	}
}

func TestCredentialCheckerReturnsScanError(t *testing.T) {
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, 0, 0}, scanErr: errors.New("scan failed")}}}},
	}}, "uni1_", "secret")

	if _, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{}); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestCredentialCheckerRejectsUnsafePrefix(t *testing.T) {
	checker := NewCredentialCheckerWithQueryer(&fakeQueryer{}, "uni1_;DROP", "secret")

	if _, err := checker.CheckLoginCredentials(context.Background(), domain.LoginDraft{}); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

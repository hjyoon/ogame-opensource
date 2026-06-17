package mysqlregistration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestSessionStoreSavesLegacySessionFields(t *testing.T) {
	execer := &fakeExecer{}
	store := NewSessionStoreWithExecer(execer, "uni1_")

	err := store.SaveLoginSession(context.Background(), domain.LoginSession{
		PlayerID:  42,
		PublicID:  "public123456",
		PrivateID: "private1234567890private1234567890",
		LastLogin: 1700000000,
	}, "203.0.113.10")

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(execer.query, "`uni1_users`") {
		t.Fatalf("expected prefixed users table, got %q", execer.query)
	}
	expectedArgs := []any{int64(1700000000), "public123456", "private1234567890private1234567890", "203.0.113.10", 42}
	for i, expected := range expectedArgs {
		if execer.args[i] != expected {
			t.Fatalf("arg %d: expected %#v, got %#v", i, expected, execer.args[i])
		}
	}
}

func TestSessionStoreReturnsExecError(t *testing.T) {
	wantErr := errors.New("update failed")
	store := NewSessionStoreWithExecer(&fakeExecer{err: wantErr}, "uni1_")

	err := store.SaveLoginSession(context.Background(), domain.LoginSession{}, "")

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}

func TestSessionStoreRejectsUnsafePrefix(t *testing.T) {
	store := NewSessionStoreWithExecer(&fakeExecer{}, "uni1_;DROP")

	if err := store.SaveLoginSession(context.Background(), domain.LoginSession{}, ""); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

func TestNewSessionStoreUsesSQLExecer(t *testing.T) {
	store := NewSessionStore(nil, "uni1_")

	if store.prefix != "uni1_" {
		t.Fatalf("unexpected prefix: %+v", store)
	}
	if _, ok := store.execer.(SQLExecer); !ok {
		t.Fatalf("expected SQLExecer, got %T", store.execer)
	}
	if _, ok := store.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQLQueryer, got %T", store.queryer)
	}
}

func TestSessionStoreFindsLegacyGameSession(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 1, 0, 99}}}}},
	}}
	store := NewSessionStoreWithQueryer(queryer, "uni1_")

	session, err := store.FindGameSession(context.Background(), "public")

	if err != nil {
		t.Fatal(err)
	}
	if !session.Found || session.PlayerID != 42 || session.Commander != "legor" || session.PublicID != "public" || session.PrivateID != "private" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if !session.DisableIPCheck || session.Banned || session.HomePlanetID != 99 {
		t.Fatalf("unexpected session flags: %+v", session)
	}
	if !strings.Contains(queryer.calls[0].query, "`uni1_users`") || queryer.calls[0].args[0] != "public" {
		t.Fatalf("unexpected query: %+v", queryer.calls[0])
	}
}

func TestSessionStoreReportsMissingGameSession(t *testing.T) {
	store := NewSessionStoreWithQueryer(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{}}}}, "uni1_")

	session, err := store.FindGameSession(context.Background(), "missing")

	if err != nil {
		t.Fatal(err)
	}
	if session.Found {
		t.Fatalf("expected missing session, got %+v", session)
	}
}

func TestSessionStoreFindGameSessionReturnsQueryError(t *testing.T) {
	wantErr := errors.New("session query failed")
	store := NewSessionStoreWithQueryer(&fakeQueryer{responses: []fakeResponse{{err: wantErr}}}, "uni1_")

	if _, err := store.FindGameSession(context.Background(), "public"); !errors.Is(err, wantErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestSessionStoreFindGameSessionReturnsRowsError(t *testing.T) {
	store := NewSessionStoreWithQueryer(&fakeQueryer{responses: []fakeResponse{{rows: &fakeRows{err: errors.New("rows failed")}}}}, "uni1_")

	if _, err := store.FindGameSession(context.Background(), "public"); err == nil {
		t.Fatal("expected rows error")
	}
}

func TestSessionStoreFindGameSessionReturnsScanError(t *testing.T) {
	store := NewSessionStoreWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 1, 0, 99}, scanErr: errors.New("scan failed")}}}},
	}}, "uni1_")

	if _, err := store.FindGameSession(context.Background(), "public"); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestSessionStoreFindGameSessionRejectsUnsafePrefix(t *testing.T) {
	store := NewSessionStoreWithQueryer(&fakeQueryer{}, "uni1_;DROP")

	if _, err := store.FindGameSession(context.Background(), "public"); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

type fakeExecer struct {
	query string
	args  []any
	err   error
}

func (f *fakeExecer) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return nil, f.err
}

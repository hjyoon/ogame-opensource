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
	if !strings.Contains(execer.query, "UPDATE `uni1_users` SET lastlogin = ?, session = ?, private_session = ?, ip_addr = ?, aktplanet = hplanetid WHERE player_id = ?") {
		t.Fatalf("expected legacy login update with home planet selection, got %q", execer.query)
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

func TestSessionStoreClearsLegacyPublicSession(t *testing.T) {
	execer := &fakeExecer{}
	store := NewSessionStoreWithExecer(execer, "uni1_")

	err := store.ClearGameSession(context.Background(), "public123456", 42)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(execer.query, "UPDATE `uni1_users` SET session = '' WHERE player_id = ?") {
		t.Fatalf("expected legacy logout update, got %q", execer.query)
	}
	if len(execer.args) != 1 || execer.args[0] != 42 {
		t.Fatalf("unexpected clear args: %+v", execer.args)
	}
}

func TestSessionStoreTouchesLegacyLastClick(t *testing.T) {
	execer := &fakeExecer{}
	store := NewSessionStoreWithExecer(execer, "uni1_")

	err := store.TouchGameSession(context.Background(), 42, 1700000000)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(execer.query, "UPDATE `uni1_users` SET lastclick = ? WHERE player_id = ?") {
		t.Fatalf("expected legacy lastclick update, got %q", execer.query)
	}
	if len(execer.args) != 2 || execer.args[0] != int64(1700000000) || execer.args[1] != 42 {
		t.Fatalf("unexpected touch args: %+v", execer.args)
	}
}

func TestSessionStoreTouchGameSessionReturnsExecError(t *testing.T) {
	wantErr := errors.New("touch failed")
	store := NewSessionStoreWithExecer(&fakeExecer{err: wantErr}, "uni1_")

	err := store.TouchGameSession(context.Background(), 42, 1700000000)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected touch error, got %v", err)
	}
}

func TestSessionStoreTouchGameSessionRejectsUnsafePrefix(t *testing.T) {
	store := NewSessionStoreWithExecer(&fakeExecer{}, "uni1_;DROP")

	if err := store.TouchGameSession(context.Background(), 42, 1700000000); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

func TestSessionStoreTouchGameSessionRequiresDependency(t *testing.T) {
	store := NewSessionStoreWithQueryer(&fakeQueryer{}, "uni1_")

	if err := store.TouchGameSession(context.Background(), 42, 1700000000); err == nil {
		t.Fatal("expected missing exec dependency error")
	}
}

func TestSessionStoreClearGameSessionReturnsExecError(t *testing.T) {
	wantErr := errors.New("clear failed")
	store := NewSessionStoreWithExecer(&fakeExecer{err: wantErr}, "uni1_")

	err := store.ClearGameSession(context.Background(), "public", 42)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected clear error, got %v", err)
	}
}

func TestSessionStoreClearGameSessionRejectsUnsafePrefix(t *testing.T) {
	store := NewSessionStoreWithExecer(&fakeExecer{}, "uni1_;DROP")

	if err := store.ClearGameSession(context.Background(), "public", 42); err == nil {
		t.Fatal("expected unsafe prefix error")
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
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 1, 0, 0, 0, 0, 0, 0, 99}}}}},
	}}
	store := NewSessionStoreWithQueryer(queryer, "uni1_")

	session, err := store.FindGameSession(context.Background(), "public")

	if err != nil {
		t.Fatal(err)
	}
	if !session.Found || session.PlayerID != 42 || session.Commander != "legor" || session.PublicID != "public" || session.PrivateID != "private" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if !session.DisableIPCheck || session.Banned || session.BannedUntil != 0 || session.VacationMode || session.VacationUntil != 0 || session.DeletionQueued || session.DeletionAt != 0 || session.HomePlanetID != 99 {
		t.Fatalf("unexpected session flags: %+v", session)
	}
	if !strings.Contains(queryer.calls[0].query, "`uni1_users`") || !strings.Contains(queryer.calls[0].query, "vacation_until") || !strings.Contains(queryer.calls[0].query, "disable_until") || queryer.calls[0].args[0] != "public" {
		t.Fatalf("unexpected query: %+v", queryer.calls[0])
	}
}

func TestSessionStoreFindsBannedGameSessionExpiry(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 0, 1, 12345, 0, 0, 0, 0, 99}}}}},
	}}
	store := NewSessionStoreWithQueryer(queryer, "uni1_")

	session, err := store.FindGameSession(context.Background(), "public")

	if err != nil {
		t.Fatal(err)
	}
	if !session.Banned || session.BannedUntil != 12345 {
		t.Fatalf("expected banned session expiry, got %+v", session)
	}
}

func TestSessionStoreFindsVacationAndDeletionState(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 0, 0, 0, 1, 12345, 1, 23456, 99}}}}},
	}}
	store := NewSessionStoreWithQueryer(queryer, "uni1_")

	session, err := store.FindGameSession(context.Background(), "public")

	if err != nil {
		t.Fatal(err)
	}
	if !session.VacationMode || session.VacationUntil != 12345 || !session.DeletionQueued || session.DeletionAt != 23456 {
		t.Fatalf("expected vacation/deletion state, got %+v", session)
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
		{rows: &fakeRows{items: []fakeRow{{values: []any{42, "legor", "public", "private", "203.0.113.10", 1, 0, 0, 0, 0, 0, 0, 99}, scanErr: errors.New("scan failed")}}}},
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

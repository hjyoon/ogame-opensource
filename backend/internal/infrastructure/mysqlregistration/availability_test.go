package mysqlregistration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestDSN(t *testing.T) {
	dsn := DSN(UniverseDBConfig{
		Host:     "db.example:3307",
		User:     "root",
		Password: "secret",
		Name:     "uni",
	})

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "db.example:3307" || cfg.User != "root" || cfg.Passwd != "secret" || cfg.DBName != "uni" {
		t.Fatalf("unexpected DSN config: %+v", cfg)
	}
}

func TestAvailabilityCheckerChecksLegacyRegistrationState(t *testing.T) {
	queryer := &fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{1}}}}},
		{rows: &fakeRows{}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{12}}}}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{20}}}}},
	}}
	checker := NewAvailabilityCheckerWithQueryer(queryer, "uni1_")

	availability, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{
		Character: "Commander01",
		Email:     "Commander@Example.Local",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !availability.CharacterExists || availability.EmailExists || availability.UserCount != 12 || availability.MaxUsers != 20 {
		t.Fatalf("unexpected availability: %+v", availability)
	}
	if len(queryer.calls) != 4 {
		t.Fatalf("expected four queries, got %+v", queryer.calls)
	}
	if !strings.Contains(queryer.calls[0].query, "`uni1_users`") || !strings.Contains(queryer.calls[3].query, "`uni1_uni`") {
		t.Fatalf("expected prefixed table names, got %+v", queryer.calls)
	}
	if queryer.calls[0].args[0] != "commander01" || queryer.calls[1].args[0] != "commander@example.local" {
		t.Fatalf("expected lower-cased lookup args, got %+v", queryer.calls)
	}
}

func TestNewAvailabilityCheckerUsesSQLQueryer(t *testing.T) {
	checker := NewAvailabilityChecker(nil, "uni1_")

	if checker.prefix != "uni1_" {
		t.Fatalf("unexpected prefix: %+v", checker)
	}
	if _, ok := checker.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQLQueryer, got %T", checker.queryer)
	}
}

func TestHostPort(t *testing.T) {
	cases := map[string]string{
		"":              "mysql:3306",
		"mysql":         "mysql:3306",
		"127.0.0.1":     "127.0.0.1:3306",
		"127.0.0.1:123": "127.0.0.1:123",
	}
	for input, expected := range cases {
		if got := hostPort(input); got != expected {
			t.Fatalf("%q: expected %q, got %q", input, expected, got)
		}
	}
}

func TestAvailabilityCheckerRejectsUnsafePrefix(t *testing.T) {
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{}, "uni1_;DROP")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

func TestAvailabilityCheckerReturnsQueryError(t *testing.T) {
	wantErr := errors.New("query failed")
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{{err: wantErr}}}, "uni1_")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestAvailabilityCheckerReturnsEmailQueryError(t *testing.T) {
	wantErr := errors.New("email query failed")
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{}},
		{err: wantErr},
	}}, "uni1_")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected email query error, got %v", err)
	}
}

func TestAvailabilityCheckerReturnsScanError(t *testing.T) {
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{}},
		{rows: &fakeRows{}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{12}, scanErr: errors.New("scan failed")}}}},
	}}, "uni1_")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestAvailabilityCheckerReturnsRowsError(t *testing.T) {
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{err: errors.New("rows failed")}},
	}}, "uni1_")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); err == nil {
		t.Fatal("expected rows error")
	}
}

func TestAvailabilityCheckerReturnsMaxUsersRowsError(t *testing.T) {
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{}},
		{rows: &fakeRows{}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{2}}}}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{10}}}, err: errors.New("max users rows failed")}},
	}}, "uni1_")

	if _, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{}); err == nil {
		t.Fatal("expected max users rows error")
	}
}

func TestAvailabilityCheckerAllowsMissingMaxUsersRow(t *testing.T) {
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{
		{rows: &fakeRows{}},
		{rows: &fakeRows{}},
		{rows: &fakeRows{items: []fakeRow{{values: []any{2}}}}},
		{rows: &fakeRows{}},
	}}, "uni1_")

	availability, err := checker.CheckRegistrationAvailability(context.Background(), domain.RegistrationDraft{})
	if err != nil {
		t.Fatal(err)
	}
	if availability.UserCount != 2 || availability.MaxUsers != 0 {
		t.Fatalf("unexpected availability for missing max users row: %+v", availability)
	}
}

func TestSingleIntReturnsQueryError(t *testing.T) {
	wantErr := errors.New("count failed")
	checker := NewAvailabilityCheckerWithQueryer(&fakeQueryer{responses: []fakeResponse{{err: wantErr}}}, "uni1_")

	if _, err := checker.singleInt(context.Background(), "SELECT COUNT(*) FROM `uni1_users`"); !errors.Is(err, wantErr) {
		t.Fatalf("expected singleInt query error, got %v", err)
	}
}

func TestSQLQueryerReturnsDatabaseError(t *testing.T) {
	db, err := Open(UniverseDBConfig{
		Host: "127.0.0.1:1",
		User: "root",
		Name: "uni",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = (SQLQueryer{DB: db}).QueryContext(ctx, "SELECT 1")

	if err == nil {
		t.Fatal("expected database query to fail")
	}
}

type fakeQueryer struct {
	responses []fakeResponse
	calls     []fakeCall
}

type fakeResponse struct {
	rows Rows
	err  error
}

type fakeCall struct {
	query string
	args  []any
}

func (f *fakeQueryer) QueryContext(_ context.Context, query string, args ...any) (Rows, error) {
	f.calls = append(f.calls, fakeCall{query: query, args: args})
	if len(f.responses) == 0 {
		return &fakeRows{}, nil
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.rows, response.err
}

type fakeRow struct {
	values  []any
	scanErr error
}

type fakeRows struct {
	items   []fakeRow
	index   int
	closed  bool
	err     error
	scanErr error
}

func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.items)
}

func (r *fakeRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	if r.scanErr != nil {
		return r.scanErr
	}
	if item.scanErr != nil {
		return item.scanErr
	}
	for i, value := range item.values {
		switch d := dest[i].(type) {
		case *int:
			*d = value.(int)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

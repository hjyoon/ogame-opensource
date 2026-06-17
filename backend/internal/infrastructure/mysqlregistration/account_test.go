package mysqlregistration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestAccountCreatorCreatesLegacyUserAndHomePlanet(t *testing.T) {
	txer := &fakeRegistrationTxRunner{tx: &fakeRegistrationTx{
		queryResponses: []fakeResponse{
			{rows: &fakeRows{items: []fakeRow{{values: []any{1, 499, 9, "en", 0, 5000}}}}},
			{rows: &fakeRows{}},
		},
		execResults: []sql.Result{
			fakeSQLResult{id: 0},
			fakeSQLResult{id: 42},
			fakeSQLResult{id: 99},
			fakeSQLResult{id: 0},
		},
	}}
	creator := NewAccountCreatorWithRunner(txer, "uni1_", "docker-secret", fixedAccountTime, fixedRandomBytes)

	account, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{
		Character: "Commander01",
		Password:  "E2E_http123",
		Email:     "Commander@Example.Local",
	}, "203.0.113.10")

	if err != nil {
		t.Fatal(err)
	}
	if account.PlayerID != 42 || account.HomePlanetID != 99 || account.Validated {
		t.Fatalf("unexpected account: %+v", account)
	}
	if account.ActivationCode != "000102030405060708090a0b0c0d0e0f" {
		t.Fatalf("unexpected activation code: %q", account.ActivationCode)
	}
	if !txer.called {
		t.Fatal("expected account creation to run in a transaction")
	}
	if len(txer.tx.execCalls) != 4 {
		t.Fatalf("expected four exec calls, got %+v", txer.tx.execCalls)
	}
	userInsert := txer.tx.execCalls[1]
	if !strings.Contains(userInsert.query, "INSERT INTO `uni1_users`") {
		t.Fatalf("expected users insert, got %q", userInsert.query)
	}
	if !containsArg(userInsert.args, "commander01") || !containsArg(userInsert.args, "Commander01") {
		t.Fatalf("expected normalized and original commander names, got %+v", userInsert.args)
	}
	if !containsArg(userInsert.args, "commander@example.local") || !containsArg(userInsert.args, "07569eae982eee30eaaa91c2b8b1d058") {
		t.Fatalf("expected legacy email and password hash, got %+v", userInsert.args)
	}
	planetInsert := txer.tx.execCalls[2]
	if !strings.Contains(planetInsert.query, "INSERT INTO `uni1_planets`") {
		t.Fatalf("expected planets insert, got %q", planetInsert.query)
	}
	for _, expected := range []any{"Homeplanet", 42, 1, 1, 4, 12800, 31, 500} {
		if !containsArg(planetInsert.args, expected) {
			t.Fatalf("expected planet insert arg %#v, got %+v", expected, planetInsert.args)
		}
	}
	if !strings.Contains(txer.tx.execCalls[3].query, "hplanetid") || txer.tx.execCalls[3].args[0] != 99 || txer.tx.execCalls[3].args[2] != 42 {
		t.Fatalf("unexpected home planet update: %+v", txer.tx.execCalls[3])
	}
}

func TestNewAccountCreatorUsesSQLRunnerDefaults(t *testing.T) {
	creator := NewAccountCreator(nil, "uni1_", "secret")

	if creator.prefix != "uni1_" || creator.secret != "secret" {
		t.Fatalf("unexpected creator config: %+v", creator)
	}
	if _, ok := creator.txer.(SQLTxRunner); !ok {
		t.Fatalf("expected SQLTxRunner, got %T", creator.txer)
	}
	if creator.now == nil || creator.randomBytes == nil {
		t.Fatalf("expected default clock and random generator: %+v", creator)
	}
	random, err := cryptoRandomBytes(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(random) != 2 {
		t.Fatalf("unexpected random byte length: %d", len(random))
	}
}

func TestNewAccountCreatorWithRunnerDefaultsClockAndRandom(t *testing.T) {
	creator := NewAccountCreatorWithRunner(&fakeRegistrationTxRunner{}, "uni1_", "secret", nil, nil)

	if creator.now == nil || creator.randomBytes == nil {
		t.Fatalf("expected default dependencies: %+v", creator)
	}
}

func TestNewCredentialCheckerUsesSQLQueryer(t *testing.T) {
	checker := NewCredentialChecker(nil, "uni1_", "secret")

	if checker.prefix != "uni1_" || checker.secret != "secret" {
		t.Fatalf("unexpected credential checker: %+v", checker)
	}
	if _, ok := checker.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQLQueryer, got %T", checker.queryer)
	}
}

func TestSQLExecerUsesDatabase(t *testing.T) {
	db := openRegistrationTestDB(t)
	defer db.Close()

	result, err := (SQLExecer{DB: db}).ExecContext(context.Background(), "UPDATE fake SET value = ?", 1)

	if err != nil {
		t.Fatal(err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("unexpected fake rows affected: %d %v", affected, err)
	}
}

func TestSQLTxRunnerCommitsFakeDatabaseTransaction(t *testing.T) {
	db := openRegistrationTestDB(t)
	defer db.Close()
	runner := SQLTxRunner{DB: db}
	var queried bool
	var executed bool

	err := runner.WithTx(context.Background(), func(tx registrationTx) error {
		rows, err := tx.QueryContext(context.Background(), "SELECT 1")
		if err != nil {
			return err
		}
		defer rows.Close()
		if !rows.Next() {
			return errors.New("expected fake row")
		}
		var value int
		if err := rows.Scan(&value); err != nil {
			return err
		}
		queried = value == 1
		_, err = tx.ExecContext(context.Background(), "UPDATE fake SET value = ?", value)
		executed = err == nil
		return err
	})

	if err != nil {
		t.Fatal(err)
	}
	if !queried || !executed {
		t.Fatalf("expected query and exec through sqlRegistrationTx, queried=%v executed=%v", queried, executed)
	}
}

func TestSQLTxRunnerRollsBackCallbackError(t *testing.T) {
	db := openRegistrationTestDB(t)
	defer db.Close()
	wantErr := errors.New("callback failed")

	err := (SQLTxRunner{DB: db}).WithTx(context.Background(), func(registrationTx) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestSQLTxRunnerRejectsNilDatabase(t *testing.T) {
	if err := (SQLTxRunner{}).WithTx(context.Background(), func(registrationTx) error { return nil }); err == nil {
		t.Fatal("expected nil database error")
	}
}

func TestAccountCreatorRejectsUnsafePrefix(t *testing.T) {
	creator := NewAccountCreatorWithRunner(&fakeRegistrationTxRunner{}, "uni1_;DROP", "secret", fixedAccountTime, fixedRandomBytes)

	if _, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{}, ""); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
}

func TestAccountCreatorReturnsMissingDependencyError(t *testing.T) {
	creator := NewAccountCreatorWithRunner(nil, "uni1_", "secret", fixedAccountTime, fixedRandomBytes)

	if _, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{}, ""); err == nil {
		t.Fatal("expected dependency error")
	}
}

func TestAccountCreatorReturnsRandomError(t *testing.T) {
	wantErr := errors.New("random failed")
	creator := NewAccountCreatorWithRunner(&fakeRegistrationTxRunner{}, "uni1_", "secret", fixedAccountTime, func(int) ([]byte, error) {
		return nil, wantErr
	})

	if _, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{}, ""); !errors.Is(err, wantErr) {
		t.Fatalf("expected random error, got %v", err)
	}
}

func TestAccountCreatorReturnsTransactionError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	creator := NewAccountCreatorWithRunner(&fakeRegistrationTxRunner{err: wantErr}, "uni1_", "secret", fixedAccountTime, fixedRandomBytes)

	if _, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{}, ""); !errors.Is(err, wantErr) {
		t.Fatalf("expected transaction error, got %v", err)
	}
}

func TestAccountCreatorReturnsStageErrors(t *testing.T) {
	validUniverse := func() fakeResponse {
		return fakeResponse{rows: &fakeRows{items: []fakeRow{{values: []any{1, 499, 9, "en", 0, 5000}}}}}
	}
	cases := map[string]*fakeRegistrationTx{
		"universe": {
			queryResponses: []fakeResponse{{err: errors.New("universe failed")}},
		},
		"usercount": {
			queryResponses: []fakeResponse{validUniverse()},
			execErrors:     []error{errors.New("usercount failed")},
		},
		"user insert": {
			queryResponses: []fakeResponse{validUniverse()},
			execResults:    []sql.Result{fakeSQLResult{id: 0}, fakeSQLResult{id: 0}},
		},
		"coordinates": {
			queryResponses: []fakeResponse{validUniverse(), {err: errors.New("coordinates failed")}},
			execResults:    []sql.Result{fakeSQLResult{id: 0}, fakeSQLResult{id: 42}},
		},
		"planet insert": {
			queryResponses: []fakeResponse{validUniverse(), {rows: &fakeRows{}}},
			execResults:    []sql.Result{fakeSQLResult{id: 0}, fakeSQLResult{id: 42}, fakeSQLResult{id: 0}},
		},
		"home update": {
			queryResponses: []fakeResponse{validUniverse(), {rows: &fakeRows{}}},
			execResults:    []sql.Result{fakeSQLResult{id: 0}, fakeSQLResult{id: 42}, fakeSQLResult{id: 99}, fakeSQLResult{id: 0}},
			execErrors:     []error{nil, nil, nil, errors.New("home update failed")},
		},
	}
	for name, tx := range cases {
		t.Run(name, func(t *testing.T) {
			creator := NewAccountCreatorWithRunner(&fakeRegistrationTxRunner{tx: tx}, "uni1_", "secret", fixedAccountTime, fixedRandomBytes)
			if _, err := creator.CreateRegistrationAccount(context.Background(), domain.RegistrationDraft{Password: "pw"}, ""); err == nil {
				t.Fatal("expected stage error")
			}
		})
	}
}

func TestLoadRegistrationUniverseDefaultsLanguage(t *testing.T) {
	tx := &fakeRegistrationTx{queryResponses: []fakeResponse{
		{rows: &fakeRows{items: []fakeRow{{values: []any{1, 1, 1, "", 0, 0}}}}},
	}}

	universe, err := loadRegistrationUniverse(context.Background(), tx, "`uni1_uni`")

	if err != nil {
		t.Fatal(err)
	}
	if universe.Language != "en" {
		t.Fatalf("expected default language, got %+v", universe)
	}
}

func TestLoadRegistrationUniverseReportsBadRows(t *testing.T) {
	cases := map[string]*fakeRegistrationTx{
		"missing": {
			queryResponses: []fakeResponse{{rows: &fakeRows{}}},
		},
		"scan": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{1, 1, 1, "en", 0, 0}, scanErr: errors.New("scan failed")}}}}},
		},
		"rows": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{1, 1, 1, "en", 0, 0}}}, err: errors.New("rows failed")}}},
		},
		"layout": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{1, 0, 1, "en", 0, 0}}}}}},
		},
	}
	for name, tx := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadRegistrationUniverse(context.Background(), tx, "`uni1_uni`"); err == nil {
				t.Fatal("expected universe load error")
			}
		})
	}
}

func TestNextHomePlanetCoordinatesReportsRowErrors(t *testing.T) {
	universe := registrationUniverse{Systems: 1, Galaxies: 1}
	cases := map[string]*fakeRegistrationTx{
		"query": {
			queryErr: errors.New("query failed"),
		},
		"scan": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{1, 1, 4}, scanErr: errors.New("scan failed")}}}}},
		},
		"rows": {
			queryResponses: []fakeResponse{{rows: &fakeRows{err: errors.New("rows failed")}}},
		},
	}
	for name, tx := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := nextHomePlanetCoordinates(context.Background(), tx, "`uni1_planets`", universe); err == nil {
				t.Fatal("expected coordinate error")
			}
		})
	}
}

func TestNextHomePlanetCoordinatesIgnoresInvalidOccupiedRows(t *testing.T) {
	tx := &fakeRegistrationTx{queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{0, 1, 4}}}}}}}

	coords, err := nextHomePlanetCoordinates(context.Background(), tx, "`uni1_planets`", registrationUniverse{Systems: 1, Galaxies: 1})

	if err != nil {
		t.Fatal(err)
	}
	if coords.Position != 4 {
		t.Fatalf("expected invalid row to be ignored, got %+v", coords)
	}
}

func TestInsertRegistrationUserReportsExecAndIDErrors(t *testing.T) {
	row := registrationUserRow{}
	if _, err := insertRegistrationUser(context.Background(), &fakeRegistrationTx{execErr: errors.New("insert failed")}, "`uni1_users`", row); err == nil {
		t.Fatal("expected insert error")
	}
	if _, err := insertRegistrationUser(context.Background(), &fakeRegistrationTx{execResults: []sql.Result{fakeSQLResult{err: errors.New("id failed")}}}, "`uni1_users`", row); err == nil {
		t.Fatal("expected last insert id error")
	}
}

func TestInsertHomePlanetReportsExecAndIDErrors(t *testing.T) {
	row := homePlanetRow{}
	if _, err := insertHomePlanet(context.Background(), &fakeRegistrationTx{execErr: errors.New("insert failed")}, "`uni1_planets`", row); err == nil {
		t.Fatal("expected insert error")
	}
	if _, err := insertHomePlanet(context.Background(), &fakeRegistrationTx{execResults: []sql.Result{fakeSQLResult{id: 0}}}, "`uni1_planets`", row); err == nil {
		t.Fatal("expected empty insert id error")
	}
}

func TestFirstHomePlanetSlotSkipsOccupiedLegacyPositions(t *testing.T) {
	slot, err := firstHomePlanetSlot(registrationUniverse{Systems: 2, Galaxies: 1}, map[int]bool{3: true})

	if err != nil {
		t.Fatal(err)
	}
	if slot.Galaxy != 1 || slot.System != 1 || slot.Position != 6 {
		t.Fatalf("unexpected first available slot: %+v", slot)
	}
}

func TestFirstHomePlanetSlotReportsNoSpace(t *testing.T) {
	occupied := map[int]bool{}
	for i := 0; i < 15; i++ {
		occupied[i] = true
	}

	if _, err := firstHomePlanetSlot(registrationUniverse{Systems: 1, Galaxies: 1}, occupied); err == nil {
		t.Fatal("expected no slot error")
	}
}

func TestHomePlanetTemperatureMatchesLegacyBands(t *testing.T) {
	cases := map[int]int{
		1:  81,
		4:  25,
		8:  -3,
		11: -29,
		14: -85,
	}
	for position, expected := range cases {
		if got := homePlanetTemperature(position, 3); got != expected {
			t.Fatalf("position %d: expected %d, got %d", position, expected, got)
		}
	}
}

func fixedAccountTime() time.Time {
	return time.Unix(1700000000, 0)
}

func fixedRandomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	for i := range value {
		value[i] = byte(i)
	}
	value[size-1] = 9
	return value, nil
}

func containsArg(args []any, expected any) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

type fakeRegistrationTxRunner struct {
	tx     *fakeRegistrationTx
	called bool
	err    error
}

func (f *fakeRegistrationTxRunner) WithTx(_ context.Context, fn func(registrationTx) error) error {
	f.called = true
	if f.err != nil {
		return f.err
	}
	if f.tx == nil {
		f.tx = &fakeRegistrationTx{}
	}
	return fn(f.tx)
}

type fakeRegistrationTx struct {
	queryResponses []fakeResponse
	execResults    []sql.Result
	execErrors     []error
	queryErr       error
	execErr        error
	queryCalls     []fakeCall
	execCalls      []fakeCall
}

func (f *fakeRegistrationTx) QueryContext(_ context.Context, query string, args ...any) (Rows, error) {
	f.queryCalls = append(f.queryCalls, fakeCall{query: query, args: args})
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if len(f.queryResponses) == 0 {
		return &fakeRows{}, nil
	}
	response := f.queryResponses[0]
	f.queryResponses = f.queryResponses[1:]
	return response.rows, response.err
}

func (f *fakeRegistrationTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execCalls = append(f.execCalls, fakeCall{query: query, args: args})
	if len(f.execErrors) > 0 {
		err := f.execErrors[0]
		f.execErrors = f.execErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	if f.execErr != nil {
		return nil, f.execErr
	}
	if len(f.execResults) == 0 {
		return fakeSQLResult{id: 1}, nil
	}
	result := f.execResults[0]
	f.execResults = f.execResults[1:]
	return result, nil
}

type fakeSQLResult struct {
	id       int64
	affected int64
	err      error
}

func (f fakeSQLResult) LastInsertId() (int64, error) {
	return f.id, f.err
}

func (f fakeSQLResult) RowsAffected() (int64, error) {
	return f.affected, f.err
}

var registerRegistrationTestDriver sync.Once

func openRegistrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerRegistrationTestDriver.Do(func() {
		sql.Register("registration_tx_test", registrationTestDriver{})
	})
	db, err := sql.Open("registration_tx_test", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type registrationTestDriver struct{}

func (registrationTestDriver) Open(string) (driver.Conn, error) {
	return registrationTestConn{}, nil
}

type registrationTestConn struct{}

func (registrationTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (registrationTestConn) Close() error {
	return nil
}

func (registrationTestConn) Begin() (driver.Tx, error) {
	return registrationTestTx{}, nil
}

func (registrationTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return registrationTestTx{}, nil
}

func (registrationTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &registrationTestRows{}, nil
}

func (registrationTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

type registrationTestTx struct{}

func (registrationTestTx) Commit() error {
	return nil
}

func (registrationTestTx) Rollback() error {
	return nil
}

type registrationTestRows struct {
	done bool
}

func (r *registrationTestRows) Columns() []string {
	return []string{"value"}
}

func (r *registrationTestRows) Close() error {
	return nil
}

func (r *registrationTestRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = int64(1)
	r.done = true
	return nil
}

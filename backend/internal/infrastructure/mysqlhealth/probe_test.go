package mysqlhealth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
)

func TestNilDatabaseIsNotReady(t *testing.T) {
	if New(nil).Ready(context.Background()) {
		t.Fatal("nil database must not be ready")
	}
}

func TestModRuntimeProbeRejectsMissingDatabaseAndUnsafePrefix(t *testing.T) {
	if NewModRuntimeProbe(nil, "uni1_").Ready(context.Background()) {
		t.Fatal("nil mod runtime database must not be ready")
	}
	if NewModRuntimeProbe(nil, "bad-prefix_").Ready(context.Background()) {
		t.Fatal("unsafe mod runtime prefix must not be ready")
	}
}

func TestModRuntimeProbeChecksInstalledMods(t *testing.T) {
	db := openHealthTestDB(t, "empty")
	if !NewModRuntimeProbe(db, "uni1_").Ready(context.Background()) {
		t.Fatal("empty mod list must be ready")
	}

	db = openHealthTestDB(t, "active")
	if NewModRuntimeProbe(db, "uni1_").Ready(context.Background()) {
		t.Fatal("active legacy PHP mod must make the Go runtime not ready")
	}

	db = openHealthTestDB(t, "error")
	if NewModRuntimeProbe(db, "uni1_").Ready(context.Background()) {
		t.Fatal("mod query failure must make the Go runtime not ready")
	}
}

func openHealthTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerHealthTestDriver.Do(func() { sql.Register("mysqlhealth_test", healthTestDriver{}) })
	db, err := sql.Open("mysqlhealth_test", mode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

var registerHealthTestDriver sync.Once

type healthTestDriver struct{}

func (healthTestDriver) Open(name string) (driver.Conn, error) {
	return healthTestConn{mode: name}, nil
}

type healthTestConn struct{ mode string }

func (healthTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (healthTestConn) Close() error                        { return nil }
func (healthTestConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

func (c healthTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if c.mode == "error" {
		return nil, errors.New("query failed")
	}
	value := ""
	if c.mode == "active" {
		value = "SpaceStorm"
	}
	return &healthTestRows{value: value}, nil
}

type healthTestRows struct {
	value string
	done  bool
}

func (*healthTestRows) Columns() []string { return []string{"modlist"} }
func (*healthTestRows) Close() error      { return nil }
func (r *healthTestRows) Next(values []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	values[0] = r.value
	return nil
}

package mysqlgame

import (
	"context"
	"errors"
	"testing"

	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/sqlitedb"
)

func TestRewriteSQLiteLimitedMutation(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "update one",
			query: "UPDATE `uni1_planets` SET `1` = `1` + ? WHERE planet_id = ? LIMIT 1",
			want:  "UPDATE `uni1_planets` SET `1` = `1` + ? WHERE rowid IN (SELECT rowid FROM `uni1_planets` WHERE planet_id = ? LIMIT 1)",
		},
		{
			name:  "delete oldest",
			query: "DELETE FROM `uni1_messages` WHERE owner_id = ? ORDER BY date ASC LIMIT 1",
			want:  "DELETE FROM `uni1_messages` WHERE rowid IN (SELECT rowid FROM `uni1_messages` WHERE owner_id = ? ORDER BY date ASC LIMIT 1)",
		},
		{
			name:  "delete newest batch",
			query: "DELETE FROM `uni1_reports` ORDER BY date DESC LIMIT 50",
			want:  "DELETE FROM `uni1_reports` WHERE rowid IN (SELECT rowid FROM `uni1_reports` ORDER BY date DESC LIMIT 50)",
		},
		{
			name:  "leave select unchanged",
			query: "SELECT id FROM `uni1_messages` LIMIT 1",
			want:  "SELECT id FROM `uni1_messages` LIMIT 1",
		},
		{
			name:  "leave parameter limit unchanged",
			query: "DELETE FROM `uni1_messages` WHERE owner_id = ? LIMIT ?",
			want:  "DELETE FROM `uni1_messages` WHERE owner_id = ? LIMIT ?",
		},
		{
			name:  "delete without clauses",
			query: "DELETE FROM `uni1_messages` LIMIT 1",
			want:  "DELETE FROM `uni1_messages` WHERE rowid IN (SELECT rowid FROM `uni1_messages` LIMIT 1)",
		},
		{
			name:  "leave update without where unchanged",
			query: "UPDATE `uni1_messages` SET shown = 1 LIMIT 1",
			want:  "UPDATE `uni1_messages` SET shown = 1 LIMIT 1",
		},
		{
			name:  "leave aliased mutation unchanged",
			query: "DELETE FROM `uni1_messages` AS m WHERE owner_id = ? LIMIT 1",
			want:  "DELETE FROM `uni1_messages` AS m WHERE owner_id = ? LIMIT 1",
		},
		{
			name:  "leave malformed update unchanged",
			query: "UPDATE `uni1_messages` LIMIT 1",
			want:  "UPDATE `uni1_messages` LIMIT 1",
		},
		{
			name:  "leave aliased update unchanged",
			query: "UPDATE `uni1_messages` m SET shown = 1 WHERE owner_id = ? LIMIT 1",
			want:  "UPDATE `uni1_messages` m SET shown = 1 WHERE owner_id = ? LIMIT 1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteSQLiteLimitedMutation(test.query); got != test.want {
				t.Fatalf("rewrite mismatch\n got: %s\nwant: %s", got, test.want)
			}
		})
	}
}

func TestSQLiteDialectDetectionTransactionAndLocks(t *testing.T) {
	if normalizeDialect(DialectSQLite) != DialectSQLite || normalizeDialect("unknown") != DialectMySQL {
		t.Fatal("unexpected SQL dialect normalization")
	}
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runner := SQLQueryer{DB: db}
	if detectSQLDialectFromRunner(&runner) != DialectSQLite || detectSQLDialectFromRunner(struct{}{}) != DialectMySQL {
		t.Fatal("unexpected SQL runner dialect detection")
	}
	if _, err := runner.ExecContext(context.Background(), "CREATE TABLE items (id INTEGER PRIMARY KEY, value INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ExecContext(context.Background(), "INSERT INTO items (id, value) VALUES (1, 0), (2, 0)"); err != nil {
		t.Fatal(err)
	}
	if err := runner.WithTransaction(context.Background(), func(_ Queryer, execer Execer) error {
		_, err := execer.ExecContext(context.Background(), "UPDATE items SET value = 1 WHERE value = 0 ORDER BY id ASC LIMIT 1")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var updated int
	if err := db.QueryRow("SELECT COUNT(*) FROM items WHERE value = 1").Scan(&updated); err != nil || updated != 1 {
		t.Fatalf("expected one transaction update, count=%d err=%v", updated, err)
	}

	unlock, err := acquireProcessDatabaseLock(context.Background(), db, "test-lock", "timeout")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireProcessDatabaseLock(ctx, db, "another-lock", "timeout"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled SQLite lock, got %v", err)
	}
	unlock()
	unlock()
	noop, err := acquireProcessDatabaseLock(context.Background(), nil, "", "timeout")
	if err != nil {
		t.Fatal(err)
	}
	noop()
}

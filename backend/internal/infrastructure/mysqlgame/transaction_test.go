package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSQLQueryerTransactions(t *testing.T) {
	db := openOverviewTestDB(t)
	defer db.Close()
	runner := SQLQueryer{DB: db}
	if err := runner.WithTransaction(context.Background(), func(queryer Queryer, execer Execer) error {
		rows, err := queryer.QueryContext(context.Background(), "SELECT 1")
		if err != nil {
			return err
		}
		_ = rows.Close()
		_, err = execer.ExecContext(context.Background(), "UPDATE test SET value=1")
		return err
	}); err != nil {
		t.Fatalf("expected transaction commit, got %v", err)
	}
	want := errors.New("rollback requested")
	if err := runner.WithTransaction(context.Background(), func(Queryer, Execer) error { return want }); !errors.Is(err, want) {
		t.Fatalf("expected callback error after rollback, got %v", err)
	}
	if err := (SQLQueryer{}).WithTransaction(context.Background(), func(Queryer, Execer) error { return nil }); err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("expected nil database error, got %v", err)
	}
}

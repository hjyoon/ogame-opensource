package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/sqlitedb"
)

func TestFinishDueQueueTaskAtomicallyRunsConcurrentClaimOnce(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE effects (task_id INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (77, 1, 'Fleet', 9, 0, 0, 0, 100, 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}

	runner := SQLQueryer{DB: db}
	const workers = 16
	start := make(chan struct{})
	errs := make(chan error, workers)
	var claimed atomic.Int32
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ok, err := finishDueQueueTaskAtomically(
				context.Background(),
				runner,
				runner,
				"`uni1_queue`",
				dueQueueTaskClaim{TaskID: 77, Type: queueTypeFleet, End: 100},
				100,
				func(_ Queryer, execer Execer) error {
					if _, err := execer.ExecContext(context.Background(), "INSERT INTO effects (task_id) VALUES (?)", 77); err != nil {
						return err
					}
					_, err := execer.ExecContext(context.Background(), "DELETE FROM uni1_queue WHERE task_id = ?", 77)
					return err
				},
			)
			if ok {
				claimed.Add(1)
			}
			errs <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent queue completion failed: %v", err)
		}
	}

	if claimed.Load() != 1 {
		t.Fatalf("expected one queue claimant, got %d", claimed.Load())
	}
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM effects WHERE task_id = 77", 1)
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM uni1_queue WHERE task_id = 77", 0)
}

func TestFinishDueQueueTaskAtomicallySharesSQLiteMutationLock(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (78, 1, 'Fleet', 9, 0, 0, 0, 100, 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireProcessDatabaseLock(context.Background(), db, "mutation", "mutation lock timeout")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	type completionResult struct {
		claimed bool
		err     error
	}
	done := make(chan completionResult, 1)
	runner := SQLQueryer{DB: db}
	go func() {
		claimed, err := finishDueQueueTaskAtomically(
			context.Background(),
			runner,
			runner,
			"`uni1_queue`",
			dueQueueTaskClaim{TaskID: 78, Type: queueTypeFleet, End: 100},
			100,
			func(_ Queryer, execer Execer) error {
				_, err := execer.ExecContext(context.Background(), "DELETE FROM uni1_queue WHERE task_id = ?", 78)
				return err
			},
		)
		done <- completionResult{claimed: claimed, err: err}
	}()

	select {
	case result := <-done:
		t.Fatalf("queue completion bypassed SQLite mutation lock: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}

	unlock()
	select {
	case result := <-done:
		if result.err != nil || !result.claimed {
			t.Fatalf("queue completion after unlock: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queue completion did not resume after SQLite mutation unlock")
	}
}

func TestFinishDueQueueTaskAtomicallyReusesOwnedSQLiteMutationGuard(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (79, 1, 'Fleet', 9, 0, 0, 0, 100, 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}

	guard, err := acquireDatabaseMutationGuard(context.Background(), db, "mutation", "mutation lock timeout")
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()

	runner := SQLQueryer{DB: db}
	claimed, err := finishDueQueueTaskAtomicallyWithGuard(
		context.Background(),
		runner,
		runner,
		"`uni1_queue`",
		dueQueueTaskClaim{TaskID: 79, Type: queueTypeFleet, End: 100},
		100,
		guard,
		func(_ Queryer, execer Execer) error {
			_, err := execer.ExecContext(context.Background(), "DELETE FROM uni1_queue WHERE task_id = ?", 79)
			return err
		},
	)
	if err != nil || !claimed {
		t.Fatalf("guarded queue completion failed: claimed=%v err=%v", claimed, err)
	}
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM uni1_queue WHERE task_id = 79", 0)

	guard.Release()
	if guard.coversSQLite(runner) {
		t.Fatal("released mutation guard must not cover later queue work")
	}
}

func TestRuntimeQueueCompletionDefersQuicklyWhenSQLiteMutationIsBusy(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (80, 1, 'Fleet', 9, 0, 0, 0, 100, 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireProcessDatabaseLock(context.Background(), db, "mutation", "mutation lock timeout")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	runner := SQLQueryer{DB: db}
	started := time.Now()
	claimed, err := finishDueQueueTaskAtomically(
		withRuntimeQueueCompletionPolicy(context.Background()),
		runner,
		runner,
		"`uni1_queue`",
		dueQueueTaskClaim{TaskID: 80, Type: queueTypeFleet, End: 100},
		100,
		func(Queryer, Execer) error { return nil },
	)
	if claimed || !errors.Is(err, ErrQueueSettlementBusy) {
		t.Fatalf("expected adaptive busy deferral, claimed=%v err=%v", claimed, err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("runtime queue worker waited too long for a busy mutation lock: %s", elapsed)
	}
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM uni1_queue WHERE task_id = 80", 1)
}

func TestFinishDueQueueTaskAtomicallyRollsBackClaimAndSideEffects(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE effects (task_id INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (88, 1, 'Research', 1, 113, 1, 0, 100, 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}

	want := errors.New("completion failed")
	runner := SQLQueryer{DB: db}
	claimed, err := finishDueQueueTaskAtomically(
		context.Background(),
		runner,
		runner,
		"`uni1_queue`",
		dueQueueTaskClaim{TaskID: 88, Type: queueTypeResearch, End: 100},
		100,
		func(_ Queryer, execer Execer) error {
			if _, err := execer.ExecContext(context.Background(), "INSERT INTO effects (task_id) VALUES (?)", 88); err != nil {
				return err
			}
			if _, err := execer.ExecContext(context.Background(), "DELETE FROM uni1_queue WHERE task_id = ?", 88); err != nil {
				return err
			}
			return want
		},
	)
	if !claimed || !errors.Is(err, want) {
		t.Fatalf("expected claimed rollback error, got claimed=%v err=%v", claimed, err)
	}
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM effects WHERE task_id = 88", 0)
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM uni1_queue WHERE task_id = 88", 1)
}

func TestFinishDueQueueTaskAtomicallyHonorsFrozenClaimPolicy(t *testing.T) {
	db := openQueueCompletionSQLite(t)
	defer db.Close()
	if _, err := db.Exec("INSERT INTO uni1_queue (task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (99, 1, 'coupon', 1, 0, 1, 0, 100, 0, 1, 0)"); err != nil {
		t.Fatal(err)
	}

	runner := SQLQueryer{DB: db}
	finishCalls := 0
	finish := func(_ Queryer, execer Execer) error {
		finishCalls++
		_, err := execer.ExecContext(context.Background(), "DELETE FROM uni1_queue WHERE task_id = ?", 99)
		return err
	}
	claimed, err := finishDueQueueTaskAtomically(
		context.Background(),
		runner,
		runner,
		"`uni1_queue`",
		dueQueueTaskClaim{TaskID: 99, Type: "coupon", End: 100},
		100,
		finish,
	)
	if err != nil {
		t.Fatal(err)
	}
	if claimed || finishCalls != 0 {
		t.Fatalf("default claim must skip frozen task: claimed=%v finish calls=%d", claimed, finishCalls)
	}

	claimed, err = finishDueQueueTaskAtomically(
		context.Background(),
		runner,
		runner,
		"`uni1_queue`",
		dueQueueTaskClaim{TaskID: 99, Type: "coupon", End: 100, AllowFrozen: true},
		100,
		finish,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed || finishCalls != 1 {
		t.Fatalf("allow-frozen claim must finish once: claimed=%v finish calls=%d", claimed, finishCalls)
	}
	assertQueueCompletionCount(t, db, "SELECT COUNT(*) FROM uni1_queue WHERE task_id = 99", 0)
}

func TestClaimDueQueueTaskValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	if claimed, err := claimDueQueueTask(ctx, &fakeQueryer{}, &fakeOverviewRunner{}, "`queue`", dueQueueTaskClaim{}, 100); claimed || err != nil {
		t.Fatalf("invalid claim must be ignored: claimed=%v err=%v", claimed, err)
	}

	wantExec := errors.New("claim update failed")
	execFailure := &fakeOverviewRunner{execErr: wantExec}
	if claimed, err := claimDueQueueTask(ctx, execFailure, execFailure, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || !errors.Is(err, wantExec) {
		t.Fatalf("expected update error: claimed=%v err=%v", claimed, err)
	}

	wantQuery := errors.New("claim query failed")
	queryFailure := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantQuery}}}}
	if claimed, err := claimDueQueueTask(ctx, queryFailure, queryFailure, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || !errors.Is(err, wantQuery) {
		t.Fatalf("expected query error: claimed=%v err=%v", claimed, err)
	}

	notFound := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	if claimed, err := claimDueQueueTask(ctx, notFound, notFound, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || err != nil {
		t.Fatalf("missing claim must be skipped: claimed=%v err=%v", claimed, err)
	}

	wantRows := errors.New("claim rows failed")
	rowsFailure := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(wantRows)}}}}
	if claimed, err := claimDueQueueTask(ctx, rowsFailure, rowsFailure, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || !errors.Is(err, wantRows) {
		t.Fatalf("expected rows error: claimed=%v err=%v", claimed, err)
	}

	scanFailure := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"not-an-int"})}}}}
	if claimed, err := claimDueQueueTask(ctx, scanFailure, scanFailure, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || err == nil {
		t.Fatalf("expected scan error: claimed=%v err=%v", claimed, err)
	}

	postScanFailure := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantRows, []any{1})}}}}
	if claimed, err := claimDueQueueTask(ctx, postScanFailure, postScanFailure, "`queue`", dueQueueTaskClaim{TaskID: 1, Type: "Fleet", End: 100}, 100); claimed || !errors.Is(err, wantRows) {
		t.Fatalf("expected post-scan rows error: claimed=%v err=%v", claimed, err)
	}
}

func TestQueueTransactionRunnerUsesExecerCapability(t *testing.T) {
	runner := &fakeQueueTransactionExecer{}
	if queueTransactionRunner(&fakeQueryer{}, runner) == nil {
		t.Fatal("expected transaction runner from execer")
	}
	if queueTransactionRunner(&fakeQueryer{}, &fakeOverviewRunner{}) != nil {
		t.Fatal("unexpected transaction runner")
	}
}

type fakeQueueTransactionExecer struct{}

func (*fakeQueueTransactionExecer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return fakeSQLResult(1), nil
}

func (*fakeQueueTransactionExecer) WithTransaction(context.Context, func(Queryer, Execer) error) error {
	return nil
}

func openQueueCompletionSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{
		Prefix:        "uni1_",
		Secret:        "secret",
		Universe:      1,
		AdminEmail:    "admin@example.local",
		AdminPassword: "admin",
		Now:           time.Unix(1_700_000_000, 0),
	}); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

func assertQueueCompletionCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s: got %d, want %d", query, got, want)
	}
}

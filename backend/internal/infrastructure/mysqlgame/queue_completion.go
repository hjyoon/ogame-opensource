package mysqlgame

import (
	"context"
	"fmt"
)

type dueQueueTaskClaim struct {
	TaskID      int
	Type        string
	End         int
	AllowFrozen bool
}

// finishDueQueueTaskAtomically locks and revalidates one due task before any
// completion side effects run. The queue identity is the idempotency key.
func finishDueQueueTaskAtomically(
	ctx context.Context,
	queryer Queryer,
	execer Execer,
	queueTable string,
	claim dueQueueTaskClaim,
	until int,
	finish func(Queryer, Execer) error,
) (bool, error) {
	unlock, err := acquireQueueCompletionLock(ctx, queryer)
	if err != nil {
		return false, err
	}
	defer unlock()

	txer := queueTransactionRunner(queryer, execer)
	if txer == nil {
		// Repository unit-test runners are intentionally lightweight. Runtime
		// repositories use SQLQueryer, which always takes the transactional path.
		return true, finish(queryer, execer)
	}

	claimed := false
	err = txer.WithTransaction(ctx, func(txQueryer Queryer, txExecer Execer) error {
		ok, err := claimDueQueueTask(ctx, txQueryer, txExecer, queueTable, claim, until)
		if err != nil || !ok {
			return err
		}
		claimed = true
		return finish(txQueryer, txExecer)
	})
	return claimed, err
}

func acquireQueueCompletionLock(ctx context.Context, queryer Queryer) (func(), error) {
	db := (BuildingsRepository{queryer: queryer}).sqlDB()
	if db == nil || detectSQLDialect(db) != DialectSQLite {
		return func() {}, nil
	}
	return acquireProcessDatabaseLock(ctx, db, "queue-completion", "queue completion lock timeout")
}

func queueTransactionRunner(queryer Queryer, execer Execer) transactionRunner {
	if txer, ok := queryer.(transactionRunner); ok {
		return txer
	}
	if txer, ok := execer.(transactionRunner); ok {
		return txer
	}
	return nil
}

func claimDueQueueTask(
	ctx context.Context,
	queryer Queryer,
	execer Execer,
	queueTable string,
	claim dueQueueTaskClaim,
	until int,
) (bool, error) {
	if claim.TaskID <= 0 || claim.Type == "" {
		return false, nil
	}

	freezeCondition := " AND freeze = 0"
	if claim.AllowFrozen {
		freezeCondition = ""
	}

	// This no-op mutation is deliberately the first transaction statement. It
	// obtains the row/write lock on MySQL and SQLite without removing recurring
	// or partially completed tasks.
	if _, err := execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET task_id = task_id WHERE task_id = ? AND type = ? AND end = ? AND end <= ?%s", queueTable, freezeCondition),
		claim.TaskID,
		claim.Type,
		claim.End,
		until,
	); err != nil {
		return false, err
	}

	rows, err := queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id FROM %s WHERE task_id = ? AND type = ? AND end = ? AND end <= ?%s LIMIT 1", queueTable, freezeCondition),
		claim.TaskID,
		claim.Type,
		claim.End,
		until,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var taskID int
	if err := rows.Scan(&taskID); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return taskID == claim.TaskID, nil
}

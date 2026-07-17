package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestMySQLFinishDueQueueTaskAtomicallyRunsConcurrentClaimOnce(t *testing.T) {
	dsn := os.Getenv("OGAME_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("OGAME_TEST_MYSQL_DSN is not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(16)

	suffix := time.Now().UnixNano()
	queueName := fmt.Sprintf("queue_claim_%d", suffix)
	effectsName := fmt.Sprintf("queue_effects_%d", suffix)
	queueTable := "`" + queueName + "`"
	effectsTable := "`" + effectsName + "`"
	defer db.Exec("DROP TABLE IF EXISTS " + effectsTable)
	defer db.Exec("DROP TABLE IF EXISTS " + queueTable)
	if _, err := db.Exec("CREATE TABLE " + queueTable + " (task_id INT PRIMARY KEY, type VARCHAR(32) NOT NULL, end INT NOT NULL, freeze INT NOT NULL DEFAULT 0) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE " + effectsTable + " (task_id INT NOT NULL) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO " + queueTable + " (task_id, type, end, freeze) VALUES (77, 'Fleet', 100, 0)"); err != nil {
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
				queueTable,
				dueQueueTaskClaim{TaskID: 77, Type: queueTypeFleet, End: 100},
				100,
				func(_ Queryer, execer Execer) error {
					if _, err := execer.ExecContext(context.Background(), "INSERT INTO "+effectsTable+" (task_id) VALUES (77)"); err != nil {
						return err
					}
					_, err := execer.ExecContext(context.Background(), "DELETE FROM "+queueTable+" WHERE task_id = 77")
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
			t.Fatalf("concurrent MySQL queue completion failed: %v", err)
		}
	}

	if claimed.Load() != 1 {
		t.Fatalf("expected one MySQL queue claimant, got %d", claimed.Load())
	}
	assertMySQLQueueCount(t, db, "SELECT COUNT(*) FROM "+effectsTable+" WHERE task_id = 77", 1)
	assertMySQLQueueCount(t, db, "SELECT COUNT(*) FROM "+queueTable+" WHERE task_id = 77", 0)
}

func assertMySQLQueueCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s: got %d, want %d", query, got, want)
	}
}

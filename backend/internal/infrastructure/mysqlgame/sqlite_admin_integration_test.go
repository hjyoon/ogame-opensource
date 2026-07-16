package mysqlgame

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/sqlitedb"
)

func TestSQLiteAdminCronCleanupStatements(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{Prefix: "uni1_", Secret: "secret"}); err != nil {
		t.Fatal(err)
	}
	repository := NewAdminRepository(db, "uni1_")
	tables, err := loadAdminCronTables("uni1_")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO `uni1_planets` (planet_id, name, type, g, s, p, owner_id) VALUES (500, 'Far space', ?, 1, 1, 16, 99999)", legacyPlanetTypeFarSpace); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec("INSERT INTO `uni1_queue` (owner_id, type, start, end, prio) VALUES (0, ?, 0, 1, 0)", adminQueueTypeUnloadAll)
	if err != nil {
		t.Fatal(err)
	}
	taskID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.finishAdminCronUnloadAll(context.Background(), tables, buildingQueueTask{TaskID: int(taskID), End: 1}); err != nil {
		t.Fatalf("run SQLite unload-all cron: %v", err)
	}
	assertSQLiteCount(t, db, "SELECT COUNT(*) FROM `uni1_planets` WHERE planet_id = 500", 0)

	if _, err := db.Exec("INSERT INTO `uni1_planets` (planet_id, name, type, g, s, p, owner_id, `700`, `701`) VALUES (501, 'Debris', ?, 1, 2, 2, 99999, 0, 0)", legacyPlanetTypeDebris); err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec("INSERT INTO `uni1_queue` (owner_id, type, start, end, prio) VALUES (0, ?, 0, 2, 0)", adminQueueTypeCleanDebris)
	if err != nil {
		t.Fatal(err)
	}
	taskID, err = result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.finishAdminCronCleanDebris(context.Background(), tables, buildingQueueTask{TaskID: int(taskID), End: 2}); err != nil {
		t.Fatalf("run SQLite clean-debris cron: %v", err)
	}
	assertSQLiteCount(t, db, "SELECT COUNT(*) FROM `uni1_planets` WHERE planet_id = 501", 0)

	if _, err := db.Exec("INSERT INTO `uni1_ally` (ally_id, tag, name, score1, score2, score3) VALUES (1, 'SQL', 'SQLite', 0, 0, 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO `uni1_users` (player_id, ally_id, name, oname, admin, score1, score2, score3) VALUES (2, 1, 'member', 'Member', 0, 100, 50, 25)"); err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec("INSERT INTO `uni1_queue` (owner_id, type, start, end, prio) VALUES (0, ?, 0, 3, 0)", adminQueueTypeRecalcAllyPoints)
	if err != nil {
		t.Fatal(err)
	}
	taskID, err = result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.finishAdminCronRecalcAllyPoints(context.Background(), tables, buildingQueueTask{TaskID: int(taskID), End: 3}); err != nil {
		t.Fatalf("recalculate SQLite alliance points: %v", err)
	}
	var score, place int
	if err := db.QueryRow("SELECT score1, place1 FROM `uni1_ally` WHERE ally_id = 1").Scan(&score, &place); err != nil {
		t.Fatal(err)
	}
	if score != 100 || place != 1 {
		t.Fatalf("unexpected SQLite alliance rank: score=%d place=%d", score, place)
	}
}

func TestSQLiteAdminDatabaseSerializationAndRestore(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{
		Prefix: "uni1_",
		Secret: "secret",
		Now:    time.Unix(1700000000, 0),
	}); err != nil {
		t.Fatal(err)
	}
	repository := NewAdminRepository(db, "uni1_")
	table, err := repository.serializeAdminDatabaseTable(context.Background(), "uni1_users")
	if err != nil {
		t.Fatalf("serialize SQLite users: %v", err)
	}
	if len(table.Cols) == 0 || len(table.Values) != 2 {
		t.Fatalf("unexpected SQLite users backup: columns=%d rows=%d", len(table.Cols), len(table.Values))
	}
	if _, err := db.Exec("UPDATE `uni1_users` SET oname = 'Changed' WHERE player_id = 1"); err != nil {
		t.Fatal(err)
	}
	if err := repository.deserializeAdminDatabaseBackup(context.Background(), map[string]adminDatabaseBackupTable{"users": table}); err != nil {
		t.Fatalf("restore SQLite users: %v", err)
	}
	var name string
	if err := db.QueryRow("SELECT oname FROM `uni1_users` WHERE player_id = 1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Legor" {
		t.Fatalf("expected restored SQLite user name, got %q", name)
	}
}

func assertSQLiteCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("SQLite count mismatch: got %d want %d", got, want)
	}
}

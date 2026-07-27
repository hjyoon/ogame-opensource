package sqlitedb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrapCreatesReadyDatabasesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	master := openTestDatabase(t)
	universe := openTestDatabase(t)
	options := BootstrapOptions{
		Prefix:        "u7_",
		Secret:        "secret",
		Universe:      7,
		PublicBaseURL: "http://game.example/",
		AdminEmail:    "legor@example.local",
		AdminPassword: "password",
		Now:           time.Unix(1700000000, 0),
	}
	for range 2 {
		if err := BootstrapMaster(ctx, master, options); err != nil {
			t.Fatalf("bootstrap master: %v", err)
		}
		if err := BootstrapUniverse(ctx, universe, options); err != nil {
			t.Fatalf("bootstrap universe: %v", err)
		}
	}

	var number int
	var databaseName, publicURL string
	if err := master.QueryRowContext(ctx, "SELECT num, dbname, uniurl FROM unis").Scan(&number, &databaseName, &publicURL); err != nil {
		t.Fatal(err)
	}
	if number != 7 || databaseName != "sqlite" || publicURL != "http://game.example" {
		t.Fatalf("unexpected master universe: %d %q %q", number, databaseName, publicURL)
	}

	var userCount, galaxyCount, rapidFire int
	if err := universe.QueryRowContext(ctx, "SELECT usercount, galaxies, rapid FROM u7_uni WHERE num = 7").Scan(&userCount, &galaxyCount, &rapidFire); err != nil {
		t.Fatal(err)
	}
	if userCount != 1 || galaxyCount != 9 || rapidFire != 1 {
		t.Fatalf("unexpected universe seed: users=%d galaxies=%d rapid=%d", userCount, galaxyCount, rapidFire)
	}
	var name, email, password string
	var admin int
	if err := universe.QueryRowContext(ctx, "SELECT oname, email, password, admin FROM u7_users WHERE player_id = 1").Scan(&name, &email, &password, &admin); err != nil {
		t.Fatal(err)
	}
	if name != "Legor" || email != options.AdminEmail || password != legacyPassword("password", "secret") || admin != 2 {
		t.Fatalf("unexpected Legor seed: %q %q %q %d", name, email, password, admin)
	}
	var planetCount int
	if err := universe.QueryRowContext(ctx, "SELECT COUNT(*) FROM u7_planets WHERE owner_id = 1 AND g = 1 AND s = 1 AND p = 2").Scan(&planetCount); err != nil || planetCount != 2 {
		t.Fatalf("expected planet and moon, count=%d err=%v", planetCount, err)
	}
	for table, expected := range map[string]int{"u7_exptab": 1, "u7_coltab": 1, "u7_botstrat": 1} {
		var count int
		if err := universe.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != expected {
			t.Fatalf("unexpected %s count=%d err=%v", table, count, err)
		}
	}
	result, err := universe.ExecContext(ctx, "INSERT INTO u7_messages (owner_id,pm,msgfrom,subj,text,shown,date,planet_id) VALUES (1,0,'a','b','c',0,0,0)")
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil || id != 10000 {
		t.Fatalf("expected legacy message sequence 10000, got %d err=%v", id, err)
	}
}

func TestBootstrapCanExplicitlyDisableRapidFire(t *testing.T) {
	ctx := context.Background()
	universe := openTestDatabase(t)
	rapidFire := false
	if err := BootstrapUniverse(ctx, universe, BootstrapOptions{Prefix: "u1_", RapidFire: &rapidFire}); err != nil {
		t.Fatal(err)
	}
	var stored int
	if err := universe.QueryRowContext(ctx, "SELECT rapid FROM u1_uni").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatalf("expected disabled rapid fire, got %d", stored)
	}
}

func TestOpenFileAndMemoryDatabases(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "nested", "ogame.sqlite")
	db, err := Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatal(err)
	}

	first := openTestDatabase(t)
	second := openTestDatabase(t)
	if _, err := first.Exec("CREATE TABLE isolated (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Exec("SELECT * FROM isolated"); err == nil {
		t.Fatal("independent in-memory databases unexpectedly shared state")
	}
}

func TestBootstrapValidationAndHelpers(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t)
	if err := BootstrapMaster(ctx, nil, BootstrapOptions{}); err == nil {
		t.Fatal("expected nil master error")
	}
	if err := BootstrapUniverse(ctx, nil, BootstrapOptions{}); err == nil {
		t.Fatal("expected nil universe error")
	}
	if err := BootstrapUniverse(ctx, db, BootstrapOptions{Prefix: "bad-prefix"}); err == nil {
		t.Fatal("expected invalid prefix error")
	}
	if _, err := dataSourceName(""); err == nil {
		t.Fatal("expected empty path error")
	}
	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(parentFile, "child.sqlite")); err == nil {
		t.Fatal("expected parent directory error")
	}
	if universeNumber(0) != 1 || universeNumber(3) != 3 {
		t.Fatal("unexpected universe number normalization")
	}
	if defaultString("", "fallback") != "fallback" || defaultString("value", "fallback") != "value" {
		t.Fatal("unexpected default string behavior")
	}
	if quote("u1_users") != `"u1_users"` || !strings.HasPrefix(legacyPassword("a", "b"), "187ef") {
		t.Fatal("unexpected sqlite helper output")
	}
}

func TestExecuteSchemaHonorsContextAndMissingAsset(t *testing.T) {
	db := openTestDatabase(t)
	if err := executeSchema(context.Background(), db, "schema/missing.sql", ""); err == nil {
		t.Fatal("expected missing schema error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executeSchema(ctx, db, "schema/master.sql", ""); err == nil {
		t.Fatal("expected canceled schema execution error")
	}
}

func TestBootstrapReportsSQLiteSchemaAndSeedFailures(t *testing.T) {
	ctx := context.Background()
	if db, err := Open(t.TempDir()); err == nil {
		_ = db.Close()
		t.Fatal("expected directory database path to fail")
	}

	closed := openTestDatabase(t)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapMaster(ctx, closed, BootstrapOptions{}); err == nil {
		t.Fatal("expected closed master schema error")
	}
	if err := BootstrapUniverse(ctx, closed, BootstrapOptions{Prefix: "u1_"}); err == nil {
		t.Fatal("expected closed universe schema error")
	}

	badMaster := openTestDatabase(t)
	if _, err := badMaster.Exec("CREATE TABLE unis (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapMaster(ctx, badMaster, BootstrapOptions{}); err == nil {
		t.Fatal("expected incompatible master seed error")
	}

	badUniverse := openTestDatabase(t)
	if _, err := badUniverse.Exec("CREATE TABLE u1_uni (num INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapUniverse(ctx, badUniverse, BootstrapOptions{Prefix: "u1_"}); err == nil {
		t.Fatal("expected incompatible universe seed error")
	}

	for _, table := range []string{"u1_users", "u1_planets", "u1_exptab", "u1_coltab"} {
		db := openTestDatabase(t)
		options := BootstrapOptions{Prefix: "u1_", Secret: "secret"}
		if err := BootstrapUniverse(ctx, db, options); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := seedUniverse(ctx, tx, options); err == nil {
			_ = tx.Rollback()
			t.Fatalf("expected seed error after dropping %s", table)
		}
		_ = tx.Rollback()
	}

	withoutSequence := openTestDatabase(t)
	tx, err := withoutSequence.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedSequences(ctx, tx, "u1_"); err == nil {
		_ = tx.Rollback()
		t.Fatal("expected missing sqlite_sequence error")
	}
	_ = tx.Rollback()

	for _, test := range []struct {
		name    string
		trigger string
	}{
		{name: "second user", trigger: "CREATE TRIGGER fail_legor BEFORE INSERT ON u1_users WHEN NEW.player_id = 1 BEGIN SELECT RAISE(FAIL, 'legor seed failed'); END"},
		{name: "second planet", trigger: "CREATE TRIGGER fail_moon BEFORE INSERT ON u1_planets WHEN NEW.planet_id = 2 BEGIN SELECT RAISE(FAIL, 'moon seed failed'); END"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openTestDatabase(t)
			options := BootstrapOptions{Prefix: "u1_", Secret: "secret"}
			if err := BootstrapUniverse(ctx, db, options); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(test.trigger); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := seedUniverse(ctx, tx, options); err == nil {
				_ = tx.Rollback()
				t.Fatal("expected targeted seed trigger error")
			}
			_ = tx.Rollback()
		})
	}
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

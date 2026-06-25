package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestAdminRepositoryLoadsDatabaseBackupsFromLegacyTemp(t *testing.T) {
	root := t.TempDir()
	tempDir := filepath.Join(root, "temp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	files := map[string]string{
		"backup_20262026_181235.json": "{}",
		"backup_a.json":               "{}",
		"backup_invalid.txt":          "{}",
		"engine.md5":                  "ignored",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(tempDir, "backup_dir.json"), 0o755); err != nil {
		t.Fatalf("create ignored dir: %v", err)
	}

	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(root)
	backups, err := repository.loadAdminDatabaseBackups(context.Background())

	if err != nil {
		t.Fatalf("loadAdminDatabaseBackups returned error: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected two valid backups, got %+v", backups)
	}
	if backups[0].FileName != "backup_20262026_181235.json" || backups[1].FileName != "backup_a.json" {
		t.Fatalf("unexpected backup order: %+v", backups)
	}
}

func TestAdminRepositoryDatabaseBackupErrors(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(t.TempDir())

	if _, err := repository.loadAdminDatabaseBackups(context.Background()); err == nil {
		t.Fatal("expected missing temp dir error")
	}
}

func TestAdminRepositoryDeletesDatabaseBackupsWithSafeNamesOnly(t *testing.T) {
	root := t.TempDir()
	tempDir := filepath.Join(root, "temp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	backupPath := filepath.Join(tempDir, "backup_safe.json")
	if err := os.WriteFile(backupPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	guardPath := filepath.Join(tempDir, "backup_guard.json")
	if err := os.WriteFile(guardPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write guard: %v", err)
	}

	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(root)
	name, err := repository.deleteAdminDatabaseBackup("backup_safe.json")

	if err != nil {
		t.Fatalf("deleteAdminDatabaseBackup returned error: %v", err)
	}
	if name != "temp/backup_safe.json" {
		t.Fatalf("unexpected display name: %s", name)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected backup file to be removed, stat err=%v", err)
	}
	if _, err := repository.deleteAdminDatabaseBackup("../backup_guard.json"); err == nil {
		t.Fatal("expected traversal file name to be rejected")
	}
	if _, err := os.Stat(guardPath); err != nil {
		t.Fatalf("guard backup should remain: %v", err)
	}
}

func TestAdminRepositoryRestoresDatabaseBackupTables(t *testing.T) {
	auto := int64(9)
	runner := &fakeAdminDBRunner{}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	err := repository.deserializeAdminDatabaseBackup(context.Background(), map[string]adminDatabaseBackupTable{
		"users": {
			AutoIncrement: &auto,
			Cols:          []string{"player_id", "oname"},
			Values:        [][]any{{"1", "legor"}},
		},
	})

	if err != nil {
		t.Fatalf("deserializeAdminDatabaseBackup returned error: %v", err)
	}
	joined := strings.Join(runner.execs, "\n")
	for _, want := range []string{
		"SET FOREIGN_KEY_CHECKS=0",
		"TRUNCATE TABLE `ogame_users`",
		"INSERT INTO `ogame_users` (`player_id`, `oname`) VALUES (?, ?)",
		"ALTER TABLE `ogame_users` AUTO_INCREMENT = 9",
		"SET FOREIGN_KEY_CHECKS=1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing restore SQL %q in:\n%s", want, joined)
		}
	}
	if len(runner.args) != 1 || len(runner.args[0]) != 2 || runner.args[0][0] != "1" || runner.args[0][1] != "legor" {
		t.Fatalf("unexpected insert args: %+v", runner.args)
	}
}

func TestAdminRepositoryMutatesDatabaseBackupCreateRestoreDelete(t *testing.T) {
	root := t.TempDir()
	runner := &fakeAdminDBRunner{
		fakeQueryer: fakeQueryer{results: adminDBCreateResults()},
	}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_").WithLegacyGameDir(root)
	repository.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

	createIssue, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{
		Action: domaingame.AdminActionDatabaseCreate,
	})

	if err != nil {
		t.Fatalf("create mutate returned error: %v", err)
	}
	if createIssue.Code != domaingame.AdminIssueActionSaved || !strings.Contains(createIssue.Message, "temp/backup_02012026_030405.json") {
		t.Fatalf("unexpected create issue: %+v", createIssue)
	}
	backupPath := filepath.Join(root, "temp", "backup_02012026_030405.json")
	body, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read created backup: %v", err)
	}
	if !strings.Contains(string(body), `"users"`) || !strings.Contains(string(body), `"legor"`) {
		t.Fatalf("backup body does not include serialized table: %s", string(body))
	}

	restoreIssue, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{
		Action:   domaingame.AdminActionDatabaseRestore,
		FileName: "backup_02012026_030405.json",
	})

	if err != nil {
		t.Fatalf("restore mutate returned error: %v", err)
	}
	if restoreIssue.Code != domaingame.AdminIssueActionSaved || !strings.Contains(restoreIssue.Message, "Backup restored from file") {
		t.Fatalf("unexpected restore issue: %+v", restoreIssue)
	}

	deleteIssue, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{
		Action:   domaingame.AdminActionDatabaseDelete,
		FileName: "backup_02012026_030405.json",
	})

	if err != nil {
		t.Fatalf("delete mutate returned error: %v", err)
	}
	if deleteIssue.Code != domaingame.AdminIssueActionSaved || !strings.Contains(deleteIssue.Message, "Backup deleted") {
		t.Fatalf("unexpected delete issue: %+v", deleteIssue)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected backup file to be deleted, stat err=%v", err)
	}
}

func TestAdminRepositoryDatabaseBackupGuardBranches(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeAdminDBRunner{}, "ogame_").WithLegacyGameDir(t.TempDir())

	if issue, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{Action: "noop"}); err != nil || issue.Code != domaingame.AdminIssueActionSaved {
		t.Fatalf("default mutation should be a saved no-op, issue=%+v err=%v", issue, err)
	}
	if _, err := repository.restoreAdminDatabaseBackup(context.Background(), "../backup_bad.json"); err == nil {
		t.Fatal("expected unsafe restore filename to fail")
	}
	if _, err := repository.restoreAdminDatabaseBackup(context.Background(), "backup_missing.json"); err == nil {
		t.Fatal("expected missing restore file to fail")
	}

	badJSON := filepath.Join(t.TempDir(), "temp")
	if err := os.MkdirAll(badJSON, 0o755); err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badJSON, "backup_bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	repository = repository.WithLegacyGameDir(filepath.Dir(badJSON))
	if _, err := repository.restoreAdminDatabaseBackup(context.Background(), "backup_bad.json"); err == nil {
		t.Fatal("expected invalid backup JSON to fail")
	}
	if err := os.WriteFile(filepath.Join(badJSON, "backup_empty.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write empty json: %v", err)
	}
	if _, err := repository.restoreAdminDatabaseBackup(context.Background(), "backup_empty.json"); err == nil {
		t.Fatal("expected empty backup JSON to fail")
	}

	for _, table := range []adminDatabaseBackupTable{
		{},
		{Cols: []string{"bad-name"}},
		{Cols: []string{"id"}, Values: [][]any{{"1", "extra"}}},
	} {
		if err := validateAdminDatabaseBackupTable(table); err == nil {
			t.Fatalf("expected invalid table to fail: %+v", table)
		}
	}
	if _, err := quoteAdminDatabaseIdentifier("bad-name"); err == nil {
		t.Fatal("expected unsafe identifier to fail")
	}
	if err := repository.insertAdminDatabaseBackupRows(context.Background(), "`ogame_empty`", adminDatabaseBackupTable{Cols: []string{"id"}}); err != nil {
		t.Fatalf("empty insert should be a no-op: %v", err)
	}
}

func TestAdminRepositoryDatabaseBackupQueryErrorBranches(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: sql.ErrConnDone}}}, "ogame_")
	if _, err := repository.serializeAdminDatabaseBackup(context.Background()); err == nil {
		t.Fatal("expected table list query error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7})}}}, "ogame_")
	if _, err := repository.loadAdminBackupTableNames(context.Background()); err == nil {
		t.Fatal("expected table list scan error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseColumns(context.Background(), "`ogame_empty`"); err == nil {
		t.Fatal("expected empty column list error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad-name", "int", "NO", "", nil, ""})}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseColumns(context.Background(), "`ogame_bad`"); err == nil {
		t.Fatal("expected unsafe column error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	autoIncrement, err := repository.loadAdminDatabaseAutoIncrement(context.Background(), "ogame_users")
	if err != nil || autoIncrement != nil {
		t.Fatalf("missing autoincrement row should be nil without error, auto=%v err=%v", autoIncrement, err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"only-one"})}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseValues(context.Background(), "`ogame_users`", []string{"id", "name"}); err == nil {
		t.Fatal("expected value scan count error")
	}

	rootFile := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(rootFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	repository = NewAdminRepositoryWithQueryer(&fakeAdminDBRunner{fakeQueryer: fakeQueryer{results: adminDBCreateResults()}}, "ogame_").WithLegacyGameDir(rootFile)
	if _, err := repository.createAdminDatabaseBackup(context.Background()); err == nil {
		t.Fatal("expected create backup mkdir failure")
	}
}

func TestAdminRepositoryDatabaseBackupMutationErrorBranches(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeAdminDBRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: sql.ErrConnDone}}}}, "ogame_")
	if _, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{Action: domaingame.AdminActionDatabaseCreate}); err == nil {
		t.Fatal("expected create mutation error")
	}
	if _, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{Action: domaingame.AdminActionDatabaseDelete, FileName: "backup_missing.json"}); err == nil {
		t.Fatal("expected delete mutation error")
	}
	if _, err := repository.mutateAdminDatabase(context.Background(), appgame.AdminMutationQuery{Action: domaingame.AdminActionDatabaseRestore, FileName: "backup_missing.json"}); err == nil {
		t.Fatal("expected restore mutation error")
	}

	for _, execErrAt := range []int{1, 2, 3, 4} {
		auto := int64(4)
		runner := &fakeAdminDBRunner{execErrAt: execErrAt, execErr: errors.New("exec failed")}
		repository = NewAdminRepositoryWithQueryer(runner, "ogame_")
		err := repository.deserializeAdminDatabaseBackup(context.Background(), map[string]adminDatabaseBackupTable{
			"users": {
				AutoIncrement: &auto,
				Cols:          []string{"player_id"},
				Values:        [][]any{{"1"}},
			},
		})
		if err == nil {
			t.Fatalf("expected deserialize exec error at call %d", execErrAt)
		}
	}
}

func TestAdminRepositorySerializeTableErrorBranches(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: sql.ErrConnDone}}}, "ogame_")
	if _, err := repository.serializeAdminDatabaseTable(context.Background(), "ogame_users"); err == nil {
		t.Fatal("expected column query error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"player_id", "int", "NO", "PRI", nil, "auto_increment"})},
		{err: sql.ErrConnDone},
	}}, "ogame_")
	if _, err := repository.serializeAdminDatabaseTable(context.Background(), "ogame_users"); err == nil {
		t.Fatal("expected autoincrement query error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"player_id", "int", "NO", "PRI", nil, "auto_increment"})},
		{rows: fakeRowsFromValues([]any{int64(3)})},
		{err: sql.ErrConnDone},
	}}, "ogame_")
	if _, err := repository.serializeAdminDatabaseTable(context.Background(), "ogame_users"); err == nil {
		t.Fatal("expected values query error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"not-int"})}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseAutoIncrement(context.Background(), "ogame_users"); err == nil {
		t.Fatal("expected autoincrement scan error")
	}
}

func TestAdminRepositoryDatabaseBackupRowsErrBranches(t *testing.T) {
	rowsErr := errors.New("rows failed")
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(rowsErr)}}}, "ogame_")
	if _, err := repository.loadAdminBackupTableNames(context.Background()); err == nil {
		t.Fatal("expected table-name rows error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(rowsErr)}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseColumns(context.Background(), "`ogame_users`"); err == nil {
		t.Fatal("expected column rows error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(rowsErr, []any{int64(3)})}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseAutoIncrement(context.Background(), "ogame_users"); err == nil {
		t.Fatal("expected autoincrement rows error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(rowsErr, []any{"1"})}}}, "ogame_")
	if _, err := repository.loadAdminDatabaseValues(context.Background(), "`ogame_users`", []string{"id"}); err == nil {
		t.Fatal("expected value rows error")
	}

	runner := &fakeAdminDBRunner{}
	repository = NewAdminRepositoryWithQueryer(runner, "ogame_")
	if err := repository.deserializeAdminDatabaseBackup(context.Background(), map[string]adminDatabaseBackupTable{
		"users": {Cols: []string{"id"}, Values: [][]any{{"1", "extra"}}},
	}); err == nil {
		t.Fatal("expected deserialize validation error")
	}
}

func TestAdminRepositoryDatabaseBackupPropagatesNestedErrors(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"ogame_users"})},
		{err: sql.ErrConnDone},
	}}, "ogame_")
	if _, err := repository.serializeAdminDatabaseBackup(context.Background()); err == nil {
		t.Fatal("expected serialize table error")
	}

	root := t.TempDir()
	tempDir := filepath.Join(root, "temp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "backup_invalid_table.json"), []byte(`{"users":{"auto_increment":null,"cols":["id"],"values":[["1","extra"]]}}`), 0o644); err != nil {
		t.Fatalf("write invalid restore backup: %v", err)
	}
	repository = NewAdminRepositoryWithQueryer(&fakeAdminDBRunner{}, "ogame_").WithLegacyGameDir(root)
	if _, err := repository.restoreAdminDatabaseBackup(context.Background(), "backup_invalid_table.json"); err == nil {
		t.Fatal("expected restore to propagate deserialize validation error")
	}

	if err := repository.insertAdminDatabaseBackupRows(context.Background(), "`ogame_users`", adminDatabaseBackupTable{
		Cols:   []string{"bad-name"},
		Values: [][]any{{"1"}},
	}); err == nil {
		t.Fatal("expected insert to reject unsafe column")
	}
}

type fakeAdminDBRunner struct {
	fakeQueryer
	execs     []string
	args      [][]any
	execErrAt int
	execErr   error
}

func (f *fakeAdminDBRunner) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	if f.fakeQueryer.results == nil {
		return nil, sql.ErrNoRows
	}
	return f.fakeQueryer.QueryContext(ctx, query, args...)
}

func (f *fakeAdminDBRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, query)
	if len(args) > 0 {
		f.args = append(f.args, args)
	}
	if f.execErr != nil && f.execErrAt == len(f.execs) {
		return nil, f.execErr
	}
	return adminDBSQLResult{}, nil
}

type adminDBSQLResult struct{}

func (adminDBSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (adminDBSQLResult) RowsAffected() (int64, error) { return 0, nil }

func adminDBCreateResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"ogame_users"})},
		{rows: fakeRowsFromValues(
			[]any{"player_id", "int", "NO", "PRI", nil, "auto_increment"},
			[]any{"oname", "varchar(64)", "YES", "", nil, ""},
		)},
		{rows: fakeRowsFromValues([]any{int64(3)})},
		{rows: fakeRowsFromValues([]any{"1", "legor"})},
	}
}

package mysqlgame

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

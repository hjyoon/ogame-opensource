package mysqlgame

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

var adminDatabaseBackupPattern = regexp.MustCompile(`^backup_[A-Za-z0-9_.-]+\.json$`)

func (r AdminRepository) loadAdminDatabaseBackups(_ context.Context) ([]domaingame.AdminDatabaseBackup, error) {
	entries, err := os.ReadDir(filepath.Join(r.legacyGameDir, "temp"))
	if err != nil {
		return nil, err
	}
	backups := make([]domaingame.AdminDatabaseBackup, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !adminDatabaseBackupPattern.MatchString(name) {
			continue
		}
		backups = append(backups, domaingame.AdminDatabaseBackup{FileName: name})
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].FileName < backups[j].FileName
	})
	return backups, nil
}

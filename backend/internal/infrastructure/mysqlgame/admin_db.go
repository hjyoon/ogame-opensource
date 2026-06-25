package mysqlgame

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

var adminDatabaseBackupPattern = regexp.MustCompile(`^backup_[A-Za-z0-9_.-]+\.json$`)

type adminDatabaseBackupTable struct {
	AutoIncrement *int64   `json:"auto_increment"`
	Cols          []string `json:"cols"`
	Values        [][]any  `json:"values"`
}

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

func (r AdminRepository) mutateAdminDatabase(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	switch query.Action {
	case domaingame.AdminActionDatabaseCreate:
		name, err := r.createAdminDatabaseBackup(ctx)
		if err != nil {
			return nil, err
		}
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, fmt.Sprintf("The backup is saved to file %s", name)), nil
	case domaingame.AdminActionDatabaseDelete:
		name, err := r.deleteAdminDatabaseBackup(query.FileName)
		if err != nil {
			return nil, err
		}
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, fmt.Sprintf("Backup deleted %s", name)), nil
	case domaingame.AdminActionDatabaseRestore:
		name, err := r.restoreAdminDatabaseBackup(ctx, query.FileName)
		if err != nil {
			return nil, err
		}
		return domaingame.AdminIssueWithMessage(domaingame.AdminIssueActionSaved, fmt.Sprintf("Backup restored from file %s", name)), nil
	default:
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
}

func (r AdminRepository) createAdminDatabaseBackup(ctx context.Context) (string, error) {
	backup, err := r.serializeAdminDatabaseBackup(ctx)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(backup, "", "    ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(r.legacyGameDir, "temp"), 0o755); err != nil {
		return "", err
	}
	fileName := "backup_" + r.now().Format("02012006_150405") + ".json"
	path := filepath.Join(r.legacyGameDir, "temp", fileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join("temp", fileName)), nil
}

func (r AdminRepository) deleteAdminDatabaseBackup(fileName string) (string, error) {
	path, displayName, ok := r.adminDatabaseBackupPath(fileName)
	if !ok {
		return "", errors.New("admin database backup not found")
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return displayName, nil
}

func (r AdminRepository) restoreAdminDatabaseBackup(ctx context.Context, fileName string) (string, error) {
	path, displayName, ok := r.adminDatabaseBackupPath(fileName)
	if !ok {
		return "", errors.New("admin database backup not found")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var backup map[string]adminDatabaseBackupTable
	if err := json.Unmarshal(data, &backup); err != nil {
		return "", err
	}
	if len(backup) == 0 {
		return "", errors.New("admin database backup is empty")
	}
	if err := r.deserializeAdminDatabaseBackup(ctx, backup); err != nil {
		return "", err
	}
	return displayName, nil
}

func (r AdminRepository) adminDatabaseBackupPath(fileName string) (string, string, bool) {
	if fileName != filepath.Base(fileName) || !adminDatabaseBackupPattern.MatchString(fileName) {
		return "", "", false
	}
	displayName := filepath.ToSlash(filepath.Join("temp", fileName))
	return filepath.Join(r.legacyGameDir, "temp", fileName), displayName, true
}

func (r AdminRepository) serializeAdminDatabaseBackup(ctx context.Context) (map[string]adminDatabaseBackupTable, error) {
	tableNames, err := r.loadAdminBackupTableNames(ctx)
	if err != nil {
		return nil, err
	}
	backup := make(map[string]adminDatabaseBackupTable, len(tableNames))
	for _, physicalName := range tableNames {
		logicalName := strings.TrimPrefix(physicalName, r.prefix)
		table, err := r.serializeAdminDatabaseTable(ctx, physicalName)
		if err != nil {
			return nil, err
		}
		backup[logicalName] = table
	}
	return backup, nil
}

func (r AdminRepository) loadAdminBackupTableNames(ctx context.Context) ([]string, error) {
	rows, err := r.queryer.QueryContext(ctx, "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME LIKE ?", r.prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, r.prefix) {
			names = append(names, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func (r AdminRepository) serializeAdminDatabaseTable(ctx context.Context, physicalName string) (adminDatabaseBackupTable, error) {
	quotedTable, err := quoteAdminDatabaseIdentifier(physicalName)
	if err != nil {
		return adminDatabaseBackupTable{}, err
	}
	columns, err := r.loadAdminDatabaseColumns(ctx, quotedTable)
	if err != nil {
		return adminDatabaseBackupTable{}, err
	}
	autoIncrement, err := r.loadAdminDatabaseAutoIncrement(ctx, physicalName)
	if err != nil {
		return adminDatabaseBackupTable{}, err
	}
	values, err := r.loadAdminDatabaseValues(ctx, quotedTable, columns)
	if err != nil {
		return adminDatabaseBackupTable{}, err
	}
	return adminDatabaseBackupTable{
		AutoIncrement: autoIncrement,
		Cols:          columns,
		Values:        values,
	}, nil
}

func (r AdminRepository) loadAdminDatabaseColumns(ctx context.Context, quotedTable string) ([]string, error) {
	rows, err := r.queryer.QueryContext(ctx, "SHOW COLUMNS FROM "+quotedTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var field, columnType, nullable, key, extra string
		var defaultValue sql.NullString
		if err := rows.Scan(&field, &columnType, &nullable, &key, &defaultValue, &extra); err != nil {
			return nil, err
		}
		if _, err := quoteAdminDatabaseIdentifier(field); err != nil {
			return nil, err
		}
		columns = append(columns, field)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, errors.New("admin database table has no columns")
	}
	return columns, nil
}

func (r AdminRepository) loadAdminDatabaseAutoIncrement(ctx context.Context, physicalName string) (*int64, error) {
	rows, err := r.queryer.QueryContext(ctx, "SELECT AUTO_INCREMENT FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", physicalName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var auto sql.NullInt64
	if err := rows.Scan(&auto); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !auto.Valid {
		return nil, nil
	}
	return &auto.Int64, nil
}

func (r AdminRepository) loadAdminDatabaseValues(ctx context.Context, quotedTable string, columns []string) ([][]any, error) {
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumn, err := quoteAdminDatabaseIdentifier(column)
		if err != nil {
			return nil, err
		}
		quotedColumns = append(quotedColumns, quotedColumn)
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedColumns, ", "), quotedTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([][]any, 0)
	for rows.Next() {
		scanned := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range scanned {
			dest[i] = &scanned[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := make([]any, len(columns))
		for i, value := range scanned {
			if value.Valid {
				row[i] = value.String
			}
		}
		values = append(values, row)
	}
	return values, rows.Err()
}

func (r AdminRepository) deserializeAdminDatabaseBackup(ctx context.Context, backup map[string]adminDatabaseBackupTable) error {
	if _, err := r.execer.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}
	defer r.execer.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")

	names := make([]string, 0, len(backup))
	for name := range backup {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		table := backup[name]
		physicalTable, err := tableName(r.prefix, name)
		if err != nil {
			return err
		}
		if err := validateAdminDatabaseBackupTable(table); err != nil {
			return err
		}
		if _, err := r.execer.ExecContext(ctx, "TRUNCATE TABLE "+physicalTable); err != nil {
			return err
		}
		if err := r.insertAdminDatabaseBackupRows(ctx, physicalTable, table); err != nil {
			return err
		}
		if table.AutoIncrement != nil && *table.AutoIncrement > 0 {
			if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", physicalTable, *table.AutoIncrement)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAdminDatabaseBackupTable(table adminDatabaseBackupTable) error {
	if len(table.Cols) == 0 {
		return errors.New("admin database backup table has no columns")
	}
	for _, column := range table.Cols {
		if _, err := quoteAdminDatabaseIdentifier(column); err != nil {
			return err
		}
	}
	for _, row := range table.Values {
		if len(row) != len(table.Cols) {
			return errors.New("admin database backup row has invalid column count")
		}
	}
	return nil
}

func (r AdminRepository) insertAdminDatabaseBackupRows(ctx context.Context, physicalTable string, table adminDatabaseBackupTable) error {
	if len(table.Values) == 0 {
		return nil
	}
	quotedColumns := make([]string, 0, len(table.Cols))
	for _, column := range table.Cols {
		quotedColumn, err := quoteAdminDatabaseIdentifier(column)
		if err != nil {
			return err
		}
		quotedColumns = append(quotedColumns, quotedColumn)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", physicalTable, strings.Join(quotedColumns, ", "), placeholders(len(table.Cols)))
	for _, row := range table.Values {
		if _, err := r.execer.ExecContext(ctx, query, row...); err != nil {
			return err
		}
	}
	return nil
}

func quoteAdminDatabaseIdentifier(identifier string) (string, error) {
	if !identifierPattern.MatchString(identifier) {
		return "", errors.New("invalid database identifier")
	}
	return "`" + identifier + "`", nil
}

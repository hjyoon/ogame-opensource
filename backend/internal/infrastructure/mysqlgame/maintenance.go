package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type MaintenanceRepository struct {
	queryer Queryer
	prefix  string
}

func NewMaintenanceRepository(db *sql.DB, prefix string) MaintenanceRepository {
	return MaintenanceRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewMaintenanceRepositoryWithQueryer(queryer Queryer, prefix string) MaintenanceRepository {
	return MaintenanceRepository{queryer: queryer, prefix: prefix}
}

func (r MaintenanceRepository) GetMaintenance(ctx context.Context) (domaingame.Maintenance, error) {
	if r.queryer == nil {
		return domaingame.Maintenance{}, errors.New("maintenance reader unavailable")
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.Maintenance{}, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(freeze, 0), COALESCE(lang, ''), COALESCE(ext_board, '') FROM %s LIMIT 1", uniTable))
	if err != nil {
		return domaingame.Maintenance{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domaingame.Maintenance{Language: "en"}, rows.Err()
	}
	var freeze int
	var language string
	var boardURL string
	if err := rows.Scan(&freeze, &language, &boardURL); err != nil {
		return domaingame.Maintenance{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.Maintenance{}, err
	}
	return domaingame.Maintenance{
		Frozen:   freeze != 0,
		Language: domaingame.NormalizeMaintenanceLanguage(language),
		BoardURL: boardURL,
	}, nil
}

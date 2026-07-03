package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type PrangerRepository struct {
	queryer Queryer
	prefix  string
}

func NewPrangerRepository(db *sql.DB, prefix string) PrangerRepository {
	return PrangerRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewPrangerRepositoryWithQueryer(queryer Queryer, prefix string) PrangerRepository {
	return PrangerRepository{queryer: queryer, prefix: prefix}
}

func (r PrangerRepository) GetPranger(ctx context.Context, query appgame.PrangerQuery) (domaingame.Pranger, error) {
	if r.queryer == nil {
		return domaingame.Pranger{}, errors.New("pranger reader unavailable")
	}
	prangerTable, err := tableName(r.prefix, "pranger")
	if err != nil {
		return domaingame.Pranger{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT ban_when, COALESCE(admin_name, ''), COALESCE(user_name, ''), ban_until, COALESCE(reason, '') FROM %s ORDER BY ban_when DESC LIMIT ? OFFSET ?", prangerTable),
		domaingame.PrangerPageLimit,
		query.From,
	)
	if err != nil {
		return domaingame.Pranger{}, err
	}
	defer rows.Close()

	result := domaingame.Pranger{
		Universe: query.Universe,
		From:     query.From,
		Internal: query.Internal,
		Entries:  make([]domaingame.PrangerEntry, 0, domaingame.PrangerPageLimit),
	}
	for rows.Next() {
		var entry domaingame.PrangerEntry
		if err := rows.Scan(&entry.BanWhen, &entry.AdminName, &entry.UserName, &entry.BanUntil, &entry.Reason); err != nil {
			return domaingame.Pranger{}, err
		}
		result.Entries = append(result.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return domaingame.Pranger{}, err
	}
	return result, nil
}

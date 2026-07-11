package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
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

func (r PrangerRepository) GetMCPPranger(ctx context.Context, playerID int, command domainmcp.PrangerCommand) (domainmcp.Pranger, error) {
	pranger, err := r.GetPranger(ctx, appgame.PrangerQuery{
		Universe: command.Universe,
		From:     domaingame.NormalizePrangerFrom(command.From),
		Internal: true,
	})
	if err != nil {
		return domainmcp.Pranger{}, err
	}
	return mcpPranger(playerID, pranger), nil
}

func mcpPranger(playerID int, pranger domaingame.Pranger) domainmcp.Pranger {
	return domainmcp.Pranger{
		PlayerID:    playerID,
		Universe:    pranger.Universe,
		From:        pranger.From,
		Limit:       domaingame.PrangerPageLimit,
		HasPrevious: pranger.HasPrevious(),
		Previous:    pranger.PreviousFrom(),
		HasNext:     pranger.HasNext(),
		Next:        pranger.NextFrom(),
		Entries:     mcpPrangerEntries(pranger.Entries),
	}
}

func mcpPrangerEntries(entries []domaingame.PrangerEntry) []domainmcp.PrangerEntry {
	result := make([]domainmcp.PrangerEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, domainmcp.PrangerEntry{
			BanWhen:   entry.BanWhen,
			AdminName: entry.AdminName,
			UserName:  entry.UserName,
			BanUntil:  entry.BanUntil,
			Reason:    entry.Reason,
		})
	}
	return result
}

package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type AdminRepository struct {
	queryer  Queryer
	overview OverviewRepository
	prefix   string
}

func NewAdminRepository(db *sql.DB, prefix string) AdminRepository {
	runner := SQLQueryer{DB: db}
	return AdminRepository{
		queryer:  runner,
		overview: NewOverviewRepository(db, prefix),
		prefix:   prefix,
	}
}

func NewAdminRepositoryWithQueryer(queryer Queryer, prefix string) AdminRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return AdminRepository{
		queryer:  queryer,
		overview: NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:   prefix,
	}
}

func (r AdminRepository) GetAdmin(ctx context.Context, query appgame.AdminQuery) (domaingame.Admin, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Admin{}, err
	}
	viewer, err := r.loadAdminViewer(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Admin{}, err
	}
	return domaingame.NewAdmin(overview, viewer, query.Mode), nil
}

func (r AdminRepository) loadAdminViewer(ctx context.Context, playerID int) (domaingame.AdminViewer, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.AdminViewer{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(oname, ''), COALESCE(admin, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.AdminViewer{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.AdminViewer{}, err
		}
		return domaingame.AdminViewer{}, errors.New("admin viewer not found")
	}
	var viewer domaingame.AdminViewer
	if err := rows.Scan(&viewer.PlayerID, &viewer.Name, &viewer.Level); err != nil {
		return domaingame.AdminViewer{}, err
	}
	return viewer, rows.Err()
}

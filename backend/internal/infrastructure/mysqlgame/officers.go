package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type OfficersRepository struct {
	queryer  Queryer
	execer   Execer
	overview OverviewRepository
	prefix   string
	now      func() time.Time
}

func NewOfficersRepository(db *sql.DB, prefix string) OfficersRepository {
	runner := SQLQueryer{DB: db}
	return OfficersRepository{
		queryer:  runner,
		execer:   runner,
		overview: NewOverviewRepository(db, prefix),
		prefix:   prefix,
		now:      time.Now,
	}
}

func NewOfficersRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) OfficersRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewOfficersRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewOfficersRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) OfficersRepository {
	if now == nil {
		now = time.Now
	}
	return OfficersRepository{
		queryer:  queryer,
		execer:   execer,
		overview: NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:   prefix,
		now:      now,
	}
}

func (r OfficersRepository) GetOfficers(ctx context.Context, query appgame.OfficersQuery) (domaingame.Officers, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Officers{}, err
	}
	user, timers, err := r.loadOfficersUser(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Officers{}, err
	}
	return domaingame.NewOfficers(overview, user, timers, r.now()), nil
}

func (r OfficersRepository) RecruitOfficer(ctx context.Context, query appgame.OfficersMutationQuery) (domaingame.Officers, *domaingame.OfficerActionIssue, error) {
	if r.execer == nil {
		return domaingame.Officers{}, nil, errors.New("officers updater unavailable")
	}
	current, err := r.GetOfficers(ctx, appgame.OfficersQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Officers{}, nil, err
	}
	currentTimers := timersFromRows(current.Rows)
	recruitment, issue := domaingame.ResolveOfficerRecruitment(current.User, currentTimers, query.Mutation, r.now())
	if issue != nil && !recruitment.Changed {
		return current, issue, nil
	}
	if !recruitment.Changed {
		return current, nil, nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Officers{}, nil, err
	}
	timerColumn, ok := officerTimerColumn(query.Mutation.OfficerID)
	if !ok {
		return current, nil, nil
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET dm = ?, dmfree = ?, %s = ? WHERE player_id = ? LIMIT 1", usersTable, timerColumn),
		recruitment.User.PaidDarkMatter,
		recruitment.User.FreeDarkMatter,
		domaingame.OfficerUntil(recruitment.Timers, query.Mutation.OfficerID),
		query.PlayerID,
	); err != nil {
		return domaingame.Officers{}, nil, err
	}
	updated, err := r.GetOfficers(ctx, appgame.OfficersQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Officers{}, nil, err
	}
	return updated, issue, nil
}

func (r OfficersRepository) loadOfficersUser(ctx context.Context, playerID int) (domaingame.OfficersUser, domaingame.OfficerTimers, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(dm, 0), COALESCE(dmfree, 0), COALESCE(com_until, 0), COALESCE(adm_until, 0), COALESCE(eng_until, 0), COALESCE(geo_until, 0), COALESCE(tec_until, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, err
		}
		return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, errors.New("officers user not found")
	}
	var paidDarkMatter int
	var freeDarkMatter int
	var timers domaingame.OfficerTimers
	if err := rows.Scan(
		&paidDarkMatter,
		&freeDarkMatter,
		&timers.Commander,
		&timers.Admiral,
		&timers.Engineer,
		&timers.Geologist,
		&timers.Technocrat,
	); err != nil {
		return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.OfficersUser{}, domaingame.OfficerTimers{}, err
	}
	return domaingame.OfficersUser{PaidDarkMatter: paidDarkMatter, FreeDarkMatter: freeDarkMatter}, timers, nil
}

func officerTimerColumn(officerID int) (string, bool) {
	switch officerID {
	case domaingame.OfficerCommander:
		return "com_until", true
	case domaingame.OfficerAdmiral:
		return "adm_until", true
	case domaingame.OfficerEngineer:
		return "eng_until", true
	case domaingame.OfficerGeologist:
		return "geo_until", true
	case domaingame.OfficerTechnocrat:
		return "tec_until", true
	default:
		return "", false
	}
}

func timersFromRows(rows []domaingame.OfficerRow) domaingame.OfficerTimers {
	var timers domaingame.OfficerTimers
	for _, row := range rows {
		timers = domaingame.SetOfficerUntil(timers, row.ID, row.Until)
	}
	return timers
}

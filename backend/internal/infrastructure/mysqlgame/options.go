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

type OptionsRepository struct {
	queryer  Queryer
	execer   Execer
	overview OverviewRepository
	prefix   string
	now      func() time.Time
}

func NewOptionsRepository(db *sql.DB, prefix string) OptionsRepository {
	runner := SQLQueryer{DB: db}
	return OptionsRepository{
		queryer:  runner,
		execer:   runner,
		overview: NewOverviewRepository(db, prefix),
		prefix:   prefix,
		now:      time.Now,
	}
}

func NewOptionsRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) OptionsRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewOptionsRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewOptionsRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) OptionsRepository {
	if now == nil {
		now = time.Now
	}
	return OptionsRepository{
		queryer:  queryer,
		execer:   execer,
		overview: NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:   prefix,
		now:      now,
	}
}

func (r OptionsRepository) GetOptions(ctx context.Context, query appgame.OptionsQuery) (domaingame.Options, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Options{}, err
	}
	user, universe, settings, account, flags, err := r.loadOptions(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Options{}, err
	}
	return domaingame.NewOptions(overview, user, universe, settings, account, flags), nil
}

func (r OptionsRepository) UpdateOptions(ctx context.Context, query appgame.OptionsUpdateQuery) (domaingame.Options, *domaingame.OptionsActionIssue, error) {
	if r.execer == nil {
		return domaingame.Options{}, nil, errors.New("options updater unavailable")
	}
	current, err := r.GetOptions(ctx, appgame.OptionsQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Options{}, nil, err
	}
	normalized := domaingame.NormalizeOptionsMutation(query.Mutation, current)
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Options{}, nil, err
	}

	disable := 0
	disableUntil := int64(0)
	issue := domaingame.OptionsSavedIssue()
	if normalized.DeleteAccount {
		disable = 1
		if current.Account.DeletionQueued && current.Account.DeletionAt > 0 {
			disableUntil = current.Account.DeletionAt
		} else {
			disableUntil = r.now().Add(7 * 24 * time.Hour).Unix()
		}
		if normalized.AccountDeletionChanged {
			issue = domaingame.OptionsAccountDeletionQueuedIssue(time.Unix(disableUntil, 0))
		}
	} else if normalized.AccountDeletionChanged {
		issue = domaingame.OptionsAccountDeletionClearedIssue()
	}

	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET skin = ?, useskin = ?, deact_ip = ?, sortby = ?, sortorder = ?, maxspy = ?, maxfleetmsg = ?, lang = ?, disable = ?, disable_until = ? WHERE player_id = ? LIMIT 1", usersTable),
		normalized.SkinPath,
		boolInt(normalized.UseSkin),
		boolInt(normalized.DeactivateIP),
		normalized.SortBy,
		normalized.SortOrder,
		normalized.MaxSpy,
		normalized.MaxFleetMessages,
		normalized.Language,
		disable,
		disableUntil,
		query.PlayerID,
	); err != nil {
		return domaingame.Options{}, nil, err
	}

	updated, err := r.GetOptions(ctx, appgame.OptionsQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Options{}, nil, err
	}
	return updated, issue, nil
}

func (r OptionsRepository) loadOptions(ctx context.Context, playerID int) (domaingame.OptionsUser, domaingame.OptionsUniverse, domaingame.OptionsSettings, domaingame.OptionsAccount, int64, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}

	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT oname, COALESCE(name_changed, 0), COALESCE(email, ''), COALESCE(pemail, ''), COALESCE(validated, 0), COALESCE(lang, ''), COALESCE(skin, ''), COALESCE(useskin, 0), COALESCE(deact_ip, 0), COALESCE(sortby, 0), COALESCE(sortorder, 0), COALESCE(maxspy, 1), COALESCE(maxfleetmsg, 3), COALESCE(flags, 0), COALESCE(admin, 0), COALESCE(vacation, 0), COALESCE(vacation_until, 0), COALESCE(disable, 0), COALESCE(disable_until, 0), COALESCE(com_until, 0), COALESCE(feedid, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
		}
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, errors.New("options user not found")
	}

	var name string
	var nameChanged int
	var email string
	var plainEmail string
	var validated int
	var language string
	var skin string
	var useSkin int
	var deactIP int
	var sortBy int
	var sortOrder int
	var maxSpy int
	var maxFleetMessages int
	var flags int64
	var admin int
	var vacation int
	var vacationUntil int64
	var disable int
	var disableUntil int64
	var commanderUntil int64
	var feedID string
	if err := rows.Scan(
		&name,
		&nameChanged,
		&email,
		&plainEmail,
		&validated,
		&language,
		&skin,
		&useSkin,
		&deactIP,
		&sortBy,
		&sortOrder,
		&maxSpy,
		&maxFleetMessages,
		&flags,
		&admin,
		&vacation,
		&vacationUntil,
		&disable,
		&disableUntil,
		&commanderUntil,
		&feedID,
	); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}

	uniRows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(lang, ''), COALESCE(force_lang, 0), COALESCE(feedage, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	defer uniRows.Close()
	if !uniRows.Next() {
		if err := uniRows.Err(); err != nil {
			return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
		}
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, errors.New("options universe not found")
	}
	var universeLanguage string
	var forceLanguage int
	var feedAge int
	if err := uniRows.Scan(&universeLanguage, &forceLanguage, &feedAge); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	if err := uniRows.Err(); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}

	return domaingame.OptionsUser{
			Name:        name,
			NameLocked:  nameChanged != 0,
			Email:       email,
			PlainEmail:  plainEmail,
			Validated:   validated != 0,
			Admin:       admin,
			FeedID:      feedID,
			CommanderOn: commanderUntil > r.now().Unix(),
		},
		domaingame.OptionsUniverse{
			Language:      universeLanguage,
			ForceLanguage: forceLanguage != 0,
			FeedAge:       feedAge,
		},
		domaingame.OptionsSettings{
			Language:         language,
			SkinPath:         skin,
			UseSkin:          useSkin != 0,
			DeactivateIP:     deactIP != 0,
			SortBy:           sortBy,
			SortOrder:        sortOrder,
			MaxSpy:           maxSpy,
			MaxFleetMessages: maxFleetMessages,
		},
		domaingame.OptionsAccount{
			Vacation:       vacation != 0,
			VacationUntil:  vacationUntil,
			DeletionQueued: disable != 0,
			DeletionAt:     disableUntil,
		},
		flags,
		nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

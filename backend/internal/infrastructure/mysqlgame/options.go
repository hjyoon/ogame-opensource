package mysqlgame

import (
	"context"
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type OptionsRepository struct {
	queryer  Queryer
	execer   Execer
	overview OverviewRepository
	prefix   string
	secret   string
	now      func() time.Time
}

func NewOptionsRepository(db *sql.DB, prefix string) OptionsRepository {
	return NewOptionsRepositoryWithSecret(db, prefix, "")
}

func NewOptionsRepositoryWithSecret(db *sql.DB, prefix string, secret string) OptionsRepository {
	runner := SQLQueryer{DB: db}
	return OptionsRepository{
		queryer:  runner,
		execer:   runner,
		overview: NewOverviewRepository(db, prefix),
		prefix:   prefix,
		secret:   secret,
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
	return NewOptionsRepositoryWithRunnerAndSecret(queryer, execer, prefix, "", now)
}

func NewOptionsRepositoryWithRunnerAndSecret(queryer Queryer, execer Execer, prefix string, secret string, now func() time.Time) OptionsRepository {
	if now == nil {
		now = time.Now
	}
	return OptionsRepository{
		queryer:  queryer,
		execer:   execer,
		overview: NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:   prefix,
		secret:   secret,
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
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Options{}, nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Options{}, nil, err
	}

	disable := 0
	disableUntil := int64(0)
	vacation := boolInt(current.Account.Vacation)
	vacationUntil := current.Account.VacationUntil
	issue := domaingame.OptionsSavedIssue()
	if currentIssue, err := r.applyCredentialMutations(ctx, usersTable, queueTable, query.PlayerID, normalized.OptionsMutation, current); err != nil {
		return domaingame.Options{}, nil, err
	} else if currentIssue != nil {
		issue = currentIssue
	}
	if normalized.VacationChanged {
		switch {
		case normalized.VacationMode:
			allowed, err := r.canEnableVacation(ctx, queueTable, query.PlayerID)
			if err != nil {
				return domaingame.Options{}, nil, err
			}
			if allowed {
				vacation = 1
				vacationUntil = r.now().Unix() + vacationMinimumSeconds(current.Universe.Speed)
				issue = domaingame.OptionsVacationEnabledIssue(time.Unix(vacationUntil, 0))
			} else {
				normalized.VacationMode = current.Account.Vacation
				issue = domaingame.OptionsVacationBlockedIssue()
			}
		case r.now().Unix() >= current.Account.VacationUntil:
			vacation = 0
			vacationUntil = 0
			issue = domaingame.OptionsVacationDisabledIssue(current.User.Name)
		default:
			normalized.VacationMode = current.Account.Vacation
			issue = domaingame.OptionsVacationLockedIssue(time.Unix(current.Account.VacationUntil, 0))
		}
	}
	if normalized.DeleteAccount {
		disable = 1
		if current.Account.DeletionQueued && current.Account.DeletionAt > 0 {
			disableUntil = current.Account.DeletionAt
		} else {
			disableUntil = r.now().Add(7 * 24 * time.Hour).Unix()
		}
		if normalized.AccountDeletionChanged && !normalized.VacationChanged {
			issue = domaingame.OptionsAccountDeletionQueuedIssue(time.Unix(disableUntil, 0))
		}
	} else if normalized.AccountDeletionChanged && !normalized.VacationChanged {
		issue = domaingame.OptionsAccountDeletionClearedIssue()
	}

	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET skin = ?, useskin = ?, deact_ip = ?, sortby = ?, sortorder = ?, maxspy = ?, maxfleetmsg = ?, lang = ?, vacation = ?, vacation_until = ?, disable = ?, disable_until = ? WHERE player_id = ? LIMIT 1", usersTable),
		normalized.SkinPath,
		boolInt(normalized.UseSkin),
		boolInt(normalized.DeactivateIP),
		normalized.SortBy,
		normalized.SortOrder,
		normalized.MaxSpy,
		normalized.MaxFleetMessages,
		normalized.Language,
		vacation,
		vacationUntil,
		disable,
		disableUntil,
		query.PlayerID,
	); err != nil {
		return domaingame.Options{}, nil, err
	}
	if normalized.VacationChanged && normalized.VacationMode && vacation == 1 {
		if err := r.disableProductionForVacation(ctx, planetsTable, query.PlayerID); err != nil {
			return domaingame.Options{}, nil, err
		}
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
		fmt.Sprintf("SELECT oname, COALESCE(name_changed, 0), COALESCE(email, ''), COALESCE(pemail, ''), COALESCE(validated, 0), COALESCE(lang, ''), COALESCE(skin, ''), COALESCE(useskin, 0), COALESCE(deact_ip, 0), COALESCE(sortby, 0), COALESCE(sortorder, 0), COALESCE(maxspy, 1), COALESCE(maxfleetmsg, 3), COALESCE(flags, 0), COALESCE(admin, 0), COALESCE(vacation, 0), COALESCE(vacation_until, 0), COALESCE(disable, 0), COALESCE(disable_until, 0), COALESCE(com_until, 0), COALESCE(feedid, ''), COALESCE(password, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable),
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
	var passwordHash string
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
		&passwordHash,
	); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}

	uniRows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(lang, ''), COALESCE(force_lang, 0), COALESCE(feedage, 0), COALESCE(speed, 1) FROM %s LIMIT 1", uniTable))
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
	var speed int
	if err := uniRows.Scan(&universeLanguage, &forceLanguage, &feedAge, &speed); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}
	if err := uniRows.Err(); err != nil {
		return domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0, err
	}

	return domaingame.OptionsUser{
			Name:         name,
			NameLocked:   nameChanged != 0,
			Email:        email,
			PlainEmail:   plainEmail,
			Validated:    validated != 0,
			Admin:        admin,
			FeedID:       feedID,
			CommanderOn:  commanderUntil > r.now().Unix(),
			PasswordHash: passwordHash,
		},
		domaingame.OptionsUniverse{
			Language:      universeLanguage,
			ForceLanguage: forceLanguage != 0,
			FeedAge:       feedAge,
			Speed:         speed,
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

func (r OptionsRepository) applyCredentialMutations(ctx context.Context, usersTable string, queueTable string, playerID int, mutation domaingame.OptionsMutation, current domaingame.Options) (*domaingame.OptionsActionIssue, error) {
	if issue := mutation.PasswordValidationIssue(); issue != nil {
		return issue, nil
	}
	if mutation.PasswordChangeRequested() {
		if current.User.PasswordHash != legacyPasswordHash(mutation.OldPassword, r.secret) {
			return domaingame.OptionsPasswordWrongOldIssue(), nil
		}
		if _, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET password = ?, session = '' WHERE player_id = ? LIMIT 1", usersTable),
			legacyPasswordHash(mutation.NewPassword, r.secret),
			playerID,
		); err != nil {
			return nil, err
		}
		return domaingame.OptionsPasswordChangedIssue(), nil
	}

	if issue := mutation.EmailValidationIssue(current); issue != nil {
		return issue, nil
	}
	if !mutation.EmailChangeRequested(current) {
		return nil, nil
	}
	if current.User.PasswordHash != legacyPasswordHash(mutation.OldPassword, r.secret) {
		return domaingame.OptionsEmailNeedPasswordIssue(), nil
	}
	exists, err := r.emailExists(ctx, usersTable, mutation.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return domaingame.OptionsEmailUsedIssue(), nil
	}
	now := r.now().Unix()
	code := legacyPasswordHash(fmt.Sprintf("%d", now), r.secret)
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET validated = 0, validatemd = ?, email = ? WHERE player_id = ? LIMIT 1", usersTable),
		code,
		strings.TrimSpace(mutation.Email),
		playerID,
	); err != nil {
		return nil, err
	}
	if err := r.addChangeEmailEvent(ctx, queueTable, playerID, now); err != nil {
		return nil, err
	}
	return domaingame.OptionsEmailChangedIssue(), nil
}

func (r OptionsRepository) emailExists(ctx context.Context, usersTable string, email string) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE LOWER(email) = LOWER(?) OR LOWER(pemail) = LOWER(?)", usersTable),
		strings.TrimSpace(email),
		strings.TrimSpace(email),
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("options email state not found")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r OptionsRepository) addChangeEmailEvent(ctx context.Context, queueTable string, playerID int, now int64) error {
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE type = ? AND owner_id = ?", queueTable), "ChangeEmail", playerID); err != nil {
		return err
	}
	legacyEnd := now + (now + 7*24*60*60)
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		playerID,
		"ChangeEmail",
		0,
		0,
		0,
		now,
		legacyEnd,
		0,
	)
	return err
}

func (r OptionsRepository) canEnableVacation(ctx context.Context, queueTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND type IN (?, ?, ?, ?, ?)", queueTable),
		playerID,
		queueTypeBuild,
		queueTypeDemolish,
		queueTypeResearch,
		queueTypeShipyard,
		"Fleet",
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("vacation queue state not found")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r OptionsRepository) disableProductionForVacation(ctx context.Context, planetsTable string, playerID int) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET prod%d = 0, prod%d = 0, prod%d = 0, prod%d = 0, prod%d = 0, prod%d = 0 WHERE owner_id = ?", planetsTable, domaingame.BuildingMetalMine, domaingame.BuildingCrystalMine, domaingame.BuildingDeuteriumSynth, domaingame.BuildingSolarPlant, domaingame.BuildingFusionReactor, domaingame.FleetSolarSatellite),
		playerID,
	)
	return err
}

func vacationMinimumSeconds(speed int) int64 {
	if speed <= 0 {
		speed = 1
	}
	seconds := int64((2 * 24 * 60 * 60) / speed)
	if seconds < 12*60*60 {
		return 12 * 60 * 60
	}
	return seconds
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func legacyPasswordHash(value string, secret string) string {
	sum := md5.Sum([]byte(value + secret))
	return fmt.Sprintf("%x", sum)
}

package mysqlgame

import (
	"context"
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	buildingMetalStorage     = 22
	buildingCrystalStorage   = 23
	buildingDeuteriumStorage = 24
	resourceMetal            = 700
	resourceCrystal          = 701
	resourceDeuterium        = 702
	planetTypeDebris         = 2
	planetTypeDestroyed      = 10001
	planetTypeDestroyedMoon  = 10003
	userSpace                = 99999
	queueTypeBuild           = "Build"
	queueTypeDemolish        = "Demolish"
	queueTypeResearch        = "Research"
	queueTypeShipyard        = "Shipyard"
)

type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
}

type Rows interface {
	Close() error
	Err() error
	Next() bool
	Scan(dest ...any) error
}

type SQLQueryer struct {
	DB *sql.DB
}

func (q SQLQueryer) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return q.DB.QueryContext(ctx, query, args...)
}

type OverviewRepository struct {
	queryer           Queryer
	execer            Execer
	prefix            string
	secret            string
	now               func() time.Time
	updateResources   bool
	includeUnread     bool
	includeBuildQueue bool
	includeEvents     bool
}

func NewOverviewRepository(db *sql.DB, prefix string) OverviewRepository {
	return NewOverviewRepositoryWithSecret(db, prefix, "")
}

func NewOverviewRepositoryWithSecret(db *sql.DB, prefix string, secret string) OverviewRepository {
	runner := SQLQueryer{DB: db}
	return OverviewRepository{queryer: runner, execer: runner, prefix: prefix, secret: secret, now: time.Now, updateResources: true, includeUnread: true, includeBuildQueue: true, includeEvents: true}
}

func NewOverviewRepositoryWithQueryer(queryer Queryer, prefix string) OverviewRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewOverviewRepositoryWithRunner(queryer, execer, prefix)
}

func NewOverviewRepositoryWithRunner(queryer Queryer, execer Execer, prefix string) OverviewRepository {
	return NewOverviewRepositoryWithRunnerAndSecret(queryer, execer, prefix, "")
}

func NewOverviewRepositoryWithRunnerAndSecret(queryer Queryer, execer Execer, prefix string, secret string) OverviewRepository {
	return OverviewRepository{queryer: queryer, execer: execer, prefix: prefix, secret: secret, now: time.Now}
}

func (r OverviewRepository) GetOverview(ctx context.Context, query appgame.OverviewQuery) (domaingame.Overview, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Overview{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Overview{}, err
	}
	messagesTable := ""
	if r.includeUnread {
		messagesTable, err = tableName(r.prefix, "messages")
		if err != nil {
			return domaingame.Overview{}, err
		}
	}
	buildQueueTable := ""
	if r.includeBuildQueue {
		buildQueueTable, err = tableName(r.prefix, "buildqueue")
		if err != nil {
			return domaingame.Overview{}, err
		}
	}
	queueTable := ""
	fleetTable := ""
	unionTable := ""
	if r.includeEvents {
		queueTable, err = tableName(r.prefix, "queue")
		if err != nil {
			return domaingame.Overview{}, err
		}
		fleetTable, err = tableName(r.prefix, "fleet")
		if err != nil {
			return domaingame.Overview{}, err
		}
		unionTable, err = tableName(r.prefix, "union")
		if err != nil {
			return domaingame.Overview{}, err
		}
	}

	user, err := r.loadUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Overview{}, err
	}
	if r.includeBuildQueue && r.execer != nil && user.AdminLevel == 0 {
		buildings := BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
		if err := buildings.FinishDueBuildingQueues(ctx, int(r.currentTime().Unix())); err != nil {
			return domaingame.Overview{}, err
		}
	}
	updatedPlanetID := 0
	if r.updateResources {
		candidatePlanetID := query.PlanetID
		if candidatePlanetID == 0 {
			candidatePlanetID = user.ActivePlanetID
			if candidatePlanetID == 0 {
				candidatePlanetID = user.HomePlanetID
			}
		}
		if err := r.updatePlanetResources(ctx, usersTable, planetsTable, query.PlayerID, candidatePlanetID, int(r.currentTime().Unix())); err != nil {
			return domaingame.Overview{}, err
		}
		updatedPlanetID = candidatePlanetID
	}
	planetID, current, persistActive, err := r.resolveCurrentPlanet(ctx, planetsTable, user, query)
	if err != nil {
		return domaingame.Overview{}, err
	}
	if current.ID == 0 {
		if r.updateResources && planetID != updatedPlanetID {
			if err := r.updatePlanetResources(ctx, usersTable, planetsTable, query.PlayerID, planetID, int(r.currentTime().Unix())); err != nil {
				return domaingame.Overview{}, err
			}
			updatedPlanetID = planetID
		}
		current, err = r.loadPlanet(ctx, planetsTable, query.PlayerID, planetID, user)
		if err != nil {
			return domaingame.Overview{}, err
		}
	}
	if current.ID == 0 && planetID != user.HomePlanetID {
		if r.updateResources && user.HomePlanetID != updatedPlanetID {
			if err := r.updatePlanetResources(ctx, usersTable, planetsTable, query.PlayerID, user.HomePlanetID, int(r.currentTime().Unix())); err != nil {
				return domaingame.Overview{}, err
			}
		}
		current, err = r.loadPlanet(ctx, planetsTable, query.PlayerID, user.HomePlanetID, user)
		if err != nil {
			return domaingame.Overview{}, err
		}
		persistActive = query.PlanetID != 0
	}
	if current.ID == 0 {
		return domaingame.Overview{}, errors.New("current planet not found")
	}
	if persistActive && current.ID != user.ActivePlanetID {
		if err := r.updateActivePlanet(ctx, usersTable, query.PlayerID, current.ID); err != nil {
			return domaingame.Overview{}, err
		}
	}
	if query.Login {
		if err := r.updatePlanetActivity(ctx, planetsTable, current.ID); err != nil {
			return domaingame.Overview{}, err
		}
	}

	planets, err := r.loadPlanets(ctx, planetsTable, query.PlayerID, current.ID, user.SortBy, user.SortOrder)
	if err != nil {
		return domaingame.Overview{}, err
	}
	if r.includeBuildQueue {
		if err := r.attachOverviewBuildQueues(ctx, buildQueueTable, query.PlayerID, &current, planets); err != nil {
			return domaingame.Overview{}, err
		}
	}
	universePlayers, err := r.loadUniversePlayers(ctx)
	if err != nil {
		return domaingame.Overview{}, err
	}
	unreadMessages := 0
	if r.includeUnread {
		unreadMessages, err = r.loadUnreadMessages(ctx, messagesTable, query.PlayerID)
		if err != nil {
			return domaingame.Overview{}, err
		}
	}
	events := []domaingame.FleetMission(nil)
	if r.includeEvents {
		events, err = r.loadOverviewEvents(ctx, queueTable, fleetTable, planetsTable, usersTable, unionTable, query.PlayerID)
		if err != nil {
			return domaingame.Overview{}, err
		}
	}

	return domaingame.Overview{
		Commander:  user.Commander,
		AdminLevel: user.AdminLevel,
		ServerTime: formatLegacyOverviewTime(r.currentTime()),
		Score: domaingame.ScoreSummary{
			RawScore:        user.Score,
			Rank:            user.Rank,
			UniversePlayers: universePlayers,
		},
		CurrentPlanet:  current,
		PlanetSwitcher: planets,
		Messages:       overviewMessages(user),
		UnreadMessages: unreadMessages,
		Events:         events,
	}, nil
}

func (r OverviewRepository) RenamePlanet(ctx context.Context, query appgame.OverviewRenameQuery) (domaingame.Overview, error) {
	if r.execer == nil {
		return domaingame.Overview{}, errors.New("overview updater unavailable")
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Overview{}, err
	}
	overview, err := r.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Overview{}, err
	}
	name, ok := domaingame.NormalizePlanetName(query.Name, overview.CurrentPlanet.Type)
	if !ok {
		return overview, nil
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET name = ? WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", planetsTable),
		name,
		overview.CurrentPlanet.ID,
		query.PlayerID,
		planetTypeDebris,
	); err != nil {
		return domaingame.Overview{}, err
	}
	return r.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: overview.CurrentPlanet.ID})
}

func (r OverviewRepository) DeletePlanet(ctx context.Context, query appgame.OverviewDeleteQuery) (domaingame.Overview, *domaingame.OverviewActionIssue, error) {
	if r.execer == nil {
		return domaingame.Overview{}, nil, errors.New("overview updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return domaingame.Overview{}, nil, err
	}

	overview, err := r.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	if ok, err := r.passwordMatches(ctx, usersTable, query.PlayerID, query.Password); err != nil {
		return domaingame.Overview{}, nil, err
	} else if !ok {
		return overview, overviewIssue(domaingame.OverviewIssuePasswordInvalid, "The password is wrong."), nil
	}

	user, err := r.loadUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	target, found, err := r.loadDeletePlanet(ctx, planetsTable, query.PlayerID, query.DeleteID)
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	if !found {
		return overview, nil, nil
	}
	if target.ID == user.HomePlanetID {
		return overview, overviewIssue(domaingame.OverviewIssueHomePlanet, "You can't abandon the home planet!"), nil
	}
	if exists, err := r.fleetExists(ctx, fleetTable, "target_planet = ? AND owner_id = ?", target.ID, query.PlayerID); err != nil {
		return domaingame.Overview{}, nil, err
	} else if exists {
		return overview, overviewIssue(domaingame.OverviewIssueFleetIncoming, "Your fleets are still on their way to this planet!"), nil
	}
	if exists, err := r.fleetExists(ctx, fleetTable, "start_planet = ?", target.ID); err != nil {
		return domaingame.Overview{}, nil, err
	} else if exists {
		return overview, overviewIssue(domaingame.OverviewIssueFleetOutgoing, "The fleets from this planet have not yet returned!"), nil
	}

	when := r.currentTime().Unix()
	removeAt := when + 24*3600
	if target.Type != domaingame.PlanetTypeMoon {
		moon, found, err := r.loadCoordinateMoon(ctx, planetsTable, target.Coordinates)
		if err != nil {
			return domaingame.Overview{}, nil, err
		}
		if found && moon.Type == domaingame.PlanetTypeMoon {
			moonScore, err := r.loadPlanetScore(ctx, planetsTable, moon.ID)
			if err != nil {
				return domaingame.Overview{}, nil, err
			}
			if err := r.markPlanetDestroyed(ctx, planetsTable, moon.ID, planetTypeDestroyedMoon, when, removeAt); err != nil {
				return domaingame.Overview{}, nil, err
			}
			if err := r.flushPlanetQueue(ctx, queueTable, buildQueueTable, moon.ID); err != nil {
				return domaingame.Overview{}, nil, err
			}
			if err := r.applyPlanetScoreRemoval(ctx, usersTable, moonScore); err != nil {
				return domaingame.Overview{}, nil, err
			}
		}
	}
	targetScore, err := r.loadPlanetScore(ctx, planetsTable, target.ID)
	if err != nil {
		return domaingame.Overview{}, nil, err
	}
	destroyedType := planetTypeDestroyed
	if target.Type == domaingame.PlanetTypeMoon {
		destroyedType = planetTypeDestroyedMoon
	}
	if err := r.markPlanetDestroyed(ctx, planetsTable, target.ID, destroyedType, when, removeAt); err != nil {
		return domaingame.Overview{}, nil, err
	}
	if err := r.flushPlanetQueue(ctx, queueTable, buildQueueTable, target.ID); err != nil {
		return domaingame.Overview{}, nil, err
	}
	if err := r.applyPlanetScoreRemoval(ctx, usersTable, targetScore); err != nil {
		return domaingame.Overview{}, nil, err
	}
	if err := r.updateActivePlanet(ctx, usersTable, query.PlayerID, user.HomePlanetID); err != nil {
		return domaingame.Overview{}, nil, err
	}
	overview, err = r.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: user.HomePlanetID})
	return overview, nil, err
}

func (r OverviewRepository) passwordMatches(ctx context.Context, usersTable string, playerID int, password string) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id FROM %s WHERE player_id = ? AND password = ? LIMIT 1", usersTable),
		playerID,
		hashOverviewPassword(password, r.secret),
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var matchedID int
	if err := rows.Scan(&matchedID); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return matchedID == playerID, nil
}

type overviewDeletePlanet struct {
	ID          int
	Type        int
	Coordinates domaingame.Coordinates
}

func (r OverviewRepository) loadDeletePlanet(ctx context.Context, planetsTable string, playerID int, planetID int) (overviewDeletePlanet, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, type, g, s, p FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return overviewDeletePlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return overviewDeletePlanet{}, false, err
		}
		return overviewDeletePlanet{}, false, nil
	}
	planet, err := scanOverviewDeletePlanet(rows)
	if err != nil {
		return overviewDeletePlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return overviewDeletePlanet{}, false, err
	}
	return planet, true, nil
}

func (r OverviewRepository) loadCoordinateMoon(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates) (overviewDeletePlanet, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, type, g, s, p FROM %s WHERE g = ? AND s = ? AND p = ? AND (type = ? OR type = ?) LIMIT 1", planetsTable),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		domaingame.PlanetTypeMoon,
		planetTypeDestroyedMoon,
	)
	if err != nil {
		return overviewDeletePlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return overviewDeletePlanet{}, false, err
		}
		return overviewDeletePlanet{}, false, nil
	}
	planet, err := scanOverviewDeletePlanet(rows)
	if err != nil {
		return overviewDeletePlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return overviewDeletePlanet{}, false, err
	}
	return planet, true, nil
}

func scanOverviewDeletePlanet(rows Rows) (overviewDeletePlanet, error) {
	var planet overviewDeletePlanet
	err := rows.Scan(
		&planet.ID,
		&planet.Type,
		&planet.Coordinates.Galaxy,
		&planet.Coordinates.System,
		&planet.Coordinates.Position,
	)
	return planet, err
}

type overviewPlanetScore struct {
	OwnerID int
	Score   domaingame.PlanetScore
}

func (r OverviewRepository) loadPlanetScore(ctx context.Context, planetsTable string, planetID int) (overviewPlanetScore, error) {
	buildingIDs := domaingame.BuildingIDs()
	fleetIDs := domaingame.FleetIDs()
	defenseIDs := domaingame.DefenseIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT owner_id, %s, %s, %s FROM %s WHERE planet_id = ? LIMIT 1",
			numericColumns(buildingIDs),
			numericColumns(fleetIDs),
			numericColumns(defenseIDs),
			planetsTable,
		),
		planetID,
	)
	if err != nil {
		return overviewPlanetScore{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return overviewPlanetScore{}, err
		}
		return overviewPlanetScore{}, errors.New("planet score not found")
	}
	score, err := scanPlanetScore(rows, buildingIDs, fleetIDs, defenseIDs)
	if err != nil {
		return overviewPlanetScore{}, err
	}
	if err := rows.Err(); err != nil {
		return overviewPlanetScore{}, err
	}
	return score, nil
}

func scanPlanetScore(rows Rows, buildingIDs []int, fleetIDs []int, defenseIDs []int) (overviewPlanetScore, error) {
	var ownerID int
	buildingValues := make([]int, len(buildingIDs))
	fleetValues := make([]int, len(fleetIDs))
	defenseValues := make([]int, len(defenseIDs))
	dest := []any{&ownerID}
	dest = appendIntDest(dest, buildingValues)
	dest = appendIntDest(dest, fleetValues)
	dest = appendIntDest(dest, defenseValues)
	if err := rows.Scan(dest...); err != nil {
		return overviewPlanetScore{}, err
	}
	buildings := make(domaingame.BuildingLevels, len(buildingIDs))
	for index, id := range buildingIDs {
		buildings[id] = buildingValues[index]
	}
	fleet := make(domaingame.FleetCounts, len(fleetIDs))
	for index, id := range fleetIDs {
		fleet[id] = fleetValues[index]
	}
	defense := make(domaingame.DefenseCounts, len(defenseIDs))
	for index, id := range defenseIDs {
		defense[id] = defenseValues[index]
	}
	return overviewPlanetScore{
		OwnerID: ownerID,
		Score:   domaingame.CalculatePlanetScore(buildings, fleet, defense),
	}, nil
}

func appendIntDest(dest []any, values []int) []any {
	for index := range values {
		dest = append(dest, &values[index])
	}
	return dest
}

func (r OverviewRepository) fleetExists(ctx context.Context, fleetTable string, condition string, args ...any) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fleet_id FROM %s WHERE %s LIMIT 1", fleetTable, condition), args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var fleetID int
	if err := rows.Scan(&fleetID); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return fleetID != 0, nil
}

func (r OverviewRepository) markPlanetDestroyed(ctx context.Context, planetsTable string, planetID int, destroyedType int, when int64, removeAt int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET type = ?, owner_id = ?, date = ?, remove = ?, lastakt = ? WHERE planet_id = ? LIMIT 1", planetsTable),
		destroyedType,
		userSpace,
		when,
		removeAt,
		when,
		planetID,
	)
	return err
}

func (r OverviewRepository) applyPlanetScoreRemoval(ctx context.Context, usersTable string, score overviewPlanetScore) error {
	if err := r.adjustStats(ctx, usersTable, score.OwnerID, score.Score); err != nil {
		return err
	}
	return r.recalcRanks(ctx, usersTable)
}

func (r OverviewRepository) adjustStats(ctx context.Context, usersTable string, playerID int, score domaingame.PlanetScore) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET score1 = score1 - ?, score2 = score2 - ?, score3 = score3 - ? WHERE player_id = ? AND banned = 0 AND admin = 0", usersTable),
		score.Points,
		score.FleetPoints,
		0,
		playerID,
	)
	return err
}

func (r OverviewRepository) recalcRanks(ctx context.Context, usersTable string) error {
	statements := []string{
		fmt.Sprintf("UPDATE %s SET score1 = -1, score2 = -1, score3 = -1 WHERE admin > 0", usersTable),
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place1 = (SELECT @pos := @pos+1) ORDER BY score1 DESC", usersTable),
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place2 = (SELECT @pos := @pos+1) ORDER BY score2 DESC", usersTable),
		"SET @pos := 0",
		fmt.Sprintf("UPDATE %s SET place3 = (SELECT @pos := @pos+1) ORDER BY score3 DESC", usersTable),
		fmt.Sprintf("UPDATE %s SET place1 = 0, place2 = 0, place3 = 0 WHERE admin > 0", usersTable),
	}
	for _, statement := range statements {
		if _, err := r.execer.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (r OverviewRepository) flushPlanetQueue(ctx context.Context, queueTable string, buildQueueTable string, planetID int) error {
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("DELETE FROM %s WHERE type = ? AND sub_id = ?", queueTable),
		queueTypeShipyard,
		planetID,
	); err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("DELETE FROM %s WHERE (type = ? OR type = ?) AND sub_id IN (SELECT id FROM %s WHERE planet_id = ?)", queueTable, buildQueueTable),
		queueTypeBuild,
		queueTypeDemolish,
		planetID,
	); err != nil {
		return err
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("DELETE FROM %s WHERE planet_id = ?", buildQueueTable),
		planetID,
	)
	return err
}

func (r OverviewRepository) currentTime() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

func hashOverviewPassword(password string, secret string) string {
	sum := md5.Sum([]byte(password + secret))
	return fmt.Sprintf("%x", sum)
}

func overviewIssue(code string, message string) *domaingame.OverviewActionIssue {
	return &domaingame.OverviewActionIssue{Code: code, Message: message}
}

func (r OverviewRepository) resolveCurrentPlanet(ctx context.Context, planetsTable string, user overviewUser, query appgame.OverviewQuery) (int, domaingame.PlanetOverview, bool, error) {
	if query.PlanetID == 0 {
		if user.ActivePlanetID != 0 {
			return user.ActivePlanetID, domaingame.PlanetOverview{}, false, nil
		}
		return user.HomePlanetID, domaingame.PlanetOverview{}, false, nil
	}
	current, err := r.loadPlanet(ctx, planetsTable, query.PlayerID, query.PlanetID, user)
	if err != nil {
		return 0, domaingame.PlanetOverview{}, false, err
	}
	if current.ID == query.PlanetID {
		return query.PlanetID, current, true, nil
	}
	exists, err := r.selectablePlanetExists(ctx, planetsTable, query.PlanetID)
	if err != nil {
		return 0, domaingame.PlanetOverview{}, false, err
	}
	if exists {
		if user.ActivePlanetID != 0 {
			return user.ActivePlanetID, domaingame.PlanetOverview{}, false, nil
		}
		return user.HomePlanetID, domaingame.PlanetOverview{}, false, nil
	}
	return user.HomePlanetID, domaingame.PlanetOverview{}, true, nil
}

type overviewUser struct {
	Commander      string
	Score          int64
	Rank           int
	ActivePlanetID int
	HomePlanetID   int
	SortBy         int
	SortOrder      int
	AdminLevel     int
	DarkMatter     int
	EnergyResearch int
	Engineer       bool
}

func (r OverviewRepository) loadUser(ctx context.Context, usersTable string, playerID int) (overviewUser, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT oname, score1, place1, aktplanet, hplanetid, sortby, sortorder, admin, COALESCE(dm, 0), COALESCE(dmfree, 0), `%d`, COALESCE(eng_until, 0) FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return overviewUser{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return overviewUser{}, err
		}
		return overviewUser{}, errors.New("overview user not found")
	}
	user, err := r.scanOverviewUser(rows)
	if err != nil {
		return overviewUser{}, err
	}
	if err := rows.Err(); err != nil {
		return overviewUser{}, err
	}
	return user, nil
}

func (r OverviewRepository) scanOverviewUser(rows Rows) (overviewUser, error) {
	var user overviewUser
	var darkMatter int
	var freeDarkMatter int
	var engineerUntil int64
	err := rows.Scan(
		&user.Commander,
		&user.Score,
		&user.Rank,
		&user.ActivePlanetID,
		&user.HomePlanetID,
		&user.SortBy,
		&user.SortOrder,
		&user.AdminLevel,
		&darkMatter,
		&freeDarkMatter,
		&user.EnergyResearch,
		&engineerUntil,
	)
	if err == nil {
		user.DarkMatter = darkMatter + freeDarkMatter
		user.Engineer = engineerUntil > r.currentTime().Unix()
		return user, nil
	}
	if !scanDestinationCountError(err) {
		return overviewUser{}, err
	}
	if err := rows.Scan(&user.Commander, &user.Score, &user.Rank, &user.ActivePlanetID, &user.HomePlanetID, &user.SortBy, &user.SortOrder, &user.AdminLevel); err != nil {
		return overviewUser{}, err
	}
	return user, nil
}

func overviewMessages(user overviewUser) []string {
	if user.AdminLevel <= 0 {
		return nil
	}
	return []string{domaingame.OverviewAdminNotice}
}

func (r OverviewRepository) loadPlanet(ctx context.Context, planetsTable string, playerID int, planetID int, user overviewUser) (domaingame.PlanetOverview, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, name, type, g, s, p, diameter, temp, fields, maxfields, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
			buildingMetalStorage,
			buildingCrystalStorage,
			buildingDeuteriumStorage,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			planetsTable,
		),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return domaingame.PlanetOverview{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.PlanetOverview{}, err
		}
		return domaingame.PlanetOverview{}, nil
	}
	planet, err := scanPlanetOverview(rows, user)
	if err != nil {
		return domaingame.PlanetOverview{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.PlanetOverview{}, err
	}
	return planet, nil
}

func (r OverviewRepository) selectablePlanetExists(ctx context.Context, planetsTable string, planetID int) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id FROM %s WHERE planet_id = ? AND type < ? LIMIT 1", planetsTable),
		planetID,
		planetTypeDebris,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var id int
	if err := rows.Scan(&id); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return id != 0, nil
}

func (r OverviewRepository) updateActivePlanet(ctx context.Context, usersTable string, playerID int, planetID int) error {
	if r.execer == nil {
		return nil
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET aktplanet = ? WHERE player_id = ? LIMIT 1", usersTable),
		planetID,
		playerID,
	)
	return err
}

func (r OverviewRepository) updatePlanetActivity(ctx context.Context, planetsTable string, planetID int) error {
	if r.execer == nil {
		return errors.New("overview activity updater unavailable")
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET lastakt = ? WHERE planet_id = ?", planetsTable),
		r.currentTime().Unix(),
		planetID,
	)
	return err
}

func (r OverviewRepository) loadPlanets(ctx context.Context, planetsTable string, playerID int, currentPlanetID int, sortBy int, sortOrder int) ([]domaingame.PlanetSummary, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, name, type, g, s, p FROM %s WHERE owner_id = ? AND type < ?%s", planetsTable, planetOrder(sortBy, sortOrder)),
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	planets := make([]domaingame.PlanetSummary, 0)
	for rows.Next() {
		var planet domaingame.PlanetSummary
		if err := rows.Scan(&planet.ID, &planet.Name, &planet.Type, &planet.Coordinates.Galaxy, &planet.Coordinates.System, &planet.Coordinates.Position); err != nil {
			return nil, err
		}
		planet.Current = planet.ID == currentPlanetID
		planets = append(planets, planet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planets, nil
}

func (r OverviewRepository) attachOverviewBuildQueues(ctx context.Context, buildQueueTable string, playerID int, current *domaingame.PlanetOverview, planets []domaingame.PlanetSummary) error {
	planetIndexes := make(map[int]int, len(planets))
	for index := range planets {
		planetIndexes[planets[index].ID] = index
	}
	currentID := 0
	if current != nil && current.ID != 0 {
		currentID = current.ID
	}
	if len(planetIndexes) == 0 && currentID == 0 {
		return nil
	}

	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, tech_id, level, destroy, end FROM %s WHERE owner_id = ? ORDER BY planet_id ASC, list_id ASC", buildQueueTable),
		playerID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := make(map[int]bool, len(planetIndexes))
	for rows.Next() {
		var planetID int
		var techID int
		var level int
		var destroy int
		var end int64
		if err := rows.Scan(&planetID, &techID, &level, &destroy, &end); err != nil {
			return err
		}
		index, ok := planetIndexes[planetID]
		if (!ok && planetID != currentID) || seen[planetID] || !containsInt(domaingame.BuildingIDs(), techID) {
			continue
		}
		queue := &domaingame.OverviewBuildQueue{
			TechID:  techID,
			Name:    overviewTechnologyName(techID),
			Level:   level,
			Destroy: destroy != 0,
			End:     end,
		}
		seen[planetID] = true
		if currentID == planetID {
			current.BuildQueue = queue
		}
		if ok {
			planets[index].BuildQueue = queue
		}
	}
	return rows.Err()
}

func (r OverviewRepository) loadUniversePlayers(ctx context.Context) (int, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return 0, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT usercount FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var players int
	if err := rows.Scan(&players); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return players, nil
}

func (r OverviewRepository) loadUnreadMessages(ctx context.Context, messagesTable string, playerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND shown = 0", messagesTable), playerID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r OverviewRepository) loadOverviewEvents(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, unionTable string, playerID int) ([]domaingame.FleetMission, error) {
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, COALESCE(f.flight_time, 0), COALESCE(f.deploy_time, 0), f.mission, COALESCE(f.ipm_amount, 0), COALESCE(f.ipm_target, 0), f.owner_id, COALESCE(owner_user.oname, ''), f.start_planet, f.target_planet, %s, COALESCE(o.name, ''), COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.name, ''), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(target_user.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s owner_user ON owner_user.player_id = f.owner_id LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s target_user ON target_user.player_id = t.owner_id WHERE q.type = ? AND COALESCE(f.union_id, 0) = 0 AND (f.owner_id = ? OR (f.target_planet IN (SELECT planet_id FROM %s WHERE owner_id = ? AND type < ?) AND (f.mission < ? OR f.mission = ?))) ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			queueTable,
			fleetTable,
			planetsTable,
			usersTable,
			planetsTable,
			usersTable,
			planetsTable,
		),
		legacyPlanetTypeAbandoned,
		queueTypeFleet,
		playerID,
		playerID,
		planetTypeDebris,
		domaingame.FleetMissionReturnOffset,
		domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	missions := make([]domaingame.FleetMission, 0)
	for rows.Next() {
		scanned, err := scanOverviewEventRow(rows, fleetIDs, playerID)
		if err != nil {
			return nil, err
		}
		missions = append(missions, overviewNonUnionMissions(scanned, playerID)...)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	events := domaingame.BuildOverviewEvents(missions)
	unionEvents, err := r.loadOverviewUnionEvents(ctx, queueTable, fleetTable, planetsTable, usersTable, unionTable, fleetIDs, playerID)
	if err != nil {
		return nil, err
	}
	events = append(events, unionEvents...)
	sort.SliceStable(events, func(left int, right int) bool {
		return events[left].ArrivalAt < events[right].ArrivalAt
	})
	return events, nil
}

type overviewUnionRow struct {
	ID           int
	TargetPlayer int
}

func (r OverviewRepository) loadOverviewUnionEvents(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, unionTable string, fleetIDs []int, playerID int) ([]domaingame.FleetMission, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT union_id, target_player FROM %s WHERE target_player = ? OR CONCAT(',', players, ',') LIKE ? ORDER BY union_id ASC", unionTable),
		playerID,
		fmt.Sprintf("%%,%d,%%", playerID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	unions := make([]overviewUnionRow, 0)
	for rows.Next() {
		var union overviewUnionRow
		if err := rows.Scan(&union.ID, &union.TargetPlayer); err != nil {
			return nil, err
		}
		unions = append(unions, union)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	events := make([]domaingame.FleetMission, 0, len(unions))
	for _, union := range unions {
		unionEvents, err := r.loadOverviewUnionEvent(ctx, queueTable, fleetTable, planetsTable, usersTable, fleetIDs, playerID, union.ID)
		if err != nil {
			return nil, err
		}
		events = append(events, unionEvents...)
	}
	return events, nil
}

func (r OverviewRepository) loadOverviewUnionEvent(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, fleetIDs []int, playerID int, unionID int) ([]domaingame.FleetMission, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, COALESCE(f.flight_time, 0), COALESCE(f.deploy_time, 0), f.mission, COALESCE(f.ipm_amount, 0), COALESCE(f.ipm_target, 0), f.owner_id, COALESCE(owner_user.oname, ''), f.start_planet, f.target_planet, %s, COALESCE(o.name, ''), COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.name, ''), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(target_user.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s owner_user ON owner_user.player_id = f.owner_id LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s target_user ON target_user.player_id = t.owner_id WHERE q.type = ? AND f.union_id = ? ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			queueTable,
			fleetTable,
			planetsTable,
			usersTable,
			planetsTable,
			usersTable,
		),
		legacyPlanetTypeAbandoned,
		queueTypeFleet,
		unionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	missions := make([]domaingame.FleetMission, 0)
	returnMissions := make([]domaingame.FleetMission, 0)
	arrivalAt := int64(0)
	for rows.Next() {
		scanned, err := scanOverviewEventRow(rows, fleetIDs, playerID)
		if err != nil {
			return nil, err
		}
		mission := scanned.Mission
		mission.UnionID = unionID
		if mission.ArrivalAt > arrivalAt {
			arrivalAt = mission.ArrivalAt
		}
		missions = append(missions, mission)
		if mission.OwnerID == playerID && mission.Mission < domaingame.FleetMissionReturnOffset {
			returnMissions = append(returnMissions, overviewUnionReturnMission(mission, scanned.FlightTime, unionID))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(missions) == 0 {
		return nil, nil
	}

	event := missions[0]
	event.ID = -unionID
	event.UnionID = unionID
	event.ArrivalAt = arrivalAt
	event.GroupMissions = missions
	events := domaingame.BuildOverviewEvents([]domaingame.FleetMission{event})
	events = append(events, domaingame.BuildOverviewEvents(returnMissions)...)
	return events, nil
}

func overviewUnionReturnMission(mission domaingame.FleetMission, flightTime int64, unionID int) domaingame.FleetMission {
	if flightTime < 0 {
		flightTime = 0
	}
	returnMission := mission
	returnMission.ID = -(unionID*1_000_000 + mission.ID)
	returnMission.UnionID = unionID
	returnMission.Mission = mission.Mission + domaingame.FleetMissionReturnOffset
	returnMission.DepartureAt = mission.ArrivalAt
	returnMission.ArrivalAt = mission.ArrivalAt + flightTime
	returnMission.Origin = mission.Target
	returnMission.OriginName = mission.TargetName
	returnMission.Target = mission.Origin
	returnMission.TargetName = mission.OriginName
	returnMission.Foreign = false
	returnMission.GroupMissions = nil
	return returnMission
}

func overviewNonUnionMissions(scanned overviewEventScan, playerID int) []domaingame.FleetMission {
	mission := overviewNormalizeEventGeometry(scanned.Mission)
	missions := make([]domaingame.FleetMission, 0, 3)
	baseMission := overviewBaseMission(mission.Mission)
	if (baseMission == domaingame.FleetMissionACSHold || baseMission == domaingame.FleetMissionExpedition) &&
		mission.Mission < domaingame.FleetMissionReturnOffset &&
		(mission.OwnerID == playerID || baseMission == domaingame.FleetMissionACSHold) {
		missions = append(missions, overviewHoldPseudoMission(mission, scanned.DeployTime))
	}
	missions = append(missions, mission)
	if overviewShouldAddReturnPseudoMission(mission, playerID) {
		missions = append(missions, overviewReturnPseudoMission(mission, scanned.FlightTime, scanned.DeployTime))
	}
	return missions
}

func overviewNormalizeEventGeometry(mission domaingame.FleetMission) domaingame.FleetMission {
	if mission.Mission >= domaingame.FleetMissionReturnOffset && mission.Mission < domaingame.FleetMissionOrbitingOffset {
		mission.Origin, mission.Target = mission.Target, mission.Origin
		mission.OriginName, mission.TargetName = mission.TargetName, mission.OriginName
	}
	return mission
}

func overviewHoldPseudoMission(mission domaingame.FleetMission, deployTime int64) domaingame.FleetMission {
	if deployTime < 0 {
		deployTime = 0
	}
	holdMission := mission
	holdMission.ID = overviewPseudoEventID(mission.ID, 1)
	holdMission.Mission = overviewBaseMission(mission.Mission) + domaingame.FleetMissionOrbitingOffset
	holdMission.DepartureAt = mission.ArrivalAt
	holdMission.ArrivalAt = mission.ArrivalAt + deployTime
	holdMission.GroupMissions = nil
	return holdMission
}

func overviewShouldAddReturnPseudoMission(mission domaingame.FleetMission, playerID int) bool {
	if mission.OwnerID != playerID {
		return false
	}
	if mission.Mission == domaingame.FleetMissionDeploy || mission.Mission == domaingame.FleetMissionMissile {
		return false
	}
	return mission.Mission < domaingame.FleetMissionReturnOffset || mission.Mission > domaingame.FleetMissionOrbitingOffset
}

func overviewReturnPseudoMission(mission domaingame.FleetMission, flightTime int64, deployTime int64) domaingame.FleetMission {
	if flightTime < 0 {
		flightTime = 0
	}
	if deployTime < 0 {
		deployTime = 0
	}
	baseMission := overviewBaseMission(mission.Mission)
	returnAt := 2*mission.ArrivalAt - mission.DepartureAt
	if baseMission == domaingame.FleetMissionACSHold || baseMission == domaingame.FleetMissionExpedition {
		returnAt = mission.ArrivalAt + deployTime
		if mission.Mission < domaingame.FleetMissionOrbitingOffset {
			returnAt += flightTime
		}
	}
	if returnAt < mission.ArrivalAt {
		returnAt = mission.ArrivalAt
	}
	returnMission := mission
	returnMission.ID = overviewPseudoEventID(mission.ID, 2)
	returnMission.Mission = baseMission + domaingame.FleetMissionReturnOffset
	returnMission.DepartureAt = mission.ArrivalAt
	returnMission.ArrivalAt = returnAt
	returnMission.Origin = mission.Target
	returnMission.OriginName = mission.TargetName
	returnMission.Target = mission.Origin
	returnMission.TargetName = mission.OriginName
	returnMission.Foreign = false
	returnMission.GroupMissions = nil
	return returnMission
}

func overviewBaseMission(mission int) int {
	if mission >= domaingame.FleetMissionOrbitingOffset {
		return mission - domaingame.FleetMissionOrbitingOffset
	}
	if mission >= domaingame.FleetMissionReturnOffset {
		return mission - domaingame.FleetMissionReturnOffset
	}
	return mission
}

func overviewPseudoEventID(id int, suffix int) int {
	return -(1_000_000_000 + id*10 + suffix)
}

type overviewEventScan struct {
	Mission    domaingame.FleetMission
	FlightTime int64
	DeployTime int64
}

func scanOverviewEventRow(rows Rows, fleetIDs []int, playerID int) (overviewEventScan, error) {
	var id int
	var departureAt int64
	var arrivalAt int64
	var flightTime int64
	var deployTime int64
	var mission int
	var missileAmount int
	var missileTargetID int
	var ownerID int
	var ownerName string
	var startPlanetID int
	var targetPlanetID int
	shipValues := make([]int, len(fleetIDs))
	var origin domaingame.Coordinates
	var originName string
	var target domaingame.Coordinates
	var targetName string
	var targetType int
	var targetOwner string

	dest := []any{&id, &departureAt, &arrivalAt, &flightTime, &deployTime, &mission, &missileAmount, &missileTargetID, &ownerID, &ownerName, &startPlanetID, &targetPlanetID}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	dest = append(dest,
		&originName,
		&origin.Galaxy,
		&origin.System,
		&origin.Position,
		&targetName,
		&target.Galaxy,
		&target.System,
		&target.Position,
		&targetType,
		&targetOwner,
	)
	if err := rows.Scan(dest...); err != nil {
		return overviewEventScan{}, err
	}

	ships := make(domaingame.FleetCounts, len(fleetIDs))
	for index, fleetID := range fleetIDs {
		ships[fleetID] = shipValues[index]
	}
	event := domaingame.BuildFleetMission(id, mission, ships, origin, target, targetType, targetOwner, departureAt, arrivalAt)
	event.OwnerID = ownerID
	event.OwnerName = ownerName
	event.OriginName = originName
	event.TargetName = targetName
	event.Foreign = ownerID != 0 && ownerID != playerID
	event.MissileAmount = missileAmount
	event.MissileTargetID = missileTargetID
	if missileTargetID > 0 {
		event.MissileTarget = overviewTechnologyName(missileTargetID)
	}
	return overviewEventScan{Mission: event, FlightTime: flightTime, DeployTime: deployTime}, nil
}

func overviewTechnologyName(techID int) string {
	name := domaingame.TechnologyName(techID)
	if name == "" {
		return fmt.Sprintf("NAME_%d", techID)
	}
	return name
}

func formatLegacyOverviewTime(now time.Time) string {
	return now.In(time.FixedZone("MSK", 3*60*60)).Format("Mon Jan 2 15:04:05")
}

func scanPlanetOverview(rows Rows, user overviewUser) (domaingame.PlanetOverview, error) {
	var planet domaingame.PlanetOverview
	var metalStorageLevel int
	var crystalStorageLevel int
	var deuteriumStorageLevel int
	var metalMine int
	var crystalMine int
	var deuteriumSynth int
	var solarPlant int
	var fusionReactor int
	var solarSatellites int
	var prodMetal float64
	var prodCrystal float64
	var prodDeuterium float64
	var prodSolar float64
	var prodFusion float64
	var prodSatellite float64
	dest := []any{
		&planet.ID,
		&planet.Name,
		&planet.Type,
		&planet.Coordinates.Galaxy,
		&planet.Coordinates.System,
		&planet.Coordinates.Position,
		&planet.Diameter,
		&planet.Temperature,
		&planet.Fields,
		&planet.MaxFields,
		&planet.Resources.Metal,
		&planet.Resources.Crystal,
		&planet.Resources.Deuterium,
		&metalStorageLevel,
		&crystalStorageLevel,
		&deuteriumStorageLevel,
		&metalMine,
		&crystalMine,
		&deuteriumSynth,
		&solarPlant,
		&fusionReactor,
		&solarSatellites,
		&prodMetal,
		&prodCrystal,
		&prodDeuterium,
		&prodSolar,
		&prodFusion,
		&prodSatellite,
	}
	if err := rows.Scan(dest...); err != nil {
		if !scanDestinationCountError(err) {
			return planet, err
		}
		if err := rows.Scan(dest[:16]...); err != nil {
			return planet, err
		}
	}
	planet.Resources.DarkMatter = user.DarkMatter
	if planet.Type == domaingame.PlanetTypeMoon {
		planet.Resources.MetalCapacity = 0
		planet.Resources.CrystalCapacity = 0
		planet.Resources.DeuteriumCapacity = 0
	} else {
		planet.Resources.MetalCapacity = storageCapacity(metalStorageLevel)
		planet.Resources.CrystalCapacity = storageCapacity(crystalStorageLevel)
		planet.Resources.DeuteriumCapacity = storageCapacity(deuteriumStorageLevel)
	}
	if planet.Type == domaingame.PlanetTypePlanet {
		production := domaingame.BuildResourceProduction(domaingame.Overview{Commander: user.Commander, CurrentPlanet: planet}, domaingame.ResourceProductionInputs{
			Levels: domaingame.BuildingLevels{
				domaingame.BuildingMetalMine:      metalMine,
				domaingame.BuildingCrystalMine:    crystalMine,
				domaingame.BuildingDeuteriumSynth: deuteriumSynth,
				domaingame.BuildingSolarPlant:     solarPlant,
				domaingame.BuildingFusionReactor:  fusionReactor,
			},
			SolarSatellites: solarSatellites,
			ProductionFactors: domaingame.ProductionFactors{
				domaingame.BuildingMetalMine:      prodMetal,
				domaingame.BuildingCrystalMine:    prodCrystal,
				domaingame.BuildingDeuteriumSynth: prodDeuterium,
				domaingame.BuildingSolarPlant:     prodSolar,
				domaingame.BuildingFusionReactor:  prodFusion,
				domaingame.FleetSolarSatellite:    prodSatellite,
			},
			EnergyResearch: user.EnergyResearch,
			UniverseSpeed:  1,
			Engineer:       user.Engineer,
		})
		planet.Resources.Energy = int(production.Totals.Hour.Energy)
		planet.Resources.EnergyCapacity = overviewEnergyCapacity(production)
	}
	return planet, nil
}

func overviewEnergyCapacity(production domaingame.ResourceProduction) int {
	capacity := 0.0
	for _, row := range production.Rows {
		if row.Values.Energy > 0 {
			capacity += math.Ceil(row.Values.Energy)
		}
	}
	return int(capacity)
}

func scanDestinationCountError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "destination count") || strings.Contains(message, "destination arguments")
}

func storageCapacity(level int) int {
	if level < 0 {
		level = 0
	}
	capacity := 100000.0 + 50000.0*(math.Ceil(math.Pow(1.6, float64(level)))-1)
	return int(capacity)
}

func planetOrder(sortBy int, sortOrder int) string {
	direction := "ASC"
	if sortOrder != 0 {
		direction = "DESC"
	}
	switch sortBy {
	case 1:
		return fmt.Sprintf(" ORDER BY g %s, s %s, p %s, type DESC", direction, direction, direction)
	case 2:
		return fmt.Sprintf(" ORDER BY name %s, type DESC", direction)
	default:
		return fmt.Sprintf(" ORDER BY planet_id %s, type DESC", direction)
	}
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func tableName(prefix string, name string) (string, error) {
	identifier := prefix + name
	if !identifierPattern.MatchString(identifier) {
		return "", errors.New("invalid database table prefix")
	}
	return "`" + identifier + "`", nil
}

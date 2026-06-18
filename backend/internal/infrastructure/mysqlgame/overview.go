package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"

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
	queryer Queryer
	prefix  string
}

func NewOverviewRepository(db *sql.DB, prefix string) OverviewRepository {
	return OverviewRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewOverviewRepositoryWithQueryer(queryer Queryer, prefix string) OverviewRepository {
	return OverviewRepository{queryer: queryer, prefix: prefix}
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

	user, err := r.loadUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Overview{}, err
	}
	planetID := query.PlanetID
	if planetID == 0 {
		planetID = user.ActivePlanetID
	}
	if planetID == 0 {
		planetID = user.HomePlanetID
	}

	current, err := r.loadPlanet(ctx, planetsTable, query.PlayerID, planetID)
	if err != nil {
		return domaingame.Overview{}, err
	}
	if current.ID == 0 && planetID != user.HomePlanetID {
		current, err = r.loadPlanet(ctx, planetsTable, query.PlayerID, user.HomePlanetID)
		if err != nil {
			return domaingame.Overview{}, err
		}
	}
	if current.ID == 0 {
		return domaingame.Overview{}, errors.New("current planet not found")
	}

	planets, err := r.loadPlanets(ctx, planetsTable, query.PlayerID, current.ID, user.SortBy, user.SortOrder)
	if err != nil {
		return domaingame.Overview{}, err
	}
	universePlayers, err := r.loadUniversePlayers(ctx)
	if err != nil {
		return domaingame.Overview{}, err
	}

	return domaingame.Overview{
		Commander: user.Commander,
		Score: domaingame.ScoreSummary{
			RawScore:        user.Score,
			Rank:            user.Rank,
			UniversePlayers: universePlayers,
		},
		CurrentPlanet:  current,
		PlanetSwitcher: planets,
	}, nil
}

type overviewUser struct {
	Commander      string
	Score          int64
	Rank           int
	ActivePlanetID int
	HomePlanetID   int
	SortBy         int
	SortOrder      int
}

func (r OverviewRepository) loadUser(ctx context.Context, usersTable string, playerID int) (overviewUser, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT oname, score1, place1, aktplanet, hplanetid, sortby, sortorder FROM %s WHERE player_id = ? LIMIT 1", usersTable),
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
	var user overviewUser
	if err := rows.Scan(&user.Commander, &user.Score, &user.Rank, &user.ActivePlanetID, &user.HomePlanetID, &user.SortBy, &user.SortOrder); err != nil {
		return overviewUser{}, err
	}
	if err := rows.Err(); err != nil {
		return overviewUser{}, err
	}
	return user, nil
}

func (r OverviewRepository) loadPlanet(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.PlanetOverview, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, name, type, g, s, p, diameter, temp, fields, maxfields, `%d`, `%d`, `%d`, `%d`, `%d`, `%d` FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
			buildingMetalStorage,
			buildingCrystalStorage,
			buildingDeuteriumStorage,
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
	planet, err := scanPlanetOverview(rows)
	if err != nil {
		return domaingame.PlanetOverview{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.PlanetOverview{}, err
	}
	return planet, nil
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

func scanPlanetOverview(rows Rows) (domaingame.PlanetOverview, error) {
	var planet domaingame.PlanetOverview
	var metalStorageLevel int
	var crystalStorageLevel int
	var deuteriumStorageLevel int
	err := rows.Scan(
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
	)
	if err != nil {
		return planet, err
	}
	planet.Resources.MetalCapacity = storageCapacity(metalStorageLevel)
	planet.Resources.CrystalCapacity = storageCapacity(crystalStorageLevel)
	planet.Resources.DeuteriumCapacity = storageCapacity(deuteriumStorageLevel)
	return planet, nil
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

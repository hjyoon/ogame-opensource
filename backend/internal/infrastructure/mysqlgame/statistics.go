package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type StatisticsRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewStatisticsRepository(db *sql.DB, prefix string) StatisticsRepository {
	return StatisticsRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewStatisticsRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) StatisticsRepository {
	if now == nil {
		now = time.Now
	}
	return StatisticsRepository{queryer: queryer, prefix: prefix, now: now}
}

func (r StatisticsRepository) GetStatistics(ctx context.Context, query appgame.StatisticsQuery) (domaingame.Statistics, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Statistics{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Statistics{}, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return domaingame.Statistics{}, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.Statistics{}, err
	}

	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Statistics{}, err
	}

	who := domaingame.NormalizeStatisticsWho(query.Who)
	statType := domaingame.NormalizeStatisticsType(query.Type)
	total := 0
	start := 1
	rows := []domaingame.StatisticsRow{}
	if who == domaingame.StatisticsWhoAlly {
		total, err = r.loadAllianceStatisticsTotal(ctx, allyTable)
		if err != nil {
			return domaingame.Statistics{}, err
		}
		ownAllianceID, ownPlace, err := r.loadOwnAllianceStatistics(ctx, usersTable, allyTable, query.PlayerID, statType)
		if err != nil {
			return domaingame.Statistics{}, err
		}
		start = domaingame.NormalizeStatisticsStart(query.Start, ownPlace)
		rows, err = r.loadAllianceStatisticsRows(ctx, allyTable, usersTable, ownAllianceID, statType, start)
		if err != nil {
			return domaingame.Statistics{}, err
		}
	} else {
		total, err = r.loadStatisticsTotal(ctx, uniTable)
		if err != nil {
			return domaingame.Statistics{}, err
		}
		ownPlace, err := r.loadOwnStatisticsPlace(ctx, usersTable, query.PlayerID, statType)
		if err != nil {
			return domaingame.Statistics{}, err
		}
		start = domaingame.NormalizeStatisticsStart(query.Start, ownPlace)
		rows, err = r.loadPlayerStatisticsRows(ctx, usersTable, planetsTable, allyTable, query.PlayerID, statType, start)
		if err != nil {
			return domaingame.Statistics{}, err
		}
	}

	return domaingame.Statistics{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Who:            who,
		Type:           statType,
		Start:          start,
		Total:          total,
		GeneratedAt:    r.now().Unix(),
		Rows:           rows,
	}, nil
}

func (r StatisticsRepository) loadStatisticsTotal(ctx context.Context, uniTable string) (int, error) {
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
	var total int
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return total, nil
}

func (r StatisticsRepository) loadAllianceStatisticsTotal(ctx context.Context, allyTable string) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", allyTable))
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
	var total int
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return total, nil
}

func (r StatisticsRepository) loadOwnStatisticsPlace(ctx context.Context, usersTable string, playerID int, statType string) (int, error) {
	_, placeColumn, _ := domaingame.StatisticsScoreColumns(statType)
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE player_id = ? LIMIT 1", placeColumn, usersTable), playerID)
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
	var place int
	if err := rows.Scan(&place); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return place, nil
}

func (r StatisticsRepository) loadOwnAllianceStatistics(ctx context.Context, usersTable string, allyTable string, playerID int, statType string) (int, int, error) {
	_, placeColumn, _ := domaingame.StatisticsScoreColumns(statType)
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(u.ally_id, 0), COALESCE(a.%s, 0) FROM %s u LEFT JOIN %s a ON a.ally_id = u.ally_id WHERE u.player_id = ? LIMIT 1", placeColumn, usersTable, allyTable),
		playerID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}
	var allianceID int
	var place int
	if err := rows.Scan(&allianceID, &place); err != nil {
		return 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	return allianceID, place, nil
}

func (r StatisticsRepository) loadPlayerStatisticsRows(ctx context.Context, usersTable string, planetsTable string, allyTable string, playerID int, statType string, start int) ([]domaingame.StatisticsRow, error) {
	scoreColumn, placeColumn, oldPlaceColumn := domaingame.StatisticsScoreColumns(statType)
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, COALESCE(u.ally_id, 0), COALESCE(a.tag, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), u.%s, u.%s, u.%s, u.scoredate FROM %s u LEFT JOIN %s a ON a.ally_id = u.ally_id LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.%s >= ? AND u.%s < ? ORDER BY u.%s ASC",
			scoreColumn,
			placeColumn,
			oldPlaceColumn,
			usersTable,
			allyTable,
			planetsTable,
			placeColumn,
			placeColumn,
			placeColumn,
		),
		start,
		start+99,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domaingame.StatisticsRow{}
	for rows.Next() {
		var row domaingame.StatisticsRow
		var allianceID int
		var allianceTag string
		if err := rows.Scan(
			&row.Player.ID,
			&row.Player.Name,
			&allianceID,
			&allianceTag,
			&row.Coordinates.Galaxy,
			&row.Coordinates.System,
			&row.Coordinates.Position,
			&row.Score,
			&row.Place,
			&row.PreviousPlace,
			&row.ScoreDate,
		); err != nil {
			return nil, err
		}
		row.Own = row.Player.ID == playerID
		if allianceID > 0 {
			row.Alliance = &domaingame.StatisticsAlliance{ID: allianceID, Tag: allianceTag}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r StatisticsRepository) loadAllianceStatisticsRows(ctx context.Context, allyTable string, usersTable string, ownAllianceID int, statType string, start int) ([]domaingame.StatisticsRow, error) {
	scoreColumn, placeColumn, oldPlaceColumn := domaingame.StatisticsScoreColumns(statType)
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT a.ally_id, a.tag, a.%s, a.%s, a.%s, a.scoredate, COUNT(u.player_id) FROM %s a LEFT JOIN %s u ON u.ally_id = a.ally_id WHERE a.%s >= ? AND a.%s < ? GROUP BY a.ally_id, a.tag, a.%s, a.%s, a.%s, a.scoredate ORDER BY a.%s ASC",
			scoreColumn,
			placeColumn,
			oldPlaceColumn,
			allyTable,
			usersTable,
			placeColumn,
			placeColumn,
			scoreColumn,
			placeColumn,
			oldPlaceColumn,
			placeColumn,
		),
		start,
		start+99,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domaingame.StatisticsRow{}
	for rows.Next() {
		var row domaingame.StatisticsRow
		var alliance domaingame.StatisticsAlliance
		if err := rows.Scan(
			&alliance.ID,
			&alliance.Tag,
			&row.Score,
			&row.Place,
			&row.PreviousPlace,
			&row.ScoreDate,
			&row.Members,
		); err != nil {
			return nil, err
		}
		row.Alliance = &alliance
		row.Own = alliance.ID == ownAllianceID
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

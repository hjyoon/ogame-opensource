package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type SearchRepository struct {
	queryer Queryer
	prefix  string
}

func NewSearchRepository(db *sql.DB, prefix string) SearchRepository {
	return SearchRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewSearchRepositoryWithQueryer(queryer Queryer, prefix string) SearchRepository {
	return SearchRepository{queryer: queryer, prefix: prefix}
}

func (r SearchRepository) GetSearch(ctx context.Context, query appgame.SearchQuery) (domaingame.Search, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Search{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Search{}, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return domaingame.Search{}, err
	}

	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Search{}, err
	}

	searchType := domaingame.NormalizeSearchType(query.Type)
	text := domaingame.NormalizeSearchText(query.Text)
	search := domaingame.Search{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Type:           searchType,
		Text:           text,
	}
	if text == "" {
		return search, nil
	}
	if domaingame.SearchTextTooShort(text) {
		search.Message = "Too few characters! Please enter at least 2 characters."
		return search, nil
	}

	switch searchType {
	case domaingame.SearchTypePlanetName:
		ownAllianceID, loadErr := r.loadViewerAllianceID(ctx, usersTable, query.PlayerID)
		if loadErr != nil {
			return domaingame.Search{}, loadErr
		}
		search.PlayerRows, search.Message, err = r.loadPlanetNameSearchRows(ctx, usersTable, planetsTable, allyTable, query.PlayerID, ownAllianceID, text)
	case domaingame.SearchTypeAllianceTag:
		search.AllianceRows, search.Message, err = r.loadAllianceSearchRows(ctx, allyTable, usersTable, query.PlayerID, text, "tag")
	case domaingame.SearchTypeAllianceName:
		search.AllianceRows, search.Message, err = r.loadAllianceSearchRows(ctx, allyTable, usersTable, query.PlayerID, text, "name")
	default:
		ownAllianceID, loadErr := r.loadViewerAllianceID(ctx, usersTable, query.PlayerID)
		if loadErr != nil {
			return domaingame.Search{}, loadErr
		}
		search.PlayerRows, search.Message, err = r.loadPlayerNameSearchRows(ctx, usersTable, planetsTable, allyTable, query.PlayerID, ownAllianceID, text)
	}
	if err != nil {
		return domaingame.Search{}, err
	}
	return search, nil
}

func (r SearchRepository) loadViewerAllianceID(ctx context.Context, usersTable string, playerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(ally_id, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
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
	var allianceID int
	if err := rows.Scan(&allianceID); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return allianceID, nil
}

func (r SearchRepository) loadPlayerNameSearchRows(ctx context.Context, usersTable string, planetsTable string, allyTable string, playerID int, ownAllianceID int, text string) ([]domaingame.SearchPlayerRow, string, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, COALESCE(u.ally_id, 0), COALESCE(a.tag, ''), COALESCE(p.planet_id, 0), COALESCE(p.name, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), u.place1 FROM %s u LEFT JOIN %s a ON a.ally_id = u.ally_id LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.oname LIKE ? LIMIT ?",
			usersTable,
			allyTable,
			planetsTable,
		),
		"%"+text+"%",
		domaingame.SearchLimit+1,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return scanSearchPlayerRows(rows, playerID, ownAllianceID, domaingame.SearchTypePlayerName)
}

func (r SearchRepository) loadPlanetNameSearchRows(ctx context.Context, usersTable string, planetsTable string, allyTable string, playerID int, ownAllianceID int, text string) ([]domaingame.SearchPlayerRow, string, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, COALESCE(u.ally_id, 0), COALESCE(a.tag, ''), p.planet_id, p.name, p.g, p.s, p.p, u.place1 FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id LEFT JOIN %s a ON a.ally_id = u.ally_id WHERE p.name LIKE ? LIMIT ?",
			planetsTable,
			usersTable,
			allyTable,
		),
		"%"+text+"%",
		domaingame.SearchLimit+1,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return scanSearchPlayerRows(rows, playerID, ownAllianceID, domaingame.SearchTypePlanetName)
}

func scanSearchPlayerRows(rows Rows, playerID int, ownAllianceID int, searchType string) ([]domaingame.SearchPlayerRow, string, error) {
	result := []domaingame.SearchPlayerRow{}
	for rows.Next() {
		var row domaingame.SearchPlayerRow
		var allianceID int
		var allianceTag string
		if err := rows.Scan(
			&row.PlayerID,
			&row.PlayerName,
			&allianceID,
			&allianceTag,
			&row.PlanetID,
			&row.PlanetName,
			&row.Coordinates.Galaxy,
			&row.Coordinates.System,
			&row.Coordinates.Position,
			&row.Place,
		); err != nil {
			return nil, "", err
		}
		if row.PlayerID == playerID {
			row.Own = true
		}
		if allianceID > 0 {
			row.Alliance = &domaingame.StatisticsAlliance{ID: allianceID, Tag: allianceTag}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	message := searchResultMessage(len(result), searchType)
	if len(result) > domaingame.SearchLimit {
		result = result[:domaingame.SearchLimit]
	}
	if ownAllianceID > 0 {
		for i := range result {
			result[i].SameAlliance = result[i].Alliance != nil && result[i].Alliance.ID == ownAllianceID && !result[i].Own
		}
	}
	return result, message, nil
}

func (r SearchRepository) loadAllianceSearchRows(ctx context.Context, allyTable string, usersTable string, playerID int, text string, column string) ([]domaingame.SearchAllianceRow, string, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT a.ally_id, a.tag, a.name, COUNT(u.player_id), a.score1, SUM(CASE WHEN u.player_id = ? THEN 1 ELSE 0 END) FROM %s a LEFT JOIN %s u ON u.ally_id = a.ally_id WHERE a.%s LIKE ? GROUP BY a.ally_id, a.tag, a.name, a.score1 LIMIT ?",
			allyTable,
			usersTable,
			column,
		),
		playerID,
		"%"+text+"%",
		domaingame.SearchLimit+1,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	result := []domaingame.SearchAllianceRow{}
	for rows.Next() {
		var row domaingame.SearchAllianceRow
		var ownCount int
		if err := rows.Scan(&row.AllianceID, &row.Tag, &row.Name, &row.Members, &row.Score, &ownCount); err != nil {
			return nil, "", err
		}
		row.Own = ownCount > 0
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	message := searchResultMessage(len(result), domaingame.SearchTypeAllianceTag)
	if len(result) > domaingame.SearchLimit {
		result = result[:domaingame.SearchLimit]
	}
	return result, message, nil
}

func searchResultMessage(rowCount int, searchType string) string {
	if rowCount == 0 {
		return "no entries found"
	}
	if rowCount > domaingame.SearchLimit {
		return domaingame.SearchOverLimitMessage(searchType)
	}
	return ""
}

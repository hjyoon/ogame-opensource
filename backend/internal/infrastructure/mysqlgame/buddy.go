package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type BuddyRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewBuddyRepository(db *sql.DB, prefix string) BuddyRepository {
	return BuddyRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewBuddyRepositoryWithQueryer(queryer Queryer, prefix string) BuddyRepository {
	return BuddyRepository{queryer: queryer, prefix: prefix, now: time.Now}
}

func (r BuddyRepository) GetBuddy(ctx context.Context, query appgame.BuddyQuery) (domaingame.Buddy, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Buddy{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Buddy{}, err
	}
	buddyTable, err := tableName(r.prefix, "buddy")
	if err != nil {
		return domaingame.Buddy{}, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return domaingame.Buddy{}, err
	}

	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Buddy{}, err
	}

	action := domaingame.NormalizeBuddyAction(query.Action)
	buddy := domaingame.Buddy{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Action:         action,
	}

	switch action {
	case domaingame.BuddyActionIncoming:
		buddy.Rows, err = r.loadBuddyRows(ctx, buddyTable, usersTable, planetsTable, allyTable, "b.request_to = ? AND b.accepted = 0", "b.request_from", query.PlayerID)
	case domaingame.BuddyActionOutgoing:
		buddy.Rows, err = r.loadBuddyRows(ctx, buddyTable, usersTable, planetsTable, allyTable, "b.request_from = ? AND b.accepted = 0", "b.request_to", query.PlayerID)
	case domaingame.BuddyActionRequest:
		if query.BuddyID > 0 {
			buddy.Target, err = r.loadBuddyTarget(ctx, usersTable, planetsTable, allyTable, query.BuddyID)
		}
	default:
		buddy.Rows, err = r.loadBuddyRows(ctx, buddyTable, usersTable, planetsTable, allyTable, "(b.request_from = ? OR b.request_to = ?) AND b.accepted = 1", "CASE WHEN b.request_from = ? THEN b.request_to ELSE b.request_from END", query.PlayerID, query.PlayerID, query.PlayerID)
	}
	if err != nil {
		return domaingame.Buddy{}, err
	}
	return buddy, nil
}

func (r BuddyRepository) loadBuddyRows(ctx context.Context, buddyTable string, usersTable string, planetsTable string, allyTable string, where string, peerExpression string, args ...any) ([]domaingame.BuddyRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT b.buddy_id, u.player_id, u.oname, COALESCE(u.ally_id, 0), COALESCE(a.tag, ''), COALESCE(u.allyrank, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), COALESCE(u.lastclick, 0), COALESCE(b.text, '') FROM %s b JOIN %s u ON u.player_id = %s LEFT JOIN %s a ON a.ally_id = u.ally_id LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE %s ORDER BY b.buddy_id",
			buddyTable,
			usersTable,
			peerExpression,
			allyTable,
			planetsTable,
			where,
		),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domaingame.BuddyRow{}
	nowUnix := r.now().Unix()
	for rows.Next() {
		var row domaingame.BuddyRow
		var allianceID int
		var allianceTag string
		var allianceRank int
		var lastClick int64
		if err := rows.Scan(
			&row.BuddyID,
			&row.Player.PlayerID,
			&row.Player.Name,
			&allianceID,
			&allianceTag,
			&allianceRank,
			&row.Player.Coordinates.Galaxy,
			&row.Player.Coordinates.System,
			&row.Player.Coordinates.Position,
			&lastClick,
			&row.Text,
		); err != nil {
			return nil, err
		}
		if allianceID > 0 {
			row.Player.Alliance = &domaingame.BuddyAlliance{ID: allianceID, Tag: allianceTag, Founder: allianceRank == 0}
		}
		row.Status = domaingame.BuddyOnlineStatus(lastClick, nowUnix)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r BuddyRepository) loadBuddyTarget(ctx context.Context, usersTable string, planetsTable string, allyTable string, playerID int) (*domaingame.BuddyPlayer, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, COALESCE(u.ally_id, 0), COALESCE(a.tag, ''), COALESCE(u.allyrank, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s a ON a.ally_id = u.ally_id LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.player_id = ? LIMIT 1",
			usersTable,
			allyTable,
			planetsTable,
		),
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var player domaingame.BuddyPlayer
	var allianceID int
	var allianceTag string
	var allianceRank int
	if err := rows.Scan(
		&player.PlayerID,
		&player.Name,
		&allianceID,
		&allianceTag,
		&allianceRank,
		&player.Coordinates.Galaxy,
		&player.Coordinates.System,
		&player.Coordinates.Position,
	); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if allianceID > 0 {
		player.Alliance = &domaingame.BuddyAlliance{ID: allianceID, Tag: allianceTag, Founder: allianceRank == 0}
	}
	return &player, nil
}

package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type BuddyRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewBuddyRepository(db *sql.DB, prefix string) BuddyRepository {
	runner := SQLQueryer{DB: db}
	return BuddyRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewBuddyRepositoryWithQueryer(queryer Queryer, prefix string) BuddyRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewBuddyRepositoryWithRunner(queryer, execer, prefix, time.Now)
}

func NewBuddyRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) BuddyRepository {
	if now == nil {
		now = time.Now
	}
	return BuddyRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
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

func (r BuddyRepository) MutateBuddy(ctx context.Context, query appgame.BuddyMutationQuery) (appgame.BuddyMutationOutcome, error) {
	if r.execer == nil {
		return appgame.BuddyMutationOutcome{}, errors.New("buddy updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return appgame.BuddyMutationOutcome{}, err
	}
	buddyTable, err := tableName(r.prefix, "buddy")
	if err != nil {
		return appgame.BuddyMutationOutcome{}, err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return appgame.BuddyMutationOutcome{}, err
	}

	playerName, err := r.loadBuddyUserName(ctx, usersTable, query.PlayerID)
	if err != nil {
		return appgame.BuddyMutationOutcome{}, err
	}

	switch domaingame.NormalizeBuddyMutationAction(query.Action) {
	case domaingame.BuddyActionAdd:
		issue, err := r.addBuddyRequest(ctx, buddyTable, messagesTable, query.PlayerID, query.BuddyID, playerName, query.Text)
		return appgame.BuddyMutationOutcome{NextAction: domaingame.BuddyActionHome, ActionIssue: issue}, err
	case domaingame.BuddyActionAccept:
		if err := r.acceptBuddyRequest(ctx, buddyTable, messagesTable, query.PlayerID, query.BuddyID, playerName); err != nil {
			return appgame.BuddyMutationOutcome{}, err
		}
		return appgame.BuddyMutationOutcome{NextAction: domaingame.BuddyActionIncoming}, nil
	case domaingame.BuddyActionDecline:
		if err := r.rejectBuddyRequest(ctx, buddyTable, messagesTable, query.PlayerID, query.BuddyID, playerName); err != nil {
			return appgame.BuddyMutationOutcome{}, err
		}
		return appgame.BuddyMutationOutcome{NextAction: domaingame.BuddyActionIncoming}, nil
	case domaingame.BuddyActionWithdraw:
		if err := r.withdrawBuddyRequest(ctx, buddyTable, messagesTable, query.PlayerID, query.BuddyID, playerName); err != nil {
			return appgame.BuddyMutationOutcome{}, err
		}
		return appgame.BuddyMutationOutcome{NextAction: domaingame.BuddyActionOutgoing}, nil
	case domaingame.BuddyActionDelete:
		if err := r.deleteBuddyRelation(ctx, buddyTable, messagesTable, query.PlayerID, query.BuddyID, playerName); err != nil {
			return appgame.BuddyMutationOutcome{}, err
		}
	}
	return appgame.BuddyMutationOutcome{NextAction: domaingame.BuddyActionHome}, nil
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

type buddyRecord struct {
	ID          int
	RequestFrom int
	RequestTo   int
	Text        string
	Accepted    int
}

func (r BuddyRepository) addBuddyRequest(ctx context.Context, buddyTable string, messagesTable string, from int, to int, fromName string, text string) (*domaingame.BuddyActionIssue, error) {
	if from == to || to <= 0 {
		return nil, nil
	}
	exists, err := r.buddyRelationshipExists(ctx, buddyTable, from, to)
	if err != nil {
		return nil, err
	}
	if exists {
		return domaingame.BuddyAlreadySentIssue(), nil
	}
	requestText := normalizeBuddyRequestText(text)
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (request_from, request_to, text, accepted) VALUES (?, ?, ?, 0)", buddyTable),
		from,
		to,
		requestText,
	); err != nil {
		return nil, err
	}
	return nil, r.sendBuddyMessage(ctx, messagesTable, to, fromName, "Buddy request", requestText)
}

func (r BuddyRepository) acceptBuddyRequest(ctx context.Context, buddyTable string, messagesTable string, playerID int, buddyID int, playerName string) error {
	buddy, err := r.loadBuddyRecord(ctx, buddyTable, buddyID)
	if err != nil || buddy == nil {
		return err
	}
	if buddy.RequestTo != playerID {
		return nil
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET accepted = 1 WHERE buddy_id = ?", buddyTable), buddyID); err != nil {
		return err
	}
	return r.sendBuddyMessage(ctx, messagesTable, buddy.RequestFrom, "Buddylist", "confirm", fmt.Sprintf("Player %s has accepted you in his or her buddylist.", playerName))
}

func (r BuddyRepository) rejectBuddyRequest(ctx context.Context, buddyTable string, messagesTable string, playerID int, buddyID int, playerName string) error {
	buddy, err := r.loadBuddyRecord(ctx, buddyTable, buddyID)
	if err != nil || buddy == nil {
		return err
	}
	if buddy.RequestTo != playerID {
		return nil
	}
	if err := r.removeBuddy(ctx, buddyTable, buddyID); err != nil {
		return err
	}
	return r.sendBuddyMessage(ctx, messagesTable, buddy.RequestFrom, "Buddylist", "Buddy request", fmt.Sprintf("Player %s has declined your buddy request.", playerName))
}

func (r BuddyRepository) withdrawBuddyRequest(ctx context.Context, buddyTable string, messagesTable string, playerID int, buddyID int, playerName string) error {
	buddy, err := r.loadBuddyRecord(ctx, buddyTable, buddyID)
	if err != nil || buddy == nil {
		return err
	}
	if buddy.RequestFrom != playerID {
		return nil
	}
	if err := r.removeBuddy(ctx, buddyTable, buddyID); err != nil {
		return err
	}
	return r.sendBuddyMessage(ctx, messagesTable, buddy.RequestTo, "Buddylist", "Buddy request", fmt.Sprintf("Player %s Buddy request cancelled.", playerName))
}

func (r BuddyRepository) deleteBuddyRelation(ctx context.Context, buddyTable string, messagesTable string, playerID int, buddyID int, playerName string) error {
	buddy, err := r.loadBuddyRecord(ctx, buddyTable, buddyID)
	if err != nil || buddy == nil {
		return err
	}
	recipient := 0
	if buddy.RequestFrom == playerID {
		recipient = buddy.RequestTo
	}
	if buddy.RequestTo == playerID {
		recipient = buddy.RequestFrom
	}
	if recipient == 0 {
		return nil
	}
	if err := r.removeBuddy(ctx, buddyTable, buddyID); err != nil {
		return err
	}
	return r.sendBuddyMessage(ctx, messagesTable, recipient, "Buddylist", "confirm", fmt.Sprintf("Player %s has deleted you from his or her buddylist.", playerName))
}

func (r BuddyRepository) buddyRelationshipExists(ctx context.Context, buddyTable string, player1 int, player2 int) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT buddy_id FROM %s WHERE ((request_from = ? AND request_to = ?) OR (request_from = ? AND request_to = ?)) LIMIT 1", buddyTable),
		player1,
		player2,
		player2,
		player1,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (r BuddyRepository) loadBuddyRecord(ctx context.Context, buddyTable string, buddyID int) (*buddyRecord, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT buddy_id, request_from, request_to, COALESCE(text, ''), accepted FROM %s WHERE buddy_id = ? LIMIT 1", buddyTable),
		buddyID,
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
	var buddy buddyRecord
	if err := rows.Scan(&buddy.ID, &buddy.RequestFrom, &buddy.RequestTo, &buddy.Text, &buddy.Accepted); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &buddy, nil
}

func (r BuddyRepository) loadBuddyUserName(ctx context.Context, usersTable string, playerID int) (string, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT oname FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", errors.New("buddy user not found")
	}
	var name string
	if err := rows.Scan(&name); err != nil {
		return "", err
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return name, nil
}

func (r BuddyRepository) removeBuddy(ctx context.Context, buddyTable string, buddyID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE buddy_id = ?", buddyTable), buddyID)
	return err
}

func (r BuddyRepository) sendBuddyMessage(ctx context.Context, messagesTable string, ownerID int, from string, subject string, text string) error {
	if ownerID <= 0 {
		return nil
	}
	messageText := truncateRunes(text, 2000)
	count, err := r.countMessages(ctx, messagesTable, ownerID)
	if err != nil {
		return err
	}
	if count >= 127 {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? ORDER BY date ASC LIMIT 1", messagesTable), ownerID); err != nil {
			return err
		}
	}
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, 0, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		from,
		subject,
		messageText,
		r.now().Unix(),
	)
	return err
}

func (r BuddyRepository) countMessages(ctx context.Context, messagesTable string, ownerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ?", messagesTable), ownerID)
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

func normalizeBuddyRequestText(text string) string {
	text = truncateRunes(text, 5000)
	if text == "" {
		return "пусто"
	}
	return text
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	var builder strings.Builder
	count := 0
	for _, r := range value {
		if count >= limit {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return builder.String()
}

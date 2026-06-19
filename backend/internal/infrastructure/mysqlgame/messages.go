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

type MessagesRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewMessagesRepository(db *sql.DB, prefix string) MessagesRepository {
	return MessagesRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewMessagesRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) MessagesRepository {
	if now == nil {
		now = time.Now
	}
	return MessagesRepository{queryer: queryer, prefix: prefix, now: now}
}

func (r MessagesRepository) GetMessages(ctx context.Context, query appgame.MessagesQuery) (domaingame.Messages, error) {
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return domaingame.Messages{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Messages{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Messages{}, err
	}

	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Messages{}, err
	}

	messages := domaingame.Messages{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Action:         domaingame.NormalizeMessagesAction(query.TargetPlayerID),
	}
	if query.TargetPlayerID > 0 {
		compose, err := r.loadComposeTarget(ctx, usersTable, planetsTable, query.TargetPlayerID)
		if err != nil {
			return domaingame.Messages{}, err
		}
		messages.Compose = &compose
		return messages, nil
	}

	commanderActive, err := r.loadCommanderActive(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Messages{}, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, query.PlayerID, domaingame.NormalizeMessagesLimit(commanderActive))
	if err != nil {
		return domaingame.Messages{}, err
	}
	messages.Rows = rows
	return messages, nil
}

func (r MessagesRepository) loadInboxRows(ctx context.Context, messagesTable string, playerID int, limit int) ([]domaingame.Message, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT msg_id, pm, msgfrom, subj, text, shown, date FROM %s WHERE owner_id = ? AND pm <> ? ORDER BY date DESC, msg_id DESC LIMIT ?", messagesTable),
		playerID,
		domaingame.MessageTypeBattleReportText,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []domaingame.Message{}
	for rows.Next() {
		message, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r MessagesRepository) loadComposeTarget(ctx context.Context, usersTable string, planetsTable string, targetPlayerID int) (domaingame.MessageCompose, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, p.g, p.s, p.p FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.player_id = ? LIMIT 1",
			usersTable,
			planetsTable,
		),
		targetPlayerID,
	)
	if err != nil {
		return domaingame.MessageCompose{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.MessageCompose{}, err
		}
		return domaingame.MessageCompose{}, errors.New("message target not found")
	}

	compose := domaingame.MessageCompose{Subject: "no subject", MaxChars: domaingame.MessageComposeMaxChars}
	if err := rows.Scan(
		&compose.Target.PlayerID,
		&compose.Target.Name,
		&compose.Target.Coordinates.Galaxy,
		&compose.Target.Coordinates.System,
		&compose.Target.Coordinates.Position,
	); err != nil {
		return domaingame.MessageCompose{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.MessageCompose{}, err
	}
	return compose, nil
}

func (r MessagesRepository) loadCommanderActive(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("message commander state not found")
	}
	var commanderUntil int64
	if err := rows.Scan(&commanderUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return commanderUntil > r.now().Unix(), nil
}

func scanMessageRow(rows Rows) (domaingame.Message, error) {
	var message domaingame.Message
	var shown int
	if err := rows.Scan(&message.ID, &message.Type, &message.From, &message.Subject, &message.Text, &shown, &message.Date); err != nil {
		return domaingame.Message{}, err
	}
	message.Unread = shown == 0
	message.Reportable = message.Type == domaingame.MessageTypePM
	return message, nil
}

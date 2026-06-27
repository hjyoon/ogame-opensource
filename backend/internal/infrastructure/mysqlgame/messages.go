package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type MessagesRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewMessagesRepository(db *sql.DB, prefix string) MessagesRepository {
	runner := SQLQueryer{DB: db}
	return MessagesRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewMessagesRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) MessagesRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewMessagesRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewMessagesRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) MessagesRepository {
	if now == nil {
		now = time.Now
	}
	return MessagesRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
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
		if query.Subject != "" {
			compose.Subject = query.Subject
		}
		messages.Compose = &compose
		return messages, nil
	}

	retention, err := r.loadMessageRetentionState(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Messages{}, err
	}
	if err := r.deleteExpiredInboxMessages(ctx, messagesTable, query.PlayerID, retention); err != nil {
		return domaingame.Messages{}, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, query.PlayerID, domaingame.NormalizeMessagesLimit(retention.CommanderActive))
	if err != nil {
		return domaingame.Messages{}, err
	}
	if err := r.markInboxRowsRead(ctx, messagesTable, query.PlayerID, rows); err != nil {
		return domaingame.Messages{}, err
	}
	messages.Rows = rows
	return messages, nil
}

func (r MessagesRepository) MutateMessages(ctx context.Context, query appgame.MessagesMutationQuery) (appgame.MessagesMutationOutcome, error) {
	if r.execer == nil {
		return appgame.MessagesMutationOutcome{}, errors.New("messages updater unavailable")
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return appgame.MessagesMutationOutcome{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return appgame.MessagesMutationOutcome{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return appgame.MessagesMutationOutcome{}, err
	}
	reportsTable, err := tableName(r.prefix, "reports")
	if err != nil {
		return appgame.MessagesMutationOutcome{}, err
	}

	switch domaingame.NormalizeMessagesMutationAction(query.Action) {
	case domaingame.MessagesMutationActionSend:
		issue, err := r.sendPrivateMessage(ctx, messagesTable, usersTable, planetsTable, query)
		return appgame.MessagesMutationOutcome{NextTargetPlayerID: query.TargetPlayerID, ActionIssue: issue}, err
	default:
		issue, err := r.mutateInboxMessages(ctx, messagesTable, usersTable, reportsTable, query)
		return appgame.MessagesMutationOutcome{ActionIssue: issue}, err
	}
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

func (r MessagesRepository) mutateInboxMessages(ctx context.Context, messagesTable string, usersTable string, reportsTable string, query appgame.MessagesMutationQuery) (*domaingame.MessageActionIssue, error) {
	deleteMode := domaingame.NormalizeMessageDeleteMode(query.DeleteMode)
	if deleteMode == domaingame.MessageDeleteModeAllMessages {
		_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", messagesTable), query.PlayerID)
		return nil, err
	}

	commanderActive, err := r.loadCommanderActive(ctx, usersTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, query.PlayerID, domaingame.NormalizeMessagesLimit(commanderActive))
	if err != nil {
		return nil, err
	}
	visible := map[int]domaingame.Message{}
	for _, row := range rows {
		visible[row.ID] = row
	}

	var issue *domaingame.MessageActionIssue
	for _, id := range query.ReportIDs {
		row, ok := visible[id]
		if !ok || row.Type != domaingame.MessageTypePM {
			continue
		}
		reportIssue, err := r.reportMessage(ctx, messagesTable, reportsTable, query.PlayerID, id)
		if err != nil {
			return nil, err
		}
		if reportIssue != nil {
			issue = reportIssue
		}
	}

	deleteIDs := r.messageDeleteIDs(deleteMode, rows, query.MessageIDs)
	for _, id := range deleteIDs {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? AND msg_id = ?", messagesTable), query.PlayerID, id); err != nil {
			return nil, err
		}
	}
	return issue, nil
}

func (r MessagesRepository) messageDeleteIDs(mode string, rows []domaingame.Message, selected []int) []int {
	selectedSet := map[int]struct{}{}
	for _, id := range selected {
		selectedSet[id] = struct{}{}
	}
	result := []int{}
	for _, row := range rows {
		_, selected := selectedSet[row.ID]
		if mode == domaingame.MessageDeleteModeMarked && selected {
			result = append(result, row.ID)
		}
		if mode == domaingame.MessageDeleteModeNonMarked && !selected {
			result = append(result, row.ID)
		}
		if mode == domaingame.MessageDeleteModeAllShown {
			result = append(result, row.ID)
		}
	}
	return result
}

func (r MessagesRepository) reportMessage(ctx context.Context, messagesTable string, reportsTable string, playerID int, messageID int) (*domaingame.MessageActionIssue, error) {
	record, err := r.loadOwnedMessage(ctx, messagesTable, playerID, messageID)
	if err != nil || record == nil || record.Type != domaingame.MessageTypePM {
		return nil, err
	}
	exists, err := r.reportExists(ctx, reportsTable, messageID)
	if err != nil {
		return nil, err
	}
	if exists {
		return domaingame.MessageReportExistsIssue(), nil
	}
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, msg_id, msgfrom, subj, text, date) VALUES (?, ?, ?, ?, ?, ?)", reportsTable),
		playerID,
		messageID,
		record.From,
		record.Subject,
		record.Text,
		record.Date,
	)
	if err != nil {
		return nil, err
	}
	return domaingame.MessageReportedIssue(), nil
}

func (r MessagesRepository) reportExists(ctx context.Context, reportsTable string, messageID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE msg_id = ? LIMIT 1", reportsTable), messageID)
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

func (r MessagesRepository) loadOwnedMessage(ctx context.Context, messagesTable string, playerID int, messageID int) (*domaingame.Message, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT msg_id, pm, msgfrom, subj, text, shown, date FROM %s WHERE owner_id = ? AND msg_id = ? LIMIT 1", messagesTable),
		playerID,
		messageID,
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
	record, err := scanMessageRow(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r MessagesRepository) sendPrivateMessage(ctx context.Context, messagesTable string, usersTable string, planetsTable string, query appgame.MessagesMutationQuery) (*domaingame.MessageActionIssue, error) {
	draft := domaingame.NormalizeMessageDraft(query.TargetPlayerID, query.Subject, query.Text)
	if draft.TargetPlayerID <= 0 {
		return nil, nil
	}
	if draft.Subject == "" {
		return domaingame.MessageMissingSubjectIssue(), nil
	}
	if draft.Text == "" {
		return domaingame.MessageMissingTextIssue(), nil
	}

	sender, err := r.loadMessageParticipant(ctx, usersTable, planetsTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	if !sender.Validated {
		return domaingame.MessageNotActivatedIssue(), nil
	}
	recipient, err := r.loadMessageParticipant(ctx, usersTable, planetsTable, draft.TargetPlayerID)
	if err != nil {
		return nil, err
	}

	from := fmt.Sprintf(
		"%s <a href=\"index.php?page=galaxy&galaxy=%d&system=%d&position=%d&session={PUBLIC_SESSION}\">[%d:%d:%d]</a>\n",
		html.EscapeString(sender.Name),
		sender.Coordinates.Galaxy,
		sender.Coordinates.System,
		sender.Coordinates.Position,
		sender.Coordinates.Galaxy,
		sender.Coordinates.System,
		sender.Coordinates.Position,
	)
	escapedSubject := html.EscapeString(draft.Subject)
	replySubject := rawURLEncode("Re:" + draft.Subject)
	subject := fmt.Sprintf(
		"%s <a href=\"index.php?page=writemessages&session={PUBLIC_SESSION}&messageziel=%d&re=1&betreff=%s\">\n<img border=\"0\" alt=\"Reply\" src=\"%simg/m.gif\" /></a>\n",
		escapedSubject,
		query.PlayerID,
		replySubject,
		messageSkinPath(recipient),
	)
	return domaingame.MessageSentIssue(), r.insertPrivateMessage(ctx, messagesTable, draft.TargetPlayerID, from, subject, formatPrivateMessageText(draft.Text))
}

type messageParticipant struct {
	PlayerID    int
	Name        string
	Validated   bool
	SkinEnabled bool
	Skin        string
	Coordinates domaingame.Coordinates
}

func (r MessagesRepository) loadMessageParticipant(ctx context.Context, usersTable string, planetsTable string, playerID int) (messageParticipant, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT u.player_id, u.oname, COALESCE(u.validated, 0), COALESCE(u.useskin, 0), COALESCE(u.skin, ''), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s p ON p.planet_id = u.hplanetid WHERE u.player_id = ? LIMIT 1",
			usersTable,
			planetsTable,
		),
		playerID,
	)
	if err != nil {
		return messageParticipant{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return messageParticipant{}, err
		}
		return messageParticipant{}, errors.New("message participant not found")
	}
	var participant messageParticipant
	var validated int
	var skinEnabled int
	if err := rows.Scan(
		&participant.PlayerID,
		&participant.Name,
		&validated,
		&skinEnabled,
		&participant.Skin,
		&participant.Coordinates.Galaxy,
		&participant.Coordinates.System,
		&participant.Coordinates.Position,
	); err != nil {
		return messageParticipant{}, err
	}
	if err := rows.Err(); err != nil {
		return messageParticipant{}, err
	}
	participant.Validated = validated != 0
	participant.SkinEnabled = skinEnabled != 0
	return participant, nil
}

func (r MessagesRepository) insertPrivateMessage(ctx context.Context, messagesTable string, ownerID int, from string, subject string, text string) error {
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
		fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable),
		ownerID,
		domaingame.MessageTypePM,
		from,
		subject,
		text,
		r.now().Unix(),
	)
	return err
}

func (r MessagesRepository) countMessages(ctx context.Context, messagesTable string, ownerID int) (int, error) {
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

func rawURLEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func messageSkinPath(participant messageParticipant) string {
	if participant.SkinEnabled && participant.Skin != "" {
		return participant.Skin
	}
	return "evolution/"
}

func formatPrivateMessageText(value string) string {
	escaped := html.EscapeString(value)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	escaped = strings.ReplaceAll(escaped, "\r", "\n")
	return strings.ReplaceAll(escaped, "\n", "<br />")
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

	compose := domaingame.MessageCompose{Subject: "No Subject", MaxChars: domaingame.MessageComposeMaxChars}
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

type messageRetentionState struct {
	CommanderActive bool
	Admin           bool
}

func (r MessagesRepository) loadMessageRetentionState(ctx context.Context, usersTable string, playerID int) (messageRetentionState, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until, admin FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return messageRetentionState{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return messageRetentionState{}, err
		}
		return messageRetentionState{}, errors.New("message retention state not found")
	}
	var commanderUntil int64
	var adminLevel int
	if err := rows.Scan(&commanderUntil, &adminLevel); err != nil {
		return messageRetentionState{}, err
	}
	if err := rows.Err(); err != nil {
		return messageRetentionState{}, err
	}
	return messageRetentionState{
		CommanderActive: commanderUntil > r.now().Unix(),
		Admin:           adminLevel > domaingame.AdminLevelPlayer,
	}, nil
}

func (r MessagesRepository) deleteExpiredInboxMessages(ctx context.Context, messagesTable string, playerID int, state messageRetentionState) error {
	if r.execer == nil || state.Admin {
		return nil
	}
	retentionDays := 1
	if state.CommanderActive {
		retentionDays = 7
	}
	expiredBefore := r.now().Unix() - int64(retentionDays*24*60*60)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? AND date <= ?", messagesTable), playerID, expiredBefore)
	return err
}

func (r MessagesRepository) markInboxRowsRead(ctx context.Context, messagesTable string, playerID int, rows []domaingame.Message) error {
	if r.execer == nil || len(rows) == 0 {
		return nil
	}
	args := make([]any, 0, len(rows)+1)
	args = append(args, playerID)
	for _, row := range rows {
		args = append(args, row.ID)
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET shown = 1 WHERE owner_id = ? AND msg_id IN (%s)", messagesTable, placeholders(len(rows))), args...)
	return err
}

func scanMessageRow(rows Rows) (domaingame.Message, error) {
	var message domaingame.Message
	var shown int
	if err := rows.Scan(&message.ID, &message.Type, &message.From, &message.Subject, &message.Text, &shown, &message.Date); err != nil {
		return domaingame.Message{}, err
	}
	message.From = legacyStripSlashes(message.From)
	message.Subject = legacyStripSlashes(message.Subject)
	message.Text = legacyStripSlashes(message.Text)
	message.Unread = shown == 0
	message.Reportable = message.Type == domaingame.MessageTypePM
	return message, nil
}

func legacyStripSlashes(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			builder.WriteByte(value[index])
			continue
		}
		next := value[index+1]
		switch next {
		case '\\', '\'', '"':
			builder.WriteByte(next)
			index++
		case '0':
			builder.WriteByte(0)
			index++
		default:
			builder.WriteByte(value[index])
		}
	}
	return builder.String()
}

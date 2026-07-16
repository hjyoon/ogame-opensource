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
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type MessagesRepository struct {
	queryer          Queryer
	execer           Execer
	prefix           string
	now              func() time.Time
	processDueFleets bool
}

const (
	messageUserFlagDontUseFolders    int64 = 0x20
	messageUserFlagPartialReports    int64 = 0x40
	messageUserFlagFolderEspionage   int64 = 0x100
	messageUserFlagFolderCombat      int64 = 0x200
	messageUserFlagFolderExpedition  int64 = 0x400
	messageUserFlagFolderAlliance    int64 = 0x800
	messageUserFlagFolderPlayer      int64 = 0x1000
	messageUserFlagFolderOther       int64 = 0x2000
	messageUserFlagFolderCategoryAll       = messageUserFlagFolderEspionage |
		messageUserFlagFolderCombat |
		messageUserFlagFolderExpedition |
		messageUserFlagFolderAlliance |
		messageUserFlagFolderPlayer |
		messageUserFlagFolderOther
)

func NewMessagesRepository(db *sql.DB, prefix string) MessagesRepository {
	runner := SQLQueryer{DB: db}
	return MessagesRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, processDueFleets: true}
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
	if r.processDueFleets && r.execer != nil {
		fleets := NewFleetRepositoryWithRunner(r.queryer, r.execer, r.prefix, r.now)
		fleets.legacyEvents = true
		fleets.queueProduction = true
		if err := fleets.FinishDueFleetQueues(ctx, int(r.now().Unix())); err != nil {
			return domaingame.Messages{}, err
		}
	}
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
	uniTable, err := tableName(r.prefix, "uni")
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
		Action:         domaingame.NormalizeMessagesAction(query.TargetPlayerID, query.ShowSummary),
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
	messages.PartialReports = retention.Flags&messageUserFlagPartialReports != 0
	if err := r.deleteExpiredInboxMessages(ctx, messagesTable, query.PlayerID, retention); err != nil {
		return domaingame.Messages{}, err
	}
	showLegacyFolders := query.LegacyFolderDisplay &&
		retention.CommanderActive &&
		retention.Flags&messageUserFlagDontUseFolders == 0
	showSummary := query.ShowSummary || (showLegacyFolders && !query.HasMessageTypeFilter && retention.showLegacyFolderSummaryOnly())
	if showSummary {
		messages.Action = domaingame.MessagesActionSummary
		summary, err := r.loadMessageCategoryCounts(ctx, messagesTable, query.PlayerID)
		if err != nil {
			return domaingame.Messages{}, err
		}
		applyMessageCategoryChecks(summary, retention.Flags, query.HasMessageTypeFilter, query.MessageTypeFilter)
		messages.Summary = summary
		operators, err := r.loadOperators(ctx, usersTable, uniTable, query.PlayerID)
		if err != nil {
			return domaingame.Messages{}, err
		}
		messages.Operators = operators
		return messages, nil
	}
	rows, err := r.loadLegacyInboxRows(ctx, messagesTable, query.PlayerID, domaingame.NormalizeMessagesLimit(retention.CommanderActive), query, showLegacyFolders, retention.Flags)
	if err != nil {
		return domaingame.Messages{}, err
	}
	if showLegacyFolders {
		summary, err := r.loadMessageCategoryCounts(ctx, messagesTable, query.PlayerID)
		if err != nil {
			return domaingame.Messages{}, err
		}
		applyMessageCategoryChecks(summary, retention.Flags, query.HasMessageTypeFilter, query.MessageTypeFilter)
		messages.Summary = summary
	}
	if err := r.markInboxRowsRead(ctx, messagesTable, query.PlayerID, rows); err != nil {
		return domaingame.Messages{}, err
	}
	if messages.PartialReports {
		applyPartialSpyReports(rows)
	}
	messages.Rows = rows
	operators, err := r.loadOperators(ctx, usersTable, uniTable, query.PlayerID)
	if err != nil {
		return domaingame.Messages{}, err
	}
	messages.Operators = operators
	return messages, nil
}

func (r MessagesRepository) loadMessageCategoryCounts(ctx context.Context, messagesTable string, playerID int) ([]domaingame.MessageCategoryCount, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT pm, COUNT(*), COALESCE(SUM(CASE WHEN shown = 0 THEN 1 ELSE 0 END), 0) FROM %s WHERE owner_id = ? AND pm <> ? GROUP BY pm", messagesTable),
		playerID,
		domaingame.MessageTypeBattleReportText,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := defaultMessageCategoryCounts()
	for rows.Next() {
		var messageType int
		var total int
		var unread int
		if err := rows.Scan(&messageType, &total, &unread); err != nil {
			return nil, err
		}
		index := messageCategoryIndex(messageType)
		counts[index].Total += total
		counts[index].Unread += unread
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func defaultMessageCategoryCounts() []domaingame.MessageCategoryCount {
	return []domaingame.MessageCategoryCount{
		{Key: "spy", Label: "Spy Reports"},
		{Key: "battle", Label: "Combat Reports"},
		{Key: "expedition", Label: "Expedition Reports"},
		{Key: "alliance", Label: "Alliance Reports"},
		{Key: "personal", Label: "Personal Messages"},
		{Key: "other", Label: "Other"},
	}
}

func messageCategoryIndex(messageType int) int {
	switch messageType {
	case domaingame.MessageTypeSpyReport:
		return 0
	case domaingame.MessageTypeBattleReportLink:
		return 1
	case domaingame.MessageTypeExpedition:
		return 2
	case domaingame.MessageTypeAlliance:
		return 3
	case domaingame.MessageTypePM:
		return 4
	default:
		return 5
	}
}

func (r MessagesRepository) loadOperators(ctx context.Context, usersTable string, uniTable string, playerID int) ([]domaingame.MessageOperator, error) {
	subject, err := r.loadOperatorSubject(ctx, usersTable, uniTable, playerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(oname, ''), COALESCE(email, ''), COALESCE(flags, 0) FROM %s WHERE admin = 1 ORDER BY player_id ASC", usersTable),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	operators := []domaingame.MessageOperator{}
	for rows.Next() {
		var operator domaingame.MessageOperator
		var flags int64
		if err := rows.Scan(&operator.PlayerID, &operator.Name, &operator.Email, &flags); err != nil {
			return nil, err
		}
		operator.HideEmail = flags&domaingame.UserFlagHideGOEmail != 0
		operator.Subject = subject
		operators = append(operators, operator)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return operators, nil
}

func (r MessagesRepository) loadOperatorSubject(ctx context.Context, usersTable string, uniTable string, playerID int) (string, error) {
	userName := ""
	userRows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(oname, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return "", err
	}
	if userRows.Next() {
		if err := userRows.Scan(&userName); err != nil {
			userRows.Close()
			return "", err
		}
	}
	if err := userRows.Err(); err != nil {
		userRows.Close()
		return "", err
	}
	userRows.Close()

	universeNumber := 1
	uniRows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(num, 1) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return "", err
	}
	if uniRows.Next() {
		if err := uniRows.Scan(&universeNumber); err != nil {
			uniRows.Close()
			return "", err
		}
	}
	if err := uniRows.Err(); err != nil {
		uniRows.Close()
		return "", err
	}
	uniRows.Close()

	return fmt.Sprintf("Question from %s of the %d universe", userName, universeNumber), nil
}

func (r MessagesRepository) MutateMessages(ctx context.Context, query appgame.MessagesMutationQuery) (appgame.MessagesMutationOutcome, error) {
	if r.execer == nil {
		return appgame.MessagesMutationOutcome{}, errors.New("messages updater unavailable")
	}
	if txer, ok := r.execer.(transactionRunner); ok {
		var outcome appgame.MessagesMutationOutcome
		err := txer.WithTransaction(ctx, func(queryer Queryer, execer Execer) error {
			transactionRepository := r
			transactionRepository.queryer = queryer
			transactionRepository.execer = execer
			var err error
			outcome, err = transactionRepository.mutateMessages(ctx, query)
			return err
		})
		return outcome, err
	}
	return r.mutateMessages(ctx, query)
}

func (r MessagesRepository) mutateMessages(ctx context.Context, query appgame.MessagesMutationQuery) (appgame.MessagesMutationOutcome, error) {
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

func (r MessagesRepository) PreviewMCPSendMessage(ctx context.Context, playerID int, command domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error) {
	return r.mcpSendMessage(ctx, playerID, command, false)
}

func (r MessagesRepository) SendMCPMessage(ctx context.Context, playerID int, command domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error) {
	return r.mcpSendMessage(ctx, playerID, command, true)
}

func (r MessagesRepository) PreviewMCPDeleteMessages(ctx context.Context, playerID int, command domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error) {
	return r.mcpDeleteMessages(ctx, playerID, command, false)
}

func (r MessagesRepository) DeleteMCPMessages(ctx context.Context, playerID int, command domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error) {
	return r.mcpDeleteMessages(ctx, playerID, command, true)
}

func (r MessagesRepository) PreviewMCPReportMessage(ctx context.Context, playerID int, command domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error) {
	return r.mcpReportMessage(ctx, playerID, command, false)
}

func (r MessagesRepository) ReportMCPMessage(ctx context.Context, playerID int, command domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error) {
	return r.mcpReportMessage(ctx, playerID, command, true)
}

func (r MessagesRepository) mcpSendMessage(ctx context.Context, playerID int, command domainmcp.SendMessageCommand, execute bool) (domainmcp.SendMessageResult, error) {
	if r.queryer == nil {
		return domainmcp.SendMessageResult{}, errors.New("messages queryer unavailable")
	}
	if execute && r.execer == nil {
		return domainmcp.SendMessageResult{}, errors.New("messages updater unavailable")
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return domainmcp.SendMessageResult{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domainmcp.SendMessageResult{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domainmcp.SendMessageResult{}, err
	}

	draft := domaingame.NormalizeMessageDraft(command.TargetPlayerID, command.Subject, command.Text)
	result := domainmcp.SendMessageResult{
		PlayerID:       playerID,
		TargetPlayerID: draft.TargetPlayerID,
		Subject:        draft.Subject,
		TextChars:      len([]rune(draft.Text)),
		DryRun:         !execute,
	}
	if draft.TargetPlayerID <= 0 {
		return result, nil
	}
	if draft.Subject == "" {
		result.Issue = mcpActionIssue(domaingame.MessageMissingSubjectIssue())
		return result, nil
	}
	if draft.Text == "" {
		result.Issue = mcpActionIssue(domaingame.MessageMissingTextIssue())
		return result, nil
	}

	if !execute {
		sender, err := r.loadMessageParticipant(ctx, usersTable, planetsTable, playerID)
		if err != nil {
			return domainmcp.SendMessageResult{}, err
		}
		if !sender.Validated {
			result.Issue = mcpActionIssue(domaingame.MessageNotActivatedIssue())
			return result, nil
		}
		if _, err := r.loadMessageParticipant(ctx, usersTable, planetsTable, draft.TargetPlayerID); err != nil {
			return domainmcp.SendMessageResult{}, err
		}
		return result, nil
	}

	issue, err := r.sendPrivateMessage(ctx, messagesTable, usersTable, planetsTable, appgame.MessagesMutationQuery{
		PlayerID:       playerID,
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: draft.TargetPlayerID,
		Subject:        draft.Subject,
		Text:           draft.Text,
	})
	if err != nil {
		return domainmcp.SendMessageResult{}, err
	}
	result.Issue = mcpActionIssue(issue)
	result.Executed = issue != nil && issue.Code == domaingame.MessageIssueSent
	return result, nil
}

func mcpActionIssue(issue *domaingame.MessageActionIssue) *domainmcp.ActionIssue {
	if issue == nil {
		return nil
	}
	return &domainmcp.ActionIssue{Code: issue.Code, Message: issue.Message}
}

func (r MessagesRepository) mcpDeleteMessages(ctx context.Context, playerID int, command domainmcp.DeleteMessagesCommand, execute bool) (domainmcp.DeleteMessagesResult, error) {
	if r.queryer == nil {
		return domainmcp.DeleteMessagesResult{}, errors.New("messages queryer unavailable")
	}
	if execute && r.execer == nil {
		return domainmcp.DeleteMessagesResult{}, errors.New("messages updater unavailable")
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return domainmcp.DeleteMessagesResult{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domainmcp.DeleteMessagesResult{}, err
	}
	ids := domaingame.NormalizeMessageIDs(command.MessageIDs)
	result := domainmcp.DeleteMessagesResult{PlayerID: playerID, DryRun: !execute}
	if len(ids) == 0 {
		return result, nil
	}
	commanderActive, err := r.loadCommanderActive(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.DeleteMessagesResult{}, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, playerID, domaingame.NormalizeMessagesLimit(commanderActive), 0, false)
	if err != nil {
		return domainmcp.DeleteMessagesResult{}, err
	}
	deleteIDs := r.messageDeleteIDs(domaingame.MessageDeleteModeMarked, rows, ids)
	result.MessageIDs = deleteIDs
	result.DeleteCount = len(deleteIDs)
	if !execute {
		return result, nil
	}
	if _, err := r.MutateMessages(ctx, appgame.MessagesMutationQuery{
		PlayerID:   playerID,
		Action:     domaingame.MessagesMutationActionDelete,
		DeleteMode: domaingame.MessageDeleteModeMarked,
		MessageIDs: ids,
	}); err != nil {
		return domainmcp.DeleteMessagesResult{}, err
	}
	result.Executed = result.DeleteCount > 0
	return result, nil
}

func (r MessagesRepository) mcpReportMessage(ctx context.Context, playerID int, command domainmcp.ReportMessageCommand, execute bool) (domainmcp.ReportMessageResult, error) {
	if r.queryer == nil {
		return domainmcp.ReportMessageResult{}, errors.New("messages queryer unavailable")
	}
	if execute && r.execer == nil {
		return domainmcp.ReportMessageResult{}, errors.New("messages updater unavailable")
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	reportsTable, err := tableName(r.prefix, "reports")
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	result := domainmcp.ReportMessageResult{PlayerID: playerID, MessageID: command.MessageID, DryRun: !execute}
	if command.MessageID <= 0 {
		return result, nil
	}
	commanderActive, err := r.loadCommanderActive(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, playerID, domaingame.NormalizeMessagesLimit(commanderActive), 0, false)
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	for _, row := range rows {
		if row.ID == command.MessageID && row.Type == domaingame.MessageTypePM {
			result.Reportable = true
			break
		}
	}
	if !result.Reportable {
		return result, nil
	}
	if !execute {
		exists, err := r.reportExists(ctx, reportsTable, command.MessageID)
		if err != nil {
			return domainmcp.ReportMessageResult{}, err
		}
		if exists {
			result.Issue = mcpActionIssue(domaingame.MessageReportExistsIssue())
		}
		return result, nil
	}
	outcome, err := r.MutateMessages(ctx, appgame.MessagesMutationQuery{
		PlayerID:  playerID,
		ReportIDs: []int{command.MessageID},
	})
	if err != nil {
		return domainmcp.ReportMessageResult{}, err
	}
	result.Issue = mcpActionIssue(outcome.ActionIssue)
	result.Executed = result.Issue != nil && result.Issue.Code == domaingame.MessageIssueReported
	return result, nil
}

func (r MessagesRepository) loadInboxRows(ctx context.Context, messagesTable string, playerID int, limit int, messageTypeFilter int, hasMessageTypeFilter bool) ([]domaingame.Message, error) {
	messageTypes := []int{}
	if hasMessageTypeFilter {
		messageTypes = append(messageTypes, messageTypeFilter)
	}
	return r.loadInboxRowsByMessageTypes(ctx, messagesTable, playerID, limit, messageTypes)
}

func (r MessagesRepository) loadLegacyInboxRows(ctx context.Context, messagesTable string, playerID int, limit int, query appgame.MessagesQuery, showLegacyFolders bool, flags int64) ([]domaingame.Message, error) {
	if query.HasMessageTypeFilter {
		return r.loadInboxRowsByMessageTypes(ctx, messagesTable, playerID, limit, []int{query.MessageTypeFilter})
	}
	if showLegacyFolders {
		return r.loadInboxRowsByMessageTypes(ctx, messagesTable, playerID, limit, legacyMessageTypesForFolderFlags(flags))
	}
	return r.loadInboxRowsByMessageTypes(ctx, messagesTable, playerID, limit, nil)
}

func (r MessagesRepository) loadInboxRowsByMessageTypes(ctx context.Context, messagesTable string, playerID int, limit int, messageTypes []int) ([]domaingame.Message, error) {
	statement := fmt.Sprintf("SELECT msg_id, pm, msgfrom, subj, text, shown, date FROM %s WHERE owner_id = ? AND pm <> ?", messagesTable)
	args := []any{playerID, domaingame.MessageTypeBattleReportText}
	if len(messageTypes) == 1 {
		statement += " AND pm = ?"
		args = append(args, messageTypes[0])
	} else if len(messageTypes) > 1 {
		placeholders := make([]string, 0, len(messageTypes))
		for _, messageType := range messageTypes {
			placeholders = append(placeholders, "?")
			args = append(args, messageType)
		}
		statement += " AND pm IN (" + strings.Join(placeholders, ", ") + ")"
	}
	statement += " ORDER BY date DESC, msg_id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := r.queryer.QueryContext(
		ctx,
		statement,
		args...,
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

func applyPartialSpyReports(rows []domaingame.Message) {
	for index := range rows {
		if rows[index].Type != domaingame.MessageTypeSpyReport {
			continue
		}
		rows[index].Subject = fmt.Sprintf(
			"<a href=\"#\" onclick=\"fenster('index.php?page=bericht&session={PUBLIC_SESSION}&bericht=%d', 'Bericht_Spionage');\" >%s</a>",
			rows[index].ID,
			rows[index].Subject,
		)
		rows[index].Text = ""
	}
}

func applyMessageCategoryChecks(counts []domaingame.MessageCategoryCount, flags int64, hasMessageTypeFilter bool, messageTypeFilter int) {
	for index := range counts {
		messageType := messageTypeForMessageCategoryKey(counts[index].Key)
		if hasMessageTypeFilter {
			counts[index].Checked = messageType == messageTypeFilter
			continue
		}
		counts[index].Checked = flags&messageFolderFlagForMessageType(messageType) != 0
	}
}

func legacyMessageTypesForFolderFlags(flags int64) []int {
	messageTypes := []int{}
	for _, messageType := range []int{
		domaingame.MessageTypeSpyReport,
		domaingame.MessageTypeBattleReportLink,
		domaingame.MessageTypeExpedition,
		domaingame.MessageTypeAlliance,
		domaingame.MessageTypePM,
		domaingame.MessageTypeMisc,
	} {
		if flags&messageFolderFlagForMessageType(messageType) != 0 {
			messageTypes = append(messageTypes, messageType)
		}
	}
	return messageTypes
}

func messageTypeForMessageCategoryKey(key string) int {
	switch key {
	case "spy":
		return domaingame.MessageTypeSpyReport
	case "battle":
		return domaingame.MessageTypeBattleReportLink
	case "expedition":
		return domaingame.MessageTypeExpedition
	case "alliance":
		return domaingame.MessageTypeAlliance
	case "personal":
		return domaingame.MessageTypePM
	default:
		return domaingame.MessageTypeMisc
	}
}

func messageFolderFlagForMessageType(messageType int) int64 {
	switch messageType {
	case domaingame.MessageTypeSpyReport:
		return messageUserFlagFolderEspionage
	case domaingame.MessageTypeBattleReportLink:
		return messageUserFlagFolderCombat
	case domaingame.MessageTypeExpedition:
		return messageUserFlagFolderExpedition
	case domaingame.MessageTypeAlliance:
		return messageUserFlagFolderAlliance
	case domaingame.MessageTypePM:
		return messageUserFlagFolderPlayer
	default:
		return messageUserFlagFolderOther
	}
}

func (r MessagesRepository) mutateInboxMessages(ctx context.Context, messagesTable string, usersTable string, reportsTable string, query appgame.MessagesMutationQuery) (*domaingame.MessageActionIssue, error) {
	deleteMode := domaingame.NormalizeMessageDeleteMode(query.DeleteMode)
	if deleteMode == domaingame.MessageDeleteModeAllMessages {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE owner_id = ?", messagesTable), query.PlayerID); err != nil {
			return nil, err
		}
		return nil, r.updateMessageUserFlags(ctx, usersTable, query)
	}

	commanderActive, err := r.loadCommanderActive(ctx, usersTable, query.PlayerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.loadInboxRows(ctx, messagesTable, query.PlayerID, domaingame.NormalizeMessagesLimit(commanderActive), 0, false)
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
	if err := r.updateMessageUserFlags(ctx, usersTable, query); err != nil {
		return nil, err
	}
	return issue, nil
}

func (r MessagesRepository) updateMessageUserFlags(ctx context.Context, usersTable string, query appgame.MessagesMutationQuery) error {
	if query.PartialReports == nil && query.FolderSelection == nil {
		return nil
	}
	state, err := r.loadMessageRetentionState(ctx, usersTable, query.PlayerID)
	if err != nil {
		return err
	}
	flags := state.Flags
	if query.PartialReports != nil {
		if *query.PartialReports {
			flags |= messageUserFlagPartialReports
		} else {
			flags &^= messageUserFlagPartialReports
		}
	}
	if query.FolderSelection != nil && state.CommanderActive && flags&messageUserFlagDontUseFolders == 0 {
		flags = setMessageFolderFlag(flags, messageUserFlagFolderEspionage, query.FolderSelection.Spy)
		flags = setMessageFolderFlag(flags, messageUserFlagFolderCombat, query.FolderSelection.Battle)
		flags = setMessageFolderFlag(flags, messageUserFlagFolderExpedition, query.FolderSelection.Expedition)
		flags = setMessageFolderFlag(flags, messageUserFlagFolderAlliance, query.FolderSelection.Alliance)
		flags = setMessageFolderFlag(flags, messageUserFlagFolderPlayer, query.FolderSelection.Personal)
		flags = setMessageFolderFlag(flags, messageUserFlagFolderOther, query.FolderSelection.Other)
	}
	if flags == state.Flags {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET flags = ? WHERE player_id = ?", usersTable), flags, query.PlayerID)
	return err
}

func setMessageFolderFlag(flags int64, flag int64, enabled bool) int64 {
	if enabled {
		return flags | flag
	}
	return flags &^ flag
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
	Flags           int64
}

func (s messageRetentionState) showLegacyFolderSummaryOnly() bool {
	return s.CommanderActive &&
		s.Flags&messageUserFlagDontUseFolders == 0 &&
		s.Flags&messageUserFlagFolderCategoryAll == 0
}

func (r MessagesRepository) loadMessageRetentionState(ctx context.Context, usersTable string, playerID int) (messageRetentionState, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until, admin, COALESCE(flags, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
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
	var flags int64
	if err := rows.Scan(&commanderUntil, &adminLevel, &flags); err != nil {
		return messageRetentionState{}, err
	}
	if err := rows.Err(); err != nil {
		return messageRetentionState{}, err
	}
	return messageRetentionState{
		CommanderActive: commanderUntil > r.now().Unix(),
		Admin:           adminLevel > domaingame.AdminLevelPlayer,
		Flags:           flags,
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

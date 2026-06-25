package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type FeedRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

type feedUser struct {
	playerID int
	owner    string
	flags    int
	lastFeed int64
}

func NewFeedRepository(db *sql.DB, prefix string) FeedRepository {
	runner := SQLQueryer{DB: db}
	return NewFeedRepositoryWithRunner(runner, runner, prefix, time.Now)
}

func NewFeedRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) FeedRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewFeedRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewFeedRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) FeedRepository {
	if now == nil {
		now = time.Now
	}
	return FeedRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r FeedRepository) GetFeed(ctx context.Context, query appgame.FeedQuery) (domaingame.Feed, error) {
	tables, err := r.feedTables()
	if err != nil {
		return domaingame.Feed{}, err
	}
	feedAge, err := r.loadFeedAge(ctx, tables.uni)
	if err != nil {
		return domaingame.Feed{}, err
	}
	if feedAge < 0 {
		return domaingame.Feed{}, nil
	}
	user, ok, err := r.loadFeedUser(ctx, tables.users, query.FeedID)
	if err != nil || !ok || user.flags&domaingame.UserFlagFeedEnable == 0 {
		return domaingame.Feed{}, err
	}

	lastFeed, err := r.currentFeedTimestamp(ctx, tables.users, user, feedAge)
	if err != nil {
		return domaingame.Feed{}, err
	}
	messages, err := r.loadFeedMessages(ctx, tables.messages, user.playerID, lastFeed)
	if err != nil {
		return domaingame.Feed{}, err
	}
	return domaingame.Feed{
		FeedID:   query.FeedID,
		Owner:    user.owner,
		LastFeed: lastFeed,
		Atom:     user.flags&domaingame.UserFlagFeedAtom != 0,
		Messages: messages,
	}, nil
}

func (r FeedRepository) GetFeedItem(ctx context.Context, query appgame.FeedItemQuery) (domaingame.FeedItem, error) {
	tables, err := r.feedTables()
	if err != nil {
		return domaingame.FeedItem{}, err
	}
	feedAge, err := r.loadFeedAge(ctx, tables.uni)
	if err != nil {
		return domaingame.FeedItem{}, err
	}
	if feedAge < 0 {
		return domaingame.FeedItem{}, nil
	}
	user, ok, err := r.loadFeedUser(ctx, tables.users, query.FeedID)
	if err != nil || !ok || user.flags&domaingame.UserFlagFeedEnable == 0 {
		return domaingame.FeedItem{}, err
	}
	item, ok, err := r.loadFeedItem(ctx, tables.messages, query.MessageID)
	if err != nil || !ok {
		return domaingame.FeedItem{}, err
	}
	if item.ownerID != user.playerID || (user.lastFeed != 0 && item.date > user.lastFeed) {
		return domaingame.FeedItem{}, nil
	}
	return domaingame.FeedItem{Subject: item.subject, Text: item.text}, nil
}

type feedTables struct {
	uni      string
	users    string
	messages string
}

func (r FeedRepository) feedTables() (feedTables, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return feedTables{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return feedTables{}, err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return feedTables{}, err
	}
	return feedTables{uni: uniTable, users: usersTable, messages: messagesTable}, nil
}

func (r FeedRepository) loadFeedAge(ctx context.Context, uniTable string) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(feedage, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var feedAge int
	if err := rows.Scan(&feedAge); err != nil {
		return 0, err
	}
	return feedAge, rows.Err()
}

func (r FeedRepository) loadFeedUser(ctx context.Context, usersTable string, feedID string) (feedUser, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(oname, ''), COALESCE(flags, 0), COALESCE(lastfeed, 0) FROM %s WHERE feedid = ? LIMIT 1", usersTable),
		feedID,
	)
	if err != nil {
		return feedUser{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return feedUser{}, false, rows.Err()
	}
	var user feedUser
	if err := rows.Scan(&user.playerID, &user.owner, &user.flags, &user.lastFeed); err != nil {
		return feedUser{}, false, err
	}
	return user, true, rows.Err()
}

func (r FeedRepository) currentFeedTimestamp(ctx context.Context, usersTable string, user feedUser, feedAge int) (int64, error) {
	now := r.now().Unix()
	if now < user.lastFeed+int64(feedAge*60) {
		return user.lastFeed, nil
	}
	if r.execer == nil {
		return 0, fmt.Errorf("feed updater unavailable")
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET lastfeed = ? WHERE player_id = ?", usersTable),
		now,
		user.playerID,
	); err != nil {
		return 0, err
	}
	return now, nil
}

func (r FeedRepository) loadFeedMessages(ctx context.Context, messagesTable string, playerID int, lastFeed int64) ([]domaingame.FeedMessage, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT msg_id, subj, text, date FROM %s WHERE owner_id = ? AND date < ? AND pm <> ? ORDER BY date DESC, msg_id DESC LIMIT ?", messagesTable),
		playerID,
		lastFeed,
		domaingame.MessageTypeBattleReportText,
		domaingame.FeedMaxMessages,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []domaingame.FeedMessage{}
	for rows.Next() {
		var message domaingame.FeedMessage
		if err := rows.Scan(&message.ID, &message.Subject, &message.Text, &message.Date); err != nil {
			return nil, err
		}
		message.Subject = legacyStripSlashes(message.Subject)
		message.Text = legacyStripSlashes(message.Text)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

type feedItemRow struct {
	ownerID int
	subject string
	text    string
	date    int64
}

func (r FeedRepository) loadFeedItem(ctx context.Context, messagesTable string, messageID int) (feedItemRow, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT owner_id, subj, text, date FROM %s WHERE msg_id = ? LIMIT 1", messagesTable),
		messageID,
	)
	if err != nil {
		return feedItemRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return feedItemRow{}, false, rows.Err()
	}
	var item feedItemRow
	if err := rows.Scan(&item.ownerID, &item.subject, &item.text, &item.date); err != nil {
		return feedItemRow{}, false, err
	}
	item.subject = legacyStripSlashes(item.subject)
	item.text = legacyStripSlashes(item.text)
	return item, true, rows.Err()
}

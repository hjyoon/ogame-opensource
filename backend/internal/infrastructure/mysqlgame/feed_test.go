package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFeedRepositoryReadsRSSAndUpdatesLastfeed(t *testing.T) {
	runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{5})},
		{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(100)})},
		{rows: fakeRowsFromValues(
			[]any{11, `Subject \"One\"`, "<a href=\"x\">Hello&nbsp;World</a>", int64(900)},
			[]any{10, "Older", "Plain", int64(800)},
		)},
	}}}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })

	feed, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"})
	if err != nil {
		t.Fatalf("GetFeed returned error: %v", err)
	}
	if feed.Owner != "Legor" || feed.LastFeed != 1000 || feed.Atom {
		t.Fatalf("unexpected feed metadata: %+v", feed)
	}
	if len(feed.Messages) != 2 || feed.Messages[0].Subject != `Subject "One"` || feed.Messages[0].Text != "<a href=\"x\">Hello&nbsp;World</a>" {
		t.Fatalf("unexpected messages: %+v", feed.Messages)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "UPDATE `ogame_users` SET lastfeed") {
		t.Fatalf("expected lastfeed update, got %+v", runner.execs)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(runner.calls))
	}
	args := runner.calls[2].args
	if len(args) != 4 || args[0] != 42 || args[1] != int64(1000) || args[2] != domaingame.MessageTypeBattleReportText || args[3] != domaingame.FeedMaxMessages {
		t.Fatalf("unexpected message query args: %+v", args)
	}
}

func TestFeedRepositoryReadsFreshAtomWithoutUpdating(t *testing.T) {
	runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{5})},
		{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable | domaingame.UserFlagFeedAtom, int64(900)})},
		{rows: fakeRowsFromValues([]any{11, "Subject", "Text", int64(800)})},
	}}}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })

	feed, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"})
	if err != nil {
		t.Fatalf("GetFeed returned error: %v", err)
	}
	if !feed.Atom || feed.LastFeed != 900 || len(runner.execs) != 0 {
		t.Fatalf("fresh atom feed should not update lastfeed: feed=%+v execs=%+v", feed, runner.execs)
	}
}

func TestFeedRepositoryRejectsDisabledFeed(t *testing.T) {
	runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{5})},
		{rows: fakeRowsFromValues([]any{42, "Legor", 0, int64(900)})},
	}}}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })

	feed, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"})
	if err != nil {
		t.Fatalf("GetFeed returned error: %v", err)
	}
	if feed.FeedID != "" || len(runner.execs) != 0 || len(runner.calls) != 2 {
		t.Fatalf("disabled feed should return empty without message query: feed=%+v calls=%d execs=%+v", feed, len(runner.calls), runner.execs)
	}
}

func TestFeedRepositoryHonorsGlobalFeedDisable(t *testing.T) {
	runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{-1})},
	}}}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	feed, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"})
	if err != nil {
		t.Fatalf("GetFeed returned error: %v", err)
	}
	if feed.FeedID != "" || len(runner.calls) != 1 {
		t.Fatalf("global feed disable should stop at universe query: feed=%+v calls=%d", feed, len(runner.calls))
	}
}

func TestFeedRepositoryReadsFeedItemWhenOwnerAndLastfeedAllow(t *testing.T) {
	runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{5})},
		{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(1000)})},
		{rows: fakeRowsFromValues([]any{42, `Subj \"safe\"`, "<b>Body</b>", int64(900)})},
	}}}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	item, err := repository.GetFeedItem(context.Background(), appgame.FeedItemQuery{FeedID: "abcdef", MessageID: 11})
	if err != nil {
		t.Fatalf("GetFeedItem returned error: %v", err)
	}
	if item.Subject != `Subj "safe"` || item.Text != "<b>Body</b>" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestFeedRepositoryRejectsForeignOrFutureFeedItem(t *testing.T) {
	tests := []struct {
		name string
		row  []any
	}{
		{name: "foreign owner", row: []any{43, "Subject", "Text", int64(900)}},
		{name: "future item", row: []any{42, "Subject", "Text", int64(1001)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(1000)})},
				{rows: fakeRowsFromValues(tt.row)},
			}}}
			repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			item, err := repository.GetFeedItem(context.Background(), appgame.FeedItemQuery{FeedID: "abcdef", MessageID: 11})
			if err != nil {
				t.Fatalf("GetFeedItem returned error: %v", err)
			}
			if item.Subject != "" || item.Text != "" {
				t.Fatalf("expected empty item, got %+v", item)
			}
		})
	}
}

func TestFeedRepositoryErrors(t *testing.T) {
	if _, err := NewFeedRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", time.Now).GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"}); err == nil {
		t.Fatalf("expected bad prefix error")
	}
	runner := &fakeFeedRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{5})},
			{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(100)})},
		}},
		execErr: errors.New("update failed"),
	}
	repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })
	if _, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"}); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestFeedRepositoryConstructors(t *testing.T) {
	repository := NewFeedRepository(nil, "ogame_")
	if repository.prefix != "ogame_" || repository.queryer == nil || repository.execer == nil || repository.now == nil {
		t.Fatalf("unexpected NewFeedRepository result: %+v", repository)
	}
	queryerOnly := &fakeQueryer{}
	repository = NewFeedRepositoryWithQueryer(queryerOnly, "ogame_", nil)
	if repository.execer != nil || repository.now == nil {
		t.Fatalf("queryer-only constructor should not infer execer and should set default clock: %+v", repository)
	}
	runner := &fakeFeedRunner{}
	repository = NewFeedRepositoryWithQueryer(runner, "ogame_", time.Now)
	if repository.execer == nil {
		t.Fatalf("runner constructor should infer execer: %+v", repository)
	}
}

func TestFeedRepositoryRequiresUpdaterWhenLastfeedIsDue(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{5})},
		{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(100)})},
	}}
	repository := NewFeedRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return time.Unix(1000, 0) })
	if _, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"}); err == nil || !strings.Contains(err.Error(), "feed updater unavailable") {
		t.Fatalf("expected updater error, got %v", err)
	}
}

func TestFeedRepositoryGetFeedErrorBranches(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		wantErr string
	}{
		{
			name:    "feed age query error",
			results: []fakeQueryResult{{err: errors.New("feedage failed")}},
			wantErr: "feedage failed",
		},
		{
			name: "feed age scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"bad"})},
			},
			wantErr: "expected int",
		},
		{
			name: "user query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{err: errors.New("user failed")},
			},
			wantErr: "user failed",
		},
		{
			name: "user scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{"bad", "Legor", 0, int64(0)})},
			},
			wantErr: "expected int",
		},
		{
			name: "messages query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(900)})},
				{err: errors.New("messages failed")},
			},
			wantErr: "messages failed",
		},
		{
			name: "messages scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(900)})},
				{rows: fakeRowsFromValues([]any{"bad", "Subject", "Text", int64(800)})},
			},
			wantErr: "expected int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFeedRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFeedRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })
			if _, err := repository.GetFeed(context.Background(), appgame.FeedQuery{FeedID: "abcdef"}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFeedRepositoryGetFeedItemEmptyBranches(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
	}{
		{
			name:    "global feed disabled",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{-1})}},
		},
		{
			name: "missing user",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "feed disabled",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", 0, int64(1000)})},
			},
		},
		{
			name: "missing item",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(1000)})},
				{rows: fakeRowsFromValues()},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewFeedRepositoryWithRunner(&fakeFeedRunner{fakeQueryer: fakeQueryer{results: tt.results}}, nil, "ogame_", time.Now)
			item, err := repository.GetFeedItem(context.Background(), appgame.FeedItemQuery{FeedID: "abcdef", MessageID: 11})
			if err != nil || item.Subject != "" || item.Text != "" {
				t.Fatalf("expected empty item without error, got %+v err=%v", item, err)
			}
		})
	}
}

func TestFeedRepositoryGetFeedItemErrorBranches(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		wantErr string
	}{
		{
			name:    "bad prefix",
			results: nil,
			wantErr: "invalid database table prefix",
		},
		{
			name:    "feed age query error",
			results: []fakeQueryResult{{err: errors.New("feedage failed")}},
			wantErr: "feedage failed",
		},
		{
			name: "user query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{err: errors.New("user failed")},
			},
			wantErr: "user failed",
		},
		{
			name: "item query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(1000)})},
				{err: errors.New("item failed")},
			},
			wantErr: "item failed",
		},
		{
			name: "item scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{5})},
				{rows: fakeRowsFromValues([]any{42, "Legor", domaingame.UserFlagFeedEnable, int64(1000)})},
				{rows: fakeRowsFromValues([]any{"bad", "Subject", "Text", int64(900)})},
			},
			wantErr: "expected int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := "ogame_"
			if tt.name == "bad prefix" {
				prefix = "bad-prefix_"
			}
			repository := NewFeedRepositoryWithRunner(&fakeFeedRunner{fakeQueryer: fakeQueryer{results: tt.results}}, nil, prefix, time.Now)
			if _, err := repository.GetFeedItem(context.Background(), appgame.FeedItemQuery{FeedID: "abcdef", MessageID: 11}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFeedRepositoryPostRowErrorBranches(t *testing.T) {
	repository := NewFeedRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("feedage rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadFeedAge(context.Background(), "`ogame_uni`"); err == nil || !strings.Contains(err.Error(), "feedage rows failed") {
		t.Fatalf("expected feedage rows error, got %v", err)
	}

	repository = NewFeedRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("feed user rows failed"))}}}, "ogame_", time.Now)
	if _, _, err := repository.loadFeedUser(context.Background(), "`ogame_users`", "abcdef"); err == nil || !strings.Contains(err.Error(), "feed user rows failed") {
		t.Fatalf("expected feed user rows error, got %v", err)
	}

	repository = NewFeedRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("feed messages rows failed"), []any{11, "Subject", "Text", int64(900)})}}}, "ogame_", time.Now)
	if _, err := repository.loadFeedMessages(context.Background(), "`ogame_messages`", 42, 1_000); err == nil || !strings.Contains(err.Error(), "feed messages rows failed") {
		t.Fatalf("expected feed messages rows error, got %v", err)
	}

	repository = NewFeedRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("feed item rows failed"))}}}, "ogame_", time.Now)
	if _, _, err := repository.loadFeedItem(context.Background(), "`ogame_messages`", 11); err == nil || !strings.Contains(err.Error(), "feed item rows failed") {
		t.Fatalf("expected feed item rows error, got %v", err)
	}

	repository = NewFeedRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("feed item post rows failed"), []any{42, "Subject", "Text", int64(900)})}}}, "ogame_", time.Now)
	if _, _, err := repository.loadFeedItem(context.Background(), "`ogame_messages`", 11); err == nil || !strings.Contains(err.Error(), "feed item post rows failed") {
		t.Fatalf("expected feed item post rows error, got %v", err)
	}
}

type fakeFeedExec struct {
	sql  string
	args []any
}

type fakeFeedRunner struct {
	fakeQueryer
	execs   []fakeFeedExec
	execErr error
}

func (f *fakeFeedRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, fakeFeedExec{sql: query, args: args})
	if f.execErr != nil {
		return nil, f.execErr
	}
	return fakeSQLResult(1), nil
}

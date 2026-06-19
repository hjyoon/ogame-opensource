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

func TestOptionsRepositoryReadsLegacyOptions(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"en", 0, 60})},
	)}
	repository := NewOptionsRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	options, err := repository.GetOptions(context.Background(), appgame.OptionsQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if options.User.Name != "Legor" || options.Settings.MaxSpy != 5 || !options.Flags.ShowEspionageButton || options.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected options: %+v", options)
	}
	if !strings.Contains(queryer.calls[4].sql, "maxfleetmsg") || queryer.calls[4].args[0] != 42 {
		t.Fatalf("expected options user query, got %+v", queryer.calls[4])
	}
}

func TestOptionsRepositoryUpdatesLegacyOptionsAndQueuesDeletion(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResults(now, 0, 0), optionsReadResults(now, 1, now.Add(7*24*time.Hour).Unix())...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "fr",
			SkinPath:         "http://127.0.0.1:8890/evolution",
			UseSkin:          true,
			DeactivateIP:     true,
			SortBy:           999,
			SortOrder:        -42,
			MaxSpy:           -1,
			MaxFleetMessages: 999,
			DeleteAccount:    true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueAccountDeletionQueued || !options.Account.DeletionQueued {
		t.Fatalf("unexpected update result: options=%+v issue=%+v", options, issue)
	}
	if !strings.Contains(runner.execSQL, "UPDATE `ogame_users` SET skin = ?") || len(runner.execArgs) != 11 {
		t.Fatalf("unexpected update SQL: %s args=%+v", runner.execSQL, runner.execArgs)
	}
	if runner.execArgs[0] != "/evolution/" || runner.execArgs[3] != 2 || runner.execArgs[4] != 0 ||
		runner.execArgs[5] != 1 || runner.execArgs[6] != 99 || runner.execArgs[7] != "fr" ||
		runner.execArgs[8] != 1 || runner.execArgs[9] != now.Add(7*24*time.Hour).Unix() {
		t.Fatalf("unexpected update args: %+v", runner.execArgs)
	}
}

func TestOptionsRepositoryKeepsExistingDeletionDateAndClearsDeletion(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	existingDeletion := now.Add(3 * 24 * time.Hour).Unix()
	results := append(optionsReadResults(now, 1, existingDeletion), optionsReadResults(now, 1, existingDeletion)...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			DeleteAccount:    true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueSaved || runner.execArgs[9] != existingDeletion {
		t.Fatalf("expected existing deletion date to be preserved, issue=%+v args=%+v", issue, runner.execArgs)
	}

	results = append(optionsReadResults(now, 1, existingDeletion), optionsReadResults(now, 0, 0)...)
	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, issue, err = repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueAccountDeletionClear || runner.execArgs[8] != 0 || runner.execArgs[9] != int64(0) {
		t.Fatalf("expected deletion clear update, issue=%+v args=%+v", issue, runner.execArgs)
	}
}

func TestOptionsRepositoryReturnsErrors(t *testing.T) {
	if _, _, err := NewOptionsRepositoryWithRunner(&fakeOptionsRunner{}, nil, "ogame_", time.Now).UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{}); err == nil || !strings.Contains(err.Error(), "options updater unavailable") {
		t.Fatalf("expected missing updater error, got %v", err)
	}
	if _, err := NewOptionsRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", time.Now).GetOptions(context.Background(), appgame.OptionsQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: optionsReadResults(time.Unix(1_700_000_000, 0), 0, 0)}, execErr: errors.New("update failed")}
	if _, _, err := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", time.Now).UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected exec error, got %v", err)
	}
}

func TestOptionsRepositoryLoadErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{
			name:    "overview query",
			results: []fakeQueryResult{{err: errors.New("overview failed")}},
			want:    "overview failed",
		},
		{
			name:    "user query",
			results: append(optionsOverviewResults(), fakeQueryResult{err: errors.New("user failed")}),
			want:    "user failed",
		},
		{
			name:    "missing user",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()}),
			want:    "options user not found",
		},
		{
			name:    "user rows",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), optionsUserRow(now, 0, 0))}),
			want:    "user rows failed",
		},
		{
			name:    "universe query",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))}, fakeQueryResult{err: errors.New("uni failed")}),
			want:    "uni failed",
		},
		{
			name:    "missing universe",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))}, fakeQueryResult{rows: fakeRowsFromValues()}),
			want:    "options universe not found",
		},
		{
			name:    "universe rows",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))}, fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("uni rows failed"), []any{"en", 0, 60})}),
			want:    "uni rows failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_", func() time.Time { return now }).GetOptions(context.Background(), appgame.OptionsQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestNewOptionsRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewOptionsRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	if repository.now == nil {
		t.Fatal("expected default clock")
	}
	repository = NewOptionsRepositoryWithQueryer(&fakeOptionsRunner{}, "ogame_", nil)
	if repository.execer == nil || repository.now == nil {
		t.Fatalf("expected runner execer and default clock, got %+v", repository)
	}
}

func optionsReadResults(now time.Time, deletionQueued int, deletionAt int64) []fakeQueryResult {
	return append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, deletionQueued, deletionAt))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"en", 0, 60})},
	)
}

func optionsOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 1000.0, 2000.0, 3000.0, 10000, 10000, 10000})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
	}
}

func optionsUserRow(now time.Time, deletionQueued int, deletionAt int64) []any {
	return []any{
		"Legor", 0, "legor@example.test", "permanent@example.test", 1, "en", "/evolution/", 1, 0, 1, 1, 5, 8,
		int64(0x1), 0, 0, int64(0), deletionQueued, deletionAt, now.Add(time.Hour).Unix(), "feedid",
	}
}

type fakeOptionsRunner struct {
	fakeQueryer
	execSQL  string
	execArgs []any
	execErr  error
}

func (f *fakeOptionsRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execSQL = query
	f.execArgs = args
	return fakeSQLResult(1), f.execErr
}

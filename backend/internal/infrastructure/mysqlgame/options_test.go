package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
		fakeQueryResult{rows: fakeRowsFromValues([]any{"en", 0, 60, 128})},
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
	if !strings.Contains(runner.execSQL, "UPDATE `ogame_users` SET skin = ?") || len(runner.execArgs) != 13 {
		t.Fatalf("unexpected update SQL: %s args=%+v", runner.execSQL, runner.execArgs)
	}
	if runner.execArgs[0] != "/evolution/" || runner.execArgs[3] != 2 || runner.execArgs[4] != 0 ||
		runner.execArgs[5] != 1 || runner.execArgs[6] != 99 || runner.execArgs[7] != "fr" ||
		runner.execArgs[8] != 0 || runner.execArgs[9] != int64(0) ||
		runner.execArgs[10] != 1 || runner.execArgs[11] != now.Add(7*24*time.Hour).Unix() {
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
	if issue == nil || issue.Code != domaingame.OptionsIssueSaved || runner.execArgs[11] != existingDeletion {
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
	if issue == nil || issue.Code != domaingame.OptionsIssueAccountDeletionClear || runner.execArgs[10] != 0 || runner.execArgs[11] != int64(0) {
		t.Fatalf("expected deletion clear update, issue=%+v args=%+v", issue, runner.execArgs)
	}
}

func TestOptionsRepositoryEnablesVacationAndDisablesProduction(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	vacationUntil := now.Unix() + 12*60*60
	results := append(optionsReadResults(now, 0, 0),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
	)
	results = append(results, optionsReadResultsWithVacation(now, 0, 0, 1, vacationUntil)...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     true,
			VacationModeSet:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueVacationEnabled || !options.Account.Vacation || options.Account.VacationUntil != vacationUntil {
		t.Fatalf("unexpected vacation enable result: options=%+v issue=%+v", options, issue)
	}
	if len(runner.execs) != 2 || !strings.Contains(runner.execs[1].sql, "prod1 = 0") || runner.execs[1].args[0] != 42 {
		t.Fatalf("expected vacation production reset after user update, execs=%+v", runner.execs)
	}
	if runner.execs[0].args[8] != 1 || runner.execs[0].args[9] != vacationUntil {
		t.Fatalf("expected vacation user fields, args=%+v", runner.execs[0].args)
	}
}

func TestOptionsRepositoryBlocksVacationWhenQueueActive(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResults(now, 0, 0),
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	results = append(results, optionsReadResults(now, 0, 0)...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     true,
			VacationModeSet:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueVacationBlocked || options.Account.Vacation {
		t.Fatalf("unexpected blocked vacation result: options=%+v issue=%+v", options, issue)
	}
	if len(runner.execs) != 1 || runner.execs[0].args[8] != 0 {
		t.Fatalf("blocked vacation should only save non-vacation fields, execs=%+v", runner.execs)
	}
}

func TestOptionsRepositoryDisablesVacationAfterMinimumAndLocksBeforeMinimum(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	expiredUntil := now.Add(-time.Minute).Unix()
	results := append(optionsReadResultsWithVacation(now, 0, 0, 1, expiredUntil), optionsReadResults(now, 0, 0)...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     false,
			VacationModeSet:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueVacationDisabled || options.Account.Vacation {
		t.Fatalf("unexpected vacation disable result: options=%+v issue=%+v", options, issue)
	}
	if runner.execs[0].args[8] != 0 || runner.execs[0].args[9] != int64(0) {
		t.Fatalf("expected vacation user fields cleared, args=%+v", runner.execs[0].args)
	}

	lockedUntil := now.Add(time.Hour).Unix()
	results = append(optionsReadResultsWithVacation(now, 0, 0, 1, lockedUntil), optionsReadResultsWithVacation(now, 0, 0, 1, lockedUntil)...)
	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	options, issue, err = repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     false,
			VacationModeSet:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueVacationLocked || !options.Account.Vacation {
		t.Fatalf("unexpected vacation locked result: options=%+v issue=%+v", options, issue)
	}
	if runner.execs[0].args[8] != 1 || runner.execs[0].args[9] != lockedUntil {
		t.Fatalf("expected vacation user fields preserved, args=%+v", runner.execs[0].args)
	}
}

func TestOptionsRepositoryChangesPasswordAndLogsOutPublicSession(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret")),
		optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("newpass123", "secret"))...,
	)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })

	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:          "en",
			SkinPath:          "/evolution/",
			MaxSpy:            5,
			MaxFleetMessages:  8,
			OldPassword:       "oldpass123",
			NewPassword:       "newpass123",
			NewPasswordRepeat: "newpass123",
			Email:             "permanent@example.test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssuePasswordChanged {
		t.Fatalf("expected password changed issue, got %+v", issue)
	}
	if len(runner.execs) < 2 || !strings.Contains(runner.execs[0].sql, "password = ?") || !strings.Contains(runner.execs[0].sql, "session = ''") {
		t.Fatalf("expected password update before settings update, execs=%+v", runner.execs)
	}
	if runner.execs[0].args[0] != legacyPasswordHash("newpass123", "secret") || runner.execs[0].args[1] != 42 {
		t.Fatalf("unexpected password update args: %+v", runner.execs[0].args)
	}
}

func TestOptionsRepositoryRejectsPasswordAndEmailCredentialErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name     string
		mutation domaingame.OptionsMutation
		want     string
	}{
		{
			name: "password mismatch",
			mutation: domaingame.OptionsMutation{
				NewPassword: "newpass123", NewPasswordRepeat: "different",
			},
			want: domaingame.OptionsIssuePasswordMismatch,
		},
		{
			name: "wrong old password",
			mutation: domaingame.OptionsMutation{
				OldPassword: "badpass123", NewPassword: "newpass123", NewPasswordRepeat: "newpass123",
			},
			want: domaingame.OptionsIssuePasswordWrongOld,
		},
		{
			name: "email needs password",
			mutation: domaingame.OptionsMutation{
				Email: "new@example.test",
			},
			want: domaingame.OptionsIssueEmailNeedPassword,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutation := tt.mutation
			mutation.Language = "en"
			mutation.SkinPath = "/evolution/"
			mutation.MaxSpy = 5
			mutation.MaxFleetMessages = 8
			results := append(optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret")),
				optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret"))...,
			)
			runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
			repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
			_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42, PlanetID: 99, Mutation: mutation})
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.want {
				t.Fatalf("expected issue %q, got %+v", tt.want, issue)
			}
			if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "UPDATE `ogame_users` SET skin = ?") {
				t.Fatalf("credential rejection should only persist regular settings, execs=%+v", runner.execs)
			}
		})
	}
}

func TestOptionsRepositoryChangesEmailAndQueuesPermanentUpdate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret")),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
	)
	results = append(results, optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret"))...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })

	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			OldPassword:      "oldpass123",
			Email:            "new@example.test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueEmailChanged {
		t.Fatalf("expected email changed issue, got %+v", issue)
	}
	if len(runner.execs) < 4 ||
		!strings.Contains(runner.execs[0].sql, "validated = 0") ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_queue`") ||
		!strings.Contains(runner.execs[2].sql, "INSERT INTO `ogame_queue`") {
		t.Fatalf("expected email update, queue delete, queue insert before settings update, execs=%+v", runner.execs)
	}
	if runner.execs[0].args[0] != legacyPasswordHash(fmt.Sprintf("%d", now.Unix()), "secret") || runner.execs[0].args[1] != "new@example.test" {
		t.Fatalf("unexpected email update args: %+v", runner.execs[0].args)
	}
	if runner.execs[2].args[0] != 42 || runner.execs[2].args[1] != "ChangeEmail" || runner.execs[2].args[6] != now.Unix()+(now.Unix()+7*24*60*60) {
		t.Fatalf("unexpected change-email queue args: %+v", runner.execs[2].args)
	}
}

func TestOptionsRepositoryCredentialMutationErrorBranches(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	current := domaingame.NewOptions(domaingame.Overview{}, domaingame.OptionsUser{
		Email:        "legor@example.test",
		PlainEmail:   "permanent@example.test",
		Validated:    true,
		PasswordHash: legacyPasswordHash("oldpass123", "secret"),
	}, domaingame.OptionsUniverse{Language: "en"}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0)

	runner := &fakeOptionsRunner{execErr: errors.New("password update failed")}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	_, err := repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		OldPassword: "oldpass123", NewPassword: "newpass123", NewPasswordRepeat: "newpass123",
	}, current)
	if err == nil || !strings.Contains(err.Error(), "password update failed") {
		t.Fatalf("expected password update error, got %v", err)
	}

	repository = NewOptionsRepositoryWithRunnerAndSecret(&fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}}, runner, "ogame_", "secret", func() time.Time { return now })
	issue, err := repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		OldPassword: "oldpass123", Email: "new@example.test",
	}, current)
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssueEmailUsed {
		t.Fatalf("expected duplicate email issue, issue=%+v err=%v", issue, err)
	}

	runner = &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}},
		execErr:     errors.New("email update failed"),
	}
	repository = NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	_, err = repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		OldPassword: "oldpass123", Email: "new@example.test",
	}, current)
	if err == nil || !strings.Contains(err.Error(), "email update failed") {
		t.Fatalf("expected email update error, got %v", err)
	}

	runner = &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}},
		execErrs:    []error{nil, errors.New("queue delete failed")},
	}
	repository = NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	_, err = repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		OldPassword: "oldpass123", Email: "new@example.test",
	}, current)
	if err == nil || !strings.Contains(err.Error(), "queue delete failed") {
		t.Fatalf("expected queue delete error, got %v", err)
	}
}

func TestOptionsRepositoryEmailAndQueueHelpers(t *testing.T) {
	repository := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("email query failed")}}}, "ogame_", nil)
	if _, err := repository.emailExists(context.Background(), "`ogame_users`", "new@example.test"); err == nil || !strings.Contains(err.Error(), "email query failed") {
		t.Fatalf("expected email query error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	if _, err := repository.emailExists(context.Background(), "`ogame_users`", "new@example.test"); err == nil || !strings.Contains(err.Error(), "options email state not found") {
		t.Fatalf("expected missing email state error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("email rows failed"), []any{0})}}}, "ogame_", nil)
	if _, err := repository.emailExists(context.Background(), "`ogame_users`", "new@example.test"); err == nil || !strings.Contains(err.Error(), "email rows failed") {
		t.Fatalf("expected email rows error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", nil)
	if _, err := repository.emailExists(context.Background(), "`ogame_users`", "new@example.test"); err == nil {
		t.Fatal("expected email count scan error")
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", nil)
	exists, err := repository.emailExists(context.Background(), "`ogame_users`", "new@example.test")
	if err != nil || !exists {
		t.Fatalf("expected duplicate email, exists=%v err=%v", exists, err)
	}

	runner := &fakeOptionsRunner{execErrs: []error{nil, errors.New("queue insert failed")}}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.addChangeEmailEvent(context.Background(), "`ogame_queue`", 42, 1_700_000_000); err == nil || !strings.Contains(err.Error(), "queue insert failed") {
		t.Fatalf("expected queue insert error, got %v", err)
	}
}

func TestOptionsRepositoryVacationHelpers(t *testing.T) {
	if vacationMinimumSeconds(0) != 2*24*60*60 {
		t.Fatalf("speed zero should fall back to 2 days, got %d", vacationMinimumSeconds(0))
	}
	if vacationMinimumSeconds(128) != 12*60*60 {
		t.Fatalf("high speed should keep 12h minimum, got %d", vacationMinimumSeconds(128))
	}

	repository := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("queue failed")}}}, "ogame_", nil)
	if _, err := repository.canEnableVacation(context.Background(), "`ogame_queue`", 42); err == nil || !strings.Contains(err.Error(), "queue failed") {
		t.Fatalf("expected queue query error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	if _, err := repository.canEnableVacation(context.Background(), "`ogame_queue`", 42); err == nil || !strings.Contains(err.Error(), "vacation queue state not found") {
		t.Fatalf("expected missing queue state error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, "ogame_", nil)
	allowed, err := repository.canEnableVacation(context.Background(), "`ogame_queue`", 42)
	if err != nil || !allowed {
		t.Fatalf("expected vacation to be allowed with no queue rows, allowed=%v err=%v", allowed, err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue rows failed"), []any{0})}}}, "ogame_", nil)
	if _, err := repository.canEnableVacation(context.Background(), "`ogame_queue`", 42); err == nil || !strings.Contains(err.Error(), "queue rows failed") {
		t.Fatalf("expected queue rows error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", nil)
	if _, err := repository.canEnableVacation(context.Background(), "`ogame_queue`", 42); err == nil {
		t.Fatal("expected queue count scan error")
	}
}

func TestOptionsRepositoryReturnsVacationMutationErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResults(now, 0, 0),
		fakeQueryResult{err: errors.New("vacation queue failed")},
	)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, _, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     true,
			VacationModeSet:  true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "vacation queue failed") {
		t.Fatalf("expected vacation queue error, got %v", err)
	}

	results = append(optionsReadResults(now, 0, 0),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
	)
	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}, execErrs: []error{nil, errors.New("production reset failed")}}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, _, err = repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			VacationMode:     true,
			VacationModeSet:  true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "production reset failed") {
		t.Fatalf("expected production reset error, got %v execs=%+v", err, runner.execs)
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
			name: "user scan",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{
				"Legor", "bad-name-changed", "legor@example.test", "permanent@example.test", 1, "en", "/evolution/", 1, 0, 1, 1, 5, 8,
				int64(0x1), 0, 0, 0, 0, int64(0), now.Add(time.Hour).Unix(), "feedid", legacyPasswordHash("oldpass123", "secret"),
			})}),
			want: "expected int",
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
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))}, fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("uni rows failed"), []any{"en", 0, 60, 128})}),
			want:    "uni rows failed",
		},
		{
			name:    "universe scan",
			results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 0, 0))}, fakeQueryResult{rows: fakeRowsFromValues([]any{"en", "bad-force", 60, 128})}),
			want:    "expected int",
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
	return optionsReadResultsWithPassword(now, deletionQueued, deletionAt, legacyPasswordHash("oldpass123", "secret"))
}

func optionsReadResultsWithVacation(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64) []fakeQueryResult {
	return optionsReadResultsWithVacationAndPassword(now, deletionQueued, deletionAt, vacation, vacationUntil, legacyPasswordHash("oldpass123", "secret"))
}

func optionsReadResultsWithPassword(now time.Time, deletionQueued int, deletionAt int64, passwordHash string) []fakeQueryResult {
	return optionsReadResultsWithVacationAndPassword(now, deletionQueued, deletionAt, 0, 0, passwordHash)
}

func optionsReadResultsWithVacationAndPassword(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64, passwordHash string) []fakeQueryResult {
	return append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRowWithVacationAndPassword(now, deletionQueued, deletionAt, vacation, vacationUntil, passwordHash))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"en", 0, 60, 128})},
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
	return optionsUserRowWithVacationAndPassword(now, deletionQueued, deletionAt, 0, 0, legacyPasswordHash("oldpass123", "secret"))
}

func optionsUserRowWithVacation(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64) []any {
	return optionsUserRowWithVacationAndPassword(now, deletionQueued, deletionAt, vacation, vacationUntil, legacyPasswordHash("oldpass123", "secret"))
}

func optionsUserRowWithVacationAndPassword(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64, passwordHash string) []any {
	return []any{
		"Legor", 0, "legor@example.test", "permanent@example.test", 1, "en", "/evolution/", 1, 0, 1, 1, 5, 8,
		int64(0x1), 0, vacation, vacationUntil, deletionQueued, deletionAt, now.Add(time.Hour).Unix(), "feedid", passwordHash,
	}
}

type fakeOptionsExec struct {
	sql  string
	args []any
}

type fakeOptionsRunner struct {
	fakeQueryer
	execSQL  string
	execArgs []any
	execs    []fakeOptionsExec
	execErr  error
	execErrs []error
}

func (f *fakeOptionsRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execSQL = query
	f.execArgs = args
	f.execs = append(f.execs, fakeOptionsExec{sql: query, args: append([]any(nil), args...)})
	if len(f.execErrs) > 0 {
		err := f.execErrs[0]
		f.execErrs = f.execErrs[1:]
		return fakeSQLResult(1), err
	}
	return fakeSQLResult(1), f.execErr
}

package mysqlgame

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
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

func TestOptionsRepositoryMapsMCPOptions(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRow(now, 1, now.Add(7*24*time.Hour).Unix()))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"en", 0, 60, 128})},
	)}
	repository := NewOptionsRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	options, err := repository.GetMCPOptions(context.Background(), 42, domainmcp.OptionsStatusCommand{PlanetID: 99})
	if err != nil {
		t.Fatalf("GetMCPOptions returned error: %v", err)
	}
	if options.PlayerID != 42 || options.Planet.ID != 99 || options.Planet.TypeName != "planet" ||
		options.User.Name != "Legor" || !options.User.Validated || !options.User.CommanderActive ||
		options.Universe.Speed != 128 || options.Settings.MaxSpy != 5 || !options.Flags.ShowEspionageButton ||
		!options.Account.DeletionQueued {
		t.Fatalf("unexpected mcp options: %+v", options)
	}
	if _, err := (OptionsRepository{}).GetMCPOptions(context.Background(), 42, domainmcp.OptionsStatusCommand{}); err == nil {
		t.Fatalf("expected unavailable reader error")
	}
	if _, err := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, "ogame_", nil).GetMCPOptions(context.Background(), 42, domainmcp.OptionsStatusCommand{}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected MCP options read error, got %v", err)
	}
}

func TestMCPOptionsStatusMapsMoonAndFlags(t *testing.T) {
	options := mcpOptionsStatus(42, domaingame.Options{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:   99,
			Name: "Moon",
			Type: domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{
				Galaxy:   1,
				System:   2,
				Position: 3,
			},
		},
		User: domaingame.OptionsUser{Name: "Legor", NameLocked: true, Admin: 1},
		Universe: domaingame.OptionsUniverse{
			Language:      "en",
			ForceLanguage: true,
			FeedAge:       60,
			Speed:         128,
		},
		Settings: domaingame.OptionsSettings{
			Language:         "en",
			SkinPath:         "/evolution/",
			UseSkin:          true,
			DeactivateIP:     true,
			SortBy:           2,
			SortOrder:        1,
			MaxSpy:           5,
			MaxFleetMessages: 8,
		},
		Account: domaingame.OptionsAccount{
			Vacation:       true,
			VacationUntil:  1700000000,
			DeletionQueued: true,
			DeletionAt:     1700600000,
		},
		Flags: domaingame.OptionsFlags{
			ShowWriteMessage: true,
			ShowBuddy:        true,
			FeedEnabled:      true,
			FeedAtom:         true,
			HideGOEmail:      true,
		},
	})
	if options.Planet.TypeName != "moon" || !options.User.NameLocked || !options.Universe.ForceLanguage ||
		!options.Settings.UseSkin || !options.Account.Vacation || !options.Flags.ShowBuddy ||
		!options.Flags.HideGOEmail {
		t.Fatalf("unexpected mapped options status: %+v", options)
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
	if !strings.Contains(runner.execSQL, "UPDATE `ogame_users` SET skin = ?") || len(runner.execArgs) != 14 {
		t.Fatalf("unexpected update SQL: %s args=%+v", runner.execSQL, runner.execArgs)
	}
	if runner.execArgs[0] != "/evolution/" || runner.execArgs[3] != 2 || runner.execArgs[4] != 0 ||
		runner.execArgs[5] != 1 || runner.execArgs[6] != 99 || runner.execArgs[7] != "fr" ||
		runner.execArgs[8] != 0 || runner.execArgs[9] != int64(0) ||
		runner.execArgs[10] != 1 || runner.execArgs[11] != now.Add(7*24*time.Hour).Unix() || runner.execArgs[12] != int64(0) {
		t.Fatalf("unexpected update args: %+v", runner.execArgs)
	}
}

func TestOptionsRepositoryForcedUniverseLanguageOverridesUserMutation(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRowWithLanguage(now, 0, 0, "en"))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"de", 1, 60, 128})},
	)
	results = append(results, optionsOverviewResults()...)
	results = append(results,
		fakeQueryResult{rows: fakeRowsFromValues(optionsUserRowWithLanguage(now, 0, 0, "de"))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"de", 1, 60, 128})},
	)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Language:         "fr",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueSaved {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if runner.execArgs[7] != "de" {
		t.Fatalf("forced universe language should be stored instead of user mutation, args=%+v", runner.execArgs)
	}
	if !options.Universe.ForceLanguage || options.Settings.Language != "de" {
		t.Fatalf("forced universe language should be reflected in the response, got %+v", options)
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
	if runner.execs[0].args[0] != vacationUntil || runner.execs[0].args[1] != 42 || !strings.Contains(runner.execs[0].sql, "vacation = 1") {
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
			DisableVacation:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueVacationDisabled || options.Account.Vacation {
		t.Fatalf("unexpected vacation disable result: options=%+v issue=%+v", options, issue)
	}
	if len(runner.execs) != 1 || runner.execs[0].args[0] != 42 || !strings.Contains(runner.execs[0].sql, "vacation = 0") {
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
			DisableVacation:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueSaved || !options.Account.Vacation {
		t.Fatalf("unexpected vacation locked result: options=%+v issue=%+v", options, issue)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("locked vacation page must ignore the request, execs=%+v", runner.execs)
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

func TestOptionsRepositoryTrustedCredentialMutationsSkipCurrentPassword(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	current := domaingame.NewOptions(domaingame.Overview{}, domaingame.OptionsUser{
		Email:        "legor@example.test",
		PlainEmail:   "permanent@example.test",
		Validated:    true,
		PasswordHash: legacyPasswordHash("oldpass123", "secret"),
	}, domaingame.OptionsUniverse{Language: "en"}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0)

	passwordRunner := &fakeOptionsRunner{}
	repository := NewOptionsRepositoryWithRunnerAndSecret(passwordRunner, passwordRunner, "ogame_", "secret", func() time.Time { return now })
	issue, err := repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		NewPassword: "newpass123", NewPasswordRepeat: "newpass123",
	}, current, true)
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssuePasswordChanged ||
		len(passwordRunner.execs) != 1 || passwordRunner.execs[0].args[0] != legacyPasswordHash("newpass123", "secret") {
		t.Fatalf("unexpected trusted password change: issue=%+v err=%v execs=%+v", issue, err, passwordRunner.execs)
	}

	emailRunner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}}
	repository = NewOptionsRepositoryWithRunnerAndSecret(emailRunner, emailRunner, "ogame_", "secret", func() time.Time { return now })
	issue, err = repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		Email: "new@example.test",
	}, current, true)
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssueEmailChanged || len(emailRunner.execs) != 3 {
		t.Fatalf("unexpected trusted email change: issue=%+v err=%v execs=%+v", issue, err, emailRunner.execs)
	}
}

func TestOptionsRepositoryChangesEmailAndQueuesPermanentUpdate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret")),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
	)
	updatedResults := optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret"))
	updatedRow := optionsUserRowWithVacationAndPassword(now, 0, 0, 0, 0, legacyPasswordHash("oldpass123", "secret"))
	updatedRow[2] = "new@example.test"
	updatedRow[4] = 0
	updatedRow[22] = legacyPasswordHash(fmt.Sprintf("%d", now.Unix()), "secret")
	updatedResults[4] = fakeQueryResult{rows: fakeRowsFromValues(updatedRow)}
	results = append(results, updatedResults...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
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
	if options.OutboundMail == nil || options.OutboundMail.Character != "Legor" ||
		options.OutboundMail.Recipient != "permanent@example.test" ||
		options.OutboundMail.PendingEmail != "new@example.test" ||
		options.OutboundMail.ActivationCode != legacyPasswordHash(fmt.Sprintf("%d", now.Unix()), "secret") {
		t.Fatalf("unexpected options change mail: %+v", options.OutboundMail)
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

func TestOptionsRepositoryHandlesUnvalidatedAccountBranches(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	passwordHash := legacyPasswordHash("oldpass123", "secret")
	tests := []struct {
		name                     string
		mutation                 domaingame.OptionsMutation
		extra                    []fakeQueryResult
		updated                  string
		wantIssue                string
		wantExecs                int
		wantQueued               bool
		wantMail                 bool
		skipPasswordVerification bool
	}{
		{
			name:      "resend activation",
			mutation:  domaingame.OptionsMutation{ResendActivation: true},
			updated:   "pending@example.test",
			wantIssue: domaingame.OptionsIssueActivationResent,
			wantMail:  true,
		},
		{
			name:      "wrong password",
			mutation:  domaingame.OptionsMutation{Email: "new@example.test", OldPassword: "wrong"},
			updated:   "pending@example.test",
			wantIssue: domaingame.OptionsIssueEmailNeedPassword,
		},
		{
			name:      "invalid email",
			mutation:  domaingame.OptionsMutation{Email: "bad-email", OldPassword: "oldpass123"},
			updated:   "pending@example.test",
			wantIssue: domaingame.OptionsIssueEmailInvalid,
		},
		{
			name:      "duplicate email",
			mutation:  domaingame.OptionsMutation{Email: "used@example.test", OldPassword: "oldpass123"},
			extra:     []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}},
			updated:   "pending@example.test",
			wantIssue: domaingame.OptionsIssueEmailUsed,
		},
		{
			name:       "change email",
			mutation:   domaingame.OptionsMutation{Email: "new@example.test", OldPassword: "oldpass123"},
			extra:      []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}},
			updated:    "new@example.test",
			wantIssue:  domaingame.OptionsIssueEmailChanged,
			wantExecs:  3,
			wantQueued: true,
			wantMail:   true,
		},
		{
			name:                     "trusted change email",
			mutation:                 domaingame.OptionsMutation{Email: "new@example.test"},
			extra:                    []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}},
			updated:                  "new@example.test",
			wantIssue:                domaingame.OptionsIssueEmailChanged,
			wantExecs:                3,
			wantQueued:               true,
			wantMail:                 true,
			skipPasswordVerification: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := optionsReadResultsUnvalidated(now, "pending@example.test", passwordHash)
			results = append(results, tt.extra...)
			results = append(results, optionsReadResultsUnvalidated(now, tt.updated, passwordHash)...)
			runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
			repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })

			options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
				PlayerID:                 42,
				PlanetID:                 99,
				Mutation:                 tt.mutation,
				SkipPasswordVerification: tt.skipPasswordVerification,
			})
			if err != nil || issue == nil || issue.Code != tt.wantIssue || options.User.Email != tt.updated || options.User.Validated {
				t.Fatalf("unexpected unvalidated result options=%+v issue=%+v err=%v", options.User, issue, err)
			}
			if len(runner.execs) != tt.wantExecs {
				t.Fatalf("unexpected unvalidated writes: %+v", runner.execs)
			}
			if tt.wantQueued && (!strings.Contains(runner.execs[0].sql, "validatemd") || runner.execs[2].args[1] != "ChangeEmail") {
				t.Fatalf("expected validation and queue writes, got %+v", runner.execs)
			}
			if tt.wantMail {
				if options.OutboundMail == nil || options.OutboundMail.Recipient != "permanent@example.test" ||
					options.OutboundMail.PendingEmail != tt.updated || options.OutboundMail.ActivationCode != "validation-code" {
					t.Fatalf("unexpected options change mail: %+v", options.OutboundMail)
				}
			} else if options.OutboundMail != nil {
				t.Fatalf("unexpected options change mail: %+v", options.OutboundMail)
			}
		})
	}
}

func TestOptionsRepositoryChangesNameWithLegacyCooldownQueue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResults(now, 0, 0),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
	)
	updated := optionsReadResults(now, 0, 0)
	updatedRow := optionsUserRow(now, 0, 0)
	updatedRow[0], updatedRow[1] = "NewPilot", 1
	updated[4] = fakeQueryResult{rows: fakeRowsFromValues(updatedRow)}
	results = append(results, updated...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name:             "NewPilot",
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.OptionsIssueNameChanged || options.User.Name != "NewPilot" || !options.User.NameLocked {
		t.Fatalf("unexpected name-change result options=%+v issue=%+v", options.User, issue)
	}
	if len(runner.execs) != 3 || !strings.Contains(runner.execs[0].sql, "name_changed = 1") || !strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_queue`") {
		t.Fatalf("expected name, queue, and settings writes, execs=%+v", runner.execs)
	}
	if runner.execs[0].args[0] != "newpilot" || runner.execs[0].args[1] != "NewPilot" || runner.execs[0].args[2] != now.Unix()+7*24*60*60 {
		t.Fatalf("unexpected username update args: %+v", runner.execs[0].args)
	}
	if runner.execs[1].args[1] != "AllowName" || runner.execs[1].args[5] != now.Unix() || runner.execs[1].args[6] != now.Unix()+(now.Unix()+7*24*60*60) {
		t.Fatalf("unexpected legacy AllowName queue args: %+v", runner.execs[1].args)
	}
}

func TestOptionsRepositoryPreservesNameBranchPrecedence(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret")),
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	results = append(results, optionsReadResultsWithPassword(now, 0, 0, legacyPasswordHash("oldpass123", "secret"))...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })

	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name:              "ExistingPilot",
			Language:          "en",
			SkinPath:          "/evolution/",
			MaxSpy:            5,
			MaxFleetMessages:  8,
			OldPassword:       "oldpass123",
			NewPassword:       "newpass123",
			NewPasswordRepeat: "newpass123",
		},
	})
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssueNameExists {
		t.Fatalf("expected duplicate name issue, issue=%+v err=%v", issue, err)
	}
	if len(runner.execs) != 1 || strings.Contains(runner.execs[0].sql, "password = ?") {
		t.Fatalf("name branch must suppress password mutation, execs=%+v", runner.execs)
	}
}

func TestOptionsRepositoryRejectsNameDuringCooldown(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := append(optionsReadResults(now, 0, 0),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	results = append(results, optionsReadResults(now, 0, 0)...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name: "NewPilot", Language: "en", SkinPath: "/evolution/", MaxSpy: 5, MaxFleetMessages: 8,
		},
	})
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssueNameCooldown {
		t.Fatalf("expected name cooldown issue, issue=%+v err=%v", issue, err)
	}
	if len(runner.execs) != 1 || strings.Contains(runner.execs[0].sql, "name_changed") {
		t.Fatalf("cooldown must not change identity, execs=%+v", runner.execs)
	}
}

func TestOptionsRepositoryIdentityMutationErrors(t *testing.T) {
	current := domaingame.NewOptions(domaingame.Overview{}, domaingame.OptionsUser{Name: "Legor"}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0)
	mutation := domaingame.OptionsMutation{Name: "NewPilot"}

	repository := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("name query failed")}}}, "ogame_", nil)
	if _, err := repository.applyIdentityMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, mutation, current, false); err == nil || !strings.Contains(err.Error(), "name query failed") {
		t.Fatalf("expected name query error, got %v", err)
	}

	repository = NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{err: errors.New("cooldown query failed")},
	}}, "ogame_", nil)
	if _, err := repository.applyIdentityMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, mutation, current, false); err == nil || !strings.Contains(err.Error(), "cooldown query failed") {
		t.Fatalf("expected cooldown query error, got %v", err)
	}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{0})},
	}}, execErr: errors.New("name update failed")}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, err := repository.applyIdentityMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, mutation, current, false); err == nil || !strings.Contains(err.Error(), "name update failed") {
		t.Fatalf("expected name update error, got %v", err)
	}

	runner = &fakeOptionsRunner{execErrs: []error{nil, errors.New("name queue failed")}}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.changeName(context.Background(), "`ogame_users`", "`ogame_queue`", 42, "NewPilot"); err == nil || !strings.Contains(err.Error(), "name queue failed") {
		t.Fatalf("expected name queue error, got %v", err)
	}
}

func TestOptionsRepositoryUpdatesCommanderFeedState(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	results := optionsReadResults(now, 0, 0)
	updated := optionsReadResults(now, 0, 0)
	updatedRow := optionsUserRow(now, 0, 0)
	updatedRow[13] = int64(0x2 | 0x8000)
	updatedRow[20] = "00112233445566778899aabbccddeeff"
	updated[4] = fakeQueryResult{rows: fakeRowsFromValues(updatedRow)}
	results = append(results, updated...)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.feedID = func() (string, error) { return "00112233445566778899aabbccddeeff", nil }

	options, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name:             "Legor",
			Language:         "en",
			SkinPath:         "/evolution/",
			MaxSpy:           5,
			MaxFleetMessages: 8,
			ShowWriteMessage: true,
			FeedEnabled:      true,
		},
	})
	if err != nil || issue == nil || issue.Code != domaingame.OptionsIssueSaved || !options.Flags.FeedEnabled || options.User.FeedID == "" {
		t.Fatalf("unexpected feed update options=%+v issue=%+v err=%v", options, issue, err)
	}
	if len(runner.execs) != 2 || runner.execs[0].args[12] != int64(0x2|0x8000) || runner.execs[1].args[0] != "00112233445566778899aabbccddeeff" || !strings.Contains(runner.execs[1].sql, "lastfeed = 0") {
		t.Fatalf("unexpected feed persistence: %+v", runner.execs)
	}
}

func TestOptionsRepositoryReturnsFeedGenerationError(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: optionsReadResults(now, 0, 0)}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.feedID = func() (string, error) { return "", errors.New("random failed") }

	_, _, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name: "Legor", Language: "en", SkinPath: "/evolution/", MaxSpy: 5, MaxFleetMessages: 8, FeedEnabled: true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "random failed") {
		t.Fatalf("expected feed generation error, got %v", err)
	}
}

func TestOptionsRepositoryReturnsFeedPersistenceError(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: optionsReadResults(now, 0, 0)},
		execErrs:    []error{nil, errors.New("feed update failed")},
	}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.feedID = func() (string, error) { return "00112233445566778899aabbccddeeff", nil }
	_, _, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{
			Name: "Legor", Language: "en", SkinPath: "/evolution/", MaxSpy: 5, MaxFleetMessages: 8, FeedEnabled: true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "feed update failed") {
		t.Fatalf("expected feed persistence error, got %v", err)
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
	}, current, false)
	if err == nil || !strings.Contains(err.Error(), "password update failed") {
		t.Fatalf("expected password update error, got %v", err)
	}

	repository = NewOptionsRepositoryWithRunnerAndSecret(&fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}}, runner, "ogame_", "secret", func() time.Time { return now })
	issue, err := repository.applyCredentialMutations(context.Background(), "`ogame_users`", "`ogame_queue`", 42, domaingame.OptionsMutation{
		OldPassword: "oldpass123", Email: "new@example.test",
	}, current, false)
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
	}, current, false)
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
	}, current, false)
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

func TestOptionsRepositoryCountErrors(t *testing.T) {
	tests := []struct {
		name   string
		result fakeQueryResult
		want   string
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("count query failed")}, want: "count query failed"},
		{name: "missing", result: fakeQueryResult{rows: fakeRowsFromValues()}, want: "missing count"},
		{name: "rows", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("count rows failed"), []any{0})}, want: "count rows failed"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, want: "expected int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOptionsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{tt.result}}, "ogame_", nil)
			if _, err := repository.optionsCount(context.Background(), "SELECT 1", "missing count"); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOptionsRepositoryUnvalidatedMutationErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	current := domaingame.NewOptions(domaingame.Overview{}, domaingame.OptionsUser{
		Name: "Legor", Email: "pending@example.test", PlainEmail: "permanent@example.test", PasswordHash: legacyPasswordHash("oldpass123", "secret"),
	}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{}, 0)
	mutation := domaingame.OptionsMutation{Email: "new@example.test", OldPassword: "oldpass123"}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("email query failed")}}}}
	repository := NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	if _, _, err := repository.updateUnvalidatedOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42}, "`ogame_users`", "`ogame_queue`", current, mutation); err == nil || !strings.Contains(err.Error(), "email query failed") {
		t.Fatalf("expected unvalidated email query error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, execErr: errors.New("email update failed")}
	repository = NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	if _, _, err := repository.updateUnvalidatedOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42}, "`ogame_users`", "`ogame_queue`", current, mutation); err == nil || !strings.Contains(err.Error(), "email update failed") {
		t.Fatalf("expected unvalidated email update error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, execErrs: []error{nil, errors.New("queue delete failed")}}
	repository = NewOptionsRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret", func() time.Time { return now })
	if _, _, err := repository.updateUnvalidatedOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42}, "`ogame_users`", "`ogame_queue`", current, mutation); err == nil || !strings.Contains(err.Error(), "queue delete failed") {
		t.Fatalf("expected unvalidated queue error, got %v", err)
	}
}

func TestOptionsRepositoryVacationDisableError(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	current := domaingame.NewOptions(domaingame.Overview{}, domaingame.OptionsUser{}, domaingame.OptionsUniverse{}, domaingame.OptionsSettings{}, domaingame.OptionsAccount{
		Vacation: true, VacationUntil: now.Add(-time.Minute).Unix(),
	}, 0)
	runner := &fakeOptionsRunner{execErr: errors.New("vacation update failed")}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, _, err := repository.updateVacationOptions(context.Background(), appgame.OptionsUpdateQuery{PlayerID: 42}, "`ogame_users`", current, domaingame.OptionsMutation{DisableVacation: true}); err == nil || !strings.Contains(err.Error(), "vacation update failed") {
		t.Fatalf("expected vacation update error, got %v", err)
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
				int64(0x1), 0, 0, 0, 0, int64(0), now.Add(time.Hour).Unix(), "feedid", legacyPasswordHash("oldpass123", "secret"), "validation-code",
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
	readRepository := NewOptionsReadRepository(nil, "ogame_")
	if readRepository.execer != nil || readRepository.prefix != "ogame_" {
		t.Fatalf("unexpected read-only repository: %+v", readRepository)
	}
}

func TestOptionsRepositoryUsesTransactionRunner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsTransactionRunner{fakeOptionsRunner: fakeOptionsRunner{fakeQueryer: fakeQueryer{
		results: append(optionsReadResults(now, 0, 0), optionsReadResults(now, 0, 0)...),
	}}}
	repository := NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, issue, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OptionsMutation{Language: "en", SkinPath: "/evolution/", MaxSpy: 5, MaxFleetMessages: 8},
	})
	if err != nil || !runner.called || issue == nil || issue.Code != domaingame.OptionsIssueSaved {
		t.Fatalf("unexpected transactional update issue=%+v called=%v err=%v", issue, runner.called, err)
	}

	want := errors.New("transaction failed")
	runner = &fakeOptionsTransactionRunner{transactionErr: want}
	repository = NewOptionsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, _, err := repository.UpdateOptions(context.Background(), appgame.OptionsUpdateQuery{}); !errors.Is(err, want) || !runner.called {
		t.Fatalf("expected transaction error, called=%v err=%v", runner.called, err)
	}
}

func TestSecureOptionsFeedID(t *testing.T) {
	feedID, err := secureOptionsFeedID()
	if err != nil || len(feedID) != 32 {
		t.Fatalf("unexpected feed id %q: %v", feedID, err)
	}
	if decoded, err := hex.DecodeString(feedID); err != nil || len(decoded) != 16 {
		t.Fatalf("feed id must encode 16 random bytes: %q err=%v", feedID, err)
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

func optionsReadResultsUnvalidated(now time.Time, pendingEmail string, passwordHash string) []fakeQueryResult {
	results := optionsReadResultsWithPassword(now, 0, 0, passwordHash)
	row := optionsUserRowWithVacationAndPassword(now, 0, 0, 0, 0, passwordHash)
	row[2] = pendingEmail
	row[4] = 0
	results[4] = fakeQueryResult{rows: fakeRowsFromValues(row)}
	return results
}

func optionsOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 1000.0, 2000.0, 3000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
	}
}

func optionsUserRow(now time.Time, deletionQueued int, deletionAt int64) []any {
	return optionsUserRowWithVacationAndPassword(now, deletionQueued, deletionAt, 0, 0, legacyPasswordHash("oldpass123", "secret"))
}

func optionsUserRowWithLanguage(now time.Time, deletionQueued int, deletionAt int64, language string) []any {
	row := optionsUserRow(now, deletionQueued, deletionAt)
	row[5] = language
	return row
}

func optionsUserRowWithVacation(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64) []any {
	return optionsUserRowWithVacationAndPassword(now, deletionQueued, deletionAt, vacation, vacationUntil, legacyPasswordHash("oldpass123", "secret"))
}

func optionsUserRowWithVacationAndPassword(now time.Time, deletionQueued int, deletionAt int64, vacation int, vacationUntil int64, passwordHash string) []any {
	return []any{
		"Legor", 0, "legor@example.test", "permanent@example.test", 1, "en", "/evolution/", 1, 0, 1, 1, 5, 8,
		int64(0x1), 0, vacation, vacationUntil, deletionQueued, deletionAt, now.Add(time.Hour).Unix(), "feedid", passwordHash, "validation-code",
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

type fakeOptionsTransactionRunner struct {
	fakeOptionsRunner
	called         bool
	transactionErr error
}

func (f *fakeOptionsTransactionRunner) WithTransaction(ctx context.Context, run func(Queryer, Execer) error) error {
	f.called = true
	if f.transactionErr != nil {
		return f.transactionErr
	}
	return run(f, f)
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

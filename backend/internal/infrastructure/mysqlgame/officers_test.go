package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestOfficersRepositoryReadsOfficerStatus(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{4000, 7000, now.Add(time.Hour).Unix(), int64(0), int64(0), int64(0), int64(0)})},
	)}
	repository := NewOfficersRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	officers, err := repository.GetOfficers(context.Background(), appgame.OfficersQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatalf("GetOfficers returned error: %v", err)
	}
	if officers.User.PaidDarkMatter != 4000 || officers.User.FreeDarkMatter != 7000 || !officers.Rows[0].Active {
		t.Fatalf("unexpected officers result: %+v", officers)
	}
	if !strings.Contains(queryer.calls[len(queryer.calls)-1].sql, "com_until") {
		t.Fatalf("expected officer timer query, got %s", queryer.calls[len(queryer.calls)-1].sql)
	}

	_ = NewOfficersRepository(nil, "ogame_")
	_ = NewOfficersRepositoryWithQueryer(&fakeOptionsRunner{}, "ogame_", nil)

	queryer = &fakeQueryer{results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}
	repository = NewOfficersRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.GetOfficers(context.Background(), appgame.OfficersQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected load user error, got %v", err)
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if _, err := repository.GetOfficers(context.Background(), appgame.OfficersQuery{PlayerID: 42, PlanetID: 99}); err == nil {
		t.Fatalf("expected overview query error")
	}
}

func TestOfficersRepositoryRecruitOfficerUpdatesDMAndTimer(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{4000, 7000, int64(0), int64(0), int64(0), int64(0), int64(0)})}),
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0, 1000, int64(0), int64(0), now.Add(7 * 24 * time.Hour).Unix(), int64(0), int64(0)})})...,
	)}}
	repository := NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	officers, issue, err := repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerEngineer,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err != nil {
		t.Fatalf("RecruitOfficer returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.OfficerIssueRecruited {
		t.Fatalf("expected recruited issue, got %+v", issue)
	}
	if !strings.Contains(runner.execSQL, "eng_until") {
		t.Fatalf("expected engineer timer update, got %s", runner.execSQL)
	}
	if len(runner.execArgs) != 4 || runner.execArgs[0] != 0 || runner.execArgs[1] != 1000 {
		t.Fatalf("unexpected exec args: %+v", runner.execArgs)
	}
	if officers.User.PaidDarkMatter != 0 || officers.User.FreeDarkMatter != 1000 {
		t.Fatalf("unexpected updated officers: %+v", officers)
	}
}

func TestOfficersRepositoryMCPRecruitOfficerPreviewAndExecute(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	command := domainmcp.RecruitOfficerCommand{
		PlanetID:  99,
		OfficerID: domaingame.OfficerCommander,
		Days:      domaingame.OfficerWeekDays,
	}
	previewQueryer := &fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{15000, 5000, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}
	repository := NewOfficersRepositoryWithQueryer(previewQueryer, "ogame_", func() time.Time { return now })

	preview, err := repository.PreviewMCPRecruitOfficer(context.Background(), 42, command)
	if err != nil {
		t.Fatalf("PreviewMCPRecruitOfficer returned error: %v", err)
	}
	if preview.PlayerID != 42 || preview.PlanetID != 99 || preview.OfficerID != domaingame.OfficerCommander || preview.Name != "Commander" || preview.Cost != domaingame.OfficerWeekCost {
		t.Fatalf("unexpected preview result: %+v", preview)
	}
	if preview.PaidDarkMatter != 5000 || preview.FreeDarkMatter != 5000 || preview.Until != now.Add(7*24*time.Hour).Unix() || !preview.Active {
		t.Fatalf("expected preview to show post-recruitment state, got %+v", preview)
	}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(
		append(
			append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{15000, 5000, int64(0), int64(0), int64(0), int64(0), int64(0)})}),
			append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{15000, 5000, int64(0), int64(0), int64(0), int64(0), int64(0)})})...,
		),
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{5000, 5000, now.Add(7 * 24 * time.Hour).Unix(), int64(0), int64(0), int64(0), int64(0)})})...,
	)}}
	repository = NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	recruited, err := repository.RecruitMCPOfficer(context.Background(), 42, command)
	if err != nil {
		t.Fatalf("RecruitMCPOfficer returned error: %v", err)
	}
	if !recruited.Executed || recruited.PaidDarkMatter != 5000 || recruited.FreeDarkMatter != 5000 || recruited.Until != now.Add(7*24*time.Hour).Unix() {
		t.Fatalf("unexpected recruited result: %+v", recruited)
	}
	if !strings.Contains(runner.execSQL, "com_until") {
		t.Fatalf("expected commander timer update, got %s", runner.execSQL)
	}
}

func TestOfficersRepositoryMCPRecruitOfficerEdges(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	command := domainmcp.RecruitOfficerCommand{PlanetID: 99, OfficerID: domaingame.OfficerAdmiral, Days: domaingame.OfficerWeekDays}
	if _, err := (OfficersRepository{}).PreviewMCPRecruitOfficer(context.Background(), 42, command); err == nil {
		t.Fatalf("expected missing reader error")
	}
	if _, err := (OfficersRepository{}).RecruitMCPOfficer(context.Background(), 42, command); err == nil {
		t.Fatalf("expected missing updater error")
	}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{9999, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository := NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	preview, err := repository.PreviewMCPRecruitOfficer(context.Background(), 42, command)
	if err != nil {
		t.Fatalf("PreviewMCPRecruitOfficer returned error: %v", err)
	}
	if preview.Issue == nil || preview.Issue.Code != domaingame.OfficerIssueNotEnough || runner.execSQL != "" {
		t.Fatalf("expected insufficient dark matter without exec, result=%+v exec=%s", preview, runner.execSQL)
	}
	recruitRunner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{9999, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository = NewOfficersRepositoryWithRunner(recruitRunner, recruitRunner, "ogame_", func() time.Time { return now })
	recruited, err := repository.RecruitMCPOfficer(context.Background(), 42, command)
	if err != nil {
		t.Fatalf("RecruitMCPOfficer returned error for blocking issue: %v", err)
	}
	if recruited.Issue == nil || recruited.Executed {
		t.Fatalf("expected blocking issue without execution, got %+v", recruited)
	}
}

func TestOfficersRepositoryRejectsInsufficientDMWithoutExec(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{9999, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository := NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	_, issue, err := repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerAdmiral,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err != nil {
		t.Fatalf("RecruitOfficer returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.OfficerIssueNotEnough {
		t.Fatalf("expected insufficient issue, got %+v", issue)
	}
	if runner.execSQL != "" {
		t.Fatalf("insufficient DM should not execute update: %s", runner.execSQL)
	}
}

func TestOfficersRepositoryHandlesMutationNoopAndUpdateErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{}
	repository := NewOfficersRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })
	if _, _, err := repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected updater error, got %v", err)
	}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{50000, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository = NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, issue, err := repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: 99,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err != nil || issue != nil || runner.execSQL != "" {
		t.Fatalf("invalid legacy mutation should be ignored, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}

	runner = &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{50000, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
		)},
		execErr: errors.New("update failed"),
	}
	repository = NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, _, err = repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerCommander,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{50000, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository = NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	_, _, err = repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerCommander,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected query") {
		t.Fatalf("expected reload error after officer update, got %v", err)
	}
}

func TestOfficersRepositoryLoadUserErrorsAndTimerColumns(t *testing.T) {
	repository := NewOfficersRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("query failed")}}}, "ogame_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected query error, got %v", err)
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("empty rows failed"))}}}, "ogame_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "empty rows failed") {
		t.Fatalf("expected empty rows error, got %v", err)
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, int64(0), int64(0), int64(0), int64(0), int64(0)})}}}, "ogame_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil {
		t.Fatalf("expected scan error")
	}

	repository = NewOfficersRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"), []any{1, 2, int64(3), int64(4), int64(5), int64(6), int64(7)})}}}, "ogame_", nil)
	if _, _, err := repository.loadOfficersUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}

	for _, test := range []struct {
		officerID int
		column    string
		ok        bool
	}{
		{domaingame.OfficerCommander, "com_until", true},
		{domaingame.OfficerAdmiral, "adm_until", true},
		{domaingame.OfficerEngineer, "eng_until", true},
		{domaingame.OfficerGeologist, "geo_until", true},
		{domaingame.OfficerTechnocrat, "tec_until", true},
		{99, "", false},
	} {
		column, ok := officerTimerColumn(test.officerID)
		if column != test.column || ok != test.ok {
			t.Fatalf("officerTimerColumn(%d)=%q,%v want %q,%v", test.officerID, column, ok, test.column, test.ok)
		}
	}
}

func TestOfficersRepositoryPrefixErrorAfterCurrentLoad(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{50000, 0, int64(0), int64(0), int64(0), int64(0), int64(0)})},
	)}}
	repository := NewOfficersRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.prefix = "bad-prefix_"
	_, _, err := repository.RecruitOfficer(context.Background(), appgame.OfficersMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerCommander,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected update prefix error, got %v", err)
	}
}

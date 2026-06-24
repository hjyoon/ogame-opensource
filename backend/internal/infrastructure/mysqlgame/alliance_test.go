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

func TestAllianceRepositoryReadsNoAllianceSearchAndApplyViews(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues([]any{7, "TAG", "The Alliance", 2})},
	)}
	repository := NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)

	alliance, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID:   42,
		PlanetID:   99,
		View:       domaingame.AllianceViewSearch,
		SearchText: "TA",
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewSearch || alliance.SearchText != "TA" || len(alliance.SearchResults) != 1 ||
		alliance.SearchResults[0].Tag != "TAG" || alliance.Viewer.Validated != true {
		t.Fatalf("unexpected search alliance: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID:   42,
		PlanetID:   99,
		View:       domaingame.AllianceViewApply,
		AllianceID: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewApply || alliance.Target == nil || alliance.Target.Tag != "TAG" {
		t.Fatalf("unexpected apply alliance: %+v", alliance)
	}
}

func TestAllianceRepositoryReadsOwnApplicationsAndMembers(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 1))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 123))},
	)}
	repository := NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)

	alliance, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID:      42,
		PlanetID:      99,
		View:          domaingame.AllianceViewApplications,
		ApplicationID: 11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewApplications || alliance.Own == nil || alliance.Own.ApplicationCount != 1 ||
		len(alliance.Applications) != 1 || alliance.SelectedApp == nil || alliance.SelectedApp.PlayerName != "newcomer" {
		t.Fatalf("unexpected applications view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", 0, "Founder", int64(9000000), int64(100), int64(200), 1, 2, 3})},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID: 42,
		PlanetID: 99,
		View:     domaingame.AllianceViewMembers,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewMembers || len(alliance.Members) != 1 || alliance.Members[0].Galaxy != 1 {
		t.Fatalf("unexpected members view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights), allianceRankRow(1, "Newcomer", 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID: 42,
		PlanetID: 99,
		View:     domaingame.AllianceViewManagement,
		TextKind: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewManagement || alliance.TextKind != 3 || len(alliance.Ranks) != 2 || alliance.Ranks[0].Name != "Founder" {
		t.Fatalf("unexpected management view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights), allianceRankRow(1, "Newcomer", 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID: 42,
		PlanetID: 99,
		View:     domaingame.AllianceViewRanks,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewRanks || len(alliance.Ranks) != 2 || alliance.Ranks[1].Name != "Newcomer" {
		t.Fatalf("unexpected ranks view: %+v", alliance)
	}
}

func TestAllianceRepositoryCreatesAllianceWithDefaultRanks(t *testing.T) {
	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
			{rows: fakeRowsFromValues()},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		)...),
		},
		insertID: 7,
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })

	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewCreate},
		Mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "The Alliance"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueCreated || alliance.Own == nil || alliance.Own.ID != 7 {
		t.Fatalf("unexpected create result alliance=%+v issue=%+v", alliance, issue)
	}
	if len(runner.execCalls) != 3 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_ally`") ||
		!strings.Contains(runner.execCalls[1].sql, "INSERT INTO `ogame_allyranks`") ||
		runner.execCalls[2].args[0] != 7 || runner.execCalls[2].args[1] != int64(1000) {
		t.Fatalf("unexpected exec calls: %+v", runner.execCalls)
	}
}

func TestAllianceRepositoryAppliesAndReviewsApplications(t *testing.T) {
	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 1))},
		)...),
		},
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })

	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 43,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 43, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7, Text: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueApplied || alliance.Pending == nil ||
		len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_allyapps`") {
		t.Fatalf("unexpected apply result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...),
		},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2000, 0) })
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "accept", ApplicationID: 11},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueAccepted || len(alliance.Applications) != 0 ||
		len(runner.execCalls) != 2 || runner.execCalls[0].args[0] != 7 || runner.execCalls[0].args[1] != domaingame.AllianceRankNewcomer {
		t.Fatalf("unexpected accept result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			{rows: fakeRowsFromValues(allianceApplicationRow(12, 7, 44, "rejectme", "no", 1001))},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...),
		},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "reject", ApplicationID: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueRejected || len(runner.execCalls) != 1 ||
		!strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_allyapps`") {
		t.Fatalf("unexpected reject result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}
}

func TestAllianceRepositoryWithdrawsAndLeavesAlliance(t *testing.T) {
	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
			{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...),
		},
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 43,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 43, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "withdraw"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueWithdrawn || alliance.Pending != nil ||
		len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_allyapps`") {
		t.Fatalf("unexpected withdraw result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(43, "member", 1, 7, 1, "Newcomer", 0))},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "member", 1, 0, 0, "", 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...),
		},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 43,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 43, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "leave"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueLeft || alliance.Viewer.AllianceID != 0 ||
		len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_users` SET ally_id = 0") {
		t.Fatalf("unexpected leave result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}
}

func TestAllianceRepositoryReadsHomeCreateAndDeniedMemberViews(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
	)}
	repository := NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewHome || alliance.Own == nil || alliance.Own.Tag != "TAG" {
		t.Fatalf("unexpected home view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 43, PlanetID: 99, View: domaingame.AllianceViewCreate})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewCreate {
		t.Fatalf("unexpected create view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "member", 1, 7, 1, "Newcomer", 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 43, PlanetID: 99, View: domaingame.AllianceViewMembers})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewMembers || len(alliance.Members) != 0 {
		t.Fatalf("expected denied member list without rows, got %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "member", 1, 7, 1, "Newcomer", 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 43, PlanetID: 99, View: domaingame.AllianceViewManagement})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewManagement || len(alliance.Ranks) != 0 {
		t.Fatalf("expected denied management without rank rows, got %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{err: errors.New("rank load failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewManagement}); err == nil || !strings.Contains(err.Error(), "rank load failed") {
		t.Fatalf("expected management rank load error, got %v", err)
	}
}

func TestAllianceRepositoryGetAlliancePropagatesLoaderErrors(t *testing.T) {
	repository := NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected overview error, got %v", err)
	}

	queryer := &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("viewer failed")})}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "viewer failed") {
		t.Fatalf("expected viewer error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{err: errors.New("own failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "own failed") {
		t.Fatalf("expected own alliance error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{err: errors.New("pending failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "pending failed") {
		t.Fatalf("expected pending error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 42, "legor", "hello", 1000))},
		fakeQueryResult{err: errors.New("pending target failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "pending target failed") {
		t.Fatalf("expected pending target error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{err: errors.New("search failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewSearch, SearchText: "TAG"}); err == nil || !strings.Contains(err.Error(), "search failed") {
		t.Fatalf("expected search error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{err: errors.New("apply target failed")},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewApply, AllianceID: 7}); err == nil || !strings.Contains(err.Error(), "apply target failed") {
		t.Fatalf("expected apply target error, got %v", err)
	}
}

func TestAllianceRepositoryHandlesPermissionAndValidationIssues(t *testing.T) {
	tests := []struct {
		name     string
		viewer   []any
		mutation domaingame.AllianceMutation
		results  []fakeQueryResult
		want     string
	}{
		{
			name:     "create while already allied",
			viewer:   allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights),
			mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "Alliance"},
			want:     domaingame.AllianceIssueNoPermission,
		},
		{
			name:     "invalid create tag",
			viewer:   allianceViewerRow(42, "legor", 1, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "create", Tag: "AB", Name: "Alliance"},
			want:     domaingame.AllianceIssueInvalidTag,
		},
		{
			name:     "tag exists",
			viewer:   allianceViewerRow(42, "legor", 1, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "Alliance"},
			results:  []fakeQueryResult{{rows: fakeRowsFromValues([]any{7})}},
			want:     domaingame.AllianceIssueTagExists,
		},
		{
			name:     "apply not activated",
			viewer:   allianceViewerRow(43, "newcomer", 0, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
			want:     domaingame.AllianceIssueNotActivated,
		},
		{
			name:     "apply already pending",
			viewer:   allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
			results:  []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}},
			want:     domaingame.AllianceIssueAlreadyApplied,
		},
		{
			name:     "apply alliance missing",
			viewer:   allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
			},
			want: domaingame.AllianceIssueAllianceNotFound,
		},
		{
			name:     "apply closed",
			viewer:   allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0),
			mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "Alliance", 0, 0))},
			},
			want: domaingame.AllianceIssueApplicationsClosed,
		},
		{
			name:     "review denied",
			viewer:   allianceViewerRow(43, "newcomer", 1, 7, 1, "Newcomer", 0),
			mutation: domaingame.AllianceMutation{Action: "accept", ApplicationID: 11},
			want:     domaingame.AllianceIssueNoPermission,
		},
		{
			name:     "founder leave denied",
			viewer:   allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights),
			mutation: domaingame.AllianceMutation{Action: "leave"},
			want:     domaingame.AllianceIssueFounderCannotLeave,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := append([]fakeQueryResult{{rows: fakeRowsFromValues(tt.viewer)}}, tt.results...)
			results = append(results, append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues(tt.viewer)},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
			)...)
			runner := &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: results}, insertID: 7}
			repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			_, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
				PlayerID: 42,
				PlanetID: 99,
				Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
				Mutation: tt.mutation,
			})
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.want {
				t.Fatalf("expected issue %q, got %+v", tt.want, issue)
			}
		})
	}
}

func TestAllianceRepositoryMutatesSearchAndUnknownActions(t *testing.T) {
	runner := &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues([]any{7, "TAG", "The Alliance", 2})},
	)...)}}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewSearch, SearchText: "TA"},
		Mutation: domaingame.AllianceMutation{Action: "search"},
	})
	if err != nil || issue != nil || len(alliance.SearchResults) != 1 {
		t.Fatalf("unexpected search mutation alliance=%+v issue=%+v err=%v", alliance, issue, err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	_, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "15"},
	})
	if err != nil || issue == nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected no permission for unknown mutation, issue=%+v err=%v", issue, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{}, "ogame_", time.Now)
	if _, _, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected updater unavailable, got %v", err)
	}
}

func TestAllianceRepositorySavesManagementSettings(t *testing.T) {
	founder := allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights)
	runner := &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(founder)},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(founder)},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
	)...)}}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "save_text", TextKind: 3, Text: "Application text", InsertApp: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueSaved || alliance.View != domaingame.AllianceViewManagement ||
		len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "SET apptext = ?") ||
		runner.execCalls[0].args[1] != 1 {
		t.Fatalf("unexpected save text result alliance=%+v issue=%+v exec=%+v", alliance, issue, runner.execCalls)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(founder)},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(founder)},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 0, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Right Hand", domaingame.AllianceFounderRights))},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	_, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{
			Action:          "save_settings",
			Homepage:        "https://example.com/alliance",
			ImageLogo:       "javascript:alert(1)",
			Open:            false,
			FounderRankName: "Right Hand",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 2 ||
		!strings.Contains(runner.execCalls[0].sql, "SET open = ?") || runner.execCalls[0].args[0] != 0 ||
		runner.execCalls[0].args[1] != "https://example.com/alliance" || runner.execCalls[0].args[2] != "" ||
		!strings.Contains(runner.execCalls[1].sql, "rank_id = ?") {
		t.Fatalf("unexpected save settings issue=%+v exec=%+v", issue, runner.execCalls)
	}
}

func TestAllianceRepositoryManagementMutationErrorsReload(t *testing.T) {
	founder := allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights)
	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founder)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founder)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
		)...)},
		execErr: errors.New("save text failed"),
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "save_text", TextKind: 1, Text: "Updated"},
	}); err == nil || !strings.Contains(err.Error(), "save text failed") || issue != nil {
		t.Fatalf("expected save text mutation error, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founder)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founder)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
		)...)},
		execErr: errors.New("save settings failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "save_settings", Open: true},
	}); err == nil || !strings.Contains(err.Error(), "save settings failed") || issue != nil {
		t.Fatalf("expected save settings mutation error, issue=%+v err=%v", issue, err)
	}
}

func TestAllianceRepositoryManagementHelperEdges(t *testing.T) {
	ctx := context.Background()
	founder := domaingame.AllianceViewer{PlayerID: 42, AllianceID: 7, Founder: true}
	member := domaingame.AllianceViewer{PlayerID: 43, AllianceID: 7}

	repository := NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if issue, err := repository.saveAllianceText(ctx, member, domaingame.AllianceMutation{Action: "save_text"}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected text permission issue, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.saveAllianceSettings(ctx, member, domaingame.AllianceMutation{Action: "save_settings"}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected settings permission issue, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.saveAllianceSettings(ctx, founder, domaingame.AllianceMutation{Action: "save_settings", FounderRankName: "bad/rank"}); err != nil || issue.Code != domaingame.AllianceIssueInvalidRankName {
		t.Fatalf("expected invalid rank issue, got issue=%+v err=%v", issue, err)
	}

	for _, tt := range []struct {
		kind int
		sql  string
	}{
		{kind: 1, sql: "SET exttext = ?"},
		{kind: 2, sql: "SET inttext = ?"},
	} {
		runner := &fakeAllianceRunner{}
		repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
		issue, err := repository.saveAllianceText(ctx, founder, domaingame.AllianceMutation{TextKind: tt.kind, Text: "Updated"})
		if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, tt.sql) {
			t.Fatalf("unexpected text helper kind=%d issue=%+v err=%v exec=%+v", tt.kind, issue, err, runner.execCalls)
		}
	}

	runner := &fakeAllianceRunner{execErr: errors.New("text exec failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.saveAllianceText(ctx, founder, domaingame.AllianceMutation{TextKind: 1, Text: "Updated"}); err == nil || !strings.Contains(err.Error(), "text exec failed") {
		t.Fatalf("expected text exec error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.saveAllianceText(ctx, founder, domaingame.AllianceMutation{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected text prefix error, got %v", err)
	}

	runner = &fakeAllianceRunner{}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err := repository.saveAllianceSettings(ctx, founder, domaingame.AllianceMutation{Homepage: "https://example.com", Open: true})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 1 || runner.execCalls[0].args[0] != 1 {
		t.Fatalf("unexpected settings without founder rank issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{execErr: errors.New("settings exec failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.saveAllianceSettings(ctx, founder, domaingame.AllianceMutation{Open: true}); err == nil || !strings.Contains(err.Error(), "settings exec failed") {
		t.Fatalf("expected settings exec error, got %v", err)
	}
	runner = &fakeAllianceRunner{execErr: errors.New("rank exec failed"), execErrAt: 2}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.saveAllianceSettings(ctx, founder, domaingame.AllianceMutation{Open: true, FounderRankName: "Right Hand"}); err == nil || !strings.Contains(err.Error(), "rank exec failed") {
		t.Fatalf("expected rank exec error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.saveAllianceSettings(ctx, founder, domaingame.AllianceMutation{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected settings prefix error, got %v", err)
	}
}

func TestAllianceRepositoryLoadAllianceRanksErrors(t *testing.T) {
	if _, err := NewAllianceRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", time.Now).loadAllianceRanks(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected rank prefix error, got %v", err)
	}
	repository := NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("rank query failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceRanks(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "rank query failed") {
		t.Fatalf("expected rank query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Founder", 511})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceRanks(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected rank scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("rank rows failed"), allianceRankRow(0, "Founder", 511))}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceRanks(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "rank rows failed") {
		t.Fatalf("expected rank rows error, got %v", err)
	}
}

func TestAllianceRepositoryMutationReloadErrors(t *testing.T) {
	ctx := context.Background()
	viewer := allianceViewerRow(42, "legor", 1, 0, 0, "", 0)

	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(viewer)},
			{rows: fakeRowsFromValues()},
			{err: errors.New("create reload failed")},
		}},
		insertID: 7,
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "Alliance"},
	}); err == nil || !strings.Contains(err.Error(), "create reload failed") || issue == nil || issue.Code != domaingame.AllianceIssueCreated {
		t.Fatalf("expected create reload failure with created issue, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(viewer)},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "Alliance", 1, 0))},
			{err: errors.New("apply reload failed")},
		}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
	}); err == nil || !strings.Contains(err.Error(), "apply reload failed") || issue == nil || issue.Code != domaingame.AllianceIssueApplied {
		t.Fatalf("expected apply reload failure with applied issue, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
			{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))},
			{err: errors.New("review reload failed")},
		}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "accept", ApplicationID: 11},
	}); err == nil || !strings.Contains(err.Error(), "review reload failed") || issue == nil || issue.Code != domaingame.AllianceIssueAccepted {
		t.Fatalf("expected review reload failure with accepted issue, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(allianceViewerRow(43, "member", 1, 7, 1, "Newcomer", 0))},
			{err: errors.New("leave reload failed")},
		}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 43,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 43, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "leave"},
	}); err == nil || !strings.Contains(err.Error(), "leave reload failed") || issue == nil || issue.Code != domaingame.AllianceIssueLeft {
		t.Fatalf("expected leave reload failure with left issue, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(viewer)},
			{err: errors.New("invalid reload failed")},
		}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "create", Tag: "AB", Name: "Alliance"},
	}); err == nil || !strings.Contains(err.Error(), "invalid reload failed") || issue != nil {
		t.Fatalf("expected validation reload failure without issue, issue=%+v err=%v", issue, err)
	}
}

func TestAllianceRepositoryMutationActionErrorsReloadView(t *testing.T) {
	ctx := context.Background()
	viewer := allianceViewerRow(42, "legor", 1, 0, 0, "", 0)

	repository := NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("viewer failed")},
	}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if _, _, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "viewer failed") {
		t.Fatalf("expected viewer failure, got %v", err)
	}

	runner := &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(viewer)},
		{err: errors.New("tag query failed")},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(viewer)},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if alliance, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "Alliance"},
	}); err == nil || !strings.Contains(err.Error(), "tag query failed") || issue != nil || alliance.View != domaingame.AllianceViewNoAlliance {
		t.Fatalf("expected create action error after reload, alliance=%+v issue=%+v err=%v", alliance, issue, err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(viewer)},
		{err: errors.New("apply pending failed")},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(viewer)},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "apply", AllianceID: 7},
	}); err == nil || !strings.Contains(err.Error(), "apply pending failed") || issue != nil {
		t.Fatalf("expected apply action error after reload, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(viewer)},
		{err: errors.New("withdraw pending failed")},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(viewer)},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "withdraw"},
	}); err == nil || !strings.Contains(err.Error(), "withdraw pending failed") || issue != nil {
		t.Fatalf("expected withdraw action error after reload, issue=%+v err=%v", issue, err)
	}

	founder := allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights)
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues(founder)},
		{err: errors.New("review app failed")},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(founder)},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "Alliance", 1, 0))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(ctx, appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "accept", ApplicationID: 11},
	}); err == nil || !strings.Contains(err.Error(), "review app failed") || issue != nil {
		t.Fatalf("expected review action error after reload, issue=%+v err=%v", issue, err)
	}
}

func TestAllianceRepositoryConstructorsAndLoaderErrors(t *testing.T) {
	repository := NewAllianceRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	runner := &fakeAllianceRunner{}
	repository = NewAllianceRepositoryWithQueryer(runner, "ogame_", time.Now)
	if repository.execer == nil {
		t.Fatal("expected queryer implementing Execer to be reused as alliance execer")
	}
	withDefaultClock := NewAllianceRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected default clock")
	}
	if _, err := NewAllianceRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", time.Now).loadAllianceViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("viewer failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "viewer failed") {
		t.Fatalf("expected viewer query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "viewer not found") {
		t.Fatalf("expected viewer not found error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceInfo(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected alliance scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("app rows failed"), allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceApplications(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "app rows failed") {
		t.Fatalf("expected applications rows error, got %v", err)
	}
}

func TestAllianceRepositoryHelperErrorEdges(t *testing.T) {
	repository := NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("tag failed")}}}, "ogame_", time.Now)
	if _, err := repository.allianceTagExists(context.Background(), "TAG"); err == nil || !strings.Contains(err.Error(), "tag failed") {
		t.Fatalf("expected tag query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("tag rows failed"), []any{7})}}}, "ogame_", time.Now)
	if _, err := repository.allianceTagExists(context.Background(), "TAG"); err == nil || !strings.Contains(err.Error(), "tag rows failed") {
		t.Fatalf("expected tag rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "TAG", "Name", 1})}}}, "ogame_", time.Now)
	if _, err := repository.searchAlliances(context.Background(), "TA"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected search scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("search rows failed"), []any{7, "TAG", "Name", 1})}}}, "ogame_", time.Now)
	if _, err := repository.searchAlliances(context.Background(), "TA"); err == nil || !strings.Contains(err.Error(), "search rows failed") {
		t.Fatalf("expected search rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("user app failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadUserApplication(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "user app failed") {
		t.Fatalf("expected user application query error, got %v", err)
	}
	if app, err := repository.loadApplication(context.Background(), 0); err != nil || app != nil {
		t.Fatalf("zero application id should return nil, got app=%+v err=%v", app, err)
	}
	if info, err := repository.loadAllianceInfo(context.Background(), 0); err != nil || info != nil {
		t.Fatalf("zero alliance id should return nil, got info=%+v err=%v", info, err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "name", 1, "rank", int64(1), int64(1), int64(1), 1, 2, 3})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceMembers(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected member scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("member rows failed"), []any{42, "name", 1, "rank", int64(1), int64(1), int64(1), 1, 2, 3})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceMembers(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "member rows failed") {
		t.Fatalf("expected member rows error, got %v", err)
	}
	runner := &fakeAllianceRunner{execErr: errors.New("insert failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.insertAlliance(context.Background(), 42, "TAG", "Alliance"); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected insert exec error, got %v", err)
	}
	runner = &fakeAllianceRunner{lastIDErr: errors.New("last id failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.insertAlliance(context.Background(), 42, "TAG", "Alliance"); err == nil || !strings.Contains(err.Error(), "last id failed") {
		t.Fatalf("expected last insert id error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("info failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceInfo(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "info failed") {
		t.Fatalf("expected info query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", time.Now)
	if info, err := repository.loadAllianceInfo(context.Background(), 7); err != nil || info != nil {
		t.Fatalf("empty info should be nil, got info=%+v err=%v", info, err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("info rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceInfo(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "info rows failed") {
		t.Fatalf("expected info rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "legor", 1, 0, 0, "", 0})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected viewer scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("viewer rows failed"), allianceViewerRow(42, "legor", 1, 0, 0, "", 0))}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "viewer rows failed") {
		t.Fatalf("expected viewer rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("apps failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceApplications(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "apps failed") {
		t.Fatalf("expected applications query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 7, 43, "newcomer", "hello", int64(1)})}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceApplications(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected applications scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("members failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadAllianceMembers(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "members failed") {
		t.Fatalf("expected members query error, got %v", err)
	}
	if app, err := scanOneAllianceApplication(fakeRowsFromValuesWithErr(errors.New("single app rows failed"))); err == nil || app != nil || !strings.Contains(err.Error(), "single app rows failed") {
		t.Fatalf("expected single app rows error, got app=%+v err=%v", app, err)
	}
	if _, err := scanOneAllianceApplication(fakeRowsFromValues([]any{"bad", 7, 43, "newcomer", "hello", int64(1)})); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected single app scan error, got %v", err)
	}
	badPrefix := NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := badPrefix.loadAllianceInfo(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected load info prefix error, got %v", err)
	}
	if _, err := badPrefix.searchAlliances(context.Background(), "TAG"); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected search prefix error, got %v", err)
	}
	if _, err := badPrefix.loadUserApplication(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected user app prefix error, got %v", err)
	}
	if _, err := badPrefix.loadApplication(context.Background(), 11); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected application prefix error, got %v", err)
	}
	if _, err := badPrefix.loadAllianceApplications(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected applications prefix error, got %v", err)
	}
	if _, err := badPrefix.loadAllianceMembers(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected members prefix error, got %v", err)
	}
	if _, err := badPrefix.insertAlliance(context.Background(), 42, "TAG", "Alliance"); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected insert alliance prefix error, got %v", err)
	}
	if err := badPrefix.insertDefaultAllianceRanks(context.Background(), 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected default ranks prefix error, got %v", err)
	}
	if err := badPrefix.updateFounderAlliance(context.Background(), 42, 7); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected founder update prefix error, got %v", err)
	}
	if err := badPrefix.insertApplication(context.Background(), 7, 42, "hello"); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected insert application prefix error, got %v", err)
	}
	if err := badPrefix.acceptApplication(context.Background(), 7, 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected accept prefix error, got %v", err)
	}
	if err := badPrefix.deleteApplication(context.Background(), 11); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected delete prefix error, got %v", err)
	}
	if _, err := badPrefix.leaveAlliance(context.Background(), domaingame.AllianceViewer{PlayerID: 43, AllianceID: 7, RankID: 1}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected leave prefix error, got %v", err)
	}
}

func TestAllianceRepositoryMutationHelperIssuesAndErrors(t *testing.T) {
	ctx := context.Background()
	member := domaingame.AllianceViewer{PlayerID: 43, Validated: true, AllianceID: 7, RankID: 1}
	outsider := domaingame.AllianceViewer{PlayerID: 43, Validated: true}
	founder := domaingame.AllianceViewer{PlayerID: 42, Validated: true, AllianceID: 7, RankID: 0, Founder: true, RankRights: domaingame.AllianceFounderRights}

	repository := NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if issue, err := repository.createAlliance(ctx, member, domaingame.AllianceMutation{Tag: "TAG", Name: "Alliance"}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected create permission issue, got issue=%+v err=%v", issue, err)
	}

	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("tag lookup failed")}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if _, err := repository.createAlliance(ctx, outsider, domaingame.AllianceMutation{Tag: "TAG", Name: "Alliance"}); err == nil || !strings.Contains(err.Error(), "tag lookup failed") {
		t.Fatalf("expected tag lookup error, got %v", err)
	}

	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		insertID:    7,
		execErr:     errors.New("rank insert failed"),
		execErrAt:   2,
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.createAlliance(ctx, outsider, domaingame.AllianceMutation{Tag: "TAG", Name: "Alliance"}); err == nil || !strings.Contains(err.Error(), "rank insert failed") {
		t.Fatalf("expected rank insert error, got %v", err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		insertID:    7,
		execErr:     errors.New("founder update failed"),
		execErrAt:   3,
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.createAlliance(ctx, outsider, domaingame.AllianceMutation{Tag: "TAG", Name: "Alliance"}); err == nil || !strings.Contains(err.Error(), "founder update failed") {
		t.Fatalf("expected founder update error, got %v", err)
	}

	if issue, err := repository.applyAlliance(ctx, member, domaingame.AllianceMutation{AllianceID: 7}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected apply permission issue, got issue=%+v err=%v", issue, err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("pending failed")}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if _, err := repository.applyAlliance(ctx, outsider, domaingame.AllianceMutation{AllianceID: 7}); err == nil || !strings.Contains(err.Error(), "pending failed") {
		t.Fatalf("expected pending error, got %v", err)
	}
	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "Alliance", 1, 0))},
		}},
		execErr: errors.New("application insert failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.applyAlliance(ctx, outsider, domaingame.AllianceMutation{AllianceID: 7}); err == nil || !strings.Contains(err.Error(), "application insert failed") {
		t.Fatalf("expected application insert error, got %v", err)
	}

	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if issue, err := repository.withdrawApplication(ctx, outsider); err != nil || issue.Code != domaingame.AllianceIssueApplicationNotFound {
		t.Fatalf("expected missing withdraw issue, got issue=%+v err=%v", issue, err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("withdraw load failed")}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if _, err := repository.withdrawApplication(ctx, outsider); err == nil || !strings.Contains(err.Error(), "withdraw load failed") {
		t.Fatalf("expected withdraw load error, got %v", err)
	}
	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}}},
		execErr:     errors.New("delete failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.withdrawApplication(ctx, outsider); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected withdraw delete error, got %v", err)
	}

	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("load app failed")}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if _, err := repository.reviewApplication(ctx, founder, domaingame.AllianceMutation{Action: "accept", ApplicationID: 11}); err == nil || !strings.Contains(err.Error(), "load app failed") {
		t.Fatalf("expected load app error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 8, 43, "newcomer", "hello", 1000))}}}}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if issue, err := repository.reviewApplication(ctx, founder, domaingame.AllianceMutation{Action: "accept", ApplicationID: 11}); err != nil || issue.Code != domaingame.AllianceIssueApplicationNotFound {
		t.Fatalf("expected wrong alliance issue, got issue=%+v err=%v", issue, err)
	}
	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}}},
		execErr:     errors.New("accept failed"),
		execErrAt:   1,
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reviewApplication(ctx, founder, domaingame.AllianceMutation{Action: "accept", ApplicationID: 11}); err == nil || !strings.Contains(err.Error(), "accept failed") {
		t.Fatalf("expected accept error, got %v", err)
	}
	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}}},
		execErr:     errors.New("accept delete failed"),
		execErrAt:   2,
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reviewApplication(ctx, founder, domaingame.AllianceMutation{Action: "accept", ApplicationID: 11}); err == nil || !strings.Contains(err.Error(), "accept delete failed") {
		t.Fatalf("expected accept delete error, got %v", err)
	}
	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "hello", 1000))}}},
		execErr:     errors.New("reject delete failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reviewApplication(ctx, founder, domaingame.AllianceMutation{Action: "reject", ApplicationID: 11}); err == nil || !strings.Contains(err.Error(), "reject delete failed") {
		t.Fatalf("expected reject delete error, got %v", err)
	}

	if issue, err := repository.leaveAlliance(ctx, outsider); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected leave permission issue, got issue=%+v err=%v", issue, err)
	}
	runner = &fakeAllianceRunner{execErr: errors.New("leave failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.leaveAlliance(ctx, member); err == nil || !strings.Contains(err.Error(), "leave failed") {
		t.Fatalf("expected leave exec error, got %v", err)
	}
}

func allianceViewerRow(playerID int, name string, validated int, allianceID int, rankID int, rankName string, rights int) []any {
	return []any{playerID, name, validated, allianceID, rankID, rankName, rights}
}

func allianceInfoRow(id int, tag string, name string, open int, applications int) []any {
	return []any{id, tag, name, 42, "", "", open, 0, "Welcome to the alliance page", "Internal", "Apply here", "", "", int64(0), int64(0), 2, applications}
}

func allianceApplicationRow(id int, allianceID int, playerID int, playerName string, text string, date int64) []any {
	return []any{id, allianceID, playerID, playerName, text, date}
}

func allianceRankRow(id int, name string, rights int) []any {
	return []any{id, name, rights}
}

type fakeAllianceRunner struct {
	fakeQueryer
	insertID  int64
	execErr   error
	execErrAt int
	lastIDErr error
	execCalls []fakeExecCall
}

func (f *fakeAllianceRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execCalls = append(f.execCalls, fakeExecCall{sql: query, args: args})
	if f.execErr != nil && (f.execErrAt == 0 || len(f.execCalls) == f.execErrAt) {
		return allianceSQLResult{insertID: f.insertID, err: f.lastIDErr}, f.execErr
	}
	return allianceSQLResult{insertID: f.insertID, err: f.lastIDErr}, nil
}

type allianceSQLResult struct {
	insertID int64
	err      error
}

func (r allianceSQLResult) LastInsertId() (int64, error) {
	return r.insertID, r.err
}

func (r allianceSQLResult) RowsAffected() (int64, error) {
	return 1, nil
}

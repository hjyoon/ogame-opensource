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

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights), allianceRankRow(1, "Newcomer", 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{
		PlayerID: 42,
		PlanetID: 99,
		View:     domaingame.AllianceViewCircular,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewCircular || len(alliance.Ranks) != 2 {
		t.Fatalf("unexpected circular view: %+v", alliance)
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
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(8, "APP", "Apply Alliance", 1, 1))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 42, PlanetID: 99, View: domaingame.AllianceViewApply, AllianceID: 8})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewApply || alliance.Own == nil || alliance.Target == nil || alliance.Target.Tag != "APP" {
		t.Fatalf("expected own-alliance apply view to preserve target, got %+v", alliance)
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
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "newcomer", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceApplicationRow(11, 7, 43, "newcomer", "pending", 1234))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 2, 1))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 43, PlanetID: 99, View: domaingame.AllianceViewSearch})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewNoAlliance || alliance.Pending == nil ||
		alliance.Pending.Text != "pending" || alliance.Target == nil || alliance.Target.Tag != "TAG" {
		t.Fatalf("unexpected pending no-alliance view: %+v", alliance)
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allianceViewerRow(43, "viewer", 1, 0, 0, "", 0))},
		fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(8, "PUB", "Public Alliance", 3, 0))},
	)}
	repository = NewAllianceRepositoryWithQueryer(queryer, "ogame_", time.Now)
	alliance, err = repository.GetAlliance(context.Background(), appgame.AllianceQuery{PlayerID: 43, PlanetID: 99, View: domaingame.AllianceViewInfo, AllianceID: 8})
	if err != nil {
		t.Fatal(err)
	}
	if alliance.View != domaingame.AllianceViewInfo || alliance.Target == nil || alliance.Target.ID != 8 ||
		alliance.Target.MemberCount != 2 || !alliance.Target.Open {
		t.Fatalf("unexpected public info view: %+v", alliance)
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

func TestAllianceRepositoryManagesRanksMembersAndCircularMessages(t *testing.T) {
	ctx := context.Background()
	founder := domaingame.AllianceViewer{PlayerID: 42, Name: "legor", AllianceID: 7, Founder: true}

	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2})}}},
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1000, 0) })
	issue, err := repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{Action: "add_rank", RankName: "Officer"})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 2 ||
		!strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_allyranks`") ||
		runner.execCalls[0].args[0] != 2 || runner.execCalls[0].args[2] != "Officer" ||
		!strings.Contains(runner.execCalls[1].sql, "nextrank = nextrank + 1") {
		t.Fatalf("unexpected add rank issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues(
				allianceRankRow(0, "Founder", domaingame.AllianceFounderRights),
				allianceRankRow(1, "Newcomer", 0),
				allianceRankRow(2, "Officer", 0),
				allianceRankRow(3, "Silent", 0x1ff),
			),
		}}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err = repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{
		Action:     "save_ranks",
		RankRights: []domaingame.AllianceRank{{ID: 2, Rights: domaingame.AllianceRightMembers | domaingame.AllianceRightCircular}},
	})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 2 ||
		runner.execCalls[0].args[0] != domaingame.AllianceRightMembers|domaingame.AllianceRightCircular ||
		runner.execCalls[1].args[0] != 0 {
		t.Fatalf("unexpected save rank rights issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err = repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{Action: "delete_rank", RankID: 2})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 1 ||
		!strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_allyranks`") ||
		runner.execCalls[0].args[0] != 7 || runner.execCalls[0].args[1] != 2 {
		t.Fatalf("unexpected delete rank issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err = repository.assignMemberRank(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43, TargetRankID: 2})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 1 ||
		!strings.Contains(runner.execCalls[0].sql, "SET allyrank = ?") ||
		runner.execCalls[0].args[0] != 2 || runner.execCalls[0].args[1] != 43 {
		t.Fatalf("unexpected assign rank issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err = repository.kickAllianceMember(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || len(runner.execCalls) != 1 ||
		!strings.Contains(runner.execCalls[0].sql, "SET ally_id = 0") ||
		runner.execCalls[0].args[0] != 43 {
		t.Fatalf("unexpected kick member issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			{rows: fakeRowsFromValues([]any{43, "member"})},
			{rows: fakeRowsFromValues([]any{0})},
		}},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1234, 0) })
	issue, circular, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{CircularRankID: 2, Text: "hello 'rank'"})
	if err != nil || issue.Code != domaingame.AllianceIssueSent || circular == nil || len(circular.Recipients) != 1 ||
		len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_messages`") ||
		runner.execCalls[0].args[0] != 43 || runner.execCalls[0].args[1] != domaingame.MessageTypeAlliance ||
		!strings.Contains(runner.execCalls[0].args[4].(string), "&rsquo;rank&rsquo;") {
		t.Fatalf("unexpected circular issue=%+v result=%+v err=%v exec=%+v", issue, circular, err, runner.execCalls)
	}
}

func TestAllianceRepositoryMutatesRankMemberCircularBranches(t *testing.T) {
	founderRow := allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights)

	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
			{rows: fakeRowsFromValues([]any{2})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights), allianceRankRow(2, "Officer", 0))},
		)...)},
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "add_rank", RankName: "Officer"},
	})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || alliance.View != domaingame.AllianceViewRanks || len(alliance.Ranks) != 2 {
		t.Fatalf("unexpected mutate add rank alliance=%+v issue=%+v err=%v", alliance, issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues([]any{43, "member", 2, "Officer", int64(1000), int64(500), int64(600), 1, 2, 3})},
		)...)},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "assign_rank", TargetPlayerID: 43, TargetRankID: 2},
	})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || alliance.View != domaingame.AllianceViewMembers || len(alliance.Members) != 1 || len(runner.execCalls) != 1 {
		t.Fatalf("unexpected mutate assign rank alliance=%+v issue=%+v err=%v exec=%+v", alliance, issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...)},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "kick_member", TargetPlayerID: 43},
	})
	if err != nil || issue.Code != domaingame.AllianceIssueSaved || alliance.View != domaingame.AllianceViewMembers || len(runner.execCalls) != 1 {
		t.Fatalf("unexpected mutate kick member alliance=%+v issue=%+v err=%v exec=%+v", alliance, issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			{rows: fakeRowsFromValues()},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
		)...)},
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	alliance, issue, err = repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "send_circular", Text: "hello"},
	})
	if err != nil || issue.Code != domaingame.AllianceIssueSent || alliance.View != domaingame.AllianceViewCircular || alliance.CircularResult == nil {
		t.Fatalf("unexpected mutate circular alliance=%+v issue=%+v err=%v", alliance, issue, err)
	}
}

func TestAllianceRepositoryMutateRankMemberCircularErrorsReload(t *testing.T) {
	founderRow := allianceViewerRow(42, "legor", 1, 7, 0, "Founder", domaingame.AllianceFounderRights)

	runner := &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
			{rows: fakeRowsFromValues([]any{2})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
		)...)},
		execErr: errors.New("mutate add rank failed"),
	}
	repository := NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "add_rank", RankName: "Officer"},
	}); err == nil || !strings.Contains(err.Error(), "mutate add rank failed") || issue != nil {
		t.Fatalf("expected mutate add rank error, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...)},
		execErr: errors.New("mutate assign failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "assign_rank", TargetPlayerID: 43, TargetRankID: 2},
	}); err == nil || !strings.Contains(err.Error(), "mutate assign failed") || issue != nil {
		t.Fatalf("expected mutate assign error, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues()},
		)...)},
		execErr: errors.New("mutate kick failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "kick_member", TargetPlayerID: 43},
	}); err == nil || !strings.Contains(err.Error(), "mutate kick failed") || issue != nil {
		t.Fatalf("expected mutate kick error, issue=%+v err=%v", issue, err)
	}

	runner = &fakeAllianceRunner{
		fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(founderRow)},
			{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			{rows: fakeRowsFromValues([]any{43, "member"})},
			{rows: fakeRowsFromValues([]any{0})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(founderRow)},
			fakeQueryResult{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
			fakeQueryResult{rows: fakeRowsFromValues(allianceRankRow(0, "Founder", domaingame.AllianceFounderRights))},
		)...)},
		execErr: errors.New("mutate circular failed"),
	}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, issue, err := repository.MutateAlliance(context.Background(), appgame.AllianceMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Query:    appgame.AllianceQuery{PlayerID: 42, PlanetID: 99},
		Mutation: domaingame.AllianceMutation{Action: "send_circular", Text: "hello"},
	}); err == nil || !strings.Contains(err.Error(), "mutate circular failed") || issue != nil {
		t.Fatalf("expected mutate circular error, issue=%+v err=%v", issue, err)
	}
}

func TestAllianceRepositoryPopulateOwnAlliancePermissionAndErrorBranches(t *testing.T) {
	ctx := context.Background()
	base := domaingame.Alliance{Viewer: domaingame.AllianceViewer{PlayerID: 43, AllianceID: 7, RankID: 1, RankName: "Newcomer"}}

	repository := NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	}}, "ogame_", time.Now)
	alliance, err := repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewMembers})
	if err != nil || alliance.View != domaingame.AllianceViewMembers || len(alliance.Members) != 0 {
		t.Fatalf("expected members permission short-circuit, alliance=%+v err=%v", alliance, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	}}, "ogame_", time.Now)
	alliance, err = repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewManagement, TextKind: 9})
	if err != nil || alliance.View != domaingame.AllianceViewManagement || alliance.TextKind != 1 || len(alliance.Ranks) != 0 {
		t.Fatalf("expected management permission short-circuit, alliance=%+v err=%v", alliance, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	}}, "ogame_", time.Now)
	alliance, err = repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewRenameTag})
	if err != nil || alliance.View != domaingame.AllianceViewRenameTag || alliance.Own == nil || alliance.Own.Tag != "TAG" {
		t.Fatalf("expected rename tag view to keep own alliance data, alliance=%+v err=%v", alliance, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	}}, "ogame_", time.Now)
	alliance, err = repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewRenameName})
	if err != nil || alliance.View != domaingame.AllianceViewRenameName || alliance.Own == nil || alliance.Own.Name != "The Alliance" {
		t.Fatalf("expected rename name view to keep own alliance data, alliance=%+v err=%v", alliance, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
	}}, "ogame_", time.Now)
	alliance, err = repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewRanks})
	if err != nil || alliance.View != domaingame.AllianceViewRanks || len(alliance.Ranks) != 0 {
		t.Fatalf("expected ranks permission short-circuit, alliance=%+v err=%v", alliance, err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		{err: errors.New("circular ranks failed")},
	}}, "ogame_", time.Now)
	if _, err := repository.populateOwnAlliance(ctx, base, appgame.AllianceQuery{View: domaingame.AllianceViewCircular}); err == nil || !strings.Contains(err.Error(), "circular ranks failed") {
		t.Fatalf("expected circular ranks error, got %v", err)
	}
}

func TestAllianceRepositoryRankMemberCircularHelperEdges(t *testing.T) {
	ctx := context.Background()
	founder := domaingame.AllianceViewer{PlayerID: 42, Name: "legor", AllianceID: 7, Founder: true}
	member := domaingame.AllianceViewer{PlayerID: 43, Name: "member", AllianceID: 7}

	repository := NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "ogame_", time.Now)
	if issue, err := repository.mutateAllianceRanks(ctx, member, domaingame.AllianceMutation{Action: "add_rank", RankName: "Officer"}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected rank permission issue, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.assignMemberRank(ctx, member, domaingame.AllianceMutation{TargetPlayerID: 44, TargetRankID: 2}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected assign permission issue, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.kickAllianceMember(ctx, member, domaingame.AllianceMutation{TargetPlayerID: 44}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected kick permission issue, got issue=%+v err=%v", issue, err)
	}
	if issue, circular, err := repository.sendCircularMessage(ctx, member, domaingame.AllianceMutation{}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission || circular != nil {
		t.Fatalf("expected circular permission issue, got issue=%+v circular=%+v err=%v", issue, circular, err)
	}
	if issue, err := repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{Action: "add_rank", RankName: "bad/rank"}); err != nil || issue.Code != domaingame.AllianceIssueInvalidRankName {
		t.Fatalf("expected invalid rank name issue, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{Action: "delete_rank", RankID: 1}); err != nil || issue.Code != domaingame.AllianceIssueSaved {
		t.Fatalf("expected protected rank delete no-op, got issue=%+v err=%v", issue, err)
	}
	if issue, err := repository.mutateAllianceRanks(ctx, founder, domaingame.AllianceMutation{Action: "unknown"}); err != nil || issue.Code != domaingame.AllianceIssueNoPermission {
		t.Fatalf("expected unknown rank mutation issue, got issue=%+v err=%v", issue, err)
	}

	runner := &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if issue, err := repository.addAllianceRank(ctx, 7, "Officer"); err != nil || issue.Code != domaingame.AllianceIssueSaved || runner.execCalls[0].args[0] != 2 {
		t.Fatalf("expected low nextrank to normalize to 2, issue=%+v err=%v exec=%+v", issue, err, runner.execCalls)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2})}}}, execErr: errors.New("rank insert failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.addAllianceRank(ctx, 7, "Officer"); err == nil || !strings.Contains(err.Error(), "rank insert failed") {
		t.Fatalf("expected rank insert error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2})}}}, execErr: errors.New("rank sequence update failed"), execErrAt: 2}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.addAllianceRank(ctx, 7, "Officer"); err == nil || !strings.Contains(err.Error(), "rank sequence update failed") {
		t.Fatalf("expected rank sequence update error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.addAllianceRank(ctx, 7, "Officer"); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected add rank prefix error, got %v", err)
	}

	runner = &fakeAllianceRunner{execErr: errors.New("delete rank failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.deleteAllianceRank(ctx, 7, 2); err == nil || !strings.Contains(err.Error(), "delete rank failed") {
		t.Fatalf("expected delete rank error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.deleteAllianceRank(ctx, 7, 2); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected delete rank prefix error, got %v", err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("rank list failed")}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.saveAllianceRankRights(ctx, 7, nil); err == nil || !strings.Contains(err.Error(), "rank list failed") {
		t.Fatalf("expected rank list error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
		rows: fakeRowsFromValues(allianceRankRow(2, "Officer", 0)),
	}}}, execErr: errors.New("rank rights failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.saveAllianceRankRights(ctx, 7, []domaingame.AllianceRank{{ID: 2, Rights: 0x3ff}}); err == nil || !strings.Contains(err.Error(), "rank rights failed") {
		t.Fatalf("expected rank rights exec error, got %v", err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", time.Now)
	if _, err := repository.loadNextAllianceRankID(ctx, "`ogame_ally`", 7); err == nil || !strings.Contains(err.Error(), "rank sequence not found") {
		t.Fatalf("expected missing rank sequence error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", time.Now)
	if _, err := repository.loadNextAllianceRankID(ctx, "`ogame_ally`", 7); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected rank sequence scan error, got %v", err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("recipients failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadCircularRecipients(ctx, 7, 0); err == nil || !strings.Contains(err.Error(), "recipients failed") {
		t.Fatalf("expected recipients query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "name"})}}}, "ogame_", time.Now)
	if _, err := repository.loadCircularRecipients(ctx, 7, 0); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected recipients scan error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{43, "member"})}}}, "ogame_", time.Now)
	recipients, err := repository.loadCircularRecipients(ctx, 7, 0)
	if err != nil || len(recipients) != 1 || recipients[0].PlayerID != 43 {
		t.Fatalf("unexpected all-rank recipients=%+v err=%v", recipients, err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		{rows: fakeRowsFromValues()},
	}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, circular, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{})
	if err != nil || issue.Code != domaingame.AllianceIssueSent || circular == nil || len(circular.Recipients) != 0 || len(runner.execCalls) != 0 {
		t.Fatalf("expected empty circular result, issue=%+v circular=%+v err=%v exec=%+v", issue, circular, err, runner.execCalls)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if issue, _, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{}); err != nil || issue.Code != domaingame.AllianceIssueAllianceNotFound {
		t.Fatalf("expected missing alliance issue, issue=%+v err=%v", issue, err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("circular alliance failed")}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, _, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{}); err == nil || !strings.Contains(err.Error(), "circular alliance failed") {
		t.Fatalf("expected circular alliance load error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		{err: errors.New("circular recipients failed")},
	}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, _, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{}); err == nil || !strings.Contains(err.Error(), "circular recipients failed") {
		t.Fatalf("expected circular recipients error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(allianceInfoRow(7, "TAG", "The Alliance", 1, 0))},
		{rows: fakeRowsFromValues([]any{43, "member"})},
		{rows: fakeRowsFromValues([]any{0})},
	}}, execErr: errors.New("circular insert failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, _, err := repository.sendCircularMessage(ctx, founder, domaingame.AllianceMutation{}); err == nil || !strings.Contains(err.Error(), "circular insert failed") {
		t.Fatalf("expected circular insert error, got %v", err)
	}

	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{127})}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(123, 0) })
	if err := repository.insertAllianceMessage(ctx, "`ogame_messages`", 43, "from", "subject", "text"); err != nil || len(runner.execCalls) != 2 ||
		!strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_messages`") || !strings.Contains(runner.execCalls[1].sql, "INSERT INTO `ogame_messages`") {
		t.Fatalf("expected retention delete then insert, err=%v exec=%+v", err, runner.execCalls)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("count failed")}}}}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.insertAllianceMessage(ctx, "`ogame_messages`", 43, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "count failed") {
		t.Fatalf("expected count error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{127})}}}, execErr: errors.New("delete oldest failed"), execErrAt: 1}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.insertAllianceMessage(ctx, "`ogame_messages`", 43, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "delete oldest failed") {
		t.Fatalf("expected delete oldest error, got %v", err)
	}
	runner = &fakeAllianceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, execErr: errors.New("insert message failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.insertAllianceMessage(ctx, "`ogame_messages`", 43, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "insert message failed") {
		t.Fatalf("expected insert message error, got %v", err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", time.Now)
	if _, err := repository.countAllianceMessages(ctx, "`ogame_messages`", 43); err == nil || !strings.Contains(err.Error(), "message count not found") {
		t.Fatalf("expected missing count error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", time.Now)
	if _, err := repository.countAllianceMessages(ctx, "`ogame_messages`", 43); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected count scan error, got %v", err)
	}

	runner = &fakeAllianceRunner{execErr: errors.New("assign failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.assignMemberRank(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43, TargetRankID: 2}); err == nil || !strings.Contains(err.Error(), "assign failed") {
		t.Fatalf("expected assign exec error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.assignMemberRank(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43, TargetRankID: 2}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected assign prefix error, got %v", err)
	}
	runner = &fakeAllianceRunner{execErr: errors.New("kick failed")}
	repository = NewAllianceRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.kickAllianceMember(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43}); err == nil || !strings.Contains(err.Error(), "kick failed") {
		t.Fatalf("expected kick exec error, got %v", err)
	}
	repository = NewAllianceRepositoryWithRunner(&fakeAllianceRunner{}, &fakeAllianceRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.kickAllianceMember(ctx, founder, domaingame.AllianceMutation{TargetPlayerID: 43}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected kick prefix error, got %v", err)
	}

	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("next rank query failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadNextAllianceRankID(ctx, "`ogame_ally`", 7); err == nil || !strings.Contains(err.Error(), "next rank query failed") {
		t.Fatalf("expected next rank query error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("next rank rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadNextAllianceRankID(ctx, "`ogame_ally`", 7); err == nil || !strings.Contains(err.Error(), "next rank rows failed") {
		t.Fatalf("expected next rank rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("next rank post rows failed"), []any{2})}}}, "ogame_", time.Now)
	if _, err := repository.loadNextAllianceRankID(ctx, "`ogame_ally`", 7); err == nil || !strings.Contains(err.Error(), "next rank post rows failed") {
		t.Fatalf("expected next rank post rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("recipient rows failed"), []any{43, "member"})}}}, "ogame_", time.Now)
	if _, err := repository.loadCircularRecipients(ctx, 7, 2); err == nil || !strings.Contains(err.Error(), "recipient rows failed") {
		t.Fatalf("expected recipient rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("message count rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.countAllianceMessages(ctx, "`ogame_messages`", 43); err == nil || !strings.Contains(err.Error(), "message count rows failed") {
		t.Fatalf("expected count rows error, got %v", err)
	}
	repository = NewAllianceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("message count post rows failed"), []any{1})}}}, "ogame_", time.Now)
	if _, err := repository.countAllianceMessages(ctx, "`ogame_messages`", 43); err == nil || !strings.Contains(err.Error(), "message count post rows failed") {
		t.Fatalf("expected count post rows error, got %v", err)
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

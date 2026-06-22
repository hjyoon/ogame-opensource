package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestAllianceServiceReturnsAllianceForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeAllianceRepository{alliance: domaingame.Alliance{Commander: "legor"}}
	service := NewAllianceService(sessions, repository)

	result, err := service.GetAlliance(context.Background(), AllianceCommand{
		PublicSession: "pub",
		PlanetID:      99,
		View:          domaingame.AllianceViewSearch,
		SearchText:    "TAG",
		AllianceID:    7,
		ApplicationID: 11,
	})
	if err != nil {
		t.Fatalf("GetAlliance returned error: %v", err)
	}
	if !result.Authenticated || result.Alliance.Commander != "legor" ||
		repository.query.PlayerID != 42 || repository.query.PlanetID != 99 ||
		repository.query.View != domaingame.AllianceViewSearch || repository.query.SearchText != "TAG" ||
		repository.query.AllianceID != 7 || repository.query.ApplicationID != 11 {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.query)
	}
}

func TestAllianceServiceMutatesForAuthenticatedSession(t *testing.T) {
	issue := domaingame.AllianceIssue(domaingame.AllianceIssueCreated)
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeAllianceRepository{alliance: domaingame.Alliance{Commander: "legor"}, issue: issue}
	service := NewAllianceService(sessions, repository)

	result, err := service.MutateAlliance(context.Background(), AllianceMutationCommand{
		PlanetID: 99,
		Query:    AllianceQuery{View: domaingame.AllianceViewCreate},
		Mutation: domaingame.AllianceMutation{Action: "create", Tag: "TAG", Name: "Alliance"},
	})
	if err != nil {
		t.Fatalf("MutateAlliance returned error: %v", err)
	}
	if !result.Authenticated || result.ActionIssue.Code != domaingame.AllianceIssueCreated ||
		repository.mutationQuery.PlayerID != 42 || repository.mutationQuery.PlanetID != 99 ||
		repository.mutationQuery.Query.View != domaingame.AllianceViewCreate ||
		repository.mutationQuery.Mutation.Tag != "TAG" {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.mutationQuery)
	}
}

func TestAllianceServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewAllianceService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeAllianceRepository{})
	result, err := service.GetAlliance(context.Background(), AllianceCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got result=%+v err=%v", result, err)
	}
	result, err = service.MutateAlliance(context.Background(), AllianceMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation result, got result=%+v err=%v", result, err)
	}
	if _, err := (AllianceService{}).GetAlliance(context.Background(), AllianceCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (AllianceService{}).MutateAlliance(context.Background(), AllianceMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewAllianceService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAllianceRepository{}).GetAlliance(context.Background(), AllianceCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewAllianceService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAllianceRepository{}).MutateAlliance(context.Background(), AllianceMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	authenticated := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	if _, err := NewAllianceService(authenticated, &fakeAllianceRepository{err: errors.New("alliance failed")}).GetAlliance(context.Background(), AllianceCommand{}); err == nil || !strings.Contains(err.Error(), "alliance failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewAllianceService(authenticated, &fakeAllianceRepository{err: errors.New("alliance failed")}).MutateAlliance(context.Background(), AllianceMutationCommand{}); err == nil || !strings.Contains(err.Error(), "alliance failed") {
		t.Fatalf("expected mutation repository error, got %v", err)
	}
}

type fakeAllianceRepository struct {
	alliance      domaingame.Alliance
	issue         *domaingame.AllianceActionIssue
	err           error
	query         AllianceQuery
	mutationQuery AllianceMutationQuery
}

func (f *fakeAllianceRepository) GetAlliance(_ context.Context, query AllianceQuery) (domaingame.Alliance, error) {
	f.query = query
	return f.alliance, f.err
}

func (f *fakeAllianceRepository) MutateAlliance(_ context.Context, query AllianceMutationQuery) (domaingame.Alliance, *domaingame.AllianceActionIssue, error) {
	f.mutationQuery = query
	return f.alliance, f.issue, f.err
}

package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestOfficersServiceReturnsOfficersForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOfficersRepository{officers: domaingame.Officers{Commander: "legor"}}
	service := NewOfficersService(sessions, repository)

	result, err := service.GetOfficers(context.Background(), OfficersCommand{PublicSession: "pub", PlanetID: 99})
	if err != nil {
		t.Fatalf("GetOfficers returned error: %v", err)
	}
	if !result.Authenticated || result.Officers.Commander != "legor" || repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.query)
	}
}

func TestOfficersServiceRecruitOfficerForAuthenticatedSession(t *testing.T) {
	issue := domaingame.OfficerRecruitedIssue()
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOfficersRepository{officers: domaingame.Officers{Commander: "legor"}, issue: issue}
	service := NewOfficersService(sessions, repository)

	result, err := service.RecruitOfficer(context.Background(), OfficersMutationCommand{
		PlanetID: 99,
		Mutation: domaingame.OfficerMutation{
			OfficerID: domaingame.OfficerEngineer,
			Days:      domaingame.OfficerWeekDays,
		},
	})
	if err != nil {
		t.Fatalf("RecruitOfficer returned error: %v", err)
	}
	if !result.Authenticated || result.ActionIssue.Code != domaingame.OfficerIssueRecruited ||
		repository.mutationQuery.Mutation.OfficerID != domaingame.OfficerEngineer {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.mutationQuery)
	}
}

func TestOfficersServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewOfficersService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeOfficersRepository{})
	result, err := service.GetOfficers(context.Background(), OfficersCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got result=%+v err=%v", result, err)
	}

	result, err = service.RecruitOfficer(context.Background(), OfficersMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation result, got result=%+v err=%v", result, err)
	}

	if _, err := (OfficersService{}).GetOfficers(context.Background(), OfficersCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (OfficersService{}).RecruitOfficer(context.Background(), OfficersMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewOfficersService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeOfficersRepository{}).GetOfficers(context.Background(), OfficersCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewOfficersService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeOfficersRepository{}).RecruitOfficer(context.Background(), OfficersMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	if _, err := NewOfficersService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, &fakeOfficersRepository{err: errors.New("officers failed")}).GetOfficers(context.Background(), OfficersCommand{}); err == nil || !strings.Contains(err.Error(), "officers failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewOfficersService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, &fakeOfficersRepository{err: errors.New("officers failed")}).RecruitOfficer(context.Background(), OfficersMutationCommand{}); err == nil || !strings.Contains(err.Error(), "officers failed") {
		t.Fatalf("expected mutation repository error, got %v", err)
	}
}

type fakeOfficersRepository struct {
	officers      domaingame.Officers
	issue         *domaingame.OfficerActionIssue
	err           error
	query         OfficersQuery
	mutationQuery OfficersMutationQuery
}

func (f *fakeOfficersRepository) GetOfficers(_ context.Context, query OfficersQuery) (domaingame.Officers, error) {
	f.query = query
	return f.officers, f.err
}

func (f *fakeOfficersRepository) RecruitOfficer(_ context.Context, query OfficersMutationQuery) (domaingame.Officers, *domaingame.OfficerActionIssue, error) {
	f.mutationQuery = query
	return f.officers, f.issue, f.err
}

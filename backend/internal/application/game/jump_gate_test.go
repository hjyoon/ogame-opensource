package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestJumpGateServiceReturnsAuthenticatedScreenAndMutation(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeJumpGateRepository{jumpGate: domaingame.JumpGate{Commander: "legor"}}
	service := NewJumpGateService(sessions, repository)

	result, err := service.GetJumpGate(context.Background(), JumpGateCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.JumpGate.Commander != "legor" || repository.query.PlayerID != 42 || repository.query.PlanetID != 10 {
		t.Fatalf("unexpected jump gate result=%+v query=%+v", result, repository.query)
	}
	if sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}

	mutation, err := service.Jump(context.Background(), JumpGateMutationCommand{
		PublicSession: "public",
		PlanetID:      10,
		SourceMoonID:  10,
		TargetMoonID:  20,
		Ships:         map[int]int{domaingame.FleetSmallCargo: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !mutation.Authenticated || repository.mutation.PlayerID != 42 || repository.mutation.TargetMoonID != 20 || repository.mutation.Ships[domaingame.FleetSmallCargo] != 2 {
		t.Fatalf("unexpected jump gate mutation=%+v query=%+v", mutation, repository.mutation)
	}
}

func TestJumpGateServiceReturnsUnauthenticatedIssuesAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewJumpGateService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeJumpGateRepository{})
	result, err := service.GetJumpGate(context.Background(), JumpGateCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
	mutation, err := service.Jump(context.Background(), JumpGateMutationCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if mutation.Authenticated || len(mutation.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation issue, got %+v", mutation)
	}
	if _, err := (JumpGateService{}).GetJumpGate(context.Background(), JumpGateCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (JumpGateService{}).Jump(context.Background(), JumpGateMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewJumpGateService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeJumpGateRepository{}).GetJumpGate(context.Background(), JumpGateCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected get session error, got %v", err)
	}
	if _, err := NewJumpGateService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeJumpGateRepository{}).Jump(context.Background(), JumpGateMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewJumpGateService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeJumpGateRepository{err: errors.New("jump failed")}).GetJumpGate(context.Background(), JumpGateCommand{}); err == nil || !strings.Contains(err.Error(), "jump failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewJumpGateService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeJumpGateRepository{err: errors.New("jump failed")}).Jump(context.Background(), JumpGateMutationCommand{}); err == nil || !strings.Contains(err.Error(), "jump failed") {
		t.Fatalf("expected mutation repository error, got %v", err)
	}
}

type fakeJumpGateRepository struct {
	jumpGate domaingame.JumpGate
	query    JumpGateQuery
	mutation JumpGateMutationQuery
	err      error
}

func (f *fakeJumpGateRepository) GetJumpGate(_ context.Context, query JumpGateQuery) (domaingame.JumpGate, error) {
	f.query = query
	if f.err != nil {
		return domaingame.JumpGate{}, f.err
	}
	return f.jumpGate, nil
}

func (f *fakeJumpGateRepository) Jump(_ context.Context, query JumpGateMutationQuery) (domaingame.JumpGate, error) {
	f.mutation = query
	if f.err != nil {
		return domaingame.JumpGate{}, f.err
	}
	return f.jumpGate, nil
}

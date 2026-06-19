package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestEmpireServiceReturnsEmpireForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeEmpireRepository{empire: domaingame.Empire{Commander: "legor"}}
	service := NewEmpireService(sessions, repository)

	result, err := service.GetEmpire(context.Background(), EmpireCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
		PlanetType:      domaingame.EmpirePlanetTypeMoons,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Empire.Commander != "legor" ||
		repository.query.PlayerID != 42 || repository.query.PlanetID != 99 ||
		repository.query.PlanetType != domaingame.EmpirePlanetTypeMoons ||
		sessions.command.PublicSession != "public" {
		t.Fatalf("unexpected empire result/query: result=%+v query=%+v session=%+v", result, repository.query, sessions.command)
	}
}

func TestEmpireServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewEmpireService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeEmpireRepository{})
	result, err := service.GetEmpire(context.Background(), EmpireCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got %+v", result)
	}

	if _, err := (EmpireService{}).GetEmpire(context.Background(), EmpireCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewEmpireService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeEmpireRepository{}).GetEmpire(context.Background(), EmpireCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewEmpireService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeEmpireRepository{err: errors.New("empire failed")}).GetEmpire(context.Background(), EmpireCommand{}); err == nil || !strings.Contains(err.Error(), "empire failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeEmpireRepository struct {
	empire domaingame.Empire
	issue  *domaingame.EmpireActionIssue
	err    error
	query  EmpireQuery
}

func (f *fakeEmpireRepository) GetEmpire(_ context.Context, query EmpireQuery) (domaingame.Empire, *domaingame.EmpireActionIssue, error) {
	f.query = query
	return f.empire, f.issue, f.err
}

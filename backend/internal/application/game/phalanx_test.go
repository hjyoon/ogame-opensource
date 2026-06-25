package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestPhalanxServiceReturnsAuthenticatedReport(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakePhalanxRepository{phalanx: domaingame.Phalanx{Commander: "legor"}}
	service := NewPhalanxService(sessions, repository)

	result, err := service.GetPhalanx(context.Background(), PhalanxCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        10,
		TargetPlanetID:  20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Phalanx.Commander != "legor" {
		t.Fatalf("unexpected phalanx result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 10 || repository.query.TargetPlanetID != 20 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestPhalanxServiceReturnsUnauthenticatedIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewPhalanxService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakePhalanxRepository{})

	result, err := service.GetPhalanx(context.Background(), PhalanxCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestPhalanxServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (PhalanxService{}).GetPhalanx(context.Background(), PhalanxCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewPhalanxService(&fakeSessionLookup{err: errors.New("session failed")}, &fakePhalanxRepository{}).GetPhalanx(context.Background(), PhalanxCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewPhalanxService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakePhalanxRepository{err: errors.New("phalanx failed")}).GetPhalanx(context.Background(), PhalanxCommand{}); err == nil || !strings.Contains(err.Error(), "phalanx failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakePhalanxRepository struct {
	phalanx domaingame.Phalanx
	query   PhalanxQuery
	err     error
}

func (f *fakePhalanxRepository) GetPhalanx(_ context.Context, query PhalanxQuery) (domaingame.Phalanx, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Phalanx{}, f.err
	}
	return f.phalanx, nil
}

package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestNotesServiceReturnsNotesForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeNotesRepository{notes: domaingame.Notes{Commander: "legor", Action: domaingame.NotesActionList}}
	service := NewNotesService(sessions, repository)

	result, err := service.GetNotes(context.Background(), NotesCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
		Action:          2,
		NoteID:          7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Notes.Commander != "legor" {
		t.Fatalf("unexpected notes result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Action != 2 || repository.query.NoteID != 7 {
		t.Fatalf("unexpected notes query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestNotesServiceReturnsUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewNotesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeNotesRepository{})

	result, err := service.GetNotes(context.Background(), NotesCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestNotesServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (NotesService{}).GetNotes(context.Background(), NotesCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewNotesService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeNotesRepository{}).GetNotes(context.Background(), NotesCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewNotesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeNotesRepository{err: errors.New("notes failed")}).GetNotes(context.Background(), NotesCommand{}); err == nil || !strings.Contains(err.Error(), "notes failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeNotesRepository struct {
	notes domaingame.Notes
	query NotesQuery
	err   error
}

func (f *fakeNotesRepository) GetNotes(_ context.Context, query NotesQuery) (domaingame.Notes, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Notes{}, f.err
	}
	return f.notes, nil
}

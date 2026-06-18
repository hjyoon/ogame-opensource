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

func TestNotesServiceCreatesUpdatesAndDeletesNotes(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeNotesRepository{notes: domaingame.Notes{Commander: "legor"}}
	service := NewNotesService(sessions, repository)

	result, err := service.CreateNote(context.Background(), NotesMutationCommand{
		PublicSession: "public",
		PlanetID:      99,
		Subject:       `<subject>`,
		Text:          "body",
		Priority:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || repository.mutationQuery.PlayerID != 42 || repository.mutationQuery.PlanetID != 99 ||
		repository.mutationQuery.Draft.Subject != "&lt;subject&gt;" || repository.mutationQuery.Draft.Priority != 2 {
		t.Fatalf("unexpected create mutation: result=%+v query=%+v", result, repository.mutationQuery)
	}

	_, err = service.UpdateNote(context.Background(), NotesMutationCommand{NoteID: 11, Subject: "", Text: "", Priority: -1})
	if err != nil {
		t.Fatal(err)
	}
	if repository.mutationQuery.NoteID != 11 || repository.mutationQuery.Draft.Subject != "no subject" || repository.mutationQuery.Draft.Priority != 0 {
		t.Fatalf("unexpected update mutation: %+v", repository.mutationQuery)
	}

	_, err = service.DeleteNotes(context.Background(), NotesMutationCommand{NoteIDs: []int{11, 0, 11, 12}})
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.deleteQuery.NoteIDs) != 2 || repository.deleteQuery.NoteIDs[0] != 11 || repository.deleteQuery.NoteIDs[1] != 12 {
		t.Fatalf("unexpected delete mutation: %+v", repository.deleteQuery)
	}
}

func TestNotesServiceMutationsReturnUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewNotesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeNotesRepository{})

	for _, call := range []struct {
		name string
		fn   func(context.Context, NotesMutationCommand) (NotesResult, error)
	}{
		{"create", service.CreateNote},
		{"update", service.UpdateNote},
		{"delete", service.DeleteNotes},
	} {
		t.Run(call.name, func(t *testing.T) {
			result, err := call.fn(context.Background(), NotesMutationCommand{PublicSession: "bad"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
				t.Fatalf("expected unauthenticated issue, got %+v", result)
			}
		})
	}
}

func TestNotesServiceMutationErrors(t *testing.T) {
	authenticated := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	for _, call := range []struct {
		name string
		fn   func(NotesService, context.Context, NotesMutationCommand) (NotesResult, error)
	}{
		{"create", NotesService.CreateNote},
		{"update", NotesService.UpdateNote},
		{"delete", NotesService.DeleteNotes},
	} {
		t.Run(call.name+" dependencies", func(t *testing.T) {
			if _, err := call.fn(NotesService{}, context.Background(), NotesMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
				t.Fatalf("expected dependency error, got %v", err)
			}
		})
		t.Run(call.name+" session", func(t *testing.T) {
			service := NewNotesService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeNotesRepository{})
			if _, err := call.fn(service, context.Background(), NotesMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
				t.Fatalf("expected session error, got %v", err)
			}
		})
		t.Run(call.name+" repository", func(t *testing.T) {
			service := NewNotesService(authenticated, &fakeNotesRepository{err: errors.New("notes failed")})
			if _, err := call.fn(service, context.Background(), NotesMutationCommand{NoteID: 1, NoteIDs: []int{1}}); err == nil || !strings.Contains(err.Error(), "notes failed") {
				t.Fatalf("expected repository error, got %v", err)
			}
		})
	}
}

type fakeNotesRepository struct {
	notes         domaingame.Notes
	query         NotesQuery
	mutationQuery NotesMutationQuery
	deleteQuery   NotesDeleteQuery
	err           error
}

func (f *fakeNotesRepository) GetNotes(_ context.Context, query NotesQuery) (domaingame.Notes, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Notes{}, f.err
	}
	return f.notes, nil
}

func (f *fakeNotesRepository) CreateNote(_ context.Context, query NotesMutationQuery) (domaingame.Notes, error) {
	f.mutationQuery = query
	if f.err != nil {
		return domaingame.Notes{}, f.err
	}
	return f.notes, nil
}

func (f *fakeNotesRepository) UpdateNote(_ context.Context, query NotesMutationQuery) (domaingame.Notes, error) {
	f.mutationQuery = query
	if f.err != nil {
		return domaingame.Notes{}, f.err
	}
	return f.notes, nil
}

func (f *fakeNotesRepository) DeleteNotes(_ context.Context, query NotesDeleteQuery) (domaingame.Notes, error) {
	f.deleteQuery = query
	if f.err != nil {
		return domaingame.Notes{}, f.err
	}
	return f.notes, nil
}

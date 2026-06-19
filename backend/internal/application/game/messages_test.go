package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestMessagesServiceReturnsMessagesForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeMessagesRepository{messages: domaingame.Messages{
		Commander: "legor",
		Action:    domaingame.MessagesActionInbox,
		Rows: []domaingame.Message{{
			ID:      11,
			From:    "from {PUBLIC_SESSION}",
			Subject: "subject {PUBLIC_SESSION}",
			Text:    "text {PUBLIC_SESSION}",
		}},
	}}
	service := NewMessagesService(sessions, repository)

	result, err := service.GetMessages(context.Background(), MessagesCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
		TargetPlayerID:  77,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Messages.Commander != "legor" || result.Messages.Rows[0].Text != "text public" {
		t.Fatalf("unexpected messages result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.TargetPlayerID != 77 {
		t.Fatalf("unexpected messages query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestMessagesServiceLeavesPublicSessionPlaceholderWhenMissing(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeMessagesRepository{messages: domaingame.Messages{
		Rows: []domaingame.Message{{Text: "{PUBLIC_SESSION}"}},
	}}
	service := NewMessagesService(sessions, repository)

	result, err := service.GetMessages(context.Background(), MessagesCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Messages.Rows[0].Text != "{PUBLIC_SESSION}" {
		t.Fatalf("expected placeholder to be preserved without public session, got %+v", result.Messages.Rows[0])
	}
}

func TestMessagesServiceReturnsUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewMessagesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeMessagesRepository{})

	result, err := service.GetMessages(context.Background(), MessagesCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestMessagesServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (MessagesService{}).GetMessages(context.Background(), MessagesCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewMessagesService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeMessagesRepository{}).GetMessages(context.Background(), MessagesCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewMessagesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeMessagesRepository{err: errors.New("messages failed")}).GetMessages(context.Background(), MessagesCommand{}); err == nil || !strings.Contains(err.Error(), "messages failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeMessagesRepository struct {
	messages domaingame.Messages
	query    MessagesQuery
	err      error
}

func (f *fakeMessagesRepository) GetMessages(_ context.Context, query MessagesQuery) (domaingame.Messages, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Messages{}, f.err
	}
	return f.messages, nil
}

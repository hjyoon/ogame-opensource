package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestAdminServiceReturnsAdminForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeAdminRepository{admin: domaingame.Admin{
		Commander: "legor",
		Viewer:    domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin},
	}}
	service := NewAdminService(sessions, repository)

	result, err := service.GetAdmin(context.Background(), AdminCommand{
		PublicSession:   "pub",
		PrivateSessions: map[string]string{"prsess_42_1": "priv"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Mode:            "Users",
	})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if !result.Authenticated || result.Admin.Commander != "legor" || result.ActionIssue != nil ||
		repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Mode != "Users" ||
		sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected result=%+v query=%+v session=%+v", result, repository.query, sessions.command)
	}
}

func TestAdminServiceReturnsAccessDeniedForRegularUser(t *testing.T) {
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		&fakeAdminRepository{admin: domaingame.Admin{Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelPlayer}}},
	)

	result, err := service.GetAdmin(context.Background(), AdminCommand{})

	if err != nil || !result.Authenticated || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.AdminIssueAccessDenied {
		t.Fatalf("expected access denied, result=%+v err=%v", result, err)
	}
}

func TestAdminServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewAdminService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeAdminRepository{})
	result, err := service.GetAdmin(context.Background(), AdminCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got result=%+v err=%v", result, err)
	}
	if _, err := (AdminService{}).GetAdmin(context.Background(), AdminCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewAdminService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAdminRepository{}).GetAdmin(context.Background(), AdminCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	authenticated := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{err: errors.New("admin failed")}).GetAdmin(context.Background(), AdminCommand{}); err == nil || !strings.Contains(err.Error(), "admin failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeAdminRepository struct {
	admin domaingame.Admin
	err   error
	query AdminQuery
}

func (f *fakeAdminRepository) GetAdmin(_ context.Context, query AdminQuery) (domaingame.Admin, error) {
	f.query = query
	return f.admin, f.err
}

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

func TestAdminServiceReturnsAccessDeniedForRestrictedOperatorMode(t *testing.T) {
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		&fakeAdminRepository{admin: domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelOperator}}},
	)

	result, err := service.GetAdmin(context.Background(), AdminCommand{})

	if err != nil || !result.Authenticated || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.AdminIssueAccessDenied {
		t.Fatalf("expected restricted operator access denied, result=%+v err=%v", result, err)
	}
}

func TestAdminServiceMutatesAdminAndRefreshes(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
	repository := &fakeAdminRepository{
		admin:       domaingame.Admin{Mode: "Bans", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin}},
		actionIssue: issue,
	}
	service := NewAdminService(sessions, repository)

	result, err := service.MutateAdmin(context.Background(), AdminMutationCommand{
		PublicSession:   "pub",
		PrivateSessions: map[string]string{"prsess_42_1": "priv"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Mode:            "Bans",
		Action:          "ban",
		TargetIDs:       []int{77},
		BanMode:         1,
		Hours:           2,
		Reason:          "test",
		Values:          map[string]int{"dm_factor": 9},
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if !result.Authenticated || result.ActionIssue != issue || repository.mutation.PlayerID != 42 ||
		repository.mutation.TargetIDs[0] != 77 || repository.mutation.BanMode != 1 || repository.mutation.Values["dm_factor"] != 9 || repository.query.Mode != "Bans" {
		t.Fatalf("unexpected result=%+v mutation=%+v query=%+v", result, repository.mutation, repository.query)
	}
}

func TestAdminServiceMutationReturnsAccessDeniedWithoutMutating(t *testing.T) {
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		&fakeAdminRepository{admin: domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelOperator}}},
	)

	result, err := service.MutateAdmin(context.Background(), AdminMutationCommand{Mode: "BotEdit"})

	repository := service.repository.(*fakeAdminRepository)
	if err != nil || !result.Authenticated || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.AdminIssueAccessDenied || repository.mutated {
		t.Fatalf("expected access denied without mutation, result=%+v mutated=%v err=%v", result, repository.mutated, err)
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
	if _, err := (AdminService{}).MutateAdmin(context.Background(), AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewAdminService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAdminRepository{}).GetAdmin(context.Background(), AdminCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewAdminService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAdminRepository{}).MutateAdmin(context.Background(), AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	authenticated := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{err: errors.New("admin failed")}).GetAdmin(context.Background(), AdminCommand{}); err == nil || !strings.Contains(err.Error(), "admin failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{err: errors.New("admin failed")}).MutateAdmin(context.Background(), AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "admin failed") {
		t.Fatalf("expected mutation admin load error, got %v", err)
	}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{admin: domaingame.Admin{Viewer: domaingame.AdminViewer{Level: domaingame.AdminLevelAdmin}}, mutationErr: errors.New("mutate failed")}).MutateAdmin(context.Background(), AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "mutate failed") {
		t.Fatalf("expected mutation error, got %v", err)
	}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{admin: domaingame.Admin{Viewer: domaingame.AdminViewer{Level: domaingame.AdminLevelAdmin}}, reloadErr: errors.New("reload failed")}).MutateAdmin(context.Background(), AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "reload failed") {
		t.Fatalf("expected mutation reload error, got %v", err)
	}

	service = NewAdminService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeAdminRepository{})
	result, err = service.MutateAdmin(context.Background(), AdminMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation result, got result=%+v err=%v", result, err)
	}
}

type fakeAdminRepository struct {
	admin       domaingame.Admin
	err         error
	reloadErr   error
	mutationErr error
	actionIssue *domaingame.AdminActionIssue
	query       AdminQuery
	mutation    AdminMutationQuery
	mutated     bool
	getCalls    int
}

func (f *fakeAdminRepository) GetAdmin(_ context.Context, query AdminQuery) (domaingame.Admin, error) {
	f.query = query
	f.getCalls++
	if f.reloadErr != nil && f.getCalls > 1 {
		return domaingame.Admin{}, f.reloadErr
	}
	return f.admin, f.err
}

func (f *fakeAdminRepository) MutateAdmin(_ context.Context, query AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	f.mutation = query
	f.mutated = true
	return f.actionIssue, f.mutationErr
}

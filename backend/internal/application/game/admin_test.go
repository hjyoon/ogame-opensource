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
		TaskID:          1001,
		TargetIDs:       []int{77},
		BanMode:         1,
		Hours:           2,
		Reason:          "test",
		Values:          map[string]int{"dm_factor": 9},
		Universe:        &domaingame.AdminUniverseMutation{Speed: 8, Language: "de", Freeze: true},
		Category:        3,
		Subject:         "subject",
		Text:            "text",
		ReportIDs:       []int{701},
		DeleteMode:      "deletemarked",
		FileName:        "backup_test.json",
		Amount:          5000,
		ItemID:          55,
		DayMonth:        "31.12",
		HourMinute:      "23:59",
		InactiveDays:    7,
		IngameDays:      3,
		PeriodicDays:    14,
		ModName:         "GalaxyTool",
		Name:            "Bot Alpha",
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if !result.Authenticated || result.ActionIssue != issue || repository.mutation.PlayerID != 42 ||
		repository.mutation.TaskID != 1001 || repository.mutation.TargetIDs[0] != 77 || repository.mutation.BanMode != 1 ||
		repository.mutation.Values["dm_factor"] != 9 || repository.mutation.Universe == nil ||
		repository.mutation.Universe.Speed != 8 || repository.mutation.Universe.Language != "de" || !repository.mutation.Universe.Freeze || repository.mutation.Category != 3 ||
		repository.mutation.Subject != "subject" || repository.mutation.Text != "text" ||
		repository.mutation.ReportIDs[0] != 701 || repository.mutation.DeleteMode != "deletemarked" ||
		repository.mutation.FileName != "backup_test.json" ||
		repository.mutation.Amount != 5000 || repository.mutation.ItemID != 55 ||
		repository.mutation.DayMonth != "31.12" || repository.mutation.HourMinute != "23:59" ||
		repository.mutation.InactiveDays != 7 || repository.mutation.IngameDays != 3 ||
		repository.mutation.PeriodicDays != 14 || repository.mutation.RemoteAddr != "203.0.113.10" ||
		repository.mutation.ModName != "GalaxyTool" || repository.mutation.Name != "Bot Alpha" ||
		repository.query.Mode != "Bans" {
		t.Fatalf("unexpected result=%+v mutation=%+v query=%+v", result, repository.mutation, repository.query)
	}
}

func TestAdminServiceReloadsDeletedAdminPlanetHome(t *testing.T) {
	issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
	issue.Result = &domaingame.AdminActionResult{ItemID: 7}
	repository := &fakeAdminRepository{
		admin:       domaingame.Admin{Mode: "Planets", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin}},
		actionIssue: issue,
	}
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		repository,
	)
	planet := &domaingame.AdminPlanetMutation{Delete: true}
	search := &domaingame.AdminPlanetSearch{Type: "planetname", Text: "Alpha"}
	result, err := service.MutateAdmin(context.Background(), AdminMutationCommand{
		Mode: "Planets", PlanetID: 70, TargetPlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate,
		Planet: planet, PlanetSearch: search,
	})
	if err != nil || result.ActionIssue != issue || repository.mutation.Planet != planet || repository.mutation.PlanetSearch != search ||
		repository.query.TargetPlanetID != 7 || repository.query.PlanetSearch != search {
		t.Fatalf("result=%+v mutation=%+v reload=%+v err=%v", result, repository.mutation, repository.query, err)
	}
}

func TestAdminServiceSendsAndClearsCouponMail(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
	issue.OutboundCouponMails = []domaingame.AdminCouponMail{{Character: "Legor", Recipient: "legor@example.local", Code: "CODE"}}
	repository := &fakeAdminRepository{admin: domaingame.Admin{Mode: "Queue", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin}}, actionIssue: issue}
	mailer := &fakeAdminCouponMailer{}
	result, err := NewAdminServiceWithCouponMailer(sessions, repository, mailer).MutateAdmin(context.Background(), AdminMutationCommand{Mode: "Queue", Action: domaingame.AdminActionQueueCron})
	if err != nil || len(mailer.messages) != 1 || mailer.messages[0].Code != "CODE" || len(result.ActionIssue.OutboundCouponMails) != 0 {
		t.Fatalf("result=%+v messages=%+v err=%v", result, mailer.messages, err)
	}
	issue.OutboundCouponMails = []domaingame.AdminCouponMail{{Recipient: "legor@example.local"}}
	mailer.err = errors.New("mail failed")
	if _, err := NewAdminServiceWithCouponMailer(sessions, repository, mailer).MutateAdmin(context.Background(), AdminMutationCommand{Mode: "Queue", Action: domaingame.AdminActionQueueCron}); err == nil || !strings.Contains(err.Error(), "mail failed") {
		t.Fatalf("expected mail error, got %v", err)
	}
}

func TestAdminServiceSendsAndClearsReactivationMail(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
	issue.OutboundReactivationMails = []domaingame.AdminReactivationMail{{Character: "Legor", Recipient: "legor@example.local", Password: "password"}}
	repository := &fakeAdminRepository{admin: domaingame.Admin{Mode: "Users", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin}}, actionIssue: issue}
	mailer := &fakeAdminReactivationMailer{}
	result, err := NewAdminServiceWithMailers(sessions, repository, nil, mailer).MutateAdmin(context.Background(), AdminMutationCommand{Mode: "Users", Action: domaingame.AdminActionUsersReactivate})
	if err != nil || len(mailer.messages) != 1 || mailer.messages[0].Password != "password" || len(result.ActionIssue.OutboundReactivationMails) != 0 {
		t.Fatalf("result=%+v messages=%+v err=%v", result, mailer.messages, err)
	}
	issue.OutboundReactivationMails = []domaingame.AdminReactivationMail{{Recipient: "legor@example.local"}}
	mailer.err = errors.New("reactivation mail failed")
	if _, err := NewAdminServiceWithMailers(sessions, repository, nil, mailer).MutateAdmin(context.Background(), AdminMutationCommand{Mode: "Users", Action: domaingame.AdminActionUsersReactivate}); err == nil || !strings.Contains(err.Error(), "reactivation mail failed") {
		t.Fatalf("expected mail error, got %v", err)
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

func TestAdminServiceMutationReturnsAccessDeniedForOperatorAction(t *testing.T) {
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		&fakeAdminRepository{admin: domaingame.Admin{Mode: "Queue", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelOperator}}},
	)

	result, err := service.MutateAdmin(context.Background(), AdminMutationCommand{Mode: "Queue", Action: domaingame.AdminActionQueueFreeze, TaskID: 1001})

	repository := service.repository.(*fakeAdminRepository)
	if err != nil || !result.Authenticated || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.AdminIssueAccessDenied || repository.mutated {
		t.Fatalf("expected operator action access denied without mutation, result=%+v mutated=%v err=%v", result, repository.mutated, err)
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

func TestAdminServiceMutatesBotEdit(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeAdminRepository{
		admin: domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelAdmin}},
		botResult: AdminBotEditMutationResult{
			Source:             "source",
			Name:               "strategy",
			SelectedStrategyID: 7,
			Strategies:         []domaingame.AdminBotStrategy{{ID: 7, Name: "strategy"}},
		},
	}
	service := NewAdminService(sessions, repository)

	result, err := service.MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{
		PublicSession:   "pub",
		PrivateSessions: map[string]string{"prsess_42_1": "priv"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Action:          domaingame.AdminActionBotEditRename,
		StrategyID:      7,
		Name:            "strategy",
		Source:          "source",
	})

	if err != nil {
		t.Fatalf("MutateAdminBotEdit returned error: %v", err)
	}
	if !result.Authenticated || result.Source != "source" || result.SelectedStrategyID != 7 ||
		repository.botMutation.PlayerID != 42 || repository.botMutation.Action != domaingame.AdminActionBotEditRename ||
		repository.botMutation.StrategyID != 7 || repository.botMutation.Name != "strategy" ||
		repository.botMutation.Source != "source" || repository.query.Mode != "BotEdit" ||
		sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected result=%+v mutation=%+v query=%+v session=%+v", result, repository.botMutation, repository.query, sessions.command)
	}
}

func TestAdminServiceBotEditReturnsAccessDeniedWithoutMutating(t *testing.T) {
	service := NewAdminService(
		&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}},
		&fakeAdminRepository{admin: domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{PlayerID: 42, Level: domaingame.AdminLevelOperator}}},
	)

	result, err := service.MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{Action: domaingame.AdminActionBotEditLoad})

	repository := service.repository.(*fakeAdminRepository)
	if err != nil || !result.Authenticated || result.ActionIssue == nil ||
		result.ActionIssue.Code != domaingame.AdminIssueAccessDenied || repository.botMutated {
		t.Fatalf("expected access denied without mutation, result=%+v mutated=%v err=%v", result, repository.botMutated, err)
	}
}

func TestAdminServiceBotEditErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewAdminService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeAdminRepository{})
	result, err := service.MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated botedit result, got result=%+v err=%v", result, err)
	}
	if _, err := (AdminService{}).MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected botedit dependency error, got %v", err)
	}
	if _, err := NewAdminService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeAdminRepository{}).MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected botedit session error, got %v", err)
	}
	authenticated := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{err: errors.New("admin failed")}).MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{}); err == nil || !strings.Contains(err.Error(), "admin failed") {
		t.Fatalf("expected botedit admin load error, got %v", err)
	}
	readOnly := &fakeAdminReadOnlyRepository{admin: domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{Level: domaingame.AdminLevelAdmin}}}
	if _, err := NewAdminService(authenticated, readOnly).MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{Action: domaingame.AdminActionBotEditLoad}); err == nil || !strings.Contains(err.Error(), "botedit mutation unavailable") {
		t.Fatalf("expected missing botedit repository error, got %v", err)
	}
	if _, err := NewAdminService(authenticated, &fakeAdminRepository{
		admin:     domaingame.Admin{Mode: "BotEdit", Viewer: domaingame.AdminViewer{Level: domaingame.AdminLevelAdmin}},
		botErr:    errors.New("botedit failed"),
		botResult: AdminBotEditMutationResult{SelectedStrategyID: 7},
	}).MutateAdminBotEdit(context.Background(), AdminBotEditMutationCommand{Action: domaingame.AdminActionBotEditLoad}); err == nil || !strings.Contains(err.Error(), "botedit failed") {
		t.Fatalf("expected botedit repository error, got %v", err)
	}
}

func TestAdminServiceDirectPlayerIdentityDependencyErrors(t *testing.T) {
	service := AdminService{}
	if _, err := service.GetAdminForPlayer(context.Background(), AdminQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected direct admin dependency error, got %v", err)
	}
	if _, err := service.MutateAdminForPlayer(context.Background(), 42, AdminMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected direct mutation dependency error, got %v", err)
	}
	if _, err := service.MutateAdminBotEditForPlayer(context.Background(), 42, AdminBotEditMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected direct botedit dependency error, got %v", err)
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
	botResult   AdminBotEditMutationResult
	botErr      error
	botMutation AdminBotEditMutationQuery
	botMutated  bool
}

type fakeAdminCouponMailer struct {
	messages []domaingame.AdminCouponMail
	err      error
}

type fakeAdminReactivationMailer struct {
	messages []domaingame.AdminReactivationMail
	err      error
}

func (f *fakeAdminReactivationMailer) SendAdminReactivation(_ context.Context, message domaingame.AdminReactivationMail) error {
	f.messages = append(f.messages, message)
	return f.err
}

func (f *fakeAdminCouponMailer) SendAdminCoupon(_ context.Context, message domaingame.AdminCouponMail) error {
	f.messages = append(f.messages, message)
	return f.err
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

func (f *fakeAdminRepository) MutateAdminBotEdit(_ context.Context, query AdminBotEditMutationQuery) (AdminBotEditMutationResult, error) {
	f.botMutation = query
	f.botMutated = true
	return f.botResult, f.botErr
}

type fakeAdminReadOnlyRepository struct {
	admin    domaingame.Admin
	query    AdminQuery
	mutation AdminMutationQuery
}

func (f *fakeAdminReadOnlyRepository) GetAdmin(_ context.Context, query AdminQuery) (domaingame.Admin, error) {
	f.query = query
	return f.admin, nil
}

func (f *fakeAdminReadOnlyRepository) MutateAdmin(_ context.Context, query AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	f.mutation = query
	return nil, nil
}

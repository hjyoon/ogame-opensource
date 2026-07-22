package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type fakePlayerActions struct {
	buildingQuery appgame.BuildingsMutationQuery
	researchQuery appgame.ResearchMutationQuery
	renameQuery   appgame.OverviewRenameQuery
	deleteQuery   appgame.OverviewDeleteQuery
	templateQuery appgame.FleetTemplateMutationQuery
	allianceQuery appgame.AllianceMutationQuery
	optionsQuery  appgame.OptionsUpdateQuery
	empireQuery   appgame.EmpireMutationQuery
	missileQuery  appgame.GalaxyMissileLaunchQuery
	dispatchQuery appgame.GalaxyInstantDispatchQuery
	paymentQuery  appgame.PaymentMutationQuery
	buildingIssue *domaingame.BuildingsActionIssue
	researchIssue *domaingame.BuildingsActionIssue
	fleet         domaingame.Fleet
	options       domaingame.Options
	empire        domaingame.Empire
	couponFound   bool
	err           error
	errs          map[string]error
}

func (f *fakePlayerActions) methodErr(name string) error {
	if f.errs != nil && f.errs[name] != nil {
		return f.errs[name]
	}
	return f.err
}

func (f *fakePlayerActions) MutateBuildings(_ context.Context, query appgame.BuildingsMutationQuery) (appgame.BuildingsMutationOutcome, error) {
	f.buildingQuery = query
	return appgame.BuildingsMutationOutcome{ActionIssue: f.buildingIssue}, f.methodErr("building")
}

func (f *fakePlayerActions) MutateResearch(_ context.Context, query appgame.ResearchMutationQuery) (appgame.ResearchMutationOutcome, error) {
	f.researchQuery = query
	return appgame.ResearchMutationOutcome{ActionIssue: f.researchIssue}, f.methodErr("research")
}

func (f *fakePlayerActions) RenamePlanet(_ context.Context, query appgame.OverviewRenameQuery) (domaingame.Overview, error) {
	f.renameQuery = query
	return domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{ID: query.PlanetID, Name: query.Name}}, f.methodErr("rename")
}

func (f *fakePlayerActions) DeletePlanet(_ context.Context, query appgame.OverviewDeleteQuery) (domaingame.Overview, *domaingame.OverviewActionIssue, error) {
	f.deleteQuery = query
	return domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{ID: 11}}, &domaingame.OverviewActionIssue{Code: "deleted", Message: "deleted"}, f.methodErr("delete")
}

func (f *fakePlayerActions) GetFleet(context.Context, appgame.FleetQuery) (domaingame.Fleet, error) {
	return f.fleet, f.methodErr("get_fleet")
}

func (f *fakePlayerActions) MutateFleetTemplate(_ context.Context, query appgame.FleetTemplateMutationQuery) error {
	f.templateQuery = query
	return f.methodErr("template")
}

func (f *fakePlayerActions) MutateAlliance(_ context.Context, query appgame.AllianceMutationQuery) (domaingame.Alliance, *domaingame.AllianceActionIssue, error) {
	f.allianceQuery = query
	return domaingame.Alliance{View: domaingame.AllianceViewHome}, domaingame.AllianceIssue(domaingame.AllianceIssueSaved), f.methodErr("alliance")
}

func (f *fakePlayerActions) GetOptions(context.Context, appgame.OptionsQuery) (domaingame.Options, error) {
	return f.options, f.methodErr("get_options")
}

func (f *fakePlayerActions) UpdateOptions(_ context.Context, query appgame.OptionsUpdateQuery) (domaingame.Options, *domaingame.OptionsActionIssue, error) {
	f.optionsQuery = query
	return f.options, domaingame.OptionsSavedIssue(), f.methodErr("update_options")
}

func (f *fakePlayerActions) GetEmpire(context.Context, appgame.EmpireQuery) (domaingame.Empire, *domaingame.EmpireActionIssue, error) {
	if !f.empire.CommanderActive {
		return f.empire, domaingame.EmpireActionIssueFor(domaingame.EmpireIssueCommanderRequired), f.methodErr("get_empire")
	}
	return f.empire, nil, f.methodErr("get_empire")
}

func (f *fakePlayerActions) MutateEmpire(_ context.Context, query appgame.EmpireMutationQuery) (appgame.EmpireMutationOutcome, error) {
	f.empireQuery = query
	return appgame.EmpireMutationOutcome{ActionIssue: &domaingame.EmpireActionIssue{Code: "queued", Message: "queued"}}, f.methodErr("empire")
}

func (f *fakePlayerActions) LaunchMissiles(_ context.Context, query appgame.GalaxyMissileLaunchQuery) (*domaingame.GalaxyActionIssue, error) {
	f.missileQuery = query
	return domaingame.GalaxyActionIssueFor(domaingame.GalaxyIssueRocketLaunched), f.methodErr("missile")
}

func (f *fakePlayerActions) DispatchInstantFleet(_ context.Context, query appgame.GalaxyInstantDispatchQuery) (*domaingame.GalaxyActionIssue, error) {
	f.dispatchQuery = query
	return domaingame.GalaxyFleetDispatchedIssue(), f.methodErr("dispatch")
}

func (f *fakePlayerActions) CheckCoupon(_ context.Context, query appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	f.paymentQuery = query
	return domaingame.PaymentCoupon{Amount: 2500}, f.couponFound, f.methodErr("check_coupon")
}

func (f *fakePlayerActions) ActivateCoupon(_ context.Context, query appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	f.paymentQuery = query
	return domaingame.PaymentCoupon{Amount: 2500}, f.couponFound, f.methodErr("activate_coupon")
}

type fakeOptionsMailer struct {
	mail domaingame.OptionsChangeMail
	err  error
}

func (f *fakeOptionsMailer) SendOptionsChange(_ context.Context, mail domaingame.OptionsChangeMail) error {
	f.mail = mail
	return f.err
}

func playerActionsFixture() (*fakePlayerActions, *fakeOptionsMailer, Service) {
	repository := &fakePlayerActions{
		fleet:       domaingame.Fleet{CommanderActive: true, TemplateLimit: 5, Templates: []domaingame.FleetTemplate{{ID: 1}}},
		empire:      domaingame.Empire{CommanderActive: true},
		couponFound: true,
		options: domaingame.Options{
			User:     domaingame.OptionsUser{Name: "player", Email: "p@example.test", PlainEmail: "p@example.test", Validated: true},
			Settings: domaingame.OptionsSettings{Language: "en", SkinPath: "/skin/", UseSkin: true, SortBy: 1, SortOrder: 1, MaxSpy: 5, MaxFleetMessages: 10},
			Flags:    domaingame.OptionsFlags{ShowEspionageButton: true, ShowWriteMessage: true, ShowBuddy: true, ShowRocketAttack: true, ShowViewReport: true, FeedAtom: true},
		},
	}
	mailer := &fakeOptionsMailer{}
	access := domainmcp.Access{Authenticated: true, PlayerID: 42, Scopes: []string{
		domainmcp.ScopeQueueWrite, domainmcp.ScopeFleetWrite, domainmcp.ScopePlanetWrite,
		domainmcp.ScopeAllianceWrite, domainmcp.ScopeAccountWrite, domainmcp.ScopePaymentWrite,
	}}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{"player": access}}).WithPlayerActions(PlayerActions{
		Buildings: repository, Research: repository, Overview: repository, FleetTemplates: repository,
		Alliance: repository, Options: repository, OptionsMailer: mailer, Empire: repository, Galaxy: repository, Payment: repository,
	}).WithBuildingOptionsReadRepository(&fakeBuildingOptionsReadRepository{result: domainmcp.BuildingOptions{
		Planet: domainmcp.Planet{ID: 11},
		Items:  []domainmcp.BuildingOption{{ID: domaingame.BuildingMetalMine, DurationSeconds: 30}},
	}}).WithResearchOptionsReadRepository(&fakeResearchOptionsReadRepository{result: domainmcp.ResearchOptions{
		Planet: domainmcp.Planet{ID: 11},
		Items:  []domainmcp.BuildingOption{{ID: domaingame.ResearchEspionage, DurationSeconds: 40}},
	}})
	service.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	return repository, mailer, service
}

func actionCall(t *testing.T, service Service, name string, arguments map[string]any) domainmcp.ToolCallResult {
	t.Helper()
	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: name, Arguments: arguments, AccessToken: "player"})
	if err != nil {
		t.Fatalf("%s returned error: %v", name, err)
	}
	return result
}

func actionResult(t *testing.T, result domainmcp.ToolCallResult, key string) playerActionResult {
	t.Helper()
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content has type %T", result.StructuredContent)
	}
	action, ok := structured[key].(playerActionResult)
	if !ok {
		t.Fatalf("%s has type %T", key, structured[key])
	}
	return action
}

func confirmedArguments(arguments map[string]any, confirmation string) map[string]any {
	result := make(map[string]any, len(arguments)+2)
	for key, value := range arguments {
		result[key] = value
	}
	result["dryRun"] = false
	result["confirm"] = confirmation
	return result
}

func TestPlayerActionToolsAreListedByUserScopes(t *testing.T) {
	_, _, service := playerActionsFixture()
	result, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "player"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
		if tool.InputSchema == nil || tool.OutputSchema == nil || tool.Annotations == nil {
			t.Fatalf("tool %s is missing schemas or annotations", tool.Name)
		}
	}
	for _, want := range []string{"mutate_building", "start_research", "mutate_commander_queue", "mutate_fleet_template", "launch_interplanetary_missiles", "dispatch_galaxy_action", "mutate_planet", "mutate_alliance", "update_account_options", "redeem_coupon"} {
		if !strings.Contains(","+strings.Join(names, ",")+",", ","+want+",") {
			t.Fatalf("missing tool %s from %v", want, names)
		}
	}
}

func TestSensitivePlayerActionSchemasDoNotAcceptCurrentPasswords(t *testing.T) {
	_, _, service := playerActionsFixture()
	result, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "player"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	for _, tool := range result.Tools {
		if tool.Name != "mutate_planet" && tool.Name != "update_account_options" {
			continue
		}
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s properties have type %T", tool.Name, tool.InputSchema["properties"])
		}
		for _, field := range []string{"password", "oldPassword"} {
			if _, exists := properties[field]; exists {
				t.Fatalf("%s must not accept current-password field %q", tool.Name, field)
			}
		}
	}
}

func TestPlayerActionDryRunAndConfirmedExecution(t *testing.T) {
	repository, _, service := playerActionsFixture()
	tests := []struct {
		name   string
		key    string
		args   map[string]any
		assert func(*testing.T)
	}{
		{"mutate_building", "buildingMutation", map[string]any{"planetId": 11, "action": "add", "techId": 1}, func(t *testing.T) {
			if repository.buildingQuery.TechID != 1 {
				t.Fatal("building not executed")
			}
		}},
		{"start_research", "researchMutation", map[string]any{"planetId": 11, "techId": 106}, func(t *testing.T) {
			if repository.researchQuery.TechID != 106 {
				t.Fatal("research not executed")
			}
		}},
		{"mutate_planet", "planetMutation", map[string]any{"planetId": 11, "action": "rename", "name": "Renamed"}, func(t *testing.T) {
			if repository.renameQuery.Name != "Renamed" {
				t.Fatal("rename not executed")
			}
		}},
		{"mutate_fleet_template", "fleetTemplateMutation", map[string]any{"planetId": 11, "action": "save", "templateId": 1, "name": "Raid", "ships": map[string]any{"202": float64(2)}}, func(t *testing.T) {
			if repository.templateQuery.Ships[202] != 2 {
				t.Fatal("template not executed")
			}
		}},
		{"mutate_commander_queue", "commanderQueueMutation", map[string]any{"planetId": 11, "targetPlanetId": 12, "action": "add", "techId": 1}, func(t *testing.T) {
			if repository.empireQuery.PlanetID != 12 {
				t.Fatal("Commander queue not executed")
			}
		}},
		{"launch_interplanetary_missiles", "missileLaunch", map[string]any{"planetId": 11, "targetPlanetId": 12, "amount": 3, "targetDefenseId": 401}, func(t *testing.T) {
			if repository.missileQuery.Amount != 3 {
				t.Fatal("missile not executed")
			}
		}},
		{"dispatch_galaxy_action", "galaxyDispatch", map[string]any{"planetId": 11, "action": "spy", "targetGalaxy": 1, "targetSystem": 2, "targetPosition": 3, "amount": 2}, func(t *testing.T) {
			if repository.dispatchQuery.Mission != domaingame.FleetMissionSpy {
				t.Fatal("spy not executed")
			}
		}},
		{"mutate_alliance", "allianceMutation", map[string]any{"planetId": 11, "action": "save_ranks", "rankRights": []any{map[string]any{"id": float64(2), "rights": float64(511)}}}, func(t *testing.T) {
			if len(repository.allianceQuery.Mutation.RankRights) != 1 {
				t.Fatal("alliance not executed")
			}
		}},
		{"update_account_options", "accountOptionsMutation", map[string]any{"planetId": 11, "action": "settings", "language": "de", "skinPath": "/new/", "useSkin": false, "deactivateIp": true, "sortBy": 2, "sortOrder": 0, "maxSpy": 8, "maxFleetMessages": 12, "showEspionageButton": false, "showWriteMessage": false, "showBuddy": false, "showRocketAttack": false, "showViewReport": false, "doNotUseFolders": true, "feedEnabled": true, "feedType": "rss", "hideGoEmail": true}, func(t *testing.T) {
			if repository.optionsQuery.Mutation.Language != "de" || repository.optionsQuery.Mutation.MaxSpy != 8 {
				t.Fatal("options not executed")
			}
		}},
		{"redeem_coupon", "couponRedemption", map[string]any{"couponCode": "DARK-2500"}, func(t *testing.T) {
			if repository.paymentQuery.CouponCode != "DARK-2500" {
				t.Fatal("coupon not executed")
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview := actionResult(t, actionCall(t, service, test.name, test.args), test.key)
			if !preview.DryRun || preview.Executed || !preview.RequiresConfirmation || preview.Confirmation == "" {
				t.Fatalf("unexpected preview: %+v", preview)
			}
			if test.name == "mutate_building" || test.name == "start_research" {
				if preview.Timing == nil || preview.Timing.DurationSeconds < 1 || preview.Timing.RemainingSeconds < 1 || preview.Timing.Status != "preview" {
					t.Fatalf("missing positive preview timing: %+v", preview)
				}
			}
			executed := actionResult(t, actionCall(t, service, test.name, confirmedArguments(test.args, preview.Confirmation)), test.key)
			if executed.DryRun || !executed.Executed || executed.RequiresConfirmation {
				t.Fatalf("unexpected execution: %+v", executed)
			}
			if test.name == "mutate_building" || test.name == "start_research" {
				if executed.Timing == nil || executed.Timing.DurationSeconds < 1 || executed.Timing.FinishesAt <= executed.Timing.StartsAt || executed.Timing.Status != "running" {
					t.Fatalf("missing positive execution timing: %+v", executed)
				}
			}
			test.assert(t)
		})
	}
}

func TestPlayerActionQueueTimingProjectsQueuedAndDueWork(t *testing.T) {
	now := int64(1_000)
	queue := []domainmcp.BuildingQueueEntry{
		{TechID: 1, Start: 990, End: 1_010, RemainingSeconds: 10, Status: "running"},
		{TechID: 2, Start: 1_000, End: 1_030, RemainingSeconds: 30, Status: "queued"},
	}
	timing := buildingQueueTiming(queue, 2, false, now)
	if timing == nil || timing.DurationSeconds != 30 || timing.StartsAt != 1_010 || timing.FinishesAt != 1_040 || timing.RemainingSeconds != 40 || timing.Status != "queued" {
		t.Fatalf("unexpected projected queue timing: %+v", timing)
	}
	if end := projectedBuildingQueueEnd(queue, now); end != 1_040 {
		t.Fatalf("unexpected projected queue end: %d", end)
	}

	due := buildingQueueTiming([]domainmcp.BuildingQueueEntry{{TechID: 1, Start: 900, End: 950, Status: "due"}}, 1, false, now)
	if due == nil || due.RemainingSeconds != 0 || due.Status != "due" {
		t.Fatalf("unexpected due timing: %+v", due)
	}
	if timing := newPlayerActionTiming(0, now, now, now, "running"); timing != nil {
		t.Fatalf("zero duration must not become a completion estimate: %+v", timing)
	}
	if timing := newPlayerActionTiming(1, now, now-1, now, "running"); timing != nil {
		t.Fatalf("finish before start must not become a completion estimate: %+v", timing)
	}
	finished := newPlayerActionTiming(1, now-2, now-1, now, "due")
	if finished == nil || finished.RemainingSeconds != 0 {
		t.Fatalf("past completion must clamp remaining time: %+v", finished)
	}
	if got := buildingQueueTiming([]domainmcp.BuildingQueueEntry{{TechID: 3, Start: 1_000, End: 1_000}}, 3, false, now); got == nil || got.DurationSeconds != 1 || got.Status != "running" {
		t.Fatalf("invalid queue duration must be normalized: %+v", got)
	}
	if end := projectedBuildingQueueEnd([]domainmcp.BuildingQueueEntry{{Start: 900, End: 900}}, now); end != now+1 {
		t.Fatalf("invalid projected duration must be normalized: %d", end)
	}
	if queueTimingStatus(now+1, now) != "queued" || queueTimingStatus(now, now) != "running" {
		t.Fatal("queue timing status did not distinguish queued and running work")
	}
	if mcpActionQueueStatus(0, false) != "due" || mcpActionQueueStatus(1, true) != "queued" || mcpActionQueueStatus(1, false) != "running" {
		t.Fatal("MCP action status did not cover due, queued, and running work")
	}
}

func TestPlayerActionTimingLookupBranches(t *testing.T) {
	const now = int64(1_000)
	clock := func() time.Time { return time.Unix(now, 0) }
	if timing := (Service{}).buildingMutationTiming(context.Background(), 42, 0, 1, "add", false); timing != nil {
		t.Fatalf("missing building reader returned timing: %+v", timing)
	}
	if timing := (Service{buildingRead: &fakeBuildingOptionsReadRepository{err: errors.New("down")}}).buildingMutationTiming(context.Background(), 42, 0, 1, "add", false); timing != nil {
		t.Fatalf("failed building reader returned timing: %+v", timing)
	}
	if timing := (Service{buildingRead: &fakeBuildingOptionsReadRepository{}}).buildingMutationTiming(context.Background(), 42, 0, 1, "add", false); timing != nil {
		t.Fatalf("missing building option returned timing: %+v", timing)
	}

	buildingRead := &fakeBuildingOptionsReadRepository{result: domainmcp.BuildingOptions{
		Planet: domainmcp.Planet{ID: 11},
		Queue:  []domainmcp.BuildingQueueEntry{{TechID: 1, Start: 990, End: 1_010, Status: "running"}},
		Items:  []domainmcp.BuildingOption{{ID: 2, DurationSeconds: 30}},
	}}
	service := Service{buildingRead: buildingRead, now: clock}
	if timing := service.buildingMutationTiming(context.Background(), 42, 11, 1, "add", true); timing == nil || timing.StartsAt != 990 || timing.FinishesAt != 1_010 || timing.Status != "running" {
		t.Fatalf("active building queue timing mismatch: %+v", timing)
	}
	if timing := service.buildingMutationTiming(context.Background(), 42, 11, 2, "add", true); timing == nil || timing.StartsAt != 1_010 || timing.Status != "queued" {
		t.Fatalf("queued building timing mismatch: %+v", timing)
	}

	service.technologyRead = &fakeTechnologyReadRepository{result: domainmcp.TechnologyTree{Details: &domainmcp.TechnologyDetails{Demolish: &domainmcp.TechnologyDemolish{DurationSeconds: 25}}}}
	if timing := service.buildingMutationTiming(context.Background(), 42, 11, 2, "destroy", false); timing == nil || timing.DurationSeconds != 25 || timing.Status != "preview" {
		t.Fatalf("demolition preview timing mismatch: %+v", timing)
	}
	service.technologyRead = &fakeTechnologyReadRepository{err: errors.New("down")}
	if timing := service.buildingMutationTiming(context.Background(), 42, 11, 2, "destroy", false); timing == nil || timing.DurationSeconds != 30 {
		t.Fatalf("demolition fallback timing mismatch: %+v", timing)
	}

	if timing := (Service{}).researchMutationTiming(context.Background(), 42, 0, 106, false); timing != nil {
		t.Fatalf("missing research reader returned timing: %+v", timing)
	}
	if timing := (Service{researchRead: &fakeResearchOptionsReadRepository{err: errors.New("down")}}).researchMutationTiming(context.Background(), 42, 0, 106, false); timing != nil {
		t.Fatalf("failed research reader returned timing: %+v", timing)
	}
	researchRead := &fakeResearchOptionsReadRepository{result: domainmcp.ResearchOptions{Active: &domainmcp.ResearchQueueEntry{TechID: 106, Start: 900, End: 950, RemainingSeconds: 0}}}
	service = Service{researchRead: researchRead, now: clock}
	if timing := service.researchMutationTiming(context.Background(), 42, 0, 106, true); timing == nil || timing.Status != "due" || timing.RemainingSeconds != 0 {
		t.Fatalf("due research timing mismatch: %+v", timing)
	}
	if timing := service.researchMutationTiming(context.Background(), 42, 0, 108, true); timing != nil {
		t.Fatalf("unrelated active research returned timing: %+v", timing)
	}
	researchRead.result = domainmcp.ResearchOptions{Items: []domainmcp.BuildingOption{{ID: 108, DurationSeconds: 40}}}
	if timing := service.researchMutationTiming(context.Background(), 42, 0, 108, true); timing == nil || timing.Status != "running" || timing.DurationSeconds != 40 {
		t.Fatalf("new research timing mismatch: %+v", timing)
	}
	if timing := service.researchMutationTiming(context.Background(), 42, 0, 999, false); timing != nil {
		t.Fatalf("missing research option returned timing: %+v", timing)
	}
	if (Service{}).currentTime().IsZero() {
		t.Fatal("default MCP action clock returned zero")
	}
}

func TestPlayerActionMutationIssuesDoNotInventCompletionTiming(t *testing.T) {
	repository, _, service := playerActionsFixture()
	buildingArgs := map[string]any{"planetId": 11, "action": "add", "techId": 1}
	buildingPreview := actionResult(t, actionCall(t, service, "mutate_building", buildingArgs), "buildingMutation")
	repository.buildingIssue = domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy)
	building := actionResult(t, actionCall(t, service, "mutate_building", confirmedArguments(buildingArgs, buildingPreview.Confirmation)), "buildingMutation")
	if building.Issue == nil || building.Timing != nil {
		t.Fatalf("rejected building mutation returned invalid timing: %+v", building)
	}

	researchArgs := map[string]any{"planetId": 11, "techId": 106}
	researchPreview := actionResult(t, actionCall(t, service, "start_research", researchArgs), "researchMutation")
	repository.researchIssue = domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy)
	research := actionResult(t, actionCall(t, service, "start_research", confirmedArguments(researchArgs, researchPreview.Confirmation)), "researchMutation")
	if research.Issue == nil || research.Timing != nil {
		t.Fatalf("rejected research mutation returned invalid timing: %+v", research)
	}
}

func TestPlayerActionAdditionalBranches(t *testing.T) {
	repository, mailer, service := playerActionsFixture()

	deleteArgs := map[string]any{"planetId": 11, "targetPlanetId": 12, "action": "delete"}
	preview := actionResult(t, actionCall(t, service, "mutate_planet", deleteArgs), "planetMutation")
	actionCall(t, service, "mutate_planet", confirmedArguments(deleteArgs, preview.Confirmation))
	if repository.deleteQuery.DeleteID != 12 || !repository.deleteQuery.SkipPasswordVerification || repository.deleteQuery.Password != "" {
		t.Fatalf("unexpected delete query: %+v", repository.deleteQuery)
	}

	for _, args := range []map[string]any{
		{"action": "rename", "name": "NewName"},
		{"action": "password", "newPassword": "password1", "newPasswordRepeat": "password1"},
		{"action": "email", "email": "new@example.test"},
		{"action": "vacation", "enabled": true},
		{"action": "deletion", "enabled": true},
		{"action": "resend_activation"},
	} {
		result := actionResult(t, actionCall(t, service, "update_account_options", args), "accountOptionsMutation")
		if !result.RequiresConfirmation {
			t.Fatalf("action %v did not require confirmation", args)
		}
	}

	passwordArgs := map[string]any{"action": "password", "newPassword": "password1", "newPasswordRepeat": "password1"}
	preview = actionResult(t, actionCall(t, service, "update_account_options", passwordArgs), "accountOptionsMutation")
	actionCall(t, service, "update_account_options", confirmedArguments(passwordArgs, preview.Confirmation))
	if !repository.optionsQuery.SkipPasswordVerification || repository.optionsQuery.Mutation.OldPassword != "" ||
		repository.optionsQuery.Mutation.NewPassword != "password1" {
		t.Fatalf("unexpected passwordless password mutation: %+v", repository.optionsQuery)
	}

	repository.options.OutboundMail = &domaingame.OptionsChangeMail{Recipient: "new@example.test"}
	emailArgs := map[string]any{"action": "email", "email": "new@example.test"}
	preview = actionResult(t, actionCall(t, service, "update_account_options", emailArgs), "accountOptionsMutation")
	actionCall(t, service, "update_account_options", confirmedArguments(emailArgs, preview.Confirmation))
	if mailer.mail.Recipient != "new@example.test" || !repository.optionsQuery.SkipPasswordVerification || repository.optionsQuery.Mutation.OldPassword != "" {
		t.Fatalf("unexpected passwordless email mutation: query=%+v mail=%+v", repository.optionsQuery, mailer.mail)
	}

	recycle := map[string]any{"action": "recycle", "targetGalaxy": 1, "targetSystem": 2, "targetPosition": 3, "targetType": 2, "amount": 1}
	preview = actionResult(t, actionCall(t, service, "dispatch_galaxy_action", recycle), "galaxyDispatch")
	actionCall(t, service, "dispatch_galaxy_action", confirmedArguments(recycle, preview.Confirmation))
	if repository.dispatchQuery.Mission != domaingame.FleetMissionRecycle || repository.dispatchQuery.TargetType != 2 {
		t.Fatalf("unexpected recycle query: %+v", repository.dispatchQuery)
	}

	remove := map[string]any{"targetPlanetId": 12, "action": "remove", "listId": 7}
	preview = actionResult(t, actionCall(t, service, "mutate_commander_queue", remove), "commanderQueueMutation")
	actionCall(t, service, "mutate_commander_queue", confirmedArguments(remove, preview.Confirmation))
	if repository.empireQuery.ListID != 7 {
		t.Fatalf("unexpected remove query: %+v", repository.empireQuery)
	}
}

func TestPlayerActionRestrictionsAndInvalidConfirmation(t *testing.T) {
	repository, _, service := playerActionsFixture()
	repository.fleet.CommanderActive = false
	result := actionResult(t, actionCall(t, service, "mutate_fleet_template", map[string]any{"action": "delete", "templateId": 1}), "fleetTemplateMutation")
	if result.Issue == nil || result.Issue.Code != domaingame.EmpireIssueCommanderRequired || result.RequiresConfirmation {
		t.Fatalf("unexpected Commander template restriction: %+v", result)
	}
	repository.empire.CommanderActive = false
	result = actionResult(t, actionCall(t, service, "mutate_commander_queue", map[string]any{"targetPlanetId": 12, "action": "add", "techId": 1}), "commanderQueueMutation")
	if result.Issue == nil || result.Issue.Code != domaingame.EmpireIssueCommanderRequired {
		t.Fatalf("unexpected Commander queue restriction: %+v", result)
	}

	_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_building", AccessToken: "player", Arguments: map[string]any{"action": "add", "techId": 1, "dryRun": false, "confirm": "wrong"}})
	if !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("wrong confirmation error = %v", err)
	}

	repository.couponFound = false
	result = actionResult(t, actionCall(t, service, "redeem_coupon", map[string]any{"couponCode": "missing"}), "couponRedemption")
	if result.Issue == nil || result.Issue.Code != domaingame.PaymentIssueInvalidCoupon || result.RequiresConfirmation {
		t.Fatalf("unexpected invalid coupon preview: %+v", result)
	}
}

func TestPlayerActionRepositoryFailuresAreReturned(t *testing.T) {
	repository, mailer, service := playerActionsFixture()
	boom := errors.New("action repository unavailable")
	tests := []struct {
		name   string
		key    string
		errKey string
		args   map[string]any
	}{
		{"mutate_building", "buildingMutation", "building", map[string]any{"action": "add", "techId": 1}},
		{"start_research", "researchMutation", "research", map[string]any{"techId": 106}},
		{"mutate_planet", "planetMutation", "rename", map[string]any{"planetId": 11, "action": "rename", "name": "Renamed"}},
		{"mutate_planet", "planetMutation", "delete", map[string]any{"planetId": 11, "action": "delete"}},
		{"mutate_fleet_template", "fleetTemplateMutation", "template", map[string]any{"action": "save", "name": "Raid", "ships": map[string]any{"202": 1}}},
		{"mutate_commander_queue", "commanderQueueMutation", "empire", map[string]any{"targetPlanetId": 11, "action": "add", "techId": 1}},
		{"launch_interplanetary_missiles", "missileLaunch", "missile", map[string]any{"targetPlanetId": 12, "amount": 1}},
		{"dispatch_galaxy_action", "galaxyDispatch", "dispatch", map[string]any{"action": "spy", "targetGalaxy": 1, "targetSystem": 2, "targetPosition": 3, "amount": 1}},
		{"mutate_alliance", "allianceMutation", "alliance", map[string]any{"action": "leave"}},
		{"update_account_options", "accountOptionsMutation", "update_options", map[string]any{"action": "settings"}},
		{"redeem_coupon", "couponRedemption", "activate_coupon", map[string]any{"couponCode": "VALID"}},
	}
	for _, test := range tests {
		t.Run(test.name+"_"+test.errKey, func(t *testing.T) {
			repository.errs = nil
			preview := actionResult(t, actionCall(t, service, test.name, test.args), test.key)
			repository.errs = map[string]error{test.errKey: boom}
			_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: test.name, AccessToken: "player", Arguments: confirmedArguments(test.args, preview.Confirmation)})
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	repository.errs = map[string]error{"get_fleet": boom}
	_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_fleet_template", AccessToken: "player", Arguments: map[string]any{"action": "delete", "templateId": 1}})
	if !errors.Is(err, boom) {
		t.Fatalf("fleet read error = %v", err)
	}
	repository.errs = map[string]error{"get_empire": boom}
	_, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_commander_queue", AccessToken: "player", Arguments: map[string]any{"targetPlanetId": 11, "action": "add", "techId": 1}})
	if !errors.Is(err, boom) {
		t.Fatalf("empire read error = %v", err)
	}
	repository.errs = map[string]error{"get_options": boom}
	_, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_account_options", AccessToken: "player", Arguments: map[string]any{"action": "settings"}})
	if !errors.Is(err, boom) {
		t.Fatalf("options read error = %v", err)
	}
	repository.errs = map[string]error{"check_coupon": boom}
	_, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "redeem_coupon", AccessToken: "player", Arguments: map[string]any{"couponCode": "VALID"}})
	if !errors.Is(err, boom) {
		t.Fatalf("coupon check error = %v", err)
	}

	repository.errs = nil
	repository.options.OutboundMail = &domaingame.OptionsChangeMail{Recipient: "p@example.test"}
	mailer.err = boom
	args := map[string]any{"action": "settings"}
	preview := actionResult(t, actionCall(t, service, "update_account_options", args), "accountOptionsMutation")
	_, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_account_options", AccessToken: "player", Arguments: confirmedArguments(args, preview.Confirmation)})
	if !errors.Is(err, boom) {
		t.Fatalf("mailer error = %v", err)
	}
}

func TestCouponConfirmationAndActivationRace(t *testing.T) {
	repository, _, service := playerActionsFixture()
	args := map[string]any{"couponCode": "VALID"}
	preview := actionResult(t, actionCall(t, service, "redeem_coupon", args), "couponRedemption")
	_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "redeem_coupon", AccessToken: "player", Arguments: map[string]any{"couponCode": "VALID", "dryRun": false, "confirm": "wrong"}})
	if !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("wrong coupon confirmation error = %v", err)
	}
	repository.couponFound = false
	result := actionResult(t, actionCall(t, service, "redeem_coupon", confirmedArguments(args, preview.Confirmation)), "couponRedemption")
	if result.Executed || result.Issue == nil || result.Issue.Code != domaingame.PaymentIssueInvalidCoupon {
		t.Fatalf("unexpected activation race result: %+v", result)
	}
	if ranks, err := allianceRankRightsArgument(nil); err != nil || ranks != nil {
		t.Fatalf("nil rank rights = %v, %v", ranks, err)
	}
}

func TestPlayerActionValidation(t *testing.T) {
	_, _, service := playerActionsFixture()
	tests := []struct {
		name string
		args map[string]any
	}{
		{"mutate_building", map[string]any{"action": "invalid", "techId": 1}},
		{"mutate_building", map[string]any{"action": "add", "techId": 0}},
		{"mutate_building", map[string]any{"action": "add", "techId": 1, "planetId": "bad"}},
		{"mutate_building", map[string]any{"action": "add", "techId": 1, "dryRun": "bad"}},
		{"start_research", map[string]any{"techId": "bad"}},
		{"start_research", map[string]any{"techId": 1, "planetId": "bad"}},
		{"mutate_planet", map[string]any{"planetId": 0, "action": "rename", "name": "x"}},
		{"mutate_planet", map[string]any{"planetId": 1, "action": "invalid"}},
		{"mutate_planet", map[string]any{"planetId": 1, "action": "rename", "name": ""}},
		{"mutate_planet", map[string]any{"planetId": 1, "targetPlanetId": "bad", "action": "delete"}},
		{"mutate_planet", map[string]any{"planetId": 1, "action": "delete", "password": "must-not-be-sent"}},
		{"mutate_fleet_template", map[string]any{"action": "save", "name": "x"}},
		{"mutate_fleet_template", map[string]any{"action": "delete", "templateId": 0}},
		{"mutate_fleet_template", map[string]any{"action": "invalid"}},
		{"mutate_fleet_template", map[string]any{"action": "delete", "templateId": 1, "planetId": "bad"}},
		{"mutate_fleet_template", map[string]any{"action": "delete", "templateId": "bad"}},
		{"mutate_fleet_template", map[string]any{"action": "save", "name": "x", "ships": "bad"}},
		{"mutate_commander_queue", map[string]any{"targetPlanetId": 1, "action": "remove", "listId": 0}},
		{"mutate_commander_queue", map[string]any{"targetPlanetId": 1, "action": "invalid"}},
		{"mutate_commander_queue", map[string]any{"targetPlanetId": 1, "action": "add", "techId": 1, "planetId": "bad"}},
		{"mutate_commander_queue", map[string]any{"targetPlanetId": "bad", "action": "add", "techId": 1}},
		{"mutate_commander_queue", map[string]any{"targetPlanetId": 1, "action": "add", "techId": "bad"}},
		{"launch_interplanetary_missiles", map[string]any{"targetPlanetId": 1, "amount": 0}},
		{"launch_interplanetary_missiles", map[string]any{"planetId": "bad", "targetPlanetId": 1, "amount": 1}},
		{"launch_interplanetary_missiles", map[string]any{"targetPlanetId": "bad", "amount": 1}},
		{"launch_interplanetary_missiles", map[string]any{"targetPlanetId": 1, "amount": 1, "targetDefenseId": "bad"}},
		{"dispatch_galaxy_action", map[string]any{"action": "invalid", "targetGalaxy": 1, "targetSystem": 1, "targetPosition": 1, "amount": 1}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "planetId": "bad", "targetGalaxy": 1, "targetSystem": 1, "targetPosition": 1, "amount": 1}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "targetGalaxy": 0, "targetSystem": 1, "targetPosition": 1, "amount": 1}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "targetGalaxy": 1, "targetSystem": 0, "targetPosition": 1, "amount": 1}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "targetGalaxy": 1, "targetSystem": 1, "targetPosition": 0, "amount": 1}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "targetGalaxy": 1, "targetSystem": 1, "targetPosition": 1, "amount": 0}},
		{"dispatch_galaxy_action", map[string]any{"action": "spy", "targetGalaxy": 1, "targetSystem": 1, "targetPosition": 1, "amount": 1, "targetType": "bad"}},
		{"mutate_alliance", map[string]any{"action": "invalid"}},
		{"mutate_alliance", map[string]any{"action": 7}},
		{"mutate_alliance", map[string]any{"action": "leave", "planetId": "bad"}},
		{"mutate_alliance", map[string]any{"action": "create", "tag": 7}},
		{"mutate_alliance", map[string]any{"action": "apply", "allianceId": "bad"}},
		{"mutate_alliance", map[string]any{"action": "save_settings", "open": "bad"}},
		{"mutate_alliance", map[string]any{"action": "save_settings", "insertApp": "bad"}},
		{"mutate_alliance", map[string]any{"action": "save_ranks", "rankRights": "bad"}},
		{"mutate_alliance", map[string]any{"action": "save_ranks", "rankRights": []any{"bad"}}},
		{"mutate_alliance", map[string]any{"action": "save_ranks", "rankRights": []any{map[string]any{"id": 0, "rights": 1}}}},
		{"mutate_alliance", map[string]any{"action": "save_ranks", "rankRights": []any{map[string]any{"id": 1, "rights": "bad"}}}},
		{"update_account_options", map[string]any{"action": "vacation"}},
		{"update_account_options", map[string]any{"action": "deletion"}},
		{"update_account_options", map[string]any{"action": "password", "newPassword": "", "newPasswordRepeat": "x"}},
		{"update_account_options", map[string]any{"action": "email", "email": ""}},
		{"update_account_options", map[string]any{"action": "email", "email": "x@example.test", "oldPassword": "must-not-be-sent"}},
		{"update_account_options", map[string]any{"action": "settings", "useSkin": "bad"}},
		{"update_account_options", map[string]any{"action": "invalid"}},
		{"redeem_coupon", map[string]any{"couponCode": ""}},
		{"redeem_coupon", map[string]any{"couponCode": 7}},
		{"redeem_coupon", map[string]any{"couponCode": "x", "dryRun": "bad"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: test.name, AccessToken: "player", Arguments: test.args})
			if !errors.Is(err, domainmcp.ErrInvalidParams) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if playerActionIssue("", "ignored") != nil {
		t.Fatal("empty issue code should not create an issue")
	}
}

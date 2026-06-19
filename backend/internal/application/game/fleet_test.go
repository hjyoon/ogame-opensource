package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestFleetServiceReturnsAuthenticatedFleet(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{Commander: "legor"}}
	service := NewFleetService(sessions, repository)

	result, err := service.GetFleet(context.Background(), FleetCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Fleet.Commander != "legor" {
		t.Fatalf("unexpected fleet result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestFleetServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.GetFleet(context.Background(), FleetCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestFleetServiceMutatesFleetTemplateAndReloadsFleet(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{
		CommanderActive: true,
		TemplateLimit:   4,
		Templates: []domaingame.FleetTemplate{{
			ID:   7,
			Name: "raid wing",
		}},
	}}
	service := NewFleetService(sessions, repository)

	result, err := service.MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		TemplateID:      7,
		Action:          "save",
		Name:            "raid wing",
		Ships:           map[int]int{domaingame.FleetSmallCargo: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || len(result.Fleet.Templates) != 1 {
		t.Fatalf("unexpected mutation result: %+v", result)
	}
	if repository.mutation.PlayerID != 42 || repository.mutation.TemplateID != 7 || repository.mutation.Ships[domaingame.FleetSmallCargo] != 3 {
		t.Fatalf("unexpected repository mutation: %+v", repository.mutation)
	}
	if repository.query.PlanetID != 99 {
		t.Fatalf("expected mutation to reload selected fleet screen, got %+v", repository.query)
	}
}

func TestFleetServiceMutationReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called || repository.mutated {
		t.Fatalf("expected unauthenticated mutation without repository call, got %+v", result)
	}
}

func TestFleetServicePreparesDispatchDraft(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{
		CurrentPlanet: domaingame.PlanetOverview{Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}},
		Ships: []domaingame.FleetShipSelection{{
			ID:         domaingame.FleetSmallCargo,
			Name:       "Small Cargo",
			Count:      4,
			Cargo:      5000,
			Selectable: true,
		}},
	}}
	service := NewFleetService(sessions, repository)

	result, err := service.PrepareFleetDispatch(context.Background(), FleetDispatchPrepareCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Ships:           map[int]int{domaingame.FleetSmallCargo: 3},
		Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
		TargetType:      domaingame.GamePlanetTypeMoon,
		Mission:         domaingame.FleetMissionTransport,
		Speed:           9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Fleet.DispatchDraft == nil || result.Fleet.DispatchDraft.TotalShips != 3 {
		t.Fatalf("unexpected prepare result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("expected prepare to load selected fleet screen, got %+v", repository.query)
	}
}

func TestFleetServiceValidatesDispatchDraft(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{
		CurrentPlanet: domaingame.PlanetOverview{
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   domaingame.Resources{Metal: 1000, Crystal: 500, Deuterium: 5000},
		},
		Slots: domaingame.FleetSlots{Used: 0, Max: 4},
		Ships: []domaingame.FleetShipSelection{{
			ID:          domaingame.FleetSmallCargo,
			Name:        "Small Cargo",
			Count:       4,
			Speed:       5500,
			Cargo:       5000,
			Consumption: 10,
			Selectable:  true,
		}},
	}}
	service := NewFleetService(sessions, repository)

	result, err := service.ValidateFleetDispatch(context.Background(), FleetDispatchValidateCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Ships:           map[int]int{domaingame.FleetSmallCargo: 3},
		Resources:       map[int]int{domaingame.ResourceMetal: 900},
		Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
		TargetType:      domaingame.GamePlanetTypePlanet,
		Mission:         domaingame.FleetMissionTransport,
		Speed:           9,
		HoldHours:       4,
		UnionID:         7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.ActionIssue != nil || result.Fleet.DispatchDraft == nil || !result.Fleet.DispatchDraft.Ready {
		t.Fatalf("unexpected validate result: %+v", result)
	}
	if result.Fleet.DispatchDraft.Resources[0].Requested != 900 || result.Fleet.DispatchDraft.Resources[0].Loaded != 900 {
		t.Fatalf("expected validation to include resource loading plan, got %+v", result.Fleet.DispatchDraft.Resources)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("expected validate to load selected fleet screen, got %+v", repository.query)
	}
}

func TestFleetServicePrepareReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.PrepareFleetDispatch(context.Background(), FleetDispatchPrepareCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated prepare without repository call, got %+v", result)
	}
}

func TestFleetServiceValidateReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.ValidateFleetDispatch(context.Background(), FleetDispatchValidateCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated validate without repository call, got %+v", result)
	}
}

func TestFleetServiceRecallsFleetAndReloadsFleet(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{Commander: "legor"}}
	service := NewFleetService(sessions, repository)

	result, err := service.RecallFleet(context.Background(), FleetRecallCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		FleetID:         123,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Fleet.Commander != "legor" {
		t.Fatalf("unexpected recall result: %+v", result)
	}
	if repository.recall.PlayerID != 42 || repository.recall.FleetID != 123 {
		t.Fatalf("unexpected recall query: %+v", repository.recall)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("expected recall to reload selected fleet screen, got %+v", repository.query)
	}
}

func TestFleetServiceRecallReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.RecallFleet(context.Background(), FleetRecallCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called || repository.recalled {
		t.Fatalf("expected unauthenticated recall without repository call, got %+v", result)
	}
}

func TestFleetServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewFleetService(&fakeSessionLookup{err: sessionErr}, &fakeFleetRepository{})
	if _, err := service.GetFleet(context.Background(), FleetCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := service.MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	if _, err := service.PrepareFleetDispatch(context.Background(), FleetDispatchPrepareCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected prepare session error, got %v", err)
	}
	if _, err := service.ValidateFleetDispatch(context.Background(), FleetDispatchValidateCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected validate session error, got %v", err)
	}
	if _, err := service.RecallFleet(context.Background(), FleetRecallCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected recall session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewFleetService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeFleetRepository{err: repoErr})
	if _, err := service.GetFleet(context.Background(), FleetCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	service = NewFleetService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeFleetRepository{err: repoErr})
	if _, err := service.MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected mutation repository error, got %v", err)
	}
	if _, err := service.PrepareFleetDispatch(context.Background(), FleetDispatchPrepareCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected prepare repository error, got %v", err)
	}
	if _, err := service.ValidateFleetDispatch(context.Background(), FleetDispatchValidateCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected validate repository error, got %v", err)
	}
	if _, err := service.RecallFleet(context.Background(), FleetRecallCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected recall repository error, got %v", err)
	}

	reloadErr := errors.New("fleet reload failed")
	service = NewFleetService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeFleetRepository{getErr: reloadErr})
	if _, err := service.MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{}); !errors.Is(err, reloadErr) {
		t.Fatalf("expected mutation reload error, got %v", err)
	}
	if _, err := service.RecallFleet(context.Background(), FleetRecallCommand{}); !errors.Is(err, reloadErr) {
		t.Fatalf("expected recall reload error, got %v", err)
	}
}

func TestFleetServiceRequiresDependencies(t *testing.T) {
	if _, err := (FleetService{}).GetFleet(context.Background(), FleetCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
	if _, err := (FleetService{}).MutateFleetTemplate(context.Background(), FleetTemplateMutationCommand{}); err == nil {
		t.Fatal("expected mutation dependency error")
	}
	if _, err := (FleetService{}).PrepareFleetDispatch(context.Background(), FleetDispatchPrepareCommand{}); err == nil {
		t.Fatal("expected prepare dependency error")
	}
	if _, err := (FleetService{}).ValidateFleetDispatch(context.Background(), FleetDispatchValidateCommand{}); err == nil {
		t.Fatal("expected validate dependency error")
	}
	if _, err := (FleetService{}).RecallFleet(context.Background(), FleetRecallCommand{}); err == nil {
		t.Fatal("expected recall dependency error")
	}
}

type fakeFleetRepository struct {
	result   domaingame.Fleet
	err      error
	getErr   error
	query    FleetQuery
	mutation FleetTemplateMutationQuery
	recall   FleetRecallQuery
	called   bool
	mutated  bool
	recalled bool
}

func (f *fakeFleetRepository) GetFleet(_ context.Context, query FleetQuery) (domaingame.Fleet, error) {
	f.query = query
	f.called = true
	if f.getErr != nil {
		return f.result, f.getErr
	}
	return f.result, f.err
}

func (f *fakeFleetRepository) MutateFleetTemplate(_ context.Context, query FleetTemplateMutationQuery) error {
	f.mutation = query
	f.mutated = true
	return f.err
}

func (f *fakeFleetRepository) RecallFleet(_ context.Context, query FleetRecallQuery) error {
	f.recall = query
	f.recalled = true
	return f.err
}

package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestGalaxyServiceReturnsAuthenticatedGalaxy(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeGalaxyRepository{result: domaingame.Galaxy{Commander: "legor"}}
	service := NewGalaxyService(sessions, repository)

	result, err := service.GetGalaxy(context.Background(), GalaxyCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Coordinates:     domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Galaxy.Commander != "legor" {
		t.Fatalf("unexpected galaxy result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Coordinates.System != 2 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestGalaxyServiceLaunchesMissilesAndRefreshesGalaxy(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	issue := domaingame.GalaxyMissileLaunchedIssue(2)
	repository := &fakeGalaxyRepository{
		result:      domaingame.Galaxy{Commander: "legor"},
		actionIssue: issue,
	}
	service := NewGalaxyService(sessions, repository)

	result, err := service.LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Coordinates:     domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		TargetPlanetID:  77,
		Amount:          2,
		TargetDefenseID: domaingame.DefenseRocketLauncher,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Galaxy.Commander != "legor" || result.ActionIssue != issue {
		t.Fatalf("unexpected launch result: %+v", result)
	}
	if repository.launch.PlayerID != 42 || repository.launch.PlanetID != 99 || repository.launch.TargetPlanetID != 77 || repository.launch.TargetDefenseID != domaingame.DefenseRocketLauncher {
		t.Fatalf("unexpected launch query: %+v", repository.launch)
	}
	if repository.query.PlayerID != 42 || repository.query.Coordinates.System != 2 {
		t.Fatalf("expected refreshed galaxy query, got %+v", repository.query)
	}
}

func TestGalaxyServiceDispatchesInstantFleetAndRefreshesGalaxy(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	issue := domaingame.GalaxyFleetDispatchedIssue()
	repository := &fakeGalaxyRepository{
		result:        domaingame.Galaxy{Commander: "legor"},
		dispatchIssue: issue,
	}
	service := NewGalaxyService(sessions, repository)

	result, err := service.DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Coordinates:     domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		Target:          domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
		TargetType:      domaingame.GamePlanetTypePlanet,
		Mission:         domaingame.FleetMissionSpy,
		Amount:          2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Galaxy.Commander != "legor" || result.ActionIssue != issue {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if repository.dispatch.PlayerID != 42 || repository.dispatch.PlanetID != 99 || repository.dispatch.Target.Position != 4 ||
		repository.dispatch.TargetType != domaingame.GamePlanetTypePlanet || repository.dispatch.Mission != domaingame.FleetMissionSpy || repository.dispatch.Amount != 2 {
		t.Fatalf("unexpected dispatch query: %+v", repository.dispatch)
	}
	if repository.query.PlayerID != 42 || repository.query.Coordinates.System != 2 {
		t.Fatalf("expected refreshed galaxy query, got %+v", repository.query)
	}
}

func TestGalaxyServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeGalaxyRepository{}
	service := NewGalaxyService(sessions, repository)

	result, err := service.GetGalaxy(context.Background(), GalaxyCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}

	result, err = service.LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.launchCalled {
		t.Fatalf("expected unauthenticated launch result without repository call, got %+v", result)
	}

	result, err = service.DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.dispatchCalled {
		t.Fatalf("expected unauthenticated dispatch result without repository call, got %+v", result)
	}
}

func TestGalaxyServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewGalaxyService(&fakeSessionLookup{err: sessionErr}, &fakeGalaxyRepository{})
	if _, err := service.GetGalaxy(context.Background(), GalaxyCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := service.LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected launch session error, got %v", err)
	}
	if _, err := service.DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected dispatch session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{err: repoErr})
	if _, err := service.GetGalaxy(context.Background(), GalaxyCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{launchErr: repoErr})
	if _, err := service.LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected launch error, got %v", err)
	}

	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{dispatchErr: repoErr})
	if _, err := service.DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected dispatch error, got %v", err)
	}

	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{err: repoErr})
	if _, err := service.LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected refresh error after launch, got %v", err)
	}

	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{err: repoErr})
	if _, err := service.DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected refresh error after dispatch, got %v", err)
	}
}

func TestGalaxyServiceRequiresDependencies(t *testing.T) {
	if _, err := (GalaxyService{}).GetGalaxy(context.Background(), GalaxyCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
	if _, err := (GalaxyService{}).LaunchMissiles(context.Background(), GalaxyMissileLaunchCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
	if _, err := (GalaxyService{}).DispatchInstantFleet(context.Background(), GalaxyInstantDispatchCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeGalaxyRepository struct {
	result         domaingame.Galaxy
	err            error
	launchErr      error
	dispatchErr    error
	actionIssue    *domaingame.GalaxyActionIssue
	dispatchIssue  *domaingame.GalaxyActionIssue
	query          GalaxyQuery
	launch         GalaxyMissileLaunchQuery
	dispatch       GalaxyInstantDispatchQuery
	called         bool
	launchCalled   bool
	dispatchCalled bool
}

func (f *fakeGalaxyRepository) GetGalaxy(_ context.Context, query GalaxyQuery) (domaingame.Galaxy, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}

func (f *fakeGalaxyRepository) LaunchMissiles(_ context.Context, query GalaxyMissileLaunchQuery) (*domaingame.GalaxyActionIssue, error) {
	f.launch = query
	f.launchCalled = true
	return f.actionIssue, f.launchErr
}

func (f *fakeGalaxyRepository) DispatchInstantFleet(_ context.Context, query GalaxyInstantDispatchQuery) (*domaingame.GalaxyActionIssue, error) {
	f.dispatch = query
	f.dispatchCalled = true
	return f.dispatchIssue, f.dispatchErr
}

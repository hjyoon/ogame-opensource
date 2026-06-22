package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestMerchantServiceReturnsMerchantForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeMerchantRepository{merchant: domaingame.Merchant{Commander: "legor"}}
	service := NewMerchantService(sessions, repository)

	result, err := service.GetMerchant(context.Background(), MerchantCommand{PublicSession: "pub", PlanetID: 99})
	if err != nil {
		t.Fatalf("GetMerchant returned error: %v", err)
	}
	if !result.Authenticated || result.Merchant.Commander != "legor" || repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.query)
	}
}

func TestMerchantServiceMutatesForAuthenticatedSession(t *testing.T) {
	issue := domaingame.MerchantNotEnoughResourceIssue()
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeMerchantRepository{merchant: domaingame.Merchant{Commander: "legor"}, issue: issue}
	service := NewMerchantService(sessions, repository)

	result, err := service.MutateMerchant(context.Background(), MerchantMutationCommand{
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{Action: "trade", OfferID: domaingame.MerchantResourceMetal},
	})
	if err != nil {
		t.Fatalf("MutateMerchant returned error: %v", err)
	}
	if !result.Authenticated || result.ActionIssue.Code != domaingame.MerchantIssueNotEnoughResource ||
		repository.mutationQuery.Mutation.Action != "trade" || repository.mutationQuery.PlanetID != 99 {
		t.Fatalf("unexpected result=%+v query=%+v", result, repository.mutationQuery)
	}
}

func TestMerchantServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewMerchantService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeMerchantRepository{})
	result, err := service.GetMerchant(context.Background(), MerchantCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got result=%+v err=%v", result, err)
	}

	result, err = service.MutateMerchant(context.Background(), MerchantMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation result, got result=%+v err=%v", result, err)
	}

	if _, err := (MerchantService{}).GetMerchant(context.Background(), MerchantCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (MerchantService{}).MutateMerchant(context.Background(), MerchantMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewMerchantService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeMerchantRepository{}).GetMerchant(context.Background(), MerchantCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewMerchantService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeMerchantRepository{}).MutateMerchant(context.Background(), MerchantMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	if _, err := NewMerchantService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, &fakeMerchantRepository{err: errors.New("merchant failed")}).GetMerchant(context.Background(), MerchantCommand{}); err == nil || !strings.Contains(err.Error(), "merchant failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewMerchantService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, &fakeMerchantRepository{err: errors.New("merchant failed")}).MutateMerchant(context.Background(), MerchantMutationCommand{}); err == nil || !strings.Contains(err.Error(), "merchant failed") {
		t.Fatalf("expected mutation repository error, got %v", err)
	}
}

type fakeMerchantRepository struct {
	merchant      domaingame.Merchant
	issue         *domaingame.MerchantActionIssue
	err           error
	query         MerchantQuery
	mutationQuery MerchantMutationQuery
}

func (f *fakeMerchantRepository) GetMerchant(_ context.Context, query MerchantQuery) (domaingame.Merchant, error) {
	f.query = query
	return f.merchant, f.err
}

func (f *fakeMerchantRepository) MutateMerchant(_ context.Context, query MerchantMutationQuery) (domaingame.Merchant, *domaingame.MerchantActionIssue, error) {
	f.mutationQuery = query
	return f.merchant, f.issue, f.err
}

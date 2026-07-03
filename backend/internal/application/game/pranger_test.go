package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestPrangerServiceNormalizesOffsetAndDelegates(t *testing.T) {
	repository := &fakePrangerRepository{pranger: domaingame.Pranger{Universe: 1}}
	service := NewPrangerService(repository)

	result, err := service.GetPranger(context.Background(), PrangerCommand{Universe: 1, From: -25, Internal: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Universe != 1 || repository.query.From != 0 || !repository.query.Internal {
		t.Fatalf("unexpected pranger result/query: result=%+v query=%+v", result, repository.query)
	}
}

func TestPrangerServiceDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (PrangerService{}).GetPranger(context.Background(), PrangerCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies unavailable") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewPrangerService(&fakePrangerRepository{err: errors.New("query failed")}).GetPranger(context.Background(), PrangerCommand{}); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakePrangerRepository struct {
	query   PrangerQuery
	pranger domaingame.Pranger
	err     error
}

func (f *fakePrangerRepository) GetPranger(_ context.Context, query PrangerQuery) (domaingame.Pranger, error) {
	f.query = query
	return f.pranger, f.err
}

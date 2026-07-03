package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestMaintenanceServiceDelegates(t *testing.T) {
	service := NewMaintenanceService(&fakeMaintenanceRepository{maintenance: domaingame.Maintenance{Frozen: true}})

	maintenance, err := service.GetMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !maintenance.Frozen {
		t.Fatalf("expected frozen maintenance state, got %+v", maintenance)
	}
}

func TestMaintenanceServiceErrors(t *testing.T) {
	if _, err := (MaintenanceService{}).GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "dependencies unavailable") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewMaintenanceService(&fakeMaintenanceRepository{err: errors.New("query failed")}).GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeMaintenanceRepository struct {
	maintenance domaingame.Maintenance
	err         error
}

func (f *fakeMaintenanceRepository) GetMaintenance(context.Context) (domaingame.Maintenance, error) {
	return f.maintenance, f.err
}

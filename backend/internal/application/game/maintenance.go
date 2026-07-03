package game

import (
	"context"
	"errors"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type MaintenanceRepository interface {
	GetMaintenance(context.Context) (domaingame.Maintenance, error)
}

type MaintenanceService struct {
	repository MaintenanceRepository
}

func NewMaintenanceService(repository MaintenanceRepository) MaintenanceService {
	return MaintenanceService{repository: repository}
}

func (s MaintenanceService) GetMaintenance(ctx context.Context) (domaingame.Maintenance, error) {
	if s.repository == nil {
		return domaingame.Maintenance{}, errors.New("maintenance dependencies unavailable")
	}
	return s.repository.GetMaintenance(ctx)
}

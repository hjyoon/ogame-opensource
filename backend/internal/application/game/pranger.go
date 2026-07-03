package game

import (
	"context"
	"errors"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type PrangerRepository interface {
	GetPranger(context.Context, PrangerQuery) (domaingame.Pranger, error)
}

type PrangerQuery struct {
	Universe int
	From     int
	Internal bool
}

type PrangerCommand struct {
	Universe int
	From     int
	Internal bool
}

type PrangerService struct {
	repository PrangerRepository
}

func NewPrangerService(repository PrangerRepository) PrangerService {
	return PrangerService{repository: repository}
}

func (s PrangerService) GetPranger(ctx context.Context, command PrangerCommand) (domaingame.Pranger, error) {
	if s.repository == nil {
		return domaingame.Pranger{}, errors.New("pranger dependencies unavailable")
	}
	return s.repository.GetPranger(ctx, PrangerQuery{
		Universe: command.Universe,
		From:     domaingame.NormalizePrangerFrom(command.From),
		Internal: command.Internal,
	})
}

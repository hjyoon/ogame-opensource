package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// RuntimeQueueSettler completes every due game queue independently so one
// failed queue family does not leave unrelated work stuck at zero seconds.
type RuntimeQueueSettler struct {
	finishers []func(context.Context, int) error
}

func NewRuntimeQueueSettler(db *sql.DB, prefix string) RuntimeQueueSettler {
	fleets := NewFleetRepository(db, prefix)
	buildings := NewBuildingsRepository(db, prefix)
	research := NewResearchRepository(db, prefix)
	shipyard := NewShipyardRepository(db, prefix)
	runner := SQLQueryer{DB: db}
	bots := BotRuntimeRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
	overview := NewOverviewRepositoryWithRunner(runner, runner, prefix)
	return RuntimeQueueSettler{finishers: []func(context.Context, int) error{
		fleets.FinishDueFleetQueues,
		buildings.FinishDueBuildingQueues,
		research.FinishDueResearchQueues,
		shipyard.FinishDueShipyardQueues,
		bots.FinishDueBotQueues,
		overview.FinishDueRecalcPointQueues,
	}}
}

func (s RuntimeQueueSettler) FinishDueQueues(ctx context.Context, until int) error {
	ctx = withRuntimeQueueCompletionPolicy(ctx)
	var joined error
	for _, finish := range s.finishers {
		if finish == nil {
			continue
		}
		err := finish(ctx, until)
		if errors.Is(err, ErrQueueSettlementBusy) {
			// User-facing mutations take priority over the periodic worker. The
			// next tick will re-read and revalidate every still-due task.
			return err
		}
		joined = errors.Join(joined, err)
	}
	return joined
}

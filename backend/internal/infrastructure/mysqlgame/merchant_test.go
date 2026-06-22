package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestMerchantRepositoryReadsMerchantStatus(t *testing.T) {
	queryer := &fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{4000, 7000, 1, 3.0, 2.0, 1.0})},
	)}
	repository := NewMerchantRepositoryWithQueryer(queryer, "ogame_")

	merchant, err := repository.GetMerchant(context.Background(), appgame.MerchantQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatalf("GetMerchant returned error: %v", err)
	}
	if merchant.User.PaidDarkMatter != 4000 || merchant.User.FreeDarkMatter != 7000 ||
		merchant.ActiveOfferID != domaingame.MerchantResourceMetal || !merchant.Rows[0].Offered {
		t.Fatalf("unexpected merchant result: %+v", merchant)
	}
	if !strings.Contains(queryer.calls[len(queryer.calls)-1].sql, "rate_m") {
		t.Fatalf("expected merchant user query, got %s", queryer.calls[len(queryer.calls)-1].sql)
	}

	_ = NewMerchantRepository(nil, "ogame_")
	_ = NewMerchantRepositoryWithQueryer(&fakeOptionsRunner{}, "ogame_")

	queryer = &fakeQueryer{results: append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}
	repository = NewMerchantRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.GetMerchant(context.Background(), appgame.MerchantQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected load user error, got %v", err)
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
	if _, err := repository.GetMerchant(context.Background(), appgame.MerchantQuery{PlayerID: 42, PlanetID: 99}); err == nil {
		t.Fatalf("expected overview query error")
	}
}

func TestMerchantRepositoryCallMerchantUpdatesOfferAndRates(t *testing.T) {
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{4000, 7000, 0, 0.0, 0.0, 0.0})}),
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{1500, 7000, 1, 3.0, 2.0, 1.0})})...,
	)}}
	repository := NewMerchantRepositoryWithRunner(runner, runner, "ogame_", sequenceMerchantInt(5, 180, 90))

	merchant, issue, err := repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: domaingame.MerchantResourceMetal,
		},
	})
	if err != nil {
		t.Fatalf("MutateMerchant returned error: %v", err)
	}
	if issue != nil {
		t.Fatalf("unexpected merchant issue: %+v", issue)
	}
	if !strings.Contains(runner.execSQL, "trader") || len(runner.execArgs) != 7 ||
		runner.execArgs[0] != 1500 || runner.execArgs[1] != 7000 || runner.execArgs[2] != domaingame.MerchantResourceMetal {
		t.Fatalf("unexpected merchant call exec: sql=%s args=%+v", runner.execSQL, runner.execArgs)
	}
	if merchant.ActiveOfferID != domaingame.MerchantResourceMetal || merchant.Rates.Metal != 3 {
		t.Fatalf("unexpected updated merchant: %+v", merchant)
	}
}

func TestMerchantRepositoryTradeMerchantUpdatesResources(t *testing.T) {
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})}),
		append(optionsOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 0, 0.0, 0.0, 0.0})})...,
	)}}
	repository := NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)

	merchant, issue, err := repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{
				Crystal: 100,
			},
		},
	})
	if err != nil {
		t.Fatalf("MutateMerchant returned error: %v", err)
	}
	if issue != nil {
		t.Fatalf("unexpected trade issue: %+v", issue)
	}
	if !strings.Contains(runner.execSQL, "`700` = ?") || len(runner.execArgs) != 5 ||
		runner.execArgs[0] != 850 || runner.execArgs[1] != 2100 || runner.execArgs[2] != 3000 {
		t.Fatalf("unexpected trade exec: sql=%s args=%+v", runner.execSQL, runner.execArgs)
	}
	if merchant.ActiveOfferID != 0 {
		t.Fatalf("trade should clear active offer in updated merchant: %+v", merchant)
	}
}

func TestMerchantRepositoryMutationNoopsAndErrors(t *testing.T) {
	repository := NewMerchantRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", nil)
	if _, _, err := repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected updater error, got %v", err)
	}

	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{5000, 0, 0, 0.0, 0.0, 0.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, issue, err := repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: 99,
		},
	})
	if err != nil || issue != nil || runner.execSQL != "" {
		t.Fatalf("invalid merchant offer should be ignored, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{1000, 1000, 0, 0.0, 0.0, 0.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, issue, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: domaingame.MerchantResourceCrystal,
		},
	})
	if err != nil || issue == nil || issue.Code != domaingame.MerchantIssueNotEnoughDarkMatter || runner.execSQL != "" {
		t.Fatalf("expected DM issue without exec, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}

	runner = &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{5000, 0, 0, 0.0, 0.0, 0.0})},
		)},
		execErr: errors.New("merchant update failed"),
	}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", sequenceMerchantInt(5, 180, 90))
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: domaingame.MerchantResourceDeuterium,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "merchant update failed") {
		t.Fatalf("expected update error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{5000, 0, 0, 0.0, 0.0, 0.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", sequenceMerchantInt(5, 180, 90))
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: domaingame.MerchantResourceMetal,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected query") {
		t.Fatalf("expected reload error after call update, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 0, 0.0, 0.0, 0.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, issue, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 100},
		},
	})
	if err != nil || issue != nil || runner.execSQL != "" {
		t.Fatalf("inactive trade should be ignored, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, issue, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 10_000},
		},
	})
	if err != nil || issue == nil || issue.Code != domaingame.MerchantIssueNotEnoughResource || runner.execSQL != "" {
		t.Fatalf("expected resource issue without exec, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}

	runner = &fakeOptionsRunner{
		fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})},
		)},
		execErr: errors.New("trade update failed"),
	}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 100},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "trade update failed") {
		t.Fatalf("expected trade update error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 0, 0.0, 0.0, 0.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	_, issue, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{PlayerID: 42, PlanetID: 99, Mutation: domaingame.MerchantMutation{Action: "unknown"}})
	if err != nil || issue != nil || runner.execSQL != "" {
		t.Fatalf("unknown action should reload merchant only, issue=%+v err=%v exec=%s", issue, err, runner.execSQL)
	}
}

func TestMerchantRepositoryLoadUserErrorsAndRates(t *testing.T) {
	repository := NewMerchantRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("query failed")}}}, "ogame_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected query error, got %v", err)
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("empty rows failed"))}}}, "ogame_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "empty rows failed") {
		t.Fatalf("expected empty rows error, got %v", err)
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, 0, 0.0, 0.0, 0.0})}}}, "ogame_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil {
		t.Fatalf("expected scan error")
	}

	repository = NewMerchantRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"), []any{1, 2, 3, 3.0, 2.0, 1.0})}}}, "ogame_")
	if _, _, _, err := repository.loadMerchantUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}

	repository = NewMerchantRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", sequenceMerchantInt(50, 250, 90))
	rates, ok := repository.generateRates(domaingame.MerchantResourceCrystal)
	if !ok || rates.Crystal != 2 || rates.Metal != 2.5 || rates.Deuterium != 0.9 {
		t.Fatalf("unexpected generated rates: %+v ok=%v", rates, ok)
	}
	if _, ok := repository.generateRates(99); ok {
		t.Fatalf("invalid offer should not generate rates")
	}
	if randomMerchantInt(4, 4) != 4 {
		t.Fatalf("single-value random range should return min")
	}
	value := randomMerchantInt(4, 5)
	if value < 4 || value > 5 {
		t.Fatalf("random value outside requested range: %d", value)
	}
}

func TestMerchantRepositoryPrefixAndSecondExecErrors(t *testing.T) {
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{5000, 0, 0, 0.0, 0.0, 0.0})},
	)}}
	repository := NewMerchantRepositoryWithRunner(runner, runner, "ogame_", sequenceMerchantInt(5, 180, 90))
	repository.prefix = "bad-prefix_"
	_, _, err := repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action:  "call",
			OfferID: domaingame.MerchantResourceMetal,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected call prefix error, got %v", err)
	}

	runner = &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})},
	)}}
	repository = NewMerchantRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.prefix = "bad-prefix_"
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 100},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected trade prefix error, got %v", err)
	}

	sequenceRunner := &fakeMerchantExecSequenceRunner{
		fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})},
		)},
		failOn:  2,
		execErr: errors.New("planet update failed"),
	}
	repository = NewMerchantRepositoryWithRunner(sequenceRunner, sequenceRunner, "ogame_", nil)
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 100},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "planet update failed") {
		t.Fatalf("expected planet update error, got %v", err)
	}

	sequenceRunner = &fakeMerchantExecSequenceRunner{
		fakeQueryer: fakeQueryer{results: append(optionsOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{0, 0, 1, 3.0, 2.0, 1.0})},
		)},
	}
	repository = NewMerchantRepositoryWithRunner(sequenceRunner, sequenceRunner, "ogame_", nil)
	_, _, err = repository.MutateMerchant(context.Background(), appgame.MerchantMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Mutation: domaingame.MerchantMutation{
			Action: "trade",
			Values: domaingame.MerchantTradeValues{Crystal: 100},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected query") {
		t.Fatalf("expected reload error after trade update, got %v", err)
	}
}

func sequenceMerchantInt(values ...int) func(min int, max int) int {
	index := 0
	return func(min int, max int) int {
		if index >= len(values) {
			return min
		}
		value := values[index]
		index++
		return value
	}
}

type fakeMerchantExecSequenceRunner struct {
	fakeQueryer
	execCount int
	failOn    int
	execErr   error
}

func (f *fakeMerchantExecSequenceRunner) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	f.execCount++
	if f.failOn == f.execCount {
		return fakeSQLResult(0), f.execErr
	}
	return fakeSQLResult(1), nil
}

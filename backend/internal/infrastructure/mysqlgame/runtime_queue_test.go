package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeQueueSettlerRunsEveryQueueFamilyAndJoinsErrors(t *testing.T) {
	first := errors.New("building queue failed")
	second := errors.New("fleet queue failed")
	calls := []int{}
	settler := RuntimeQueueSettler{finishers: []func(context.Context, int) error{
		func(_ context.Context, until int) error { calls = append(calls, until); return first },
		nil,
		func(_ context.Context, until int) error { calls = append(calls, until); return second },
	}}

	err := settler.FinishDueQueues(context.Background(), 1234)
	if len(calls) != 2 || calls[0] != 1234 || calls[1] != 1234 {
		t.Fatalf("unexpected settlement calls: %v", calls)
	}
	if !errors.Is(err, first) || !errors.Is(err, second) || !strings.Contains(err.Error(), first.Error()) || !strings.Contains(err.Error(), second.Error()) {
		t.Fatalf("expected joined settlement errors, got %v", err)
	}
}

func TestRuntimeQueueSettlerAllowsEmptyConfiguration(t *testing.T) {
	if err := (RuntimeQueueSettler{}).FinishDueQueues(context.Background(), 1); err != nil {
		t.Fatalf("empty settler returned error: %v", err)
	}
}

func TestRuntimeQueueSettlerStopsCurrentTickWhenMutationLockIsBusy(t *testing.T) {
	calls := 0
	settler := RuntimeQueueSettler{finishers: []func(context.Context, int) error{
		func(ctx context.Context, _ int) error {
			calls++
			if _, ok := ctx.Value(queueCompletionLockPolicyKey{}).(queueCompletionLockPolicy); !ok {
				t.Fatal("runtime queue policy was not attached to the worker context")
			}
			return ErrQueueSettlementBusy
		},
		func(context.Context, int) error {
			calls++
			return nil
		},
	}}

	err := settler.FinishDueQueues(context.Background(), 1234)
	if !errors.Is(err, ErrQueueSettlementBusy) {
		t.Fatalf("expected busy queue result, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("busy worker tick should defer remaining families, got %d calls", calls)
	}
}

func TestNewRuntimeQueueSettlerRegistersAllQueueFamilies(t *testing.T) {
	settler := NewRuntimeQueueSettler(nil, "uni1_")
	if len(settler.finishers) != 6 {
		t.Fatalf("expected six queue families, got %d", len(settler.finishers))
	}
}

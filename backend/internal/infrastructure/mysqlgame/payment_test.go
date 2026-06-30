package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
)

func TestPaymentRepositoryChecksActiveCoupon(t *testing.T) {
	_ = NewPaymentRepository(nil, nil, "ogame_", 0)
	repositoryWithDefaultUniverse := NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, &fakeQueryer{}, nil, "ogame_", 0)
	if repositoryWithDefaultUniverse.uniNumber != 1 {
		t.Fatalf("expected default universe 1, got %d", repositoryWithDefaultUniverse.uniNumber)
	}

	master := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{7, "ABCD-EFGH-IJKL-MNOP-QRST", 5000, 0, 0, 0, ""})},
	}}
	repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, master, nil, "ogame_", 1)

	coupon, found, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: " abcd-efgh-ijkl-mnop-qrst "})

	if err != nil {
		t.Fatalf("CheckCoupon returned error: %v", err)
	}
	if !found || coupon.ID != 7 || coupon.Code != "ABCD-EFGH-IJKL-MNOP-QRST" || coupon.Amount != 5000 || coupon.Used {
		t.Fatalf("unexpected coupon found=%v coupon=%+v", found, coupon)
	}
	if master.calls[0].args[0] != "ABCD-EFGH-IJKL-MNOP-QRST" {
		t.Fatalf("expected normalized coupon code, got %+v", master.calls[0].args)
	}
}

func TestPaymentRepositoryActivatesCouponAndCreditsPaidDM(t *testing.T) {
	uni := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "legor"})},
	}}}
	master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{7, "ABCD-EFGH-IJKL-MNOP-QRST", 5000, 0, 0, 0, ""})},
	}}}
	repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 3)

	coupon, activated, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "ABCD-EFGH-IJKL-MNOP-QRST"})

	if err != nil {
		t.Fatalf("ActivateCoupon returned error: %v", err)
	}
	if !activated || !coupon.Used || coupon.UserUniverse != 3 || coupon.UserID != 42 || coupon.UserName != "legor" {
		t.Fatalf("unexpected activated coupon=%+v activated=%v", coupon, activated)
	}
	if len(master.execCalls) != 1 || !strings.Contains(master.execCalls[0].sql, "UPDATE coupons SET used = 1") ||
		master.execCalls[0].args[0] != 3 || master.execCalls[0].args[1] != 42 || master.execCalls[0].args[2] != "legor" {
		t.Fatalf("unexpected master coupon update: %+v", master.execCalls)
	}
	if len(uni.execCalls) != 1 || !strings.Contains(uni.execCalls[0].sql, "UPDATE `ogame_users` SET dm = dm + ?") ||
		uni.execCalls[0].args[0] != 5000 || uni.execCalls[0].args[1] != 42 {
		t.Fatalf("unexpected paid DM update: %+v", uni.execCalls)
	}
}

func TestPaymentRepositoryHandlesMissingAndUsedCoupons(t *testing.T) {
	master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{8, "USED-CODE", 1000, 1, 1, 42, "legor"})},
	}}}
	repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, &fakeGalaxyRunner{}, master, master, "ogame_", 1)

	coupon, found, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "missing"})
	if err != nil || found || coupon.ID != 0 {
		t.Fatalf("expected missing coupon, got coupon=%+v found=%v err=%v", coupon, found, err)
	}

	coupon, found, err = repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "USED-CODE"})
	if err != nil || !found || !coupon.Used {
		t.Fatalf("expected used row mapping, got coupon=%+v found=%v err=%v", coupon, found, err)
	}

	coupon, found, err = repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "   "})
	if err != nil || found || coupon.ID != 0 {
		t.Fatalf("expected blank coupon to be ignored, got coupon=%+v found=%v err=%v", coupon, found, err)
	}
}

func TestPaymentRepositoryActivationErrors(t *testing.T) {
	t.Run("missing updater", func(t *testing.T) {
		repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, &fakeQueryer{}, nil, "ogame_", 1)
		if _, _, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
			t.Fatalf("expected updater error, got %v", err)
		}
	})

	t.Run("master unavailable", func(t *testing.T) {
		repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, &fakeGalaxyRunner{}, nil, &fakeGalaxyRunner{}, "ogame_", 1)
		if _, _, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "master DB unavailable") {
			t.Fatalf("expected master DB error, got %v", err)
		}
	})

	t.Run("user missing", func(t *testing.T) {
		uni := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{7, "CODE", 5000, 0, 0, 0, ""})},
		}}}
		repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 1)
		_, activated, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"})
		if err != nil || activated {
			t.Fatalf("expected user missing noop, activated=%v err=%v", activated, err)
		}
	})

	t.Run("coupon race", func(t *testing.T) {
		uni := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "legor"})}}}}
		master := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7, "CODE", 5000, 0, 0, 0, ""})}}},
			execResults: []sql.Result{overviewTestResult(0)},
		}
		repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 1)
		_, activated, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"})
		if err != nil || activated || len(uni.execCalls) != 0 {
			t.Fatalf("expected zero-row coupon update noop, activated=%v err=%v uniExec=%+v", activated, err, uni.execCalls)
		}
	})

	t.Run("master update error", func(t *testing.T) {
		uni := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "legor"})}}}}
		master := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7, "CODE", 5000, 0, 0, 0, ""})}}},
			execErrs:    []error{errors.New("coupon update failed")},
		}
		repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 1)
		if _, _, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "coupon update failed") {
			t.Fatalf("expected coupon update error, got %v", err)
		}
	})

	t.Run("master rows affected error", func(t *testing.T) {
		uni := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "legor"})}}}}
		master := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7, "CODE", 5000, 0, 0, 0, ""})}}},
			execResults: []sql.Result{fakeFleetSQLErrorResult{rowsErr: errors.New("rows affected failed")}},
		}
		repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 1)
		if _, _, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "rows affected failed") {
			t.Fatalf("expected rows affected error, got %v", err)
		}
	})

	t.Run("paid dm update error", func(t *testing.T) {
		uni := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "legor"})}}},
			execErrs:    []error{errors.New("dm update failed")},
		}
		master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{7, "CODE", 5000, 0, 0, 0, ""})},
		}}}
		repository := NewPaymentRepositoryWithRunners(uni, uni, master, master, "ogame_", 1)
		if _, _, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "dm update failed") {
			t.Fatalf("expected paid DM update error, got %v", err)
		}
	})

	t.Run("coupon query and scan errors", func(t *testing.T) {
		repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, &fakeQueryer{results: []fakeQueryResult{{err: errors.New("coupon query failed")}}}, nil, "ogame_", 1)
		if _, _, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "coupon query failed") {
			t.Fatalf("expected coupon query error, got %v", err)
		}

		repository = NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, nil, "ogame_", 1)
		if _, _, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected coupon scan error, got %v", err)
		}

		repository = NewPaymentRepositoryWithRunners(&fakeQueryer{}, nil, &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("coupon rows failed"))}}}, nil, "ogame_", 1)
		if _, _, err := repository.CheckCoupon(context.Background(), appgame.PaymentMutationQuery{CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "coupon rows failed") {
			t.Fatalf("expected coupon rows error, got %v", err)
		}
	})

	t.Run("user load errors", func(t *testing.T) {
		repository := NewPaymentRepositoryWithRunners(&fakeQueryer{}, &fakeGalaxyRunner{}, &fakeQueryer{}, &fakeGalaxyRunner{}, "bad-prefix_", 1)
		if _, _, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{PlayerID: 42, CouponCode: "CODE"}); err == nil || !strings.Contains(err.Error(), "unexpected query") {
			t.Fatalf("expected coupon query to fail before bad user prefix, got %v", err)
		}
		if _, _, err := repository.loadPaymentUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected user prefix error, got %v", err)
		}

		repository = NewPaymentRepositoryWithRunners(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("user query failed")}}}, nil, &fakeQueryer{}, nil, "ogame_", 1)
		if _, _, err := repository.loadPaymentUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "user query failed") {
			t.Fatalf("expected user query error, got %v", err)
		}

		repository = NewPaymentRepositoryWithRunners(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, nil, &fakeQueryer{}, nil, "ogame_", 1)
		if _, _, err := repository.loadPaymentUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected user scan error, got %v", err)
		}

		repository = NewPaymentRepositoryWithRunners(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"))}}}, nil, &fakeQueryer{}, nil, "ogame_", 1)
		if _, _, err := repository.loadPaymentUser(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "user rows failed") {
			t.Fatalf("expected user rows error, got %v", err)
		}
	})
}

func TestNormalizeCouponCode(t *testing.T) {
	if got := normalizeCouponCode(" abcd-efg "); got != "ABCD-EFG" {
		t.Fatalf("normalizeCouponCode returned %q", got)
	}
}

package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestPaymentServiceReturnsAuthenticatedPaymentShell(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	service := NewPaymentService(sessions, &fakePaymentRepository{})

	result, err := service.GetPayment(context.Background(), PaymentCommand{
		PublicSession:   "pub",
		PrivateSessions: map[string]string{"prsess_42_1": "priv"},
		RemoteAddr:      "203.0.113.10",
	})

	if err != nil {
		t.Fatalf("GetPayment returned error: %v", err)
	}
	if !result.Authenticated || sessions.command.PublicSession != "pub" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected result=%+v session=%+v", result, sessions.command)
	}
}

func TestPaymentServiceChecksAndActivatesCoupons(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakePaymentRepository{
		checkCoupon:    domaingame.PaymentCoupon{ID: 7, Code: "ABCD-EFGH-IJKL-MNOP-QRST", Amount: 5000},
		checkFound:     true,
		activateCoupon: domaingame.PaymentCoupon{ID: 7, Code: "ABCD-EFGH-IJKL-MNOP-QRST", Amount: 5000, Used: true, UserID: 42, UserName: "legor"},
		activated:      true,
	}
	service := NewPaymentService(sessions, repository)

	checkResult, err := service.MutatePayment(context.Background(), PaymentMutationCommand{Action: "check", CouponCode: " abcd-efgh-ijkl-mnop-qrst "})
	if err != nil {
		t.Fatalf("MutatePayment check returned error: %v", err)
	}
	if !checkResult.Authenticated || checkResult.ActionIssue == nil || checkResult.ActionIssue.Code != domaingame.PaymentIssueCouponValid ||
		checkResult.Payment.Coupon == nil || checkResult.Payment.Coupon.Amount != 5000 ||
		repository.checkQuery.PlayerID != 42 || repository.checkQuery.CouponCode != " abcd-efgh-ijkl-mnop-qrst " {
		t.Fatalf("unexpected check result=%+v query=%+v", checkResult, repository.checkQuery)
	}

	activateResult, err := service.MutatePayment(context.Background(), PaymentMutationCommand{Action: "activate", CouponCode: "ABCD-EFGH-IJKL-MNOP-QRST"})
	if err != nil {
		t.Fatalf("MutatePayment activate returned error: %v", err)
	}
	if !activateResult.Authenticated || activateResult.ActionIssue == nil || activateResult.ActionIssue.Code != domaingame.PaymentIssueCouponActivated ||
		activateResult.Payment.Coupon == nil || !activateResult.Payment.Coupon.Used ||
		repository.activateQuery.PlayerID != 42 {
		t.Fatalf("unexpected activate result=%+v query=%+v", activateResult, repository.activateQuery)
	}
}

func TestPaymentServiceReturnsInvalidCouponIssues(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	service := NewPaymentService(sessions, &fakePaymentRepository{})

	for _, action := range []string{"check", "activate", "unknown"} {
		result, err := service.MutatePayment(context.Background(), PaymentMutationCommand{Action: action})
		if err != nil {
			t.Fatalf("MutatePayment(%q) returned error: %v", action, err)
		}
		if !result.Authenticated || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.PaymentIssueInvalidCoupon || result.Payment.Coupon != nil {
			t.Fatalf("expected invalid coupon issue for action %q, got %+v", action, result)
		}
	}
}

func TestPaymentServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "Session is invalid."}
	service := NewPaymentService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakePaymentRepository{})
	result, err := service.GetPayment(context.Background(), PaymentCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got result=%+v err=%v", result, err)
	}
	result, err = service.MutatePayment(context.Background(), PaymentMutationCommand{})
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation result, got result=%+v err=%v", result, err)
	}

	if _, err := (PaymentService{}).GetPayment(context.Background(), PaymentCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (PaymentService{}).MutatePayment(context.Background(), PaymentMutationCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected mutation dependency error, got %v", err)
	}
	if _, err := NewPaymentService(&fakeSessionLookup{err: errors.New("session failed")}, &fakePaymentRepository{}).GetPayment(context.Background(), PaymentCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewPaymentService(&fakeSessionLookup{err: errors.New("session failed")}, &fakePaymentRepository{}).MutatePayment(context.Background(), PaymentMutationCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected mutation session error, got %v", err)
	}
	repository := &fakePaymentRepository{err: errors.New("coupon failed")}
	if _, err := NewPaymentService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, repository).MutatePayment(context.Background(), PaymentMutationCommand{Action: "check"}); err == nil || !strings.Contains(err.Error(), "coupon failed") {
		t.Fatalf("expected check repository error, got %v", err)
	}
	if _, err := NewPaymentService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Authenticated: true, Session: domainpublicsite.GameSession{PlayerID: 42}}}, repository).MutatePayment(context.Background(), PaymentMutationCommand{Action: "activate"}); err == nil || !strings.Contains(err.Error(), "coupon failed") {
		t.Fatalf("expected activate repository error, got %v", err)
	}
}

type fakePaymentRepository struct {
	checkCoupon    domaingame.PaymentCoupon
	activateCoupon domaingame.PaymentCoupon
	checkFound     bool
	activated      bool
	err            error
	checkQuery     PaymentMutationQuery
	activateQuery  PaymentMutationQuery
}

func (f *fakePaymentRepository) CheckCoupon(_ context.Context, query PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	f.checkQuery = query
	return f.checkCoupon, f.checkFound, f.err
}

func (f *fakePaymentRepository) ActivateCoupon(_ context.Context, query PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	f.activateQuery = query
	return f.activateCoupon, f.activated, f.err
}

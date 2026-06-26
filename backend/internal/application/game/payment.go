package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type PaymentRepository interface {
	CheckCoupon(context.Context, PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error)
	ActivateCoupon(context.Context, PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error)
}

type PaymentCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
}

type PaymentMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	Action          string
	CouponCode      string
}

type PaymentMutationQuery struct {
	PlayerID   int
	CouponCode string
}

type PaymentResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Payment       domaingame.Payment
	ActionIssue   *domaingame.PaymentActionIssue
}

type PaymentService struct {
	sessions   SessionLookup
	repository PaymentRepository
}

func NewPaymentService(sessions SessionLookup, repository PaymentRepository) PaymentService {
	return PaymentService{sessions: sessions, repository: repository}
}

func (s PaymentService) GetPayment(ctx context.Context, command PaymentCommand) (PaymentResult, error) {
	if s.sessions == nil {
		return PaymentResult{}, errors.New("payment dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return PaymentResult{}, err
	}
	if !session.Authenticated {
		return PaymentResult{Authenticated: false, Issues: session.Issues}, nil
	}
	return PaymentResult{Authenticated: true}, nil
}

func (s PaymentService) MutatePayment(ctx context.Context, command PaymentMutationCommand) (PaymentResult, error) {
	if s.sessions == nil || s.repository == nil {
		return PaymentResult{}, errors.New("payment dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return PaymentResult{}, err
	}
	if !session.Authenticated {
		return PaymentResult{Authenticated: false, Issues: session.Issues}, nil
	}
	query := PaymentMutationQuery{PlayerID: session.Session.PlayerID, CouponCode: command.CouponCode}
	switch command.Action {
	case "check":
		coupon, found, err := s.repository.CheckCoupon(ctx, query)
		if err != nil {
			return PaymentResult{}, err
		}
		if !found {
			return PaymentResult{Authenticated: true, ActionIssue: domaingame.PaymentIssue(domaingame.PaymentIssueInvalidCoupon)}, nil
		}
		return PaymentResult{
			Authenticated: true,
			Payment:       domaingame.Payment{Coupon: &coupon},
			ActionIssue:   domaingame.PaymentIssue(domaingame.PaymentIssueCouponValid),
		}, nil
	case "activate":
		coupon, activated, err := s.repository.ActivateCoupon(ctx, query)
		if err != nil {
			return PaymentResult{}, err
		}
		if !activated {
			return PaymentResult{Authenticated: true, ActionIssue: domaingame.PaymentIssue(domaingame.PaymentIssueInvalidCoupon)}, nil
		}
		return PaymentResult{
			Authenticated: true,
			Payment:       domaingame.Payment{Coupon: &coupon},
			ActionIssue:   domaingame.PaymentIssue(domaingame.PaymentIssueCouponActivated),
		}, nil
	default:
		return PaymentResult{Authenticated: true, ActionIssue: domaingame.PaymentIssue(domaingame.PaymentIssueInvalidCoupon)}, nil
	}
}

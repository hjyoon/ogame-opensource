package httpdelivery

import (
	"encoding/json"
	"log/slog"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gamePaymentResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Payment       *gamePaymentSummary        `json:"payment,omitempty"`
	ActionIssue   *gamePaymentActionIssue    `json:"actionIssue,omitempty"`
}

type gamePaymentMutationRequest struct {
	Action     string `json:"action"`
	CouponCode string `json:"couponCode"`
}

type gamePaymentSummary struct {
	Coupon *gamePaymentCoupon `json:"coupon,omitempty"`
}

type gamePaymentCoupon struct {
	ID           int    `json:"id"`
	Code         string `json:"code"`
	Amount       int    `json:"amount"`
	Used         bool   `json:"used"`
	UserUniverse int    `json:"userUniverse"`
	UserID       int    `json:"userId"`
	UserName     string `json:"userName"`
}

type gamePaymentActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a app) handleGamePayment(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.handleGamePaymentGet(w, r)
	case http.MethodPost:
		a.handleGamePaymentPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleGamePaymentGet(w http.ResponseWriter, r *http.Request) {
	if a.deps.GamePayment == nil {
		http.Error(w, "game payment unavailable", http.StatusServiceUnavailable)
		return
	}
	result, err := a.deps.GamePayment.GetPayment(r.Context(), appgame.PaymentCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
	})
	if err != nil {
		logGamePaymentError(a.deps.Logger, r, "game payment get failed", err)
		http.Error(w, "game payment unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGamePaymentResponse(w, result)
}

func (a app) handleGamePaymentPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.GamePayment == nil {
		http.Error(w, "game payment unavailable", http.StatusServiceUnavailable)
		return
	}
	var request gamePaymentMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid payment request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.GamePayment.MutatePayment(r.Context(), appgame.PaymentMutationCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		Action:          request.Action,
		CouponCode:      request.CouponCode,
	})
	if err != nil {
		logGamePaymentError(a.deps.Logger, r, "game payment mutation failed", err)
		http.Error(w, "game payment unavailable", http.StatusServiceUnavailable)
		return
	}
	writeGamePaymentResponse(w, result)
}

func logGamePaymentError(logger *slog.Logger, r *http.Request, message string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Error(message, "error", err, "method", r.Method, "path", r.URL.Path)
}

func writeGamePaymentResponse(w http.ResponseWriter, result appgame.PaymentResult) {
	status := http.StatusOK
	var payment *gamePaymentSummary
	if result.Authenticated {
		mapped := toGamePaymentSummary(result.Payment)
		payment = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gamePaymentResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Payment:       payment,
		ActionIssue:   toGamePaymentActionIssue(result.ActionIssue),
	})
}

func toGamePaymentSummary(payment domaingame.Payment) gamePaymentSummary {
	var coupon *gamePaymentCoupon
	if payment.Coupon != nil {
		mapped := toGamePaymentCoupon(*payment.Coupon)
		coupon = &mapped
	}
	return gamePaymentSummary{Coupon: coupon}
}

func toGamePaymentCoupon(coupon domaingame.PaymentCoupon) gamePaymentCoupon {
	return gamePaymentCoupon{
		ID:           coupon.ID,
		Code:         coupon.Code,
		Amount:       coupon.Amount,
		Used:         coupon.Used,
		UserUniverse: coupon.UserUniverse,
		UserID:       coupon.UserID,
		UserName:     coupon.UserName,
	}
}

func toGamePaymentActionIssue(issue *domaingame.PaymentActionIssue) *gamePaymentActionIssue {
	if issue == nil {
		return nil
	}
	return &gamePaymentActionIssue{Code: issue.Code, Message: issue.Message}
}

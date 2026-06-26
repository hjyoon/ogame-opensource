package game

const (
	PaymentIssueInvalidCoupon   = "invalid_coupon"
	PaymentIssueCouponValid     = "coupon_valid"
	PaymentIssueCouponActivated = "coupon_activated"
)

type Payment struct {
	Coupon *PaymentCoupon
}

type PaymentCoupon struct {
	ID           int
	Code         string
	Amount       int
	Used         bool
	UserUniverse int
	UserID       int
	UserName     string
}

type PaymentActionIssue struct {
	Code    string
	Message string
}

func PaymentIssue(code string) *PaymentActionIssue {
	switch code {
	case PaymentIssueInvalidCoupon:
		return &PaymentActionIssue{Code: code, Message: "Incorrect code or coupon already redeemed"}
	case PaymentIssueCouponValid:
		return &PaymentActionIssue{Code: code, Message: "Coupon is valid."}
	case PaymentIssueCouponActivated:
		return &PaymentActionIssue{Code: code, Message: "Coupon activated."}
	default:
		return &PaymentActionIssue{Code: code, Message: "Payment action could not be completed."}
	}
}

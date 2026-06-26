package game

import "testing"

func TestPaymentIssueMessages(t *testing.T) {
	tests := []struct {
		code    string
		message string
	}{
		{PaymentIssueInvalidCoupon, "Incorrect code or coupon already redeemed"},
		{PaymentIssueCouponValid, "Coupon is valid."},
		{PaymentIssueCouponActivated, "Coupon activated."},
		{"other", "Payment action could not be completed."},
	}
	for _, test := range tests {
		issue := PaymentIssue(test.code)
		if issue.Code != test.code || issue.Message != test.message {
			t.Fatalf("PaymentIssue(%q)=%+v", test.code, issue)
		}
	}
}

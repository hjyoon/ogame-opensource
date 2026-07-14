package mail

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestBuildAdminCouponMessageMatchesLegacyLocalization(t *testing.T) {
	for language, test := range map[string]struct {
		subject string
		wants   []string
	}{
		"en": {subject: "Present to you", wants: []string{"Dear Legor, you have present : TEST-CODE"}},
		"fr": {subject: "Un cadeau pour vous", wants: []string{"Cher/Chère Legor, vous avez un cadeau : TEST-CODE"}},
		"ru": {subject: "Купон в подарок", wants: []string{"Уважаемый Legor!", "TEST-CODE", "text/html; charset=UTF-8"}},
	} {
		subject, _ := adminCouponLocalizedContent(language, "Legor", "TEST-CODE")
		if subject != test.subject {
			t.Fatalf("%s subject=%q", language, subject)
		}
		message, err := BuildAdminCouponMessage(SMTPConfig{PublicBaseURL: "http://game.example.local:8890"}, domaingame.AdminCouponMail{
			Character: "Legor", Recipient: "legor@example.local", Language: language, Code: "TEST-CODE",
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range test.wants {
			if !strings.Contains(message, want) {
				t.Fatalf("%s message missing %q:\n%s", language, want, message)
			}
		}
		if !strings.Contains(message, "From: coupon@game.example.local") {
			t.Fatalf("unexpected sender: %s", message)
		}
	}
	if _, err := BuildAdminCouponMessage(SMTPConfig{}, domaingame.AdminCouponMail{Recipient: "bad address"}); err == nil {
		t.Fatal("expected invalid recipient")
	}
	if from := adminCouponFrom("://bad"); from != "coupon@localhost" {
		t.Fatalf("unexpected fallback sender: %s", from)
	}
}

func TestAdminCouponMailerSendsSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go serveOneSMTPMessage(listener, messages, serverErrors)
	err = NewAdminCouponMailer(SMTPConfig{Addr: listener.Addr().String(), PublicBaseURL: "http://game.test"}).SendAdminCoupon(context.Background(), domaingame.AdminCouponMail{
		Character: "Legor", Recipient: "legor@example.local", Language: "en", Code: "TEST-CODE",
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "TEST-CODE") || !strings.Contains(message, "legor@example.local") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestAdminCouponMailerErrors(t *testing.T) {
	coupon := domaingame.AdminCouponMail{Recipient: "user@example.local"}
	if err := NewAdminCouponMailer(SMTPConfig{}).SendAdminCoupon(context.Background(), coupon); err == nil || !strings.Contains(err.Error(), "SMTP address") {
		t.Fatalf("expected address error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewAdminCouponMailer(SMTPConfig{Addr: "127.0.0.1:1"}).SendAdminCoupon(ctx, coupon); err == nil {
		t.Fatal("expected context error")
	}
	if err := NewAdminCouponMailer(SMTPConfig{Addr: "127.0.0.1:1"}).SendAdminCoupon(context.Background(), domaingame.AdminCouponMail{Recipient: "bad address"}); err == nil {
		t.Fatal("expected invalid recipient error")
	}
}

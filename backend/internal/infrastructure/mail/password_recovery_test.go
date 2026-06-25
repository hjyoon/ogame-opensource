package mail

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestBuildPasswordRecoveryMessageMatchesLegacyShape(t *testing.T) {
	message, err := BuildPasswordRecoveryMessage(SMTPConfig{
		From:          "OGame <noreply@example.local>",
		PublicBaseURL: "http://game.example.local/",
	}, domain.PasswordRecoveryMail{
		Character:      "Legor",
		Email:          "legor@example.local",
		Password:       "abc123xy",
		UniverseNumber: 7,
	})
	if err != nil {
		t.Fatalf("BuildPasswordRecoveryMessage returned error: %v", err)
	}
	for _, want := range []string{
		"Subject: OGame password",
		"To: <legor@example.local>",
		"Hi Legor,",
		"your password for OGame Universe 7 is:",
		"abc123xy",
		"You may log in at http://game.example.local",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}

func TestPasswordRecoveryMailerSendsSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go serveOneSMTPMessage(listener, messages, serverErrors)

	err = NewPasswordRecoveryMailer(SMTPConfig{
		Addr: listener.Addr().String(),
		From: "OGame <noreply@example.local>",
	}).SendPasswordRecovery(context.Background(), domain.PasswordRecoveryMail{
		Character:      "Legor",
		Email:          "legor@example.local",
		Password:       "abc123xy",
		UniverseNumber: 1,
	})
	if err != nil {
		t.Fatalf("SendPasswordRecovery returned error: %v", err)
	}

	select {
	case message := <-messages:
		if !strings.Contains(message, "abc123xy") || !strings.Contains(message, "legor@example.local") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case err := <-serverErrors:
		t.Fatalf("SMTP server failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestPasswordRecoveryMailerErrors(t *testing.T) {
	if err := NewPasswordRecoveryMailer(SMTPConfig{}).SendPasswordRecovery(context.Background(), domain.PasswordRecoveryMail{Email: "user@example.local"}); err == nil || !strings.Contains(err.Error(), "SMTP address") {
		t.Fatalf("expected SMTP address error, got %v", err)
	}
	if _, err := BuildPasswordRecoveryMessage(SMTPConfig{}, domain.PasswordRecoveryMail{Email: "not an address"}); err == nil {
		t.Fatalf("expected invalid recipient error")
	}
	if _, err := BuildPasswordRecoveryMessage(SMTPConfig{From: "bad from"}, domain.PasswordRecoveryMail{Email: "user@example.local"}); err == nil {
		t.Fatalf("expected invalid from error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewPasswordRecoveryMailer(SMTPConfig{Addr: "127.0.0.1:1"}).SendPasswordRecovery(ctx, domain.PasswordRecoveryMail{Email: "user@example.local"}); err == nil {
		t.Fatalf("expected context error")
	}
}

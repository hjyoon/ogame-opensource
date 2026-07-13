package mail

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestBuildOptionsChangeMessageMatchesLegacyShape(t *testing.T) {
	message, err := BuildOptionsChangeMessage(SMTPConfig{
		From:          "OGame <noreply@example.local>",
		PublicBaseURL: "http://game.example.local/",
	}, 7, domaingame.OptionsChangeMail{
		Character:      "Legor",
		Recipient:      "permanent@example.local",
		PendingEmail:   "pending@example.local",
		ActivationCode: "activation-code",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Subject: Your in-game e-mail address has been changed ",
		"To: <permanent@example.local>",
		"Greetings Legor,",
		"account in the 7 universe",
		"pending@example.local",
		"http://game.example.local/game/validate.php?ack=activation-code",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}

func TestOptionsChangeMailerSendsSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go serveOneSMTPMessage(listener, messages, serverErrors)

	err = NewOptionsChangeMailer(SMTPConfig{
		Addr: listener.Addr().String(),
		From: "OGame <noreply@example.local>",
	}, 1).SendOptionsChange(context.Background(), domaingame.OptionsChangeMail{
		Character: "Legor", Recipient: "permanent@example.local", PendingEmail: "pending@example.local", ActivationCode: "code",
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "pending@example.local") || !strings.Contains(message, "permanent@example.local") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestOptionsChangeMailerErrors(t *testing.T) {
	change := domaingame.OptionsChangeMail{Recipient: "user@example.local"}
	if err := NewOptionsChangeMailer(SMTPConfig{}, 1).SendOptionsChange(context.Background(), change); err == nil || !strings.Contains(err.Error(), "SMTP address") {
		t.Fatalf("expected SMTP address error, got %v", err)
	}
	if _, err := BuildOptionsChangeMessage(SMTPConfig{}, 1, domaingame.OptionsChangeMail{Recipient: "bad address"}); err == nil {
		t.Fatal("expected invalid recipient error")
	}
	if _, err := BuildOptionsChangeMessage(SMTPConfig{From: "bad from"}, 1, change); err == nil {
		t.Fatal("expected invalid sender error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewOptionsChangeMailer(SMTPConfig{Addr: "127.0.0.1:1"}, 1).SendOptionsChange(ctx, change); err == nil {
		t.Fatal("expected context error")
	}
}

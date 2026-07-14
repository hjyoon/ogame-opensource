package mail

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestBuildAdminReactivationMessageMatchesWelcomeMail(t *testing.T) {
	message, err := BuildAdminReactivationMessage(SMTPConfig{PublicBaseURL: "http://game.test"}, domaingame.AdminReactivationMail{
		Character: "Legor", Password: "newpass", Recipient: "legor@example.local", ActivationCode: "ack", UniverseNumber: 7,
		Language: "en", BoardURL: "/board", TutorialURL: "/tutorial",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"To: <legor@example.local>", "Password: newpass", "Universe: 7", "game.test/game/validate.php?ack=ack", "forum (/board)", "Here (/tutorial)"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in message:\n%s", want, message)
		}
	}
}

func TestAdminReactivationMailerSendsSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go serveOneSMTPMessage(listener, messages, serverErrors)
	err = NewAdminReactivationMailer(SMTPConfig{Addr: listener.Addr().String(), PublicBaseURL: "http://game.test"}).SendAdminReactivation(context.Background(), domaingame.AdminReactivationMail{
		Character: "Legor", Password: "newpass", Recipient: "legor@example.local", ActivationCode: "ack", UniverseNumber: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "Password: newpass") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestAdminReactivationMailerErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewAdminReactivationMailer(SMTPConfig{}).SendAdminReactivation(ctx, domaingame.AdminReactivationMail{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context error, got %v", err)
	}
	mail := domaingame.AdminReactivationMail{Recipient: "legor@example.local"}
	if err := NewAdminReactivationMailer(SMTPConfig{}).SendAdminReactivation(context.Background(), mail); err == nil || !strings.Contains(err.Error(), "SMTP address") {
		t.Fatalf("expected SMTP address error, got %v", err)
	}
	if _, err := BuildAdminReactivationMessage(SMTPConfig{}, domaingame.AdminReactivationMail{Recipient: "bad address"}); err == nil {
		t.Fatal("expected invalid recipient error")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := NewAdminReactivationMailer(SMTPConfig{Addr: "127.0.0.1:1"}).SendAdminReactivation(ctx, mail); err == nil {
		t.Fatal("expected SMTP connection error")
	}
}

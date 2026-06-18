package mail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestBuildRegistrationWelcomeMessageMatchesLegacyActivationMailShape(t *testing.T) {
	message, err := BuildRegistrationWelcomeMessage(SMTPConfig{
		PublicBaseURL: "http://localhost:8890/",
	}, domain.RegistrationWelcomeMail{
		Character:      "Commander01",
		Password:       "E2E_http123",
		Email:          "pilot@example.local",
		ActivationCode: "abcdef123456",
		UniverseNumber: 7,
	})

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"From: OGame <noreply@localhost>",
		"To: <pilot@example.local>",
		"Subject: Welcome to OGame",
		"Click on this link to activate your account:",
		"http://localhost:8890/game/validate.php?ack=abcdef123456",
		"Player name: Commander01",
		"Password: E2E_http123",
		"Universe: 7",
		"Your OGame team",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected message to contain %q, got:\n%s", want, message)
		}
	}
	if strings.Contains(message, "\nFrom:") {
		t.Fatalf("unexpected header injection shape: %q", message)
	}
}

func TestActivationLinkFallsBackToLocalhost(t *testing.T) {
	link := ActivationLink("", " ack ")

	if link != "http://localhost:8890/game/validate.php?ack=ack" {
		t.Fatalf("unexpected activation link: %s", link)
	}
}

func TestRegistrationWelcomeMailerReturnsContextErrorBeforeSMTP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewRegistrationWelcomeMailer(SMTPConfig{}).SendRegistrationWelcome(ctx, domain.RegistrationWelcomeMail{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestRegistrationWelcomeMailerRequiresSMTPAddress(t *testing.T) {
	err := NewRegistrationWelcomeMailer(SMTPConfig{}).SendRegistrationWelcome(context.Background(), domain.RegistrationWelcomeMail{
		Email: "pilot@example.local",
	})

	if err == nil || !strings.Contains(err.Error(), "SMTP address") {
		t.Fatalf("expected SMTP address error, got %v", err)
	}
}

func TestRegistrationWelcomeMailerSendsSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go serveOneSMTPMessage(listener, messages, serverErrors)

	err = NewRegistrationWelcomeMailer(SMTPConfig{
		Addr:          listener.Addr().String(),
		From:          "OGame <noreply@example.local>",
		PublicBaseURL: "http://public.example.local",
	}).SendRegistrationWelcome(context.Background(), domain.RegistrationWelcomeMail{
		Character:      "Commander01",
		Password:       "E2E_http123",
		Email:          "pilot@example.local",
		ActivationCode: "abcdef123456",
		UniverseNumber: 7,
	})

	if err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "Password: E2E_http123") || !strings.Contains(message, "http://public.example.local/game/validate.php?ack=abcdef123456") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case err := <-serverErrors:
		t.Fatalf("SMTP server failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestBuildRegistrationWelcomeMessageRejectsInvalidAddress(t *testing.T) {
	_, err := BuildRegistrationWelcomeMessage(SMTPConfig{}, domain.RegistrationWelcomeMail{
		Email: "not an address",
	})

	if err == nil {
		t.Fatal("expected invalid address error")
	}
}

func TestBuildRegistrationWelcomeMessageRejectsInvalidFrom(t *testing.T) {
	_, err := BuildRegistrationWelcomeMessage(SMTPConfig{From: "not an address"}, domain.RegistrationWelcomeMail{
		Email: "pilot@example.local",
	})

	if err == nil {
		t.Fatal("expected invalid from address error")
	}
}

func serveOneSMTPMessage(listener net.Listener, messages chan<- string, errors chan<- error) {
	conn, err := listener.Accept()
	if err != nil {
		errors <- err
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeSMTPLine := func(line string) error {
		if _, err := fmt.Fprint(writer, line+"\r\n"); err != nil {
			return err
		}
		return writer.Flush()
	}
	if err := writeSMTPLine("220 localhost ESMTP"); err != nil {
		errors <- err
		return
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			errors <- err
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "EHLO ") || strings.HasPrefix(line, "HELO "):
			if err := writeSMTPLine("250-localhost"); err != nil {
				errors <- err
				return
			}
			if err := writeSMTPLine("250 OK"); err != nil {
				errors <- err
				return
			}
		case strings.HasPrefix(line, "MAIL FROM:"):
			if err := writeSMTPLine("250 sender ok"); err != nil {
				errors <- err
				return
			}
		case strings.HasPrefix(line, "RCPT TO:"):
			if err := writeSMTPLine("250 recipient ok"); err != nil {
				errors <- err
				return
			}
		case line == "DATA":
			if err := writeSMTPLine("354 end with dot"); err != nil {
				errors <- err
				return
			}
			var builder strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					errors <- err
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				builder.WriteString(dataLine)
			}
			messages <- builder.String()
			if err := writeSMTPLine("250 queued"); err != nil {
				errors <- err
				return
			}
		case line == "QUIT":
			_ = writeSMTPLine("221 bye")
			return
		default:
			errors <- fmt.Errorf("unexpected SMTP command %q", line)
			return
		}
	}
}

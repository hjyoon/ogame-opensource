package mail

import (
	"context"
	"errors"
	"fmt"
	netmail "net/mail"
	"net/smtp"
	"strings"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type SMTPConfig struct {
	Addr          string
	From          string
	PublicBaseURL string
}

type RegistrationWelcomeMailer struct {
	config SMTPConfig
}

func NewRegistrationWelcomeMailer(config SMTPConfig) RegistrationWelcomeMailer {
	return RegistrationWelcomeMailer{config: config}
}

func (m RegistrationWelcomeMailer) SendRegistrationWelcome(ctx context.Context, welcome domain.RegistrationWelcomeMail) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := strings.TrimSpace(m.config.Addr)
	if addr == "" {
		return errors.New("registration welcome SMTP address is required")
	}
	message, err := BuildRegistrationWelcomeMessage(m.config, welcome)
	if err != nil {
		return err
	}
	from, _ := netmail.ParseAddress(defaultFrom(m.config.From))
	to, _ := netmail.ParseAddress(strings.TrimSpace(welcome.Email))
	return smtp.SendMail(addr, nil, from.Address, []string{to.Address}, []byte(message))
}

func BuildRegistrationWelcomeMessage(config SMTPConfig, welcome domain.RegistrationWelcomeMail) (string, error) {
	to, err := netmail.ParseAddress(strings.TrimSpace(welcome.Email))
	if err != nil {
		return "", err
	}
	from := defaultFrom(config.From)
	if _, err := netmail.ParseAddress(from); err != nil {
		return "", err
	}
	link := ActivationLink(config.PublicBaseURL, welcome.ActivationCode)
	headers := []string{
		"From: " + cleanHeader(from),
		"To: " + cleanHeader(to.String()),
		"Subject: Welcome to OGame",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	body := fmt.Sprintf(
		"Greetings %s,\n\n"+
			"You've decided to create your own empire in OGame of the OGame universe!\n\n"+
			"Click on this link to activate your account:\n"+
			"%s\n\n"+
			"Your gaming credentials:\n"+
			"Player name: %s\n"+
			"Password: %s\n"+
			"Universe: %d\n\n\n"+
			"We wish you success in building your empire and good luck in the upcoming battles!\n\n"+
			"Your OGame team",
		strings.TrimSpace(welcome.Character),
		link,
		strings.TrimSpace(welcome.Character),
		welcome.Password,
		welcome.UniverseNumber,
	)
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"), nil
}

func ActivationLink(publicBaseURL string, activationCode string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base == "" {
		base = "http://localhost:8890"
	}
	return base + "/game/validate.php?ack=" + strings.TrimSpace(activationCode)
}

func defaultFrom(value string) string {
	from := strings.TrimSpace(value)
	if from == "" {
		return "OGame <noreply@localhost>"
	}
	return from
}

func cleanHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

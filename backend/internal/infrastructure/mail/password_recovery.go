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

type PasswordRecoveryMailer struct {
	config SMTPConfig
}

func NewPasswordRecoveryMailer(config SMTPConfig) PasswordRecoveryMailer {
	return PasswordRecoveryMailer{config: config}
}

func (m PasswordRecoveryMailer) SendPasswordRecovery(ctx context.Context, recovery domain.PasswordRecoveryMail) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := strings.TrimSpace(m.config.Addr)
	if addr == "" {
		return errors.New("password recovery SMTP address is required")
	}
	message, err := BuildPasswordRecoveryMessage(m.config, recovery)
	if err != nil {
		return err
	}
	from, _ := netmail.ParseAddress(defaultFrom(m.config.From))
	to, _ := netmail.ParseAddress(strings.TrimSpace(recovery.Email))
	return smtp.SendMail(addr, nil, from.Address, []string{to.Address}, []byte(message))
}

func BuildPasswordRecoveryMessage(config SMTPConfig, recovery domain.PasswordRecoveryMail) (string, error) {
	to, err := netmail.ParseAddress(strings.TrimSpace(recovery.Email))
	if err != nil {
		return "", err
	}
	from := defaultFrom(config.From)
	if _, err := netmail.ParseAddress(from); err != nil {
		return "", err
	}
	headers := []string{
		"From: " + cleanHeader(from),
		"To: " + cleanHeader(to.String()),
		"Subject: OGame password",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	body := fmt.Sprintf(
		"Hi %s,\n\n"+
			"your password for OGame Universe %d is:\n\n"+
			"%s\n\n"+
			"You may log in at %s with this login data.\n\n"+
			"We only send passwords to the E-Mail address entered in your account. Please ignore this email if you didn't request it.\n\n"+
			"We wish you good success while playing OGame!\n\n"+
			"Your OGame-Team",
		strings.TrimSpace(recovery.Character),
		recovery.UniverseNumber,
		recovery.Password,
		passwordRecoveryBaseURL(recovery.PublicBaseURL, config.PublicBaseURL),
	)
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"), nil
}

func passwordRecoveryBaseURL(publicBaseURL string, fallback string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(fallback), "/")
	}
	if base == "" {
		return "http://localhost:8890"
	}
	return base
}

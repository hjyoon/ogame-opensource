package mail

import (
	"context"
	"errors"
	"fmt"
	netmail "net/mail"
	"net/smtp"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type OptionsChangeMailer struct {
	config         SMTPConfig
	universeNumber int
}

func NewOptionsChangeMailer(config SMTPConfig, universeNumber int) OptionsChangeMailer {
	return OptionsChangeMailer{config: config, universeNumber: universeNumber}
}

func (m OptionsChangeMailer) SendOptionsChange(ctx context.Context, change domaingame.OptionsChangeMail) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := strings.TrimSpace(m.config.Addr)
	if addr == "" {
		return errors.New("options change SMTP address is required")
	}
	message, err := BuildOptionsChangeMessage(m.config, m.universeNumber, change)
	if err != nil {
		return err
	}
	from, _ := netmail.ParseAddress(defaultFrom(m.config.From))
	to, _ := netmail.ParseAddress(strings.TrimSpace(change.Recipient))
	return smtp.SendMail(addr, nil, from.Address, []string{to.Address}, []byte(message))
}

func BuildOptionsChangeMessage(config SMTPConfig, universeNumber int, change domaingame.OptionsChangeMail) (string, error) {
	to, err := netmail.ParseAddress(strings.TrimSpace(change.Recipient))
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
		"Subject: Your in-game e-mail address has been changed ",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	body := fmt.Sprintf(
		"Greetings %s,\n\n"+
			"The temporary e-mail address of your account in the %d universe has been changed in the settings to %s.\n"+
			"If you don't change it within a week, it will become permanent.\n\n"+
			"Confirm your new e-mail address using the following link to continue playing without any problems:\n\n"+
			"%s\n\n"+
			"Your OGame team",
		strings.TrimSpace(change.Character),
		universeNumber,
		strings.TrimSpace(change.PendingEmail),
		ActivationLinkForRequest(config.PublicBaseURL, change.PublicBaseURL, change.ActivationCode),
	)
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"), nil
}

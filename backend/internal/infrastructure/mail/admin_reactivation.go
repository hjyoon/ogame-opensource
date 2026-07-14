package mail

import (
	"context"
	"errors"
	netmail "net/mail"
	"net/smtp"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type AdminReactivationMailer struct {
	config SMTPConfig
}

func NewAdminReactivationMailer(config SMTPConfig) AdminReactivationMailer {
	return AdminReactivationMailer{config: config}
}

func (m AdminReactivationMailer) SendAdminReactivation(ctx context.Context, reactivation domaingame.AdminReactivationMail) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := strings.TrimSpace(m.config.Addr)
	if addr == "" {
		return errors.New("admin reactivation SMTP address is required")
	}
	message, err := BuildAdminReactivationMessage(m.config, reactivation)
	if err != nil {
		return err
	}
	from, _ := netmail.ParseAddress(defaultFrom(m.config.From))
	to, _ := netmail.ParseAddress(strings.TrimSpace(reactivation.Recipient))
	return smtp.SendMail(addr, nil, from.Address, []string{to.Address}, []byte(message))
}

func BuildAdminReactivationMessage(config SMTPConfig, reactivation domaingame.AdminReactivationMail) (string, error) {
	return BuildRegistrationWelcomeMessage(config, domainpublicsite.RegistrationWelcomeMail{
		Character:      reactivation.Character,
		Password:       reactivation.Password,
		Email:          reactivation.Recipient,
		ActivationCode: reactivation.ActivationCode,
		UniverseNumber: reactivation.UniverseNumber,
		Language:       reactivation.Language,
		BoardURL:       reactivation.BoardURL,
		TutorialURL:    reactivation.TutorialURL,
	})
}

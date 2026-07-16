package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	netmail "net/mail"
	"net/smtp"
	"net/url"
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
	link := ActivationLinkForRequest(config.PublicBaseURL, welcome.PublicBaseURL, welcome.ActivationCode)
	headers := []string{
		"From: " + cleanHeader(from),
		"To: " + cleanHeader(to.String()),
		"Subject: Welcome to OGame",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	body := fmt.Sprintf(
		"Greetings %s,\n\n"+
			"You've decided to create your own empire in %d of the OGame universe!\n\n"+
			"Click on this link to activate your account:\n"+
			"%s\n\n"+
			"Your gaming credentials:\n"+
			"Player name: %s\n"+
			"Password: %s\n"+
			"Universe: %d\n\n\n",
		strings.TrimSpace(welcome.Character),
		welcome.UniverseNumber,
		link,
		strings.TrimSpace(welcome.Character),
		welcome.Password,
		welcome.UniverseNumber,
	)
	if boardURL := strings.TrimSpace(welcome.BoardURL); boardURL != "" {
		body += fmt.Sprintf("If you need help or advice from other emperors, you can find it all in our forum (%s).\n\n", boardURL)
	}
	if tutorialURL := strings.TrimSpace(welcome.TutorialURL); tutorialURL != "" {
		body += fmt.Sprintf("Here (%s) is all the information gathered by players and team members to help newcomers understand the game as quickly as possible.\n\n", tutorialURL)
	}
	body += "We wish you success in building your empire and good luck in the upcoming battles!\n\nYour OGame team"
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"), nil
}

func ActivationLink(publicBaseURL string, activationCode string) string {
	return ActivationLinkForRequest(publicBaseURL, "", activationCode)
}

func ActivationLinkForRequest(configuredBaseURL string, requestBaseURL string, activationCode string) string {
	base := activationBaseURL(configuredBaseURL, requestBaseURL)
	return base + "/game/validate.php?ack=" + strings.TrimSpace(activationCode)
}

func activationBaseURL(configuredBaseURL string, requestBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(configuredBaseURL), "/")
	requestBase := strings.TrimRight(strings.TrimSpace(requestBaseURL), "/")
	if requestBase != "" && (base == "" || loopbackBaseURL(base)) {
		return requestBase
	}
	if base == "" {
		base = "http://localhost:8890"
	}
	return base
}

func loopbackBaseURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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

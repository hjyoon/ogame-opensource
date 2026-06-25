package publicsite

import (
	"net/mail"
	"strings"
)

type PasswordRecoveryCommand struct {
	Email string
}

type PasswordRecoveryAccount struct {
	Found          bool
	PlayerID       int
	Character      string
	PermanentEmail string
}

type PasswordRecoveryResult struct {
	Submitted bool
	Sent      bool
	Account   PasswordRecoveryAccount
}

type PasswordRecoveryMail struct {
	Character      string
	Email          string
	Password       string
	UniverseNumber int
	PublicBaseURL  string
}

func NormalizeRecoveryEmail(value string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" {
		return "", false
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", false
	}
	return email, true
}

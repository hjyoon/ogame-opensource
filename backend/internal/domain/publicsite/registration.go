package publicsite

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

const (
	RegistrationIssueTermsRequired      = "terms_required"
	RegistrationIssuePasswordTooShort   = "password_too_short"
	RegistrationIssueCharacterInvalid   = "character_invalid"
	RegistrationIssueCharacterExists    = "character_exists"
	RegistrationIssueEmailInvalid       = "email_invalid"
	RegistrationIssueEmailExists        = "email_exists"
	RegistrationIssueUniverseRequired   = "universe_required"
	RegistrationIssueUniverseFull       = "universe_full"
	LegacyRegistrationErrorTerms        = 204
	LegacyRegistrationErrorUserExists   = 101
	LegacyRegistrationErrorEmailExists  = 102
	LegacyRegistrationErrorCharacter    = 103
	LegacyRegistrationErrorEmail        = 104
	LegacyRegistrationErrorPassword     = 107
	LegacyRegistrationErrorMaxPlayers   = 109
	LegacyRegistrationErrorNoEquivalent = 0
)

type RegistrationDraft struct {
	Character     string
	Password      string
	Email         string
	Universe      string
	TermsAccepted bool
}

type RegistrationIssue struct {
	Field           string
	Code            string
	Message         string
	LegacyErrorCode int
}

type RegistrationValidation struct {
	Valid  bool
	Issues []RegistrationIssue
}

type RegisteredAccount struct {
	PlayerID       int
	HomePlanetID   int
	ActivationCode string
	Validated      bool
}

type RegistrationWelcomeMail struct {
	Character      string
	Password       string
	Email          string
	ActivationCode string
	UniverseNumber int
}

type ActivatedAccount struct {
	Found    bool
	PlayerID int
}

type RegistrationCreation struct {
	Valid   bool
	Issues  []RegistrationIssue
	Account RegisteredAccount
	Session LoginSession
}

type RegistrationActivation struct {
	Activated bool
	Account   ActivatedAccount
	Session   LoginSession
}

type RegistrationAvailability struct {
	CharacterExists bool
	EmailExists     bool
	UserCount       int
	MaxUsers        int
}

func (d RegistrationDraft) Validate() RegistrationValidation {
	issues := make([]RegistrationIssue, 0)

	if !d.TermsAccepted {
		issues = append(issues, RegistrationIssue{
			Field:           "agb",
			Code:            RegistrationIssueTermsRequired,
			Message:         "Basic policies must be accepted.",
			LegacyErrorCode: LegacyRegistrationErrorTerms,
		})
	}
	if utf8.RuneCountInString(d.Password) < 8 {
		issues = append(issues, RegistrationIssue{
			Field:           "password",
			Code:            RegistrationIssuePasswordTooShort,
			Message:         "Password must contain at least 8 characters.",
			LegacyErrorCode: LegacyRegistrationErrorPassword,
		})
	}
	if invalidCharacter(d.Character) {
		issues = append(issues, RegistrationIssue{
			Field:           "character",
			Code:            RegistrationIssueCharacterInvalid,
			Message:         "Commander name contains invalid characters or too few/many characters.",
			LegacyErrorCode: LegacyRegistrationErrorCharacter,
		})
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(d.Email)); err != nil {
		issues = append(issues, RegistrationIssue{
			Field:           "email",
			Code:            RegistrationIssueEmailInvalid,
			Message:         "Email address is invalid.",
			LegacyErrorCode: LegacyRegistrationErrorEmail,
		})
	}
	if strings.TrimSpace(d.Universe) == "" {
		issues = append(issues, RegistrationIssue{
			Field:           "universe",
			Code:            RegistrationIssueUniverseRequired,
			Message:         "Universe selection is required.",
			LegacyErrorCode: LegacyRegistrationErrorNoEquivalent,
		})
	}

	return RegistrationValidation{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}

func (a RegistrationAvailability) Validate() []RegistrationIssue {
	issues := make([]RegistrationIssue, 0)
	if a.CharacterExists {
		issues = append(issues, RegistrationIssue{
			Field:           "character",
			Code:            RegistrationIssueCharacterExists,
			Message:         "Commander name already exists.",
			LegacyErrorCode: LegacyRegistrationErrorUserExists,
		})
	}
	if a.EmailExists {
		issues = append(issues, RegistrationIssue{
			Field:           "email",
			Code:            RegistrationIssueEmailExists,
			Message:         "Email address already exists.",
			LegacyErrorCode: LegacyRegistrationErrorEmailExists,
		})
	}
	if a.MaxUsers > 0 && a.UserCount >= a.MaxUsers {
		issues = append(issues, RegistrationIssue{
			Field:           "universe",
			Code:            RegistrationIssueUniverseFull,
			Message:         "The maximum number of players has been reached.",
			LegacyErrorCode: LegacyRegistrationErrorMaxPlayers,
		})
	}
	return issues
}

func invalidCharacter(name string) bool {
	value := strings.TrimSpace(name)
	length := utf8.RuneCountInString(value)
	if length < 3 || length > 20 {
		return true
	}
	if strings.ContainsAny(value, ";,<>()`\"'") {
		return true
	}

	lower := strings.ToLower(value)
	for _, forbidden := range forbiddenLoginFragments {
		if strings.Contains(lower, forbidden) {
			return true
		}
	}
	return false
}

var forbiddenLoginFragments = []string{
	"adolf",
	"hitler",
	"fick",
	"legor",
	"aleena",
	"ogame",
	"kkk",
	"osama",
	"bin",
	"laden",
	"porn",
	"sex",
	"hentai",
	"god",
	"allah",
	"putin",
	"nazi",
	"gameforge",
	"stalin",
	"goebbels",
	"saddam",
	"space",
	"admin",
}

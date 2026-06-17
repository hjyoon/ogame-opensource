package publicsite

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	LoginIssueLoginRequired      = "login_required"
	LoginIssuePasswordRequired   = "password_required"
	LoginIssueUniverseRequired   = "universe_required"
	LoginIssueCredentialsInvalid = "credentials_invalid"
	LoginIssueUserBanned         = "user_banned"
	LegacyLoginErrorCredentials  = 2
	LegacyLoginErrorBanned       = 3
	LegacyLoginErrorNoEquivalent = 0
)

type LoginDraft struct {
	Login    string
	Password string
	Universe string
}

type LoginIssue struct {
	Field           string
	Code            string
	Message         string
	LegacyErrorCode int
}

type LoginValidation struct {
	Valid  bool
	Issues []LoginIssue
}

type LoginAuthentication struct {
	Valid   bool
	Issues  []LoginIssue
	Session LoginSession
}

type LoginCredentials struct {
	Authenticated bool
	PlayerID      int
	Banned        bool
	BannedUntil   int
}

type LoginSession struct {
	PlayerID       int
	PublicID       string
	PrivateID      string
	UniverseNumber int
	LastLogin      int64
	RedirectPath   string
}

func (d LoginDraft) Validate() LoginValidation {
	issues := make([]LoginIssue, 0)

	if strings.TrimSpace(d.Login) == "" {
		issues = append(issues, LoginIssue{
			Field:           "login",
			Code:            LoginIssueLoginRequired,
			Message:         "Commander login is required.",
			LegacyErrorCode: LegacyLoginErrorCredentials,
		})
	}
	if d.Password == "" {
		issues = append(issues, LoginIssue{
			Field:           "pass",
			Code:            LoginIssuePasswordRequired,
			Message:         "Password is required.",
			LegacyErrorCode: LegacyLoginErrorCredentials,
		})
	}
	if strings.TrimSpace(d.Universe) == "" {
		issues = append(issues, LoginIssue{
			Field:           "universe",
			Code:            LoginIssueUniverseRequired,
			Message:         "Universe selection is required.",
			LegacyErrorCode: LegacyLoginErrorNoEquivalent,
		})
	}

	return LoginValidation{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}

func (c LoginCredentials) Validate() []LoginIssue {
	if !c.Authenticated {
		return []LoginIssue{{
			Field:           "login",
			Code:            LoginIssueCredentialsInvalid,
			Message:         "Commander name or password is invalid.",
			LegacyErrorCode: LegacyLoginErrorCredentials,
		}}
	}
	if c.Banned {
		return []LoginIssue{{
			Field:           "login",
			Code:            LoginIssueUserBanned,
			Message:         "Commander account is banned.",
			LegacyErrorCode: LegacyLoginErrorBanned,
		}}
	}
	return nil
}

func (s LoginSession) PrivateCookieName() string {
	return fmt.Sprintf("prsess_%d_%d", s.PlayerID, s.UniverseNumber)
}

func (s LoginSession) RedirectTarget() string {
	path := strings.TrimSpace(s.RedirectPath)
	if path == "" {
		path = "/game/overview"
	}
	values := url.Values{}
	values.Set("session", s.PublicID)
	values.Set("lgn", "1")
	return path + "?" + values.Encode()
}

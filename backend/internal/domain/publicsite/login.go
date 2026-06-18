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
	SessionIssueRequired         = "session_required"
	SessionIssueInvalid          = "session_invalid"
	SessionIssuePrivateInvalid   = "private_session_invalid"
	SessionIssueIPMismatch       = "ip_mismatch"
	SessionIssueBanned           = "session_user_banned"
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

type SessionAuthentication struct {
	Authenticated bool
	Issues        []SessionIssue
	Session       GameSession
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

type GameSession struct {
	Found          bool
	PlayerID       int
	Commander      string
	PublicID       string
	PrivateID      string
	IPAddress      string
	DisableIPCheck bool
	Banned         bool
	BannedUntil    int
	HomePlanetID   int
	UniverseNumber int
}

type SessionIssue struct {
	Code        string
	Message     string
	BannedUntil int
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
	return PrivateSessionCookieName(s.PlayerID, s.UniverseNumber)
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

func (s GameSession) PrivateCookieName() string {
	return PrivateSessionCookieName(s.PlayerID, s.UniverseNumber)
}

func (s GameSession) Validate(privateSession string, remoteIP string) []SessionIssue {
	if !s.Found {
		return []SessionIssue{{Code: SessionIssueInvalid, Message: "Session is invalid."}}
	}
	if privateSession == "" || privateSession != s.PrivateID {
		return []SessionIssue{{Code: SessionIssuePrivateInvalid, Message: "Private session is invalid."}}
	}
	if s.Banned {
		return []SessionIssue{{Code: SessionIssueBanned, Message: "Commander account is banned.", BannedUntil: s.BannedUntil}}
	}
	if !s.DisableIPCheck && !legacyLocalhost(remoteIP) && remoteIP != s.IPAddress {
		return []SessionIssue{{Code: SessionIssueIPMismatch, Message: "Session IP address does not match."}}
	}
	return nil
}

func PrivateSessionCookieName(playerID int, universeNumber int) string {
	return fmt.Sprintf("prsess_%d_%d", playerID, universeNumber)
}

func legacyLocalhost(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1"
}

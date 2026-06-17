package publicsite

import "strings"

const (
	LoginIssueLoginRequired      = "login_required"
	LoginIssuePasswordRequired   = "password_required"
	LoginIssueUniverseRequired   = "universe_required"
	LegacyLoginErrorCredentials  = 2
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

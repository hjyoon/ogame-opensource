package game

import (
	"net"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	OptionsIssueSaved                 = "saved"
	OptionsIssueAccountDeletionQueued = "account_deletion_queued"
	OptionsIssueAccountDeletionClear  = "account_deletion_cleared"
	OptionsIssueVacationEnabled       = "vacation_enabled"
	OptionsIssueVacationDisabled      = "vacation_disabled"
	OptionsIssueVacationBlocked       = "vacation_blocked"
	OptionsIssueVacationLocked        = "vacation_locked"
	OptionsIssuePasswordChanged       = "password_changed"
	OptionsIssuePasswordMismatch      = "password_mismatch"
	OptionsIssuePasswordSpecial       = "password_special"
	OptionsIssuePasswordTooShort      = "password_too_short"
	OptionsIssuePasswordWrongOld      = "password_wrong_old"
	OptionsIssueEmailChanged          = "email_changed"
	OptionsIssueEmailNeedPassword     = "email_need_password"
	OptionsIssueEmailInvalid          = "email_invalid"
	OptionsIssueEmailUsed             = "email_used"
	OptionsIssueNameChanged           = "name_changed"
	OptionsIssueNameExists            = "name_exists"
	OptionsIssueNameCooldown          = "name_cooldown"
	OptionsIssueNameLength            = "name_length"
	OptionsIssueNameSpecial           = "name_special"
	OptionsIssueNameForbidden         = "name_forbidden"
	OptionsIssueFeedProhibited        = "feed_prohibited"
	OptionsIssueActivationResent      = "activation_resent"

	UserTypePlayer = 0
	UserTypeGO     = 1

	userFlagShowEspionageButton = 0x1
	userFlagShowWriteMessage    = 0x2
	userFlagShowBuddy           = 0x4
	userFlagShowRocketAttack    = 0x8
	userFlagShowViewReport      = 0x10
	userFlagDoNotUseFolders     = 0x20
	UserFlagHideGOEmail         = 0x4000
	userFlagFeedEnable          = 0x8000
	userFlagFeedAtom            = 0x10000
)

type Options struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	User           OptionsUser
	Universe       OptionsUniverse
	Settings       OptionsSettings
	Account        OptionsAccount
	Flags          OptionsFlags
	LegacyFlags    int64
	OutboundMail   *OptionsChangeMail `json:"-"`
}

type OptionsUser struct {
	Name           string
	NameLocked     bool
	Email          string
	PlainEmail     string
	Validated      bool
	Admin          int
	FeedID         string
	CommanderOn    bool
	PasswordHash   string
	ValidationCode string
}

type OptionsChangeMail struct {
	Character      string
	Recipient      string
	PendingEmail   string
	ActivationCode string
	PublicBaseURL  string
}

type OptionsUniverse struct {
	Language      string
	ForceLanguage bool
	FeedAge       int
	Speed         int
}

type OptionsSettings struct {
	Language         string
	SkinPath         string
	UseSkin          bool
	DeactivateIP     bool
	SortBy           int
	SortOrder        int
	MaxSpy           int
	MaxFleetMessages int
}

type OptionsAccount struct {
	Vacation       bool
	VacationUntil  int64
	DeletionQueued bool
	DeletionAt     int64
}

type OptionsFlags struct {
	ShowEspionageButton bool
	ShowWriteMessage    bool
	ShowBuddy           bool
	ShowRocketAttack    bool
	ShowViewReport      bool
	DoNotUseFolders     bool
	FeedEnabled         bool
	FeedAtom            bool
	HideGOEmail         bool
}

type OptionsMutation struct {
	Name                string
	Language            string
	SkinPath            string
	UseSkin             bool
	DeactivateIP        bool
	SortBy              int
	SortOrder           int
	MaxSpy              int
	MaxFleetMessages    int
	OldPassword         string
	NewPassword         string
	NewPasswordRepeat   string
	Email               string
	VacationMode        bool
	VacationModeSet     bool
	DisableVacation     bool
	DeleteAccount       bool
	ShowEspionageButton bool
	ShowWriteMessage    bool
	ShowBuddy           bool
	ShowRocketAttack    bool
	ShowViewReport      bool
	DoNotUseFolders     bool
	FeedEnabled         bool
	FeedType            string
	HideGOEmail         bool
	ResendActivation    bool
}

type NormalizedOptionsMutation struct {
	OptionsMutation
	AccountDeletionChanged bool
	VacationChanged        bool
}

type OptionsActionIssue struct {
	Code      string
	Message   string
	Timestamp int64
}

func NewOptions(overview Overview, user OptionsUser, universe OptionsUniverse, settings OptionsSettings, account OptionsAccount, rawFlags int64) Options {
	settings.SkinPath = NormalizeSkinPath(settings.SkinPath, "", 0)
	settings.SortBy = clampInt(settings.SortBy, 0, 2)
	settings.SortOrder = clampInt(settings.SortOrder, 0, 1)
	settings.MaxSpy = clampInt(settings.MaxSpy, 1, 99)
	settings.MaxFleetMessages = clampInt(settings.MaxFleetMessages, 1, 99)
	if universe.ForceLanguage {
		settings.Language = normalizeLanguage(universe.Language, universe.Language)
	} else {
		settings.Language = normalizeLanguage(settings.Language, universe.Language)
	}
	if universe.Speed <= 0 {
		universe.Speed = 1
	}
	return Options{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		User:           user,
		Universe:       universe,
		Settings:       settings,
		Account:        account,
		Flags:          OptionsFlagsFromLegacy(rawFlags),
		LegacyFlags:    rawFlags,
	}
}

func NormalizeOptionsMutation(command OptionsMutation, current Options) NormalizedOptionsMutation {
	normalized := command
	if normalized.Name == "" {
		normalized.Name = current.User.Name
	}
	normalized.SkinPath = NormalizeSkinPath(command.SkinPath, "", 0)
	normalized.SortBy = clampInt(command.SortBy, 0, 2)
	normalized.SortOrder = clampInt(command.SortOrder, 0, 1)
	normalized.MaxSpy = clampInt(command.MaxSpy, 1, 99)
	normalized.MaxFleetMessages = clampInt(command.MaxFleetMessages, 1, 99)
	if current.Universe.ForceLanguage {
		normalized.Language = normalizeLanguage(current.Universe.Language, current.Universe.Language)
	} else {
		normalized.Language = normalizeLanguage(command.Language, current.Universe.Language)
	}
	vacationChanged := false
	if command.VacationModeSet {
		vacationChanged = command.VacationMode != current.Account.Vacation
	} else {
		normalized.VacationMode = current.Account.Vacation
	}
	return NormalizedOptionsMutation{
		OptionsMutation:        normalized,
		AccountDeletionChanged: command.DeleteAccount != current.Account.DeletionQueued,
		VacationChanged:        vacationChanged,
	}
}

func (m OptionsMutation) NameChangeRequested(current Options) bool {
	return !current.User.NameLocked && m.Name != "" && m.Name != current.User.Name
}

func (m OptionsMutation) NameValidationIssue() *OptionsActionIssue {
	var issue *OptionsActionIssue
	switch length := utf8.RuneCountInString(m.Name); {
	case length < 3 || length > 20:
		issue = OptionsNameLengthIssue()
	case strings.ContainsAny(m.Name, "<>()[]{}\\/`\"'.,:;*+"):
		issue = OptionsNameSpecialIssue()
	}
	lower := strings.ToLower(m.Name)
	for _, fragment := range strings.Split("adolf,hitler,fick,legor,aleena,ogame,kkk,osama,bin,laden,porn,sex,hentai,god,allah,putin,nazi,gameforge,stalin,goebbels,saddam,space,admin", ",") {
		if strings.Contains(lower, fragment) {
			return OptionsNameForbiddenIssue()
		}
	}
	return issue
}

func (m OptionsMutation) PasswordChangeRequested() bool {
	return m.NewPassword != ""
}

func (m OptionsMutation) EmailChangeRequested(current Options) bool {
	email := strings.TrimSpace(m.Email)
	if email == "" {
		return false
	}
	return email != current.User.PlainEmail
}

func (m OptionsMutation) PasswordValidationIssue() *OptionsActionIssue {
	if !m.PasswordChangeRequested() {
		return nil
	}
	switch {
	case m.NewPassword != m.NewPasswordRepeat:
		return OptionsPasswordMismatchIssue()
	case !legacyPasswordCharacters(m.NewPassword):
		return OptionsPasswordSpecialIssue()
	case len(m.NewPassword) < 8:
		return OptionsPasswordTooShortIssue()
	default:
		return nil
	}
}

func (m OptionsMutation) EmailValidationIssue(current Options) *OptionsActionIssue {
	if !m.EmailChangeRequested(current) {
		return nil
	}
	email := strings.TrimSpace(m.Email)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" {
		return OptionsEmailInvalidIssue()
	}
	return nil
}

func legacyPasswordCharacters(password string) bool {
	if password == "" {
		return true
	}
	for _, ch := range password {
		if ch == '_' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			continue
		}
		return false
	}
	return true
}

func OptionsFlagsFromLegacy(flags int64) OptionsFlags {
	return OptionsFlags{
		ShowEspionageButton: flags&userFlagShowEspionageButton != 0,
		ShowWriteMessage:    flags&userFlagShowWriteMessage != 0,
		ShowBuddy:           flags&userFlagShowBuddy != 0,
		ShowRocketAttack:    flags&userFlagShowRocketAttack != 0,
		ShowViewReport:      flags&userFlagShowViewReport != 0,
		DoNotUseFolders:     flags&userFlagDoNotUseFolders != 0,
		FeedEnabled:         flags&userFlagFeedEnable != 0,
		FeedAtom:            flags&userFlagFeedAtom != 0,
		HideGOEmail:         flags&UserFlagHideGOEmail != 0,
	}
}

func ApplyOptionsFlagMutation(rawFlags int64, mutation OptionsMutation, current Options) (int64, bool, *OptionsActionIssue) {
	flags := rawFlags
	feedChanged := false
	var issue *OptionsActionIssue
	if current.User.CommanderOn {
		flags = setLegacyFlag(flags, userFlagShowEspionageButton, mutation.ShowEspionageButton)
		flags = setLegacyFlag(flags, userFlagShowWriteMessage, mutation.ShowWriteMessage)
		flags = setLegacyFlag(flags, userFlagShowBuddy, mutation.ShowBuddy)
		flags = setLegacyFlag(flags, userFlagShowRocketAttack, mutation.ShowRocketAttack)
		flags = setLegacyFlag(flags, userFlagShowViewReport, mutation.ShowViewReport)
		flags = setLegacyFlag(flags, userFlagDoNotUseFolders, mutation.DoNotUseFolders)

		wasFeedEnabled := rawFlags&userFlagFeedEnable != 0
		if wasFeedEnabled && mutation.FeedType != "" {
			flags = setLegacyFlag(flags, userFlagFeedAtom, mutation.FeedType == "atom")
		}
		if current.Universe.FeedAge < 0 && mutation.FeedEnabled && !wasFeedEnabled {
			issue = OptionsFeedProhibitedIssue()
		} else if mutation.FeedEnabled != wasFeedEnabled {
			flags = setLegacyFlag(flags, userFlagFeedEnable, mutation.FeedEnabled)
			feedChanged = true
		}
	}
	if current.User.Admin == UserTypeGO {
		flags = setLegacyFlag(flags, UserFlagHideGOEmail, mutation.HideGOEmail)
	}
	return flags, feedChanged, issue
}

func setLegacyFlag(flags int64, flag int64, enabled bool) int64 {
	if enabled {
		return flags | flag
	}
	return flags &^ flag
}

func NormalizeSkinPath(skin string, requestHost string, requestPort int) string {
	skin = strings.TrimSpace(skin)
	if skin == "" {
		return "/evolution/"
	}
	parsed, err := url.Parse(skin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return skin
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return skin
	}
	host := normalizeHost(parsed.Hostname())
	if !loopbackHost(host) && !sameOrigin(parsed, requestHost, requestPort) {
		return skin
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	return strings.TrimRight(path, "/") + "/"
}

func OptionsSavedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueSaved, Message: "Options saved."}
}

func OptionsAccountDeletionQueuedIssue(until time.Time) *OptionsActionIssue {
	message := "Your account was set for deletion."
	var timestamp int64
	if !until.IsZero() {
		message += " Deletion date: " + until.UTC().Format("2006-01-02 15:04:05")
		timestamp = until.Unix()
	}
	return &OptionsActionIssue{Code: OptionsIssueAccountDeletionQueued, Message: message, Timestamp: timestamp}
}

func OptionsAccountDeletionClearedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueAccountDeletionClear, Message: "Account deletion cancelled."}
}

func OptionsVacationEnabledIssue(until time.Time) *OptionsActionIssue {
	message := "Vacation mode enabled."
	var timestamp int64
	if !until.IsZero() {
		message += " Minimum until: " + until.UTC().Format("2006-01-02 15:04:05")
		timestamp = until.Unix()
	}
	return &OptionsActionIssue{Code: OptionsIssueVacationEnabled, Message: message, Timestamp: timestamp}
}

func OptionsVacationDisabledIssue(name string) *OptionsActionIssue {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Commander"
	}
	return &OptionsActionIssue{Code: OptionsIssueVacationDisabled, Message: "Welcome back from vacation " + name + ". Don't forget to increase your resource production again. Have fun with OGame."}
}

func OptionsVacationBlockedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueVacationBlocked, Message: "You can't enter vacation mode while something is being built."}
}

func OptionsVacationLockedIssue(until time.Time) *OptionsActionIssue {
	message := "Vacation mode cannot be disabled yet."
	var timestamp int64
	if !until.IsZero() {
		message += " Minimum until: " + until.UTC().Format("2006-01-02 15:04:05")
		timestamp = until.Unix()
	}
	return &OptionsActionIssue{Code: OptionsIssueVacationLocked, Message: message, Timestamp: timestamp}
}

func OptionsPasswordChangedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssuePasswordChanged, Message: "Password has been changed"}
}

func OptionsPasswordMismatchIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssuePasswordMismatch, Message: "The new passwords don't match"}
}

func OptionsPasswordSpecialIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssuePasswordSpecial, Message: "Invalid special characters in password."}
}

func OptionsPasswordTooShortIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssuePasswordTooShort, Message: "Password must contain at least eight characters"}
}

func OptionsPasswordWrongOldIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssuePasswordWrongOld, Message: "Incorrect old password"}
}

func OptionsEmailChangedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueEmailChanged, Message: "Your email address has been changed. This address will be permanent if no change is made in seven days."}
}

func OptionsEmailNeedPasswordIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueEmailNeedPassword, Message: "You need to enter your password to change the accounts E-Mail address."}
}

func OptionsEmailInvalidIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueEmailInvalid, Message: "No valid email address"}
}

func OptionsEmailUsedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueEmailUsed, Message: "This email address is already in use!"}
}

func OptionsNameChangedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameChanged, Message: "Username changed. This is permitted only once per week. Please log in again."}
}

func OptionsNameExistsIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameExists, Message: "This user name is already taken."}
}

func OptionsNameCooldownIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameCooldown, Message: "The username can only be changed once every seven days."}
}

func OptionsNameLengthIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameLength, Message: "Player's name must be between 3 and 20 characters."}
}

func OptionsNameSpecialIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameSpecial, Message: "The user name may not contain special characters."}
}

func OptionsNameForbiddenIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueNameForbidden, Message: "Forbidden user name!"}
}

func OptionsFeedProhibitedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueFeedProhibited, Message: "Feed is prohibited by Universe settings!"}
}

func OptionsActivationResentIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueActivationResent, Message: "The activation email has been sent again."}
}

func normalizeLanguage(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len(value) > 2 {
		value = value[:2]
	}
	if !supportedLegacyLanguage(value) {
		value = strings.TrimSpace(fallback)
		if len(value) > 2 {
			value = value[:2]
		}
		if !supportedLegacyLanguage(value) {
			value = "en"
		}
	}
	return value
}

func supportedLegacyLanguage(value string) bool {
	switch value {
	case "de", "en", "es", "fr", "it", "jp", "ru":
		return true
	default:
		return false
	}
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func sameOrigin(parsed *url.URL, requestHost string, requestPort int) bool {
	if requestHost == "" {
		return false
	}
	host := normalizeHost(parsed.Hostname())
	if host != normalizeHost(requestHost) {
		return false
	}
	port := parsed.Port()
	parsedPort := defaultSkinPort(parsed.Scheme)
	if port != "" {
		if converted, err := strconv.Atoi(port); err == nil {
			parsedPort = converted
		}
	}
	return parsedPort == requestPort
}

func defaultSkinPort(scheme string) int {
	if strings.EqualFold(scheme, "https") {
		return 443
	}
	return 80
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.Trim(host, "[]"))
}

func loopbackHost(host string) bool {
	host = normalizeHost(host)
	if host == "localhost" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

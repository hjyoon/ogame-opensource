package game

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	OptionsIssueSaved                 = "saved"
	OptionsIssueAccountDeletionQueued = "account_deletion_queued"
	OptionsIssueAccountDeletionClear  = "account_deletion_cleared"

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
}

type OptionsUser struct {
	Name        string
	NameLocked  bool
	Email       string
	PlainEmail  string
	Validated   bool
	Admin       int
	FeedID      string
	CommanderOn bool
}

type OptionsUniverse struct {
	Language      string
	ForceLanguage bool
	FeedAge       int
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
	Language         string
	SkinPath         string
	UseSkin          bool
	DeactivateIP     bool
	SortBy           int
	SortOrder        int
	MaxSpy           int
	MaxFleetMessages int
	DeleteAccount    bool
}

type NormalizedOptionsMutation struct {
	OptionsMutation
	AccountDeletionChanged bool
}

type OptionsActionIssue struct {
	Code    string
	Message string
}

func NewOptions(overview Overview, user OptionsUser, universe OptionsUniverse, settings OptionsSettings, account OptionsAccount, rawFlags int64) Options {
	user.CommanderOn = overview.Commander != ""
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
	return Options{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		User:           user,
		Universe:       universe,
		Settings:       settings,
		Account:        account,
		Flags:          OptionsFlagsFromLegacy(rawFlags),
	}
}

func NormalizeOptionsMutation(command OptionsMutation, current Options) NormalizedOptionsMutation {
	normalized := command
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
	return NormalizedOptionsMutation{
		OptionsMutation:        normalized,
		AccountDeletionChanged: command.DeleteAccount != current.Account.DeletionQueued,
	}
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
	if !until.IsZero() {
		message += " Deletion date: " + until.Format("2006-01-02 15:04:05")
	}
	return &OptionsActionIssue{Code: OptionsIssueAccountDeletionQueued, Message: message}
}

func OptionsAccountDeletionClearedIssue() *OptionsActionIssue {
	return &OptionsActionIssue{Code: OptionsIssueAccountDeletionClear, Message: "Account deletion cancelled."}
}

func normalizeLanguage(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len(value) > 2 {
		value = value[:2]
	}
	return value
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

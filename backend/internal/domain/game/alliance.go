package game

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	AllianceRightDismiss   = 0x001
	AllianceRightKick      = 0x002
	AllianceRightReadApps  = 0x004
	AllianceRightMembers   = 0x008
	AllianceRightWriteApps = 0x010
	AllianceRightManage    = 0x020
	AllianceRightOnline    = 0x040
	AllianceRightCircular  = 0x080
	AllianceRightHand      = 0x100
	AllianceFounderRights  = 0x1FF

	AllianceRankFounder  = 0
	AllianceRankNewcomer = 1

	AllianceIssueCreated             = "created"
	AllianceIssueApplied             = "applied"
	AllianceIssueWithdrawn           = "withdrawn"
	AllianceIssueAccepted            = "accepted"
	AllianceIssueRejected            = "rejected"
	AllianceIssueLeft                = "left"
	AllianceIssueSaved               = "saved"
	AllianceIssueSent                = "sent"
	AllianceIssueInvalidTag          = "invalid_tag"
	AllianceIssueInvalidName         = "invalid_name"
	AllianceIssueInvalidRankName     = "invalid_rank_name"
	AllianceIssueTagExists           = "tag_exists"
	AllianceIssueAllianceNotFound    = "alliance_not_found"
	AllianceIssueApplicationsClosed  = "applications_closed"
	AllianceIssueNotActivated        = "not_activated"
	AllianceIssueAlreadyApplied      = "already_applied"
	AllianceIssueNoPermission        = "no_permission"
	AllianceIssueApplicationNotFound = "application_not_found"
	AllianceIssueFounderCannotLeave  = "founder_cannot_leave"
)

type AllianceView string

const (
	AllianceViewHome         AllianceView = "home"
	AllianceViewNoAlliance   AllianceView = "no_alliance"
	AllianceViewCreate       AllianceView = "create"
	AllianceViewSearch       AllianceView = "search"
	AllianceViewInfo         AllianceView = "info"
	AllianceViewApply        AllianceView = "apply"
	AllianceViewApplications AllianceView = "applications"
	AllianceViewMembers      AllianceView = "members"
	AllianceViewManagement   AllianceView = "management"
	AllianceViewRanks        AllianceView = "ranks"
	AllianceViewCircular     AllianceView = "circular"
	AllianceViewRenameTag    AllianceView = "rename_tag"
	AllianceViewRenameName   AllianceView = "rename_name"
)

type Alliance struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	View           AllianceView
	Viewer         AllianceViewer
	Own            *AllianceInfo
	Target         *AllianceInfo
	Pending        *AllianceApplication
	SearchText     string
	TextKind       int
	SearchResults  []AllianceSearchResult
	Applications   []AllianceApplication
	SelectedApp    *AllianceApplication
	Members        []AllianceMember
	Ranks          []AllianceRank
	CircularResult *AllianceCircularResult
}

type AllianceViewer struct {
	PlayerID   int
	Name       string
	Validated  bool
	AllianceID int
	RankID     int
	RankName   string
	RankRights int
	Founder    bool
}

type AllianceInfo struct {
	ID               int
	Tag              string
	Name             string
	OwnerID          int
	Homepage         string
	ImageLogo        string
	Open             bool
	InsertApp        bool
	ExternalText     string
	InternalText     string
	ApplicationText  string
	OldTag           string
	OldName          string
	TagUntil         int64
	NameUntil        int64
	MemberCount      int
	ApplicationCount int
}

type AllianceSearchResult struct {
	ID          int
	Tag         string
	Name        string
	MemberCount int
}

type AllianceApplication struct {
	ID         int
	AllianceID int
	PlayerID   int
	PlayerName string
	Text       string
	Date       int64
}

type AllianceMember struct {
	PlayerID  int
	Name      string
	RankID    int
	RankName  string
	Score     int64
	JoinedAt  int64
	LastClick int64
	Galaxy    int
	System    int
	Position  int
}

type AllianceRank struct {
	ID     int
	Name   string
	Rights int
}

type AllianceCircularResult struct {
	Recipients []string
}

type AllianceMutation struct {
	Action          string
	Tag             string
	Name            string
	Text            string
	TextKind        int
	Homepage        string
	ImageLogo       string
	Open            bool
	InsertApp       bool
	FounderRankName string
	AllianceID      int
	ApplicationID   int
	RankID          int
	RankName        string
	RankRights      []AllianceRank
	TargetPlayerID  int
	TargetRankID    int
	CircularRankID  int
}

type AllianceActionIssue struct {
	Code    string
	Message string
}

func NewAlliance(overview Overview, viewer AllianceViewer, now time.Time) Alliance {
	view := AllianceViewNoAlliance
	if viewer.AllianceID > 0 {
		view = AllianceViewHome
	}
	_ = now
	return Alliance{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		View:           view,
		Viewer:         viewer,
	}
}

func (a Alliance) WithView(view AllianceView) Alliance {
	a.View = view
	return a
}

func NormalizeAllianceTag(tag string) string {
	return truncateRunes(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(tag), `"`, ""), `'`, ""), 8)
}

func NormalizeAllianceName(name string) string {
	return truncateRunes(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(name), `"`, ""), `'`, ""), 30)
}

func ValidateAllianceCreate(tag string, name string) *AllianceActionIssue {
	if utf8.RuneCountInString(tag) < 3 {
		return AllianceIssue(AllianceIssueInvalidTag)
	}
	if utf8.RuneCountInString(name) < 3 {
		return AllianceIssue(AllianceIssueInvalidName)
	}
	return nil
}

func (v AllianceViewer) CanReadMembers() bool {
	return v.Founder || v.RankRights&AllianceRightMembers != 0
}

func (v AllianceViewer) CanWriteApplications() bool {
	return v.Founder || v.RankRights&AllianceRightWriteApps != 0
}

func (v AllianceViewer) CanManageAlliance() bool {
	return v.Founder || v.RankRights&AllianceRightManage != 0
}

func (v AllianceViewer) CanKickMembers() bool {
	return v.Founder || v.RankRights&AllianceRightKick != 0
}

func (v AllianceViewer) CanSendCircular() bool {
	return v.Founder || v.RankRights&AllianceRightCircular != 0
}

func (v AllianceViewer) CanLeaveAlliance() bool {
	return v.AllianceID > 0 && !v.Founder
}

func NormalizeAllianceTextKind(kind int) int {
	if kind < 1 || kind > 3 {
		return 1
	}
	return kind
}

func NormalizeAllianceText(text string) string {
	return truncateRunes(text, 5000)
}

func NormalizeAllianceCircularText(text string) string {
	return truncateRunes(text, 2000)
}

func NormalizeAllianceURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "\x00\x01\x02\x03\x04\x05\x06\x07\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return value
}

var allianceRankNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._\- ]+$`)

func NormalizeAllianceRankName(name string) string {
	return truncateRunes(strings.TrimSpace(name), 30)
}

func ValidateAllianceRankName(name string) *AllianceActionIssue {
	if name == "" {
		return nil
	}
	if !allianceRankNamePattern.MatchString(name) {
		return AllianceIssue(AllianceIssueInvalidRankName)
	}
	return nil
}

func ValidateAllianceNewRankName(name string) *AllianceActionIssue {
	if name == "" {
		return AllianceIssue(AllianceIssueInvalidRankName)
	}
	return ValidateAllianceRankName(name)
}

func AllianceIssue(code string) *AllianceActionIssue {
	switch code {
	case AllianceIssueCreated:
		return &AllianceActionIssue{Code: code, Message: "Alliance has been successfully created."}
	case AllianceIssueApplied:
		return &AllianceActionIssue{Code: code, Message: "Your application has been saved. You will receive a response if accepted or rejected."}
	case AllianceIssueWithdrawn:
		return &AllianceActionIssue{Code: code, Message: "Application withdrawn."}
	case AllianceIssueAccepted:
		return &AllianceActionIssue{Code: code, Message: "Application accepted."}
	case AllianceIssueRejected:
		return &AllianceActionIssue{Code: code, Message: "Application rejected."}
	case AllianceIssueLeft:
		return &AllianceActionIssue{Code: code, Message: "You have left the alliance."}
	case AllianceIssueSaved:
		return &AllianceActionIssue{Code: code, Message: "Changes saved."}
	case AllianceIssueSent:
		return &AllianceActionIssue{Code: code, Message: "General message sent."}
	case AllianceIssueInvalidTag:
		return &AllianceActionIssue{Code: code, Message: "Alliance abbreviation is too short"}
	case AllianceIssueInvalidName:
		return &AllianceActionIssue{Code: code, Message: "Alliance name is too short"}
	case AllianceIssueInvalidRankName:
		return &AllianceActionIssue{Code: code, Message: "The rank name contains illegal characters."}
	case AllianceIssueTagExists:
		return &AllianceActionIssue{Code: code, Message: "Alliance unfortunately already exists!"}
	case AllianceIssueAllianceNotFound:
		return &AllianceActionIssue{Code: code, Message: "Alliance not found."}
	case AllianceIssueApplicationsClosed:
		return &AllianceActionIssue{Code: code, Message: "This alliance is not accepting new members at this time"}
	case AllianceIssueNotActivated:
		return &AllianceActionIssue{Code: code, Message: "This function is only possible after the player account has been activated."}
	case AllianceIssueAlreadyApplied:
		return &AllianceActionIssue{Code: code, Message: "You have already applied to an alliance."}
	case AllianceIssueNoPermission:
		return &AllianceActionIssue{Code: code, Message: "Not enough permissions to perform the operation"}
	case AllianceIssueApplicationNotFound:
		return &AllianceActionIssue{Code: code, Message: "Application not found."}
	case AllianceIssueFounderCannotLeave:
		return &AllianceActionIssue{Code: code, Message: "The founder cannot leave the alliance from this dialog."}
	default:
		return &AllianceActionIssue{Code: code, Message: code}
	}
}

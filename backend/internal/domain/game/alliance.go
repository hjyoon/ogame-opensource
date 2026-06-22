package game

import (
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
	AllianceIssueInvalidTag          = "invalid_tag"
	AllianceIssueInvalidName         = "invalid_name"
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
	AllianceViewApply        AllianceView = "apply"
	AllianceViewApplications AllianceView = "applications"
	AllianceViewMembers      AllianceView = "members"
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
	SearchResults  []AllianceSearchResult
	Applications   []AllianceApplication
	SelectedApp    *AllianceApplication
	Members        []AllianceMember
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

type AllianceMutation struct {
	Action        string
	Tag           string
	Name          string
	Text          string
	AllianceID    int
	ApplicationID int
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

func (v AllianceViewer) CanLeaveAlliance() bool {
	return v.AllianceID > 0 && !v.Founder
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
	case AllianceIssueInvalidTag:
		return &AllianceActionIssue{Code: code, Message: "Alliance abbreviation is too short"}
	case AllianceIssueInvalidName:
		return &AllianceActionIssue{Code: code, Message: "Alliance name is too short"}
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

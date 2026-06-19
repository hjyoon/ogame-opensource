package game

import "fmt"

const (
	BuddyActionHome     = 0
	BuddyActionAdd      = 1
	BuddyActionAccept   = 2
	BuddyActionDecline  = 3
	BuddyActionWithdraw = 4
	BuddyActionIncoming = 5
	BuddyActionOutgoing = 6
	BuddyActionRequest  = 7
	BuddyActionDelete   = 8

	BuddyIssueAlreadySent = "already_sent"
)

type Buddy struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Action         int
	Rows           []BuddyRow
	Target         *BuddyPlayer
}

type BuddyRow struct {
	BuddyID int
	Player  BuddyPlayer
	Text    string
	Status  BuddyStatus
}

type BuddyPlayer struct {
	PlayerID    int
	Name        string
	Alliance    *BuddyAlliance
	Coordinates Coordinates
}

type BuddyAlliance struct {
	ID      int
	Tag     string
	Founder bool
}

type BuddyStatus struct {
	Text  string
	Color string
}

type BuddyActionIssue struct {
	Code    string
	Message string
}

func NormalizeBuddyAction(action int) int {
	switch action {
	case BuddyActionIncoming, BuddyActionOutgoing, BuddyActionRequest:
		return action
	default:
		return BuddyActionHome
	}
}

func NormalizeBuddyMutationAction(action int) int {
	switch action {
	case BuddyActionAdd, BuddyActionAccept, BuddyActionDecline, BuddyActionWithdraw, BuddyActionDelete:
		return action
	default:
		return BuddyActionHome
	}
}

func BuddyOnlineStatus(lastClickUnix int64, nowUnix int64) BuddyStatus {
	minutes := int((nowUnix - lastClickUnix) / 60)
	if minutes < 0 {
		minutes = 0
	}
	if minutes < 15 {
		return BuddyStatus{Text: "On", Color: "lime"}
	}
	if minutes < 60 {
		return BuddyStatus{Text: fmt.Sprintf("%d min", minutes), Color: "yellow"}
	}
	return BuddyStatus{Text: "Off", Color: "red"}
}

func BuddyAlreadySentIssue() *BuddyActionIssue {
	return &BuddyActionIssue{Code: BuddyIssueAlreadySent, Message: "There is already a request or membership."}
}

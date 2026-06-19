package game

const (
	MessagesActionInbox   = "inbox"
	MessagesActionCompose = "compose"

	MessageTypePM               = 0
	MessageTypeSpyReport        = 1
	MessageTypeBattleReportLink = 2
	MessageTypeExpedition       = 3
	MessageTypeAlliance         = 4
	MessageTypeMisc             = 5
	MessageTypeBattleReportText = 6

	MessagesLimitRegular   = 25
	MessagesLimitCommander = 50
	MessageComposeMaxChars = 2000
)

type Messages struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Action         string
	Rows           []Message
	Compose        *MessageCompose
}

type Message struct {
	ID         int
	Type       int
	From       string
	Subject    string
	Text       string
	Date       int64
	Unread     bool
	Reportable bool
}

type MessageCompose struct {
	Target   MessageTarget
	Subject  string
	MaxChars int
}

type MessageTarget struct {
	PlayerID    int
	Name        string
	Coordinates Coordinates
}

func NormalizeMessagesLimit(commanderActive bool) int {
	if commanderActive {
		return MessagesLimitCommander
	}
	return MessagesLimitRegular
}

func NormalizeMessagesAction(targetPlayerID int) string {
	if targetPlayerID > 0 {
		return MessagesActionCompose
	}
	return MessagesActionInbox
}

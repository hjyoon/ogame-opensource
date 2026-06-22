package game

const (
	MessagesActionInbox   = "inbox"
	MessagesActionCompose = "compose"

	MessagesMutationActionSend   = "send"
	MessagesMutationActionDelete = "delete"

	MessageDeleteModeNone        = ""
	MessageDeleteModeMarked      = "deletemarked"
	MessageDeleteModeNonMarked   = "deletenonmarked"
	MessageDeleteModeAllShown    = "deleteallshown"
	MessageDeleteModeAllMessages = "deleteall"

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
	MessageSubjectMaxChars = 40

	MessageIssueMissingSubject = "missing_subject"
	MessageIssueMissingText    = "missing_text"
	MessageIssueNotActivated   = "not_activated"
	MessageIssueSent           = "sent"
	MessageIssueReported       = "reported"
	MessageIssueReportExists   = "report_exists"
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

type MessageDraft struct {
	TargetPlayerID int
	Subject        string
	Text           string
}

type MessageActionIssue struct {
	Code    string
	Message string
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

func NormalizeMessagesMutationAction(action string) string {
	switch action {
	case MessagesMutationActionSend, MessagesMutationActionDelete:
		return action
	default:
		return MessagesMutationActionDelete
	}
}

func NormalizeMessageDeleteMode(value string) string {
	switch value {
	case MessageDeleteModeMarked, MessageDeleteModeNonMarked, MessageDeleteModeAllShown, MessageDeleteModeAllMessages:
		return value
	default:
		return MessageDeleteModeNone
	}
}

func NormalizeMessageIDs(ids []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func NormalizeMessageDraft(targetPlayerID int, subject string, text string) MessageDraft {
	return MessageDraft{
		TargetPlayerID: targetPlayerID,
		Subject:        truncateRunes(subject, MessageSubjectMaxChars),
		Text:           truncateRunes(text, MessageComposeMaxChars),
	}
}

func MessageMissingSubjectIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueMissingSubject, Message: "Missing topic"}
}

func MessageMissingTextIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueMissingText, Message: "Where's the message?"}
}

func MessageNotActivatedIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueNotActivated, Message: "Your account is not activated."}
}

func MessageSentIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueSent, Message: "Message sent"}
}

func MessageReportedIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueReported, Message: "Report sent!"}
}

func MessageReportExistsIssue() *MessageActionIssue {
	return &MessageActionIssue{Code: MessageIssueReportExists, Message: "The report has already been sent earlier!"}
}

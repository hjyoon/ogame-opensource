package game

const (
	NotesActionList   = "list"
	NotesActionCreate = "create"
	NotesActionEdit   = "edit"

	NotesLimit = 20
)

type Notes struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Action         string
	Rows           []Note
	EditNote       *Note
}

type Note struct {
	ID       int
	Subject  string
	Text     string
	TextSize int
	Priority int
	Date     int64
}

type NoteDraft struct {
	Subject  string
	Text     string
	TextSize int
	Priority int
}

func NormalizeNotesAction(value int) string {
	switch value {
	case 1:
		return NotesActionCreate
	case 2:
		return NotesActionEdit
	default:
		return NotesActionList
	}
}

func NormalizeNoteDraft(subject string, text string, priority int) NoteDraft {
	if subject == "" {
		subject = "no subject"
	}
	if text == "" {
		text = "no text"
	}
	subject = truncateRunes(subject, 30)
	text = truncateRunes(text, 5000)
	if priority < 0 {
		priority = 0
	}
	if priority > 2 {
		priority = 2
	}
	return NoteDraft{
		Subject:  subject,
		Text:     text,
		TextSize: len([]rune(text)),
		Priority: priority,
	}
}

func NormalizeNoteIDs(ids []int) []int {
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

func (n Note) PriorityColor() string {
	switch n.Priority {
	case 0:
		return "lime"
	case 1:
		return "yellow"
	case 2:
		return "red"
	default:
		return "white"
	}
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

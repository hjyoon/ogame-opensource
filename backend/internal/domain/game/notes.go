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

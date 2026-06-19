package game

import (
	"strconv"
	"strings"
)

const PlanetNameLimit = 20

const (
	OverviewIssuePasswordInvalid = "password_invalid"
	OverviewIssueHomePlanet      = "home_planet"
	OverviewIssueFleetIncoming   = "fleet_incoming"
	OverviewIssueFleetOutgoing   = "fleet_outgoing"
	OverviewAdminNotice          = "In the administrator mode Overview and Admin do not update event queue."
)

type Overview struct {
	Commander      string
	ServerTime     string
	Score          ScoreSummary
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Messages       []string
	UnreadMessages int
	Events         []FleetMission
}

type OverviewActionIssue struct {
	Code    string
	Message string
}

type ScoreSummary struct {
	RawScore        int64
	Rank            int
	UniversePlayers int
}

type PlanetOverview struct {
	ID          int
	Name        string
	Type        int
	Coordinates Coordinates
	Diameter    int
	Temperature int
	Fields      int
	MaxFields   int
	Resources   Resources
	BuildQueue  *OverviewBuildQueue
}

type PlanetSummary struct {
	ID          int
	Name        string
	Type        int
	Coordinates Coordinates
	Current     bool
	BuildQueue  *OverviewBuildQueue
}

type OverviewBuildQueue struct {
	TechID  int
	Name    string
	Level   int
	Destroy bool
	End     int64
}

type Coordinates struct {
	Galaxy   int
	System   int
	Position int
}

type Resources struct {
	Metal             float64
	Crystal           float64
	Deuterium         float64
	MetalCapacity     int
	CrystalCapacity   int
	DeuteriumCapacity int
}

func (s ScoreSummary) DisplayPoints() int64 {
	if s.RawScore < 0 {
		return 0
	}
	return s.RawScore / 1000
}

func OverviewUnreadMessageText(count int) string {
	if count <= 0 {
		return ""
	}
	suffix := ""
	if count > 1 {
		suffix = "s"
	}
	return "You have " + strconv.Itoa(count) + " new message" + suffix
}

func (c Coordinates) Valid() bool {
	return c.Galaxy > 0 && c.System > 0 && c.Position > 0
}

func NormalizePlanetName(name string, planetType int) (string, bool) {
	name = truncateRunes(name, planetNameLimit(planetType))
	if strings.ContainsAny(name, ";,<>,`") {
		return "", false
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '(', ')', '*', '"', '\'':
			return -1
		default:
			return r
		}
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		if planetType == PlanetTypeMoon {
			return "Moon", true
		}
		return "\u043f\u043b\u0430\u043d\u0435\u0442\u0430", true
	}
	if planetType == PlanetTypeMoon {
		name += " (Moon)"
	}
	return name, true
}

func planetNameLimit(planetType int) int {
	if planetType == PlanetTypeMoon {
		return PlanetNameLimit - len([]rune(" (Moon)"))
	}
	return PlanetNameLimit
}

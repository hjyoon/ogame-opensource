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
	OverviewActivationNotice     = "Your game account has not been activated yet. Go to Settings, enter your e-mail address and receive an activation link to it"
	OverviewVacationNotice       = "vacation mode"
	OverviewUniverseFreezeNotice = "The universe has been put on pause."
)

type Overview struct {
	Commander      string
	AdminLevel     int
	Validated      bool
	ServerTime     string
	ServerTimeUnix int64
	Officers       OverviewOfficers
	Score          ScoreSummary
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	News           *OverviewNews
	MenuLinks      OverviewMenuLinks
	Messages       []string
	Errors         []string
	UnreadMessages int
	Events         []FleetMission
}

type OverviewOfficers struct {
	Commander          bool
	CommanderDaysLeft  int
	Admiral            bool
	AdmiralDaysLeft    int
	Engineer           bool
	EngineerDaysLeft   int
	Geologist          bool
	GeologistDaysLeft  int
	Technocrat         bool
	TechnocratDaysLeft int
}

type OverviewNews struct {
	URL   string
	Start string
	End   string
}

type OverviewMenuLinks struct {
	BoardURL   string
	DiscordURL string
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
	ProductionPerHour ResourceProductionValues
	DarkMatter        int
	Energy            int
	EnergyCapacity    int
	MetalCapacity     int
	CrystalCapacity   int
	DeuteriumCapacity int
}

func (s ScoreSummary) DisplayPoints() int64 {
	if s.RawScore < 0 {
		return 0
	}
	return s.RawScore / ScoreDisplayScale
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

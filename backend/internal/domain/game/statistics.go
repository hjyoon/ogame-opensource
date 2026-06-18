package game

const (
	StatisticsWhoPlayer = "player"
	StatisticsWhoAlly   = "ally"

	StatisticsTypeResources = "ressources"
	StatisticsTypeFleet     = "fleet"
	StatisticsTypeResearch  = "research"
)

type Statistics struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Who            string
	Type           string
	Start          int
	Total          int
	GeneratedAt    int64
	Rows           []StatisticsRow
}

type StatisticsRow struct {
	Place         int
	PreviousPlace int
	Score         int64
	ScoreDate     int64
	Player        StatisticsPlayer
	Alliance      *StatisticsAlliance
	Coordinates   Coordinates
	Members       int
	Own           bool
}

type StatisticsPlayer struct {
	ID   int
	Name string
}

type StatisticsAlliance struct {
	ID  int
	Tag string
}

func NormalizeStatisticsWho(value string) string {
	if value == StatisticsWhoAlly {
		return StatisticsWhoAlly
	}
	return StatisticsWhoPlayer
}

func NormalizeStatisticsType(value string) string {
	switch value {
	case StatisticsTypeFleet, StatisticsTypeResearch:
		return value
	default:
		return StatisticsTypeResources
	}
}

func NormalizeStatisticsStart(value int, ownPlace int) int {
	if value > 0 {
		return ((value - 1) / 100 * 100) + 1
	}
	if ownPlace > 0 {
		return (ownPlace/100)*100 + 1
	}
	return 1
}

func StatisticsScoreColumns(statType string) (score string, place string, oldPlace string) {
	switch NormalizeStatisticsType(statType) {
	case StatisticsTypeFleet:
		return "score2", "place2", "oldplace2"
	case StatisticsTypeResearch:
		return "score3", "place3", "oldplace3"
	default:
		return "score1", "place1", "oldplace1"
	}
}

func (r StatisticsRow) DisplayScore(statType string) int64 {
	if r.Score < 0 {
		return 0
	}
	if NormalizeStatisticsType(statType) == StatisticsTypeResources {
		return r.Score / 1000
	}
	return r.Score
}

func (r StatisticsRow) DisplayScorePerMember(statType string) int64 {
	if r.Members <= 0 {
		return 0
	}
	score := r.DisplayScore(statType)
	return (score + int64(r.Members) - 1) / int64(r.Members)
}

func (r StatisticsRow) PlaceDelta() int {
	return r.Place - r.PreviousPlace
}

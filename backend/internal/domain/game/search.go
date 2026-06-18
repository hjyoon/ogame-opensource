package game

import "strings"

const (
	SearchTypePlayerName   = "playername"
	SearchTypePlanetName   = "planetname"
	SearchTypeAllianceTag  = "allytag"
	SearchTypeAllianceName = "allyname"

	SearchLimit = 25
)

type Search struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Type           string
	Text           string
	Message        string
	PlayerRows     []SearchPlayerRow
	AllianceRows   []SearchAllianceRow
}

type SearchPlayerRow struct {
	PlayerID     int
	PlayerName   string
	Alliance     *StatisticsAlliance
	PlanetID     int
	PlanetName   string
	Coordinates  Coordinates
	Place        int
	Own          bool
	SameAlliance bool
}

type SearchAllianceRow struct {
	AllianceID int
	Tag        string
	Name       string
	Members    int
	Score      int64
	Own        bool
}

func NormalizeSearchType(value string) string {
	switch value {
	case SearchTypePlanetName, SearchTypeAllianceTag, SearchTypeAllianceName:
		return value
	default:
		return SearchTypePlayerName
	}
}

func NormalizeSearchText(value string) string {
	return strings.TrimSpace(value)
}

func SearchTextTooShort(value string) bool {
	return len([]rune(NormalizeSearchText(value))) > 0 && len([]rune(NormalizeSearchText(value))) < 2
}

func SearchOverLimitMessage(searchType string) string {
	switch NormalizeSearchType(searchType) {
	case SearchTypeAllianceTag, SearchTypeAllianceName:
		return "more than 25 entries found"
	default:
		return "More than 25 entries were found."
	}
}

func (r SearchAllianceRow) DisplayScore() int64 {
	if r.Score < 0 {
		return 0
	}
	return r.Score / 1000
}

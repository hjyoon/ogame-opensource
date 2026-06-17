package game

type Overview struct {
	Commander      string
	Score          ScoreSummary
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
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
}

type PlanetSummary struct {
	ID          int
	Name        string
	Type        int
	Coordinates Coordinates
	Current     bool
}

type Coordinates struct {
	Galaxy   int
	System   int
	Position int
}

type Resources struct {
	Metal     float64
	Crystal   float64
	Deuterium float64
}

func (s ScoreSummary) DisplayPoints() int64 {
	if s.RawScore < 0 {
		return 0
	}
	return s.RawScore / 1000
}

func (c Coordinates) Valid() bool {
	return c.Galaxy > 0 && c.System > 0 && c.Position > 0
}

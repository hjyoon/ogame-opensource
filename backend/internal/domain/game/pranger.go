package game

const PrangerPageLimit = 50

type Pranger struct {
	Universe int
	From     int
	Internal bool
	Entries  []PrangerEntry
}

type PrangerEntry struct {
	BanWhen   int64
	AdminName string
	UserName  string
	BanUntil  int64
	Reason    string
}

func NormalizePrangerFrom(from int) int {
	if from < 0 {
		return 0
	}
	return from
}

func (p Pranger) HasPrevious() bool {
	return p.From >= PrangerPageLimit
}

func (p Pranger) PreviousFrom() int {
	previous := p.From - PrangerPageLimit
	if previous < 0 {
		return 0
	}
	return previous
}

func (p Pranger) HasNext() bool {
	return len(p.Entries) >= PrangerPageLimit
}

func (p Pranger) NextFrom() int {
	return p.From + PrangerPageLimit
}

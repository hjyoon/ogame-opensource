package game

import (
	"math"
	"time"
)

const (
	OfficerCommander  = 1
	OfficerAdmiral    = 2
	OfficerEngineer   = 3
	OfficerGeologist  = 4
	OfficerTechnocrat = 5

	OfficerWeekDays       = 7
	OfficerThreeMonthDays = 90
	OfficerWeekCost       = 10000
	OfficerThreeMonthCost = OfficerWeekCost * 10
	OfficerIssueNotEnough = "not_enough_dark_matter"
	OfficerIssueRecruited = "recruited"
)

type Officers struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	User           OfficersUser
	Rows           []OfficerRow
}

type OfficersUser struct {
	PaidDarkMatter int
	FreeDarkMatter int
}

type OfficerTimers struct {
	Commander  int64
	Admiral    int64
	Engineer   int64
	Geologist  int64
	Technocrat int64
}

type OfficerRow struct {
	ID             int
	Key            string
	Name           string
	Description    string
	Note           string
	Image          string
	Icon           string
	Active         bool
	Until          int64
	DaysLeft       int
	WeekCost       int
	ThreeMonthCost int
}

type OfficerMutation struct {
	OfficerID int
	Days      int
}

type OfficerRecruitment struct {
	Changed bool
	User    OfficersUser
	Timers  OfficerTimers
}

type OfficerActionIssue struct {
	Code    string
	Message string
}

func NewOfficers(overview Overview, user OfficersUser, timers OfficerTimers, now time.Time) Officers {
	return Officers{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		User:           user,
		Rows:           OfficerRows(timers, now),
	}
}

func OfficerRows(timers OfficerTimers, now time.Time) []OfficerRow {
	rows := OfficerCatalog()
	for index := range rows {
		until := OfficerUntil(timers, rows[index].ID)
		rows[index].Until = until
		rows[index].DaysLeft = OfficerDaysLeft(until, now)
		rows[index].Active = rows[index].DaysLeft > 0
	}
	return rows
}

func OfficerCatalog() []OfficerRow {
	return []OfficerRow{
		{
			ID:             OfficerCommander,
			Key:            "commander",
			Name:           "Commander",
			Description:    "The rank of commander has proven itself time and time again in modern combat. Thanks to the simplified structure, your orders can be executed faster, allowing you to maintain control of your entire empire! You will be able to develop strategies that allow you to always be one step ahead of the enemy.",
			Note:           "Build queue, empire overview, enhanced galaxy overview, message filter, no ads",
			Image:          "commander_stern_gross.jpg",
			Icon:           "commander_ikon.gif",
			WeekCost:       OfficerWeekCost,
			ThreeMonthCost: OfficerThreeMonthCost,
		},
		{
			ID:             OfficerAdmiral,
			Key:            "admiral",
			Name:           "Admiral",
			Description:    "The Admiral is a war-tested veteran and a brilliant strategist. Even in the hottest battles, he does not lose his overview and maintains contact with his subordinate admirals. A wise ruler can fully rely on him in battle and thus use more ships for combat.",
			Note:           "\u00a0Max. number of fleets +2",
			Image:          "ogame_admiral.jpg",
			Icon:           "admiral_ikon.gif",
			WeekCost:       OfficerWeekCost,
			ThreeMonthCost: OfficerThreeMonthCost,
		},
		{
			ID:             OfficerEngineer,
			Key:            "engineer",
			Name:           "Engineer",
			Description:    "An engineer is an energy management specialist. In peacetime, he increases the level of energy grids on colonies. In the event of an attack, he supplies the energy systems of planetary defenses and prevents overloading, resulting in a much lower casualty rate in combat.",
			Note:           "Reduces defense losses by half+10% more energy",
			Image:          "ogame_ingenieur.jpg",
			Icon:           "ingenieur_ikon.gif",
			WeekCost:       OfficerWeekCost,
			ThreeMonthCost: OfficerThreeMonthCost,
		},
		{
			ID:             OfficerGeologist,
			Key:            "geologist",
			Name:           "Geologist",
			Description:    "Geologist is a recognized expert in astromineralogy and -crystallography. With his team of metallurgists and chemists, he supports interplanetary governments in developing new sources of resources and optimizing their purification.",
			Note:           "+10% mine income",
			Image:          "ogame_geologe.jpg",
			Icon:           "geologe_ikon.gif",
			WeekCost:       OfficerWeekCost,
			ThreeMonthCost: OfficerThreeMonthCost,
		},
		{
			ID:             OfficerTechnocrat,
			Key:            "technocrat",
			Name:           "Technocrat",
			Description:    "The Technocrat Guild are brilliant scientists, and they can always be found where the edge of the technically possible ends. Their code can never be cracked by any normal person, and by their mere presence they inspire the scientists of the Empire.",
			Note:           "+2 level of espionage, 25% less research time",
			Image:          "ogame_technokrat.jpg",
			Icon:           "technokrat_ikon.gif",
			WeekCost:       OfficerWeekCost,
			ThreeMonthCost: OfficerThreeMonthCost,
		},
	}
}

func ResolveOfficerRecruitment(user OfficersUser, timers OfficerTimers, mutation OfficerMutation, now time.Time) (OfficerRecruitment, *OfficerActionIssue) {
	if !validOfficerID(mutation.OfficerID) || (mutation.Days != OfficerWeekDays && mutation.Days != OfficerThreeMonthDays) {
		return OfficerRecruitment{}, nil
	}
	cost := OfficerWeekCost
	if mutation.Days == OfficerThreeMonthDays {
		cost = OfficerThreeMonthCost
	}
	spent, ok := SpendOfficerDarkMatter(user, cost)
	if !ok {
		return OfficerRecruitment{User: user, Timers: timers}, OfficerNotEnoughDarkMatterIssue()
	}
	until := ExtendOfficerUntil(OfficerUntil(timers, mutation.OfficerID), mutation.Days, now)
	timers = SetOfficerUntil(timers, mutation.OfficerID, until)
	return OfficerRecruitment{Changed: true, User: spent, Timers: timers}, OfficerRecruitedIssue()
}

func SpendOfficerDarkMatter(user OfficersUser, cost int) (OfficersUser, bool) {
	if cost <= 0 {
		return user, true
	}
	if user.PaidDarkMatter+user.FreeDarkMatter < cost {
		return user, false
	}
	if user.PaidDarkMatter >= cost {
		user.PaidDarkMatter -= cost
		return user, true
	}
	user.FreeDarkMatter -= cost - user.PaidDarkMatter
	user.PaidDarkMatter = 0
	return user, true
}

func ExtendOfficerUntil(currentUntil int64, days int, now time.Time) int64 {
	seconds := int64(days * 24 * 60 * 60)
	base := now.Unix()
	if currentUntil > base {
		base = currentUntil
	}
	return base + seconds
}

func OfficerDaysLeft(until int64, now time.Time) int {
	if until <= now.Unix() {
		return 0
	}
	return int(math.Ceil(float64(until-now.Unix()) / float64(24*60*60)))
}

func OfficerUntil(timers OfficerTimers, officerID int) int64 {
	switch officerID {
	case OfficerCommander:
		return timers.Commander
	case OfficerAdmiral:
		return timers.Admiral
	case OfficerEngineer:
		return timers.Engineer
	case OfficerGeologist:
		return timers.Geologist
	case OfficerTechnocrat:
		return timers.Technocrat
	default:
		return 0
	}
}

func SetOfficerUntil(timers OfficerTimers, officerID int, until int64) OfficerTimers {
	switch officerID {
	case OfficerCommander:
		timers.Commander = until
	case OfficerAdmiral:
		timers.Admiral = until
	case OfficerEngineer:
		timers.Engineer = until
	case OfficerGeologist:
		timers.Geologist = until
	case OfficerTechnocrat:
		timers.Technocrat = until
	}
	return timers
}

func OfficerNotEnoughDarkMatterIssue() *OfficerActionIssue {
	return &OfficerActionIssue{Code: OfficerIssueNotEnough, Message: "Not enough dark matter!"}
}

func OfficerRecruitedIssue() *OfficerActionIssue {
	return &OfficerActionIssue{Code: OfficerIssueRecruited, Message: "The renewal was successful!"}
}

func validOfficerID(officerID int) bool {
	return officerID >= OfficerCommander && officerID <= OfficerTechnocrat
}

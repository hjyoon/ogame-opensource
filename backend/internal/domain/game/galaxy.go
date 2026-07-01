package game

import (
	"math"
	"strconv"
)

const (
	PlanetTypeDebris          = 10000
	PlanetTypeDestroyedPlanet = 10001
	PlanetTypeDestroyedMoon   = 10003
	PlanetTypeAbandoned       = 10004

	GamePlanetTypePlanet = 1
	GamePlanetTypeDebris = 2
	GamePlanetTypeMoon   = 3

	GalaxyPositions      = 15
	GalaxyFarSpace       = 16
	GalaxyDeuteriumCost  = 10
	GalaxyPhantomDebris  = 300
	GalaxyActionSpy      = 0x1
	GalaxyActionMessage  = 0x2
	GalaxyActionBuddy    = 0x4
	GalaxyActionMissile  = 0x8
	GalaxyActionReport   = 0x10
	GalaxyNoobScoreLimit = 5000

	GalaxyIssueRocketNoTarget      = "rocket_no_target"
	GalaxyIssueRocketNoRockets     = "rocket_no_rockets"
	GalaxyIssueRocketNotEnough     = "rocket_not_enough"
	GalaxyIssueRocketWeakDrive     = "rocket_weak_drive"
	GalaxyIssueRocketVacationSelf  = "rocket_vacation_self"
	GalaxyIssueRocketVacationOther = "rocket_vacation_other"
	GalaxyIssueRocketSelf          = "rocket_self"
	GalaxyIssueRocketAdmin         = "rocket_admin"
	GalaxyIssueRocketNoob          = "rocket_noob"
	GalaxyIssueRocketFrozen        = "rocket_frozen"
	GalaxyIssueRocketLaunchRace    = "rocket_launch_race"
	GalaxyIssueRocketLaunched      = "rocket_launched"
	GalaxyIssueFleetDispatched     = "fleet_dispatched"
)

type GalaxyActionIssue struct {
	Code    string
	Message string
}

type Galaxy struct {
	Commander           string
	CurrentPlanet       PlanetOverview
	PlanetSwitcher      []PlanetSummary
	Coordinates         Coordinates
	Bounds              GalaxyBounds
	Rows                []GalaxyRow
	Populated           int
	Slots               FleetSlots
	Extra               GalaxyExtra
	NotEnoughDeuterium  bool
	RemoteSystemCostDue bool
	ViewerAllianceID    int
}

type GalaxyBounds struct {
	Galaxies int
	Systems  int
}

type GalaxyInput struct {
	Coordinates Coordinates
	Bounds      GalaxyBounds
	Viewer      GalaxyViewer
	FleetSlots  FleetSlots
	Objects     []GalaxyObject
	Now         int64
}

type GalaxyViewer struct {
	PlayerID   int
	Score      int64
	AllianceID int
	Admin      int
	Flags      int
	MaxSpy     int
	Commander  bool
	SpyProbes  int
	Recyclers  int
	Missiles   int
}

type GalaxyObject struct {
	ID            int
	Name          string
	Type          int
	Coordinates   Coordinates
	Diameter      int
	Temperature   int
	LastActivity  int64
	Owner         GalaxyObjectPlayer
	Alliance      GalaxyAlliance
	DebrisMetal   float64
	DebrisCrystal float64
}

type GalaxyObjectPlayer struct {
	ID        int
	Name      string
	Score     int64
	Rank      int
	Alliance  int
	LastClick int64
	Vacation  bool
	Banned    bool
	Admin     int
}

type GalaxyAlliance struct {
	ID      int
	Tag     string
	Rank    int
	Members int
}

type GalaxyRow struct {
	Position int
	Planet   *GalaxyPlanet
	Moon     *GalaxyPlanet
	Debris   *GalaxyDebris
}

type GalaxyPlanet struct {
	ID           int
	Name         string
	DisplayName  string
	Type         int
	Coordinates  Coordinates
	Diameter     int
	Temperature  int
	LastActivity int64
	ActivityText string
	Destroyed    bool
	Abandoned    bool
	Own          bool
	Player       *GalaxyPlayerStatus
	Alliance     *GalaxyAlliance
	Actions      GalaxyActions
}

type GalaxyPlayerStatus struct {
	ID          int
	Name        string
	Rank        int
	Status      string
	StatusClass string
	Suffixes    []GalaxyStatusSuffix
	Own         bool
}

type GalaxyStatusSuffix struct {
	Text  string
	Class string
}

type GalaxyDebris struct {
	ID         int
	Metal      float64
	Crystal    float64
	Harvesters int
	Visible    bool
}

type GalaxyActions struct {
	Deploy    bool
	Transport bool
	Spy       bool
	Message   bool
	Buddy     bool
	Missile   bool
	Attack    bool
	Defend    bool
	Destroy   bool
	Recycle   bool
}

type GalaxyExtra struct {
	Commander bool
	SpyProbes int
	Recyclers int
	Missiles  int
	MaxSpy    int
	Slots     FleetSlots
}

func BuildGalaxy(overview Overview, input GalaxyInput) Galaxy {
	bounds := normalizeGalaxyBounds(input.Bounds)
	coordinates := clampGalaxyCoordinates(input.Coordinates, overview.CurrentPlanet.Coordinates, bounds)
	rows := make([]GalaxyRow, GalaxyPositions)
	for i := range rows {
		rows[i] = GalaxyRow{Position: i + 1}
	}

	moonByPosition := map[int]GalaxyObject{}
	debrisByPosition := map[int]GalaxyObject{}
	planets := make([]GalaxyObject, 0, len(input.Objects))
	for _, object := range input.Objects {
		if object.Coordinates.Position < 1 || object.Coordinates.Position > GalaxyPositions {
			continue
		}
		switch object.Type {
		case PlanetTypeMoon, PlanetTypeDestroyedMoon:
			moonByPosition[object.Coordinates.Position] = object
		case PlanetTypeDebris:
			debrisByPosition[object.Coordinates.Position] = object
		case PlanetTypePlanet, PlanetTypeDestroyedPlanet, PlanetTypeAbandoned:
			planets = append(planets, object)
		}
	}

	populated := 0
	for _, object := range planets {
		position := object.Coordinates.Position
		moonObject, hasMoon := moonByPosition[position]
		var moon *GalaxyPlanet
		if hasMoon {
			builtMoon := buildGalaxyPlanet(moonObject, input.Viewer, input.Now, nil)
			moon = &builtMoon
		}
		planet := buildGalaxyPlanet(object, input.Viewer, input.Now, moon)
		debrisObject, hasDebris := debrisByPosition[position]
		var debris *GalaxyDebris
		if hasDebris {
			debris = buildGalaxyDebris(debrisObject)
		}
		rows[position-1].Planet = &planet
		rows[position-1].Moon = moon
		rows[position-1].Debris = debris
		populated++
	}

	for position, debrisObject := range debrisByPosition {
		if rows[position-1].Debris == nil {
			rows[position-1].Debris = buildGalaxyDebris(debrisObject)
		}
	}
	for position, moonObject := range moonByPosition {
		if rows[position-1].Moon == nil {
			moon := buildGalaxyPlanet(moonObject, input.Viewer, input.Now, nil)
			rows[position-1].Moon = &moon
		}
	}

	remoteSystem := overview.CurrentPlanet.Coordinates.Galaxy != coordinates.Galaxy || overview.CurrentPlanet.Coordinates.System != coordinates.System
	return Galaxy{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Coordinates:    coordinates,
		Bounds:         bounds,
		Rows:           rows,
		Populated:      populated,
		Slots:          input.FleetSlots,
		Extra: GalaxyExtra{
			Commander: input.Viewer.Commander,
			SpyProbes: input.Viewer.SpyProbes,
			Recyclers: input.Viewer.Recyclers,
			Missiles:  input.Viewer.Missiles,
			MaxSpy:    input.Viewer.MaxSpy,
			Slots:     input.FleetSlots,
		},
		NotEnoughDeuterium:  remoteSystem && input.Viewer.Admin == 0 && overview.CurrentPlanet.Resources.Deuterium < GalaxyDeuteriumCost,
		RemoteSystemCostDue: remoteSystem && input.Viewer.Admin == 0,
		ViewerAllianceID:    input.Viewer.AllianceID,
	}
}

func normalizeGalaxyBounds(bounds GalaxyBounds) GalaxyBounds {
	if bounds.Galaxies < 1 {
		bounds.Galaxies = 1
	}
	if bounds.Systems < 1 {
		bounds.Systems = 1
	}
	return bounds
}

func clampGalaxyCoordinates(requested Coordinates, fallback Coordinates, bounds GalaxyBounds) Coordinates {
	coordinates := requested
	if coordinates.Galaxy == 0 {
		coordinates.Galaxy = fallback.Galaxy
	}
	if coordinates.System == 0 {
		coordinates.System = fallback.System
	}
	if coordinates.Position == 0 {
		coordinates.Position = fallback.Position
	}
	if coordinates.Galaxy < 1 {
		coordinates.Galaxy = 1
	}
	if coordinates.System < 1 {
		coordinates.System = 1
	}
	if coordinates.Position < 1 {
		coordinates.Position = 1
	}
	if coordinates.Galaxy > bounds.Galaxies {
		coordinates.Galaxy = bounds.Galaxies
	}
	if coordinates.System > bounds.Systems {
		coordinates.System = bounds.Systems
	}
	if coordinates.Position > GalaxyFarSpace {
		coordinates.Position = GalaxyFarSpace
	}
	return coordinates
}

func buildGalaxyPlanet(object GalaxyObject, viewer GalaxyViewer, now int64, moon *GalaxyPlanet) GalaxyPlanet {
	own := object.Owner.ID != 0 && object.Owner.ID == viewer.PlayerID
	destroyed := object.Type == PlanetTypeDestroyedPlanet || object.Type == PlanetTypeDestroyedMoon
	abandoned := object.Type == PlanetTypeAbandoned
	activity := galaxyActivityText(object.LastActivity, 0, own, now)
	if moon != nil {
		activity = galaxyActivityText(object.LastActivity, moon.LastActivity, own, now)
	}
	displayName := object.Name
	if object.Type == PlanetTypeDestroyedPlanet {
		displayName = "Destroyed Planet"
	} else if object.Type == PlanetTypeAbandoned {
		displayName = "Abandoned Planet"
	}

	planet := GalaxyPlanet{
		ID:           object.ID,
		Name:         object.Name,
		DisplayName:  displayName,
		Type:         object.Type,
		Coordinates:  object.Coordinates,
		Diameter:     object.Diameter,
		Temperature:  object.Temperature,
		LastActivity: object.LastActivity,
		ActivityText: activity,
		Destroyed:    destroyed,
		Abandoned:    abandoned,
		Own:          own,
		Actions:      galaxyActions(object.Type, own, viewer),
	}
	if object.Alliance.ID != 0 && !destroyed && !abandoned {
		alliance := object.Alliance
		planet.Alliance = &alliance
	}
	if object.Owner.ID != 0 && !destroyed && !abandoned {
		player := galaxyPlayerStatus(object.Owner, viewer, now, own)
		planet.Player = &player
	}
	return planet
}

func galaxyActivityText(planetLast int64, moonLast int64, own bool, now int64) string {
	if own || now <= 0 {
		return ""
	}
	activity := planetLast
	if moonLast > activity {
		activity = moonLast
	}
	if activity <= 0 {
		return ""
	}
	age := now - activity
	if age < 0 {
		age = 0
	}
	if age < 15*60 {
		return "(*)"
	}
	if age < 60*60 {
		return "(" + legacyMinutes(age) + " min)"
	}
	return ""
}

func legacyMinutes(age int64) string {
	minutes := age / 60
	if minutes < 0 {
		minutes = 0
	}
	if minutes == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for minutes > 0 {
		digits = append(digits, byte('0'+minutes%10))
		minutes /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}

func galaxyPlayerStatus(owner GalaxyObjectPlayer, viewer GalaxyViewer, now int64, own bool) GalaxyPlayerStatus {
	status := GalaxyPlayerStatus{
		ID:          owner.ID,
		Name:        owner.Name,
		Rank:        owner.Rank,
		Status:      "normal",
		StatusClass: "normal",
		Own:         own,
	}
	if own {
		return status
	}

	week := now - 604800
	week4 := now - 604800*4
	switch {
	case GalaxyPlayerProtectedFromMissiles(owner, viewer, now) && owner.Score < viewer.Score:
		status.Status = "noob"
		status.StatusClass = "noob"
		status.Suffixes = []GalaxyStatusSuffix{{Text: "n", Class: "noob"}}
		return status
	case GalaxyPlayerProtectedFromMissiles(owner, viewer, now) && viewer.Score < owner.Score:
		status.Status = "strong"
		status.StatusClass = "strong"
		status.Suffixes = []GalaxyStatusSuffix{{Text: "s", Class: "strong"}}
		return status
	}

	if owner.LastClick <= week {
		status.Status = "inactive"
		status.StatusClass = "inactive"
		status.Suffixes = append(status.Suffixes, GalaxyStatusSuffix{Text: "i", Class: "inactive"})
	}
	if owner.Banned {
		status.Status = "banned"
		status.StatusClass = "banned"
		status.Suffixes = append(status.Suffixes, GalaxyStatusSuffix{Text: "b", Class: "banned"})
	}
	if owner.LastClick <= week4 {
		if status.Status != "banned" {
			status.Status = "longinactive"
			status.StatusClass = "longinactive"
		}
		status.Suffixes = append(status.Suffixes, GalaxyStatusSuffix{Text: "I", Class: "longinactive"})
	}
	if owner.Vacation {
		status.Status = "vacation"
		status.StatusClass = "vacation"
		status.Suffixes = append(status.Suffixes, GalaxyStatusSuffix{Text: "V", Class: "vacation"})
	}
	return status
}

func GalaxyPlayerProtectedFromMissiles(owner GalaxyObjectPlayer, viewer GalaxyViewer, now int64) bool {
	activeForNoobCheck := owner.LastClick > now-604800 && !owner.Vacation && !owner.Banned
	return (activeForNoobCheck && owner.Score < viewer.Score && owner.Score < GalaxyNoobScoreLimit && viewer.Score > owner.Score*5) ||
		(activeForNoobCheck && viewer.Score < owner.Score && viewer.Score < GalaxyNoobScoreLimit && owner.Score > viewer.Score*5)
}

func GalaxyMissileTargetAllowed(id int) bool {
	switch id {
	case 0,
		DefenseRocketLauncher,
		DefenseLightLaser,
		DefenseHeavyLaser,
		DefenseGaussCannon,
		DefenseIonCannon,
		DefensePlasmaTurret,
		DefenseSmallShieldDome,
		DefenseLargeShieldDome:
		return true
	default:
		return false
	}
}

func GalaxyActionIssueFor(code string) *GalaxyActionIssue {
	message := map[string]string{
		GalaxyIssueRocketNoTarget:      "There is no target",
		GalaxyIssueRocketNoRockets:     "You didn't select the number of missiles",
		GalaxyIssueRocketNotEnough:     "Not enough interplanetary rockets!",
		GalaxyIssueRocketWeakDrive:     "The range (impulse engine research level) of your interplanetary rocket is too short!",
		GalaxyIssueRocketVacationSelf:  "You can't launch rockets while in vacation mode!",
		GalaxyIssueRocketVacationOther: "This player is in vacation mode!",
		GalaxyIssueRocketSelf:          "It's impossible to attack your own planet!",
		GalaxyIssueRocketAdmin:         "You cannot launch rockets at game operators or administrators!",
		GalaxyIssueRocketNoob:          "The planet is under noob protection!",
		GalaxyIssueRocketFrozen:        "Universe is frozen.",
		GalaxyIssueRocketLaunchRace:    "Not enough interplanetary rockets!",
		GalaxyIssueRocketLaunched:      "Start of rocket 1!",
		GalaxyIssueFleetDispatched:     "done",
	}[code]
	if message == "" {
		message = "There was an error"
	}
	return &GalaxyActionIssue{Code: code, Message: message}
}

func GalaxyFleetDispatchedIssue() *GalaxyActionIssue {
	return GalaxyActionIssueFor(GalaxyIssueFleetDispatched)
}

func GalaxyActionIssueFromFleet(issue *FleetActionIssue) *GalaxyActionIssue {
	if issue == nil {
		return nil
	}
	return &GalaxyActionIssue{
		Code:    "fleet_" + issue.Code,
		Message: issue.Message,
	}
}

func GalaxyMissileLaunchedIssue(amount int) *GalaxyActionIssue {
	if amount < 0 {
		amount = -amount
	}
	if amount == 0 {
		return GalaxyActionIssueFor(GalaxyIssueRocketNoRockets)
	}
	return &GalaxyActionIssue{
		Code:    GalaxyIssueRocketLaunched,
		Message: "Start of rocket " + strconv.Itoa(amount) + "!",
	}
}

func galaxyActions(objectType int, own bool, viewer GalaxyViewer) GalaxyActions {
	destroyed := objectType == PlanetTypeDestroyedPlanet || objectType == PlanetTypeDestroyedMoon || objectType == PlanetTypeAbandoned
	if destroyed {
		return GalaxyActions{}
	}
	if own {
		return GalaxyActions{Deploy: true, Transport: true}
	}
	return GalaxyActions{
		Spy:       viewer.Flags&GalaxyActionSpy != 0,
		Message:   viewer.Flags&GalaxyActionMessage != 0,
		Buddy:     viewer.Flags&GalaxyActionBuddy != 0,
		Missile:   viewer.Flags&GalaxyActionMissile != 0 && viewer.Missiles > 0,
		Attack:    objectType == PlanetTypePlanet || objectType == PlanetTypeMoon,
		Defend:    objectType == PlanetTypePlanet || objectType == PlanetTypeMoon,
		Transport: objectType == PlanetTypePlanet || objectType == PlanetTypeMoon,
		Destroy:   objectType == PlanetTypeMoon,
	}
}

func buildGalaxyDebris(object GalaxyObject) *GalaxyDebris {
	total := object.DebrisMetal + object.DebrisCrystal
	return &GalaxyDebris{
		ID:         object.ID,
		Metal:      object.DebrisMetal,
		Crystal:    object.DebrisCrystal,
		Harvesters: int(math.Ceil(total / float64(fleetShipCargo(FleetRecycler)))),
		Visible:    total >= GalaxyPhantomDebris,
	}
}

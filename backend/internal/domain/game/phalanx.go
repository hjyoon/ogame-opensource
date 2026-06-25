package game

import "math"

const (
	PhalanxCost = 5000

	PhalanxIssueMissingSensor        = "missing_sensor"
	PhalanxIssueInsufficientDeut     = "insufficient_deuterium"
	PhalanxIssueForbidden            = "forbidden"
	PhalanxMessageMissingSensor      = "No cheating!"
	PhalanxMessageInsufficientDeut   = "Not enough deuterium!"
	PhalanxMessageManipulation       = "Congratulations! You have won a full hour of rest without VM for attempting to manipulate a phalanx!"
	PhalanxReportHeading             = "Sensor report from the moon on the coordinates"
	PhalanxEventsHeading             = "Fleet movements"
	PhalanxCustomTargetTypeThreshold = 20001
)

type Phalanx struct {
	Commander          string
	CurrentPlanet      PlanetOverview
	PlanetSwitcher     []PlanetSummary
	Source             PhalanxPlanet
	Target             PhalanxPlanet
	Events             []FleetMission
	ActionIssue        *PhalanxActionIssue
	Cost               int
	RemainingDeuterium float64
}

type PhalanxPlanet struct {
	ID            int
	OwnerID       int
	Name          string
	Type          int
	Coordinates   Coordinates
	PhalanxLevel  int
	Deuterium     float64
	ReportHeading string
}

type PhalanxActionIssue struct {
	Code    string
	Message string
}

func NewPhalanx(overview Overview, source PhalanxPlanet, target PhalanxPlanet, events []FleetMission, issue *PhalanxActionIssue) Phalanx {
	remaining := source.Deuterium
	if issue == nil {
		remaining -= PhalanxCost
		if remaining < 0 {
			remaining = 0
		}
	}
	return Phalanx{
		Commander:          overview.Commander,
		CurrentPlanet:      overview.CurrentPlanet,
		PlanetSwitcher:     overview.PlanetSwitcher,
		Source:             source,
		Target:             target,
		Events:             BuildOverviewEvents(events),
		ActionIssue:        issue,
		Cost:               PhalanxCost,
		RemainingDeuterium: remaining,
	}
}

func PhalanxScanIssue(playerID int, source PhalanxPlanet, target PhalanxPlanet) *PhalanxActionIssue {
	switch {
	case source.PhalanxLevel <= 0:
		return &PhalanxActionIssue{Code: PhalanxIssueMissingSensor, Message: PhalanxMessageMissingSensor}
	case source.Deuterium < PhalanxCost:
		return &PhalanxActionIssue{Code: PhalanxIssueInsufficientDeut, Message: PhalanxMessageInsufficientDeut}
	case target.ID <= 0,
		target.OwnerID == playerID,
		source.OwnerID != playerID,
		!phalanxTargetTypeAllowed(target.Type),
		phalanxOutOfRange(source, target):
		return &PhalanxActionIssue{Code: PhalanxIssueForbidden, Message: PhalanxMessageManipulation}
	default:
		return nil
	}
}

func PhalanxRadius(level int) int {
	if level <= 0 {
		return 0
	}
	return level*level - 1
}

func phalanxTargetTypeAllowed(planetType int) bool {
	return planetType == PlanetTypePlanet ||
		planetType == PlanetTypeDestroyedPlanet ||
		planetType >= PhalanxCustomTargetTypeThreshold
}

func phalanxOutOfRange(source PhalanxPlanet, target PhalanxPlanet) bool {
	if source.Coordinates.Galaxy != target.Coordinates.Galaxy {
		return true
	}
	return int(math.Abs(float64(source.Coordinates.System-target.Coordinates.System))) > PhalanxRadius(source.PhalanxLevel)
}

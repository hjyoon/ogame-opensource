package game

import "strings"

type FleetTemplate struct {
	ID        int
	Name      string
	UpdatedAt int64
	Ships     []FleetTemplateShip
}

type FleetTemplateShip struct {
	ID    int
	Name  string
	Count int
}

func BuildFleetTemplate(id int, name string, updatedAt int64, counts FleetCounts) FleetTemplate {
	ships := make([]FleetTemplateShip, 0, len(FleetTemplateShipIDs()))
	for _, fleetID := range FleetTemplateShipIDs() {
		count := counts[fleetID]
		if count <= 0 {
			continue
		}
		ships = append(ships, FleetTemplateShip{
			ID:    fleetID,
			Name:  fleetName(fleetID),
			Count: count,
		})
	}
	return FleetTemplate{
		ID:        id,
		Name:      NormalizeFleetTemplateName(name),
		UpdatedAt: updatedAt,
		Ships:     ships,
	}
}

func NormalizeFleetTemplateName(name string) string {
	return strings.TrimSpace(name)
}

func FleetTemplateShipIDs() []int {
	ids := make([]int, 0, len(FleetIDs())-1)
	for _, fleetID := range FleetIDs() {
		if fleetID == FleetSolarSatellite {
			continue
		}
		ids = append(ids, fleetID)
	}
	return ids
}

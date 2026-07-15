package game

import (
	"regexp"
	"strings"
)

var (
	fleetTemplateScriptPattern = regexp.MustCompile(`(?is)<script[^>]*?>.*?</script>`)
	fleetTemplateTagPattern    = regexp.MustCompile(`(?is)<[/!]*?[^<>]*?>`)
	fleetTemplateLinePattern   = regexp.MustCompile(`([\r\n])\s+`)
)

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
	name = fleetTemplateScriptPattern.ReplaceAllString(name, "")
	name = fleetTemplateTagPattern.ReplaceAllString(name, "")
	name = fleetTemplateLinePattern.ReplaceAllString(name, "$1")
	name = strings.NewReplacer("`", "", "'", "", "\"", "", "%0", "").Replace(name)
	return truncateRunes(name, 30)
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

package game

import "math"

const MaxColonizedPlanets = 9

type ColonyTier struct {
	Minimum int
	Maximum int
	Factor  int
}

type ColonySettings struct {
	Tiers [5]ColonyTier
}

func ColonyTierIndex(position int) int {
	switch {
	case position <= 3:
		return 0
	case position <= 6:
		return 1
	case position <= 9:
		return 2
	case position <= 12:
		return 3
	default:
		return 4
	}
}

func ColonyDiameter(settings ColonySettings, position int, roll int) int {
	tier := settings.Tiers[ColonyTierIndex(position)]
	minimum := max(0, tier.Minimum)
	maximum := max(minimum, tier.Maximum)
	roll = min(maximum, max(minimum, roll))
	return roll * max(0, tier.Factor)
}

func ColonyTemperature(position int, roll int) int {
	roll = min(9, max(0, roll))
	base := -60
	switch {
	case position <= 3:
		base = 80
	case position <= 6:
		base = 30
	case position <= 9:
		base = 10
	case position <= 12:
		base = -10
	}
	return base + roll - 2*position
}

func ColonyMaxFields(diameter int) int {
	return int(math.Floor(math.Pow(float64(max(0, diameter))/1000, 2)))
}

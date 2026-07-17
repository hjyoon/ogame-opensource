package game

import (
	"errors"
	"math"
)

type ExpeditionEvent int

const (
	ExpeditionNothing ExpeditionEvent = iota
	ExpeditionAliens
	ExpeditionPirates
	ExpeditionDarkMatter
	ExpeditionBlackHole
	ExpeditionDelay
	ExpeditionAccel
	ExpeditionResources
	ExpeditionFleet
	ExpeditionTrader
)

type ExpeditionSettings struct {
	ChanceSuccess     int
	DepletedMin       int
	DepletedMed       int
	DepletedMax       int
	ChanceDepletedMin int
	ChanceDepletedMed int
	ChanceDepletedMax int
	ChanceAlien       int
	ChancePirates     int
	ChanceDM          int
	ChanceLost        int
	ChanceDelay       int
	ChanceAccel       int
	ChanceRes         int
	ChanceFleet       int
	DMFactor          int
	ScoreCaps         [8]int
	PointLimits       [8]int
	PointLimitMax     int
}

type ExpeditionTraderState struct {
	OfferID int
	Metal   float64
	Crystal float64
	Deut    float64
}

type ExpeditionInput struct {
	Settings      ExpeditionSettings
	VisitCounter  int
	HoldSeconds   int
	FlightSeconds int
	Fleet         FleetCounts
	Loaded        Resources
	TopScore      int64
	Trader        ExpeditionTraderState
}

type ExpeditionOutcome struct {
	Event           ExpeditionEvent
	Tier            int
	Variant         int
	FooterVariant   int
	ReturnSeconds   int
	DarkMatter      int
	ResourceType    int
	ResourceAmount  float64
	CargoLimited    bool
	FoundFleet      FleetCounts
	FoundFleetOrder []int
	FleetPoints     int64
	FleetUnits      int64
	Trader          ExpeditionTraderState
	UpdateTrader    bool
	OpponentFleet   FleetCounts
	HasLogbook      bool
	LogbookTier     int
	LogbookVariant  int
}

func ResolveExpedition(input ExpeditionInput, random func(int) int) (ExpeditionOutcome, error) {
	if random == nil {
		return ExpeditionOutcome{}, errors.New("expedition random source unavailable")
	}
	event, err := expeditionEvent(input.Settings, input.VisitCounter, input.HoldSeconds/3600, random)
	if err != nil {
		return ExpeditionOutcome{}, err
	}
	outcome := ExpeditionOutcome{Event: event, ReturnSeconds: max(0, input.FlightSeconds), FoundFleet: FleetCounts{}, OpponentFleet: FleetCounts{}}
	switch event {
	case ExpeditionNothing:
		outcome.Variant, err = expeditionRoll(random, 12)
	case ExpeditionAliens, ExpeditionPirates:
		outcome.Tier, outcome.Variant, err = expeditionBattleVariant(event, random)
		if err == nil {
			outcome.OpponentFleet, err = expeditionOpponentFleet(event, outcome.Tier, input.Fleet, random)
		}
	case ExpeditionDarkMatter:
		outcome.Tier, outcome.DarkMatter, outcome.Variant, err = expeditionDarkMatter(input.Settings.DMFactor, random)
	case ExpeditionBlackHole:
		outcome.ReturnSeconds = 0
		outcome.Variant, err = expeditionRoll(random, 4)
	case ExpeditionDelay:
		var multiplier int
		multiplier, err = expeditionTimingFactor(random)
		outcome.ReturnSeconds = max(0, input.FlightSeconds) + max(0, input.HoldSeconds)*multiplier
		if err == nil {
			outcome.Variant, err = expeditionRoll(random, 6)
		}
	case ExpeditionAccel:
		var divisor int
		divisor, err = expeditionTimingFactor(random)
		if divisor > 0 {
			outcome.ReturnSeconds = max(0, input.FlightSeconds) / divisor
		}
		if err == nil {
			outcome.Variant, err = expeditionRoll(random, 3)
		}
	case ExpeditionResources:
		err = resolveExpeditionResources(input, &outcome, random)
	case ExpeditionFleet:
		err = resolveExpeditionFleet(input, &outcome, random)
	case ExpeditionTrader:
		err = resolveExpeditionTrader(input.Trader, &outcome, random)
	}
	if err == nil && input.Fleet[FleetEspionageProbe] > 0 {
		outcome.HasLogbook = true
		outcome.LogbookTier = 0
		variants := 2
		if input.VisitCounter > input.Settings.DepletedMin {
			outcome.LogbookTier, variants = 1, 3
		}
		if input.VisitCounter > input.Settings.DepletedMed {
			outcome.LogbookTier = 2
		}
		if input.VisitCounter > input.Settings.DepletedMax {
			outcome.LogbookTier = 3
		}
		outcome.LogbookVariant, err = expeditionRoll(random, variants)
	}
	return outcome, err
}

func ResolveExpeditionEvent(settings ExpeditionSettings, visitCounter int, holdHours int, random func(int) int) (ExpeditionEvent, error) {
	if random == nil {
		return ExpeditionNothing, errors.New("expedition random source unavailable")
	}
	return expeditionEvent(settings, visitCounter, holdHours, random)
}

func expeditionEvent(settings ExpeditionSettings, visitCounter int, holdHours int, random func(int) int) (ExpeditionEvent, error) {
	success, err := expeditionRoll(random, 100)
	if err != nil || success >= settings.ChanceSuccess+holdHours {
		return ExpeditionNothing, err
	}
	event, err := expeditionRoll(random, 100)
	if err != nil || event < ExpeditionDepletionChance(settings, visitCounter) {
		return ExpeditionNothing, err
	}
	for _, threshold := range []struct {
		value int
		event ExpeditionEvent
	}{{settings.ChanceAlien, ExpeditionAliens}, {settings.ChancePirates, ExpeditionPirates}, {settings.ChanceDM, ExpeditionDarkMatter}, {settings.ChanceLost, ExpeditionBlackHole}, {settings.ChanceDelay, ExpeditionDelay}, {settings.ChanceAccel, ExpeditionAccel}, {settings.ChanceRes, ExpeditionResources}, {settings.ChanceFleet, ExpeditionFleet}} {
		if event >= threshold.value {
			return threshold.event, nil
		}
	}
	return ExpeditionTrader, nil
}

func ExpeditionDepletionChance(settings ExpeditionSettings, visitCounter int) int {
	if visitCounter <= settings.DepletedMin {
		return 0
	}
	if visitCounter <= settings.DepletedMed {
		return settings.ChanceDepletedMin
	}
	if visitCounter <= settings.DepletedMax {
		return settings.ChanceDepletedMed
	}
	return settings.ChanceDepletedMax
}

func expeditionBattleVariant(event ExpeditionEvent, random func(int) int) (int, int, error) {
	roll, err := expeditionRoll(random, 100)
	if err != nil {
		return 0, 0, err
	}
	tier, variants := 0, 4
	if event == ExpeditionPirates {
		variants = 5
	}
	if roll >= 99 {
		tier, variants = 2, 2
	} else if roll >= 90 {
		tier, variants = 1, 3
	}
	variant, err := expeditionRoll(random, variants)
	return tier, variant, err
}

func expeditionOpponentFleet(event ExpeditionEvent, tier int, fleet FleetCounts, random func(int) int) (FleetCounts, error) {
	result := FleetCounts{}
	for _, id := range FleetIDs() {
		count := max(0, fleet[id])
		if count == 0 {
			continue
		}
		low, high := 36, 44
		if event == ExpeditionPirates {
			low, high = 27, 33
		}
		if tier == 1 {
			low, high = 54, 66
			if event == ExpeditionPirates {
				low, high = 45, 55
			}
		} else if tier == 2 {
			low, high = 81, 99
			if event == ExpeditionPirates {
				low, high = 72, 88
			}
		}
		ratio, err := expeditionRange(random, low, high)
		if err != nil {
			return nil, err
		}
		value := float64(count*ratio) / 100
		if event == ExpeditionPirates {
			result[id] = int(math.Floor(value))
		} else {
			result[id] = int(math.Ceil(value))
		}
	}
	if event == ExpeditionPirates {
		bonus := []int{FleetLightFighter, FleetCruiser, FleetBattleship}
		amount := []int{5, 3, 2}
		result[bonus[max(0, min(2, tier))]] += amount[max(0, min(2, tier))]
	} else {
		bonus := []int{FleetHeavyFighter, FleetBattlecruiser, FleetDestroyer}
		amount := []int{5, 3, 2}
		result[bonus[max(0, min(2, tier))]] += amount[max(0, min(2, tier))]
	}
	return result, nil
}

func expeditionDarkMatter(factor int, random func(int) int) (int, int, int, error) {
	roll, err := expeditionRoll(random, 100)
	if err != nil {
		return 0, 0, 0, err
	}
	tier, low, high, variants := 0, 100, 200, 5
	if roll >= 99 {
		tier, low, high, variants = 2, 501, 2076, 2
	} else if roll >= 90 {
		tier, low, high, variants = 1, 201, 500, 3
	}
	amount, err := expeditionRange(random, low, high)
	if err != nil {
		return 0, 0, 0, err
	}
	variant, err := expeditionRoll(random, variants)
	if factor == 0 {
		factor = 1
	}
	return tier, amount * factor, variant, err
}

func expeditionTimingFactor(random func(int) int) (int, error) {
	roll, err := expeditionRoll(random, 100)
	if roll >= 99 {
		return 5, err
	}
	if roll >= 90 {
		return 3, err
	}
	return 2, err
}

func resolveExpeditionResources(input ExpeditionInput, outcome *ExpeditionOutcome, random func(int) int) error {
	resourceType, err := expeditionRoll(random, 3)
	if err != nil {
		return err
	}
	chance, err := expeditionRoll(random, 100)
	if err != nil {
		return err
	}
	tier, low, high, multiplier, variants := 0, 5, 25, 2, 4
	if chance >= 99 {
		tier, low, high, variants = 2, 51, 100, 2
	} else if chance >= 90 {
		tier, low, high, variants = 1, 26, 50, 3
	}
	roll, err := expeditionRange(random, low, high)
	if err != nil {
		return err
	}
	variant, err := expeditionRoll(random, variants)
	if err != nil {
		return err
	}
	value := float64(roll * multiplier)
	if resourceType == 1 {
		value /= 2
	} else if resourceType == 2 {
		value /= 3
	}
	amount := value * float64(expeditionPoints(input.Fleet, input.Settings, input.TopScore, true))
	cargo := expeditionCargo(input.Fleet) - int(input.Loaded.Metal+input.Loaded.Crystal+input.Loaded.Deuterium)
	if float64(max(0, cargo)) < amount {
		amount = float64(max(0, cargo))
		outcome.CargoLimited = true
		outcome.FooterVariant, err = expeditionRoll(random, 4)
	}
	outcome.Tier, outcome.Variant = tier, variant
	outcome.ResourceType, outcome.ResourceAmount = resourceType, amount
	return err
}

func resolveExpeditionFleet(input ExpeditionInput, outcome *ExpeditionOutcome, random func(int) int) error {
	chance, err := expeditionRoll(random, 100)
	if err != nil {
		return err
	}
	tier, low, high, variants := 0, 2, 50, 4
	if chance >= 99 {
		tier, low, high, variants = 2, 101, 200, 2
	} else if chance >= 90 {
		tier, low, high, variants = 1, 51, 100, 2
	}
	roll, err := expeditionRange(random, low, high)
	if err != nil {
		return err
	}
	variant, err := expeditionRoll(random, variants)
	if err != nil {
		return err
	}
	structure := max(7000, roll*expeditionPoints(input.Fleet, input.Settings, input.TopScore, false)/2)
	possible := expeditionPossibleShips(input.Fleet)
	if err := expeditionShuffle(possible, random); err != nil {
		return err
	}
	selected := []int{}
	if len(possible) > 0 {
		selectChance := 100 / len(possible)
		for _, id := range possible {
			value, err := expeditionRoll(random, 100)
			if err != nil {
				return err
			}
			if value < selectChance {
				selected = append(selected, id)
			}
		}
	}
	noStructure := false
	for _, id := range selected {
		stats, ok := CombatStatsForUnit(id)
		if !ok || stats.Structure <= 0 {
			continue
		}
		maximum := structure / stats.Structure
		if maximum <= 0 {
			noStructure = true
			break
		}
		amount, err := expeditionRange(random, 1, maximum)
		if err != nil {
			return err
		}
		outcome.FoundFleet[id] = amount
		outcome.FoundFleetOrder = append(outcome.FoundFleetOrder, id)
		structure -= amount * stats.Structure
		points, units, ok := UnitScoreForCount(id, amount)
		if ok {
			outcome.FleetPoints += points
			outcome.FleetUnits += units
		}
	}
	if noStructure {
		outcome.CargoLimited = true
		outcome.FooterVariant, err = expeditionRoll(random, 3)
	}
	outcome.Tier, outcome.Variant = tier, variant
	return err
}

func resolveExpeditionTrader(current ExpeditionTraderState, outcome *ExpeditionOutcome, random func(int) int) error {
	offer := current.OfferID
	var err error
	if offer == 0 {
		offer, err = expeditionRange(random, 1, 3)
		if err != nil {
			return err
		}
	}
	roll, err := expeditionRoll(random, 100)
	if err != nil {
		return err
	}
	proposal := ExpeditionTraderState{OfferID: offer}
	if roll < 10 {
		proposal.Metal, proposal.Crystal, proposal.Deut = 3, 2, 1
	} else if roll < 20 {
		proposal.Metal, proposal.Crystal, proposal.Deut = 2.4, 1.6, 0.8
		if offer == 1 {
			proposal.Metal = 3
		} else if offer == 2 {
			proposal.Crystal = 2
		} else {
			proposal.Deut = 1
		}
	} else {
		switch offer {
		case 1:
			proposal.Metal = 3
			proposal.Crystal, err = expeditionRate(random, 140, 200)
			if err == nil {
				proposal.Deut, err = expeditionRate(random, 70, 100)
			}
		case 2:
			proposal.Metal, err = expeditionRate(random, 210, 300)
			proposal.Crystal = 2
			if err == nil {
				proposal.Deut, err = expeditionRate(random, 70, 100)
			}
		default:
			proposal.Metal, err = expeditionRate(random, 210, 300)
			if err == nil {
				proposal.Crystal, err = expeditionRate(random, 140, 200)
			}
			proposal.Deut = 1
		}
		if err != nil {
			return err
		}
	}
	outcome.Trader = proposal
	outcome.UpdateTrader = current.OfferID == 0 || proposal.Metal+proposal.Crystal+proposal.Deut > current.Metal+current.Crystal+current.Deut
	outcome.Variant, err = expeditionRoll(random, 2)
	return err
}

func expeditionPoints(fleet FleetCounts, settings ExpeditionSettings, topScore int64, minimum bool) int {
	total := 0
	for id, count := range fleet {
		if count <= 0 {
			continue
		}
		cost, ok := CombatUnitCost(id)
		if ok {
			total += int(cost.Metal+cost.Crystal) * count
		}
	}
	points := total / ScoreDisplayScale
	if minimum {
		points = max(200, points)
	}
	limit := settings.PointLimitMax
	score := int(topScore / ScoreDisplayScale)
	for index, cap := range settings.ScoreCaps {
		if score < cap {
			limit = settings.PointLimits[index]
			break
		}
	}
	if limit > 0 {
		points = min(points, limit)
	}
	return points
}

func expeditionCargo(fleet FleetCounts) int {
	total := 0
	for id, count := range fleet {
		if id != FleetEspionageProbe && count > 0 {
			total += FleetCargoCapacity(id) * count
		}
	}
	return total
}

func expeditionPossibleShips(fleet FleetCounts) []int {
	levels := []struct {
		id   int
		list []int
	}{
		{FleetEspionageProbe, []int{FleetEspionageProbe, FleetSmallCargo}},
		{FleetSmallCargo, []int{FleetEspionageProbe, FleetSmallCargo, FleetLargeCargo}},
		{FleetLightFighter, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo}},
		{FleetLargeCargo, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter}},
		{FleetHeavyFighter, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser}},
		{FleetCruiser, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser, FleetBattleship}},
		{FleetBattleship, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser, FleetBattleship, FleetBattlecruiser}},
		{FleetBattlecruiser, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser, FleetBattleship, FleetBattlecruiser, FleetBomber}},
		{FleetBomber, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser, FleetBattleship, FleetBattlecruiser, FleetBomber, FleetDestroyer}},
		{FleetDestroyer, []int{FleetEspionageProbe, FleetSmallCargo, FleetLightFighter, FleetLargeCargo, FleetHeavyFighter, FleetCruiser, FleetBattleship, FleetBattlecruiser, FleetBomber, FleetDestroyer}},
	}
	result := []int{}
	for _, level := range levels {
		if fleet[level.id] > 0 {
			result = append([]int(nil), level.list...)
		}
	}
	return result
}

func expeditionShuffle(values []int, random func(int) int) error {
	for index := len(values) - 1; index > 0; index-- {
		other, err := expeditionRoll(random, index+1)
		if err != nil {
			return err
		}
		values[index], values[other] = values[other], values[index]
	}
	return nil
}

func expeditionRate(random func(int) int, low int, high int) (float64, error) {
	value, err := expeditionRange(random, low, high)
	return float64(value) / 100, err
}

func expeditionRange(random func(int) int, low int, high int) (int, error) {
	value, err := expeditionRoll(random, high-low+1)
	return low + value, err
}

func expeditionRoll(random func(int) int, maximum int) (int, error) {
	if maximum <= 0 {
		return 0, errors.New("invalid expedition random range")
	}
	value := random(maximum)
	if value < 0 || value >= maximum {
		return 0, errors.New("expedition random result out of range")
	}
	return value, nil
}

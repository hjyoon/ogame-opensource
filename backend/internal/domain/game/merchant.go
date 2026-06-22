package game

import "math"

const (
	MerchantDMCost = 2500

	MerchantResourceMetal     = 1
	MerchantResourceCrystal   = 2
	MerchantResourceDeuterium = 3

	MerchantIssueNotEnoughDarkMatter = "not_enough_dark_matter"
	MerchantIssueNotEnoughResource   = "not_enough_resource"
	MerchantIssueNotEnoughStorage    = "not_enough_storage"
)

type Merchant struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	User           MerchantUser
	ActiveOfferID  int
	Rates          MerchantRates
	Rows           []MerchantResourceRow
}

type MerchantUser struct {
	PaidDarkMatter int
	FreeDarkMatter int
}

type MerchantRates struct {
	Metal     float64
	Crystal   float64
	Deuterium float64
}

type MerchantResourceRow struct {
	ID          int
	Name        string
	Offered     bool
	Value       int
	FreeStorage int
	Rate        float64
}

type MerchantMutation struct {
	Action  string
	OfferID int
	Values  MerchantTradeValues
}

type MerchantTradeValues struct {
	Metal     int
	Crystal   int
	Deuterium int
}

type MerchantTradeResult struct {
	Changed   bool
	Metal     int
	Crystal   int
	Deuterium int
}

type MerchantActionIssue struct {
	Code    string
	Message string
}

func NewMerchant(overview Overview, user MerchantUser, activeOfferID int, rates MerchantRates) Merchant {
	activeOfferID = NormalizeMerchantOfferID(activeOfferID)
	if activeOfferID == 0 {
		rates = MerchantRates{}
	}
	return Merchant{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		User:           user,
		ActiveOfferID:  activeOfferID,
		Rates:          rates,
		Rows:           MerchantRows(overview.CurrentPlanet.Resources, activeOfferID, rates),
	}
}

func NormalizeMerchantOfferID(offerID int) int {
	if offerID < MerchantResourceMetal || offerID > MerchantResourceDeuterium {
		return 0
	}
	return offerID
}

func MerchantRows(resources Resources, activeOfferID int, rates MerchantRates) []MerchantResourceRow {
	return []MerchantResourceRow{
		{
			ID:          MerchantResourceMetal,
			Name:        "Metal",
			Offered:     activeOfferID == MerchantResourceMetal,
			Value:       int(math.Floor(resources.Metal)),
			FreeStorage: maxInt(0, resources.MetalCapacity-int(math.Floor(resources.Metal))),
			Rate:        rates.Metal,
		},
		{
			ID:          MerchantResourceCrystal,
			Name:        "Crystal",
			Offered:     activeOfferID == MerchantResourceCrystal,
			Value:       int(math.Floor(resources.Crystal)),
			FreeStorage: maxInt(0, resources.CrystalCapacity-int(math.Floor(resources.Crystal))),
			Rate:        rates.Crystal,
		},
		{
			ID:          MerchantResourceDeuterium,
			Name:        "Deuterium",
			Offered:     activeOfferID == MerchantResourceDeuterium,
			Value:       int(math.Floor(resources.Deuterium)),
			FreeStorage: maxInt(0, resources.DeuteriumCapacity-int(math.Floor(resources.Deuterium))),
			Rate:        rates.Deuterium,
		},
	}
}

func SpendMerchantDarkMatter(user MerchantUser) (MerchantUser, bool) {
	if user.PaidDarkMatter+user.FreeDarkMatter < MerchantDMCost {
		return user, false
	}
	if user.PaidDarkMatter >= MerchantDMCost {
		user.PaidDarkMatter -= MerchantDMCost
		return user, true
	}
	user.FreeDarkMatter -= MerchantDMCost - user.PaidDarkMatter
	user.PaidDarkMatter = 0
	return user, true
}

func GenerateMerchantRates(offerID int, roll int, randomA int, randomB int) (MerchantRates, bool) {
	offerID = NormalizeMerchantOfferID(offerID)
	if offerID == 0 {
		return MerchantRates{}, false
	}
	if roll < 10 {
		return MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1}, true
	}
	if roll < 20 {
		switch offerID {
		case MerchantResourceMetal:
			return MerchantRates{Metal: 3, Crystal: 1.60, Deuterium: 0.80}, true
		case MerchantResourceCrystal:
			return MerchantRates{Metal: 2.40, Crystal: 2, Deuterium: 0.80}, true
		case MerchantResourceDeuterium:
			return MerchantRates{Metal: 2.40, Crystal: 1.60, Deuterium: 1}, true
		}
	}
	switch offerID {
	case MerchantResourceMetal:
		return MerchantRates{Metal: 3, Crystal: float64(clampInt(randomA, 140, 200)) / 100, Deuterium: float64(clampInt(randomB, 70, 100)) / 100}, true
	case MerchantResourceCrystal:
		return MerchantRates{Metal: float64(clampInt(randomA, 210, 300)) / 100, Crystal: 2, Deuterium: float64(clampInt(randomB, 70, 100)) / 100}, true
	case MerchantResourceDeuterium:
		return MerchantRates{Metal: float64(clampInt(randomA, 210, 300)) / 100, Crystal: float64(clampInt(randomB, 140, 200)) / 100, Deuterium: 1}, true
	default:
		return MerchantRates{}, false
	}
}

func ResolveMerchantTrade(offerID int, rates MerchantRates, resources Resources, values MerchantTradeValues) (MerchantTradeResult, *MerchantActionIssue) {
	values = normalizeMerchantTradeValues(values)
	metal := int(math.Floor(resources.Metal))
	crystal := int(math.Floor(resources.Crystal))
	deuterium := int(math.Floor(resources.Deuterium))
	switch NormalizeMerchantOfferID(offerID) {
	case MerchantResourceMetal:
		newCrystal := crystal + values.Crystal
		newDeuterium := deuterium + values.Deuterium
		cost := int(math.Floor(float64(values.Crystal)*safeRateRatio(rates.Metal, rates.Crystal))) +
			int(math.Floor(float64(values.Deuterium)*safeRateRatio(rates.Metal, rates.Deuterium)))
		if cost > metal {
			return MerchantTradeResult{}, MerchantNotEnoughResourceIssue()
		}
		if newCrystal > resources.MetalCapacity || newDeuterium > resources.DeuteriumCapacity {
			return MerchantTradeResult{}, MerchantNotEnoughStorageIssue()
		}
		if cost <= 0 {
			return MerchantTradeResult{}, nil
		}
		return MerchantTradeResult{Changed: true, Metal: metal - cost, Crystal: newCrystal, Deuterium: newDeuterium}, nil
	case MerchantResourceCrystal:
		newMetal := metal + values.Metal
		newDeuterium := deuterium + values.Deuterium
		cost := int(math.Floor(float64(values.Metal)*safeRateRatio(rates.Crystal, rates.Metal))) +
			int(math.Floor(float64(values.Deuterium)*safeRateRatio(rates.Crystal, rates.Deuterium)))
		if cost > crystal {
			return MerchantTradeResult{}, MerchantNotEnoughResourceIssue()
		}
		if newMetal > resources.MetalCapacity || newDeuterium > resources.DeuteriumCapacity {
			return MerchantTradeResult{}, MerchantNotEnoughStorageIssue()
		}
		if cost <= 0 {
			return MerchantTradeResult{}, nil
		}
		return MerchantTradeResult{Changed: true, Metal: newMetal, Crystal: crystal - cost, Deuterium: newDeuterium}, nil
	case MerchantResourceDeuterium:
		newMetal := metal + values.Metal
		newCrystal := crystal + values.Crystal
		cost := int(math.Floor(float64(values.Metal)*safeRateRatio(rates.Deuterium, rates.Metal))) +
			int(math.Floor(float64(values.Crystal)*safeRateRatio(rates.Deuterium, rates.Crystal)))
		if cost > deuterium {
			return MerchantTradeResult{}, MerchantNotEnoughResourceIssue()
		}
		if newMetal > resources.MetalCapacity || newCrystal > resources.CrystalCapacity {
			return MerchantTradeResult{}, MerchantNotEnoughStorageIssue()
		}
		if cost <= 0 {
			return MerchantTradeResult{}, nil
		}
		return MerchantTradeResult{Changed: true, Metal: newMetal, Crystal: newCrystal, Deuterium: deuterium - cost}, nil
	default:
		return MerchantTradeResult{}, nil
	}
}

func MerchantNotEnoughDarkMatterIssue() *MerchantActionIssue {
	return &MerchantActionIssue{Code: MerchantIssueNotEnoughDarkMatter, Message: "Not enough dark matter!"}
}

func MerchantNotEnoughResourceIssue() *MerchantActionIssue {
	return &MerchantActionIssue{Code: MerchantIssueNotEnoughResource, Message: "Not enough material to trade!"}
}

func MerchantNotEnoughStorageIssue() *MerchantActionIssue {
	return &MerchantActionIssue{Code: MerchantIssueNotEnoughStorage, Message: "Not enough storage space!"}
}

func normalizeMerchantTradeValues(values MerchantTradeValues) MerchantTradeValues {
	values.Metal = absInt(values.Metal)
	values.Crystal = absInt(values.Crystal)
	values.Deuterium = absInt(values.Deuterium)
	return values
}

func safeRateRatio(numerator float64, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

package game

import "testing"

func TestExpeditionEventMatchesLegacyThresholds(t *testing.T) {
	settings := expeditionTestSettings()
	for _, test := range []struct {
		name  string
		visit int
		rolls []int
		want  ExpeditionEvent
	}{
		{name: "failed success", rolls: []int{70}, want: ExpeditionNothing},
		{name: "depleted", visit: 26, rolls: []int{0, 24}, want: ExpeditionNothing},
		{name: "alien", rolls: []int{0, 95}, want: ExpeditionAliens},
		{name: "pirate", rolls: []int{0, 85}, want: ExpeditionPirates},
		{name: "dark matter", rolls: []int{0, 70}, want: ExpeditionDarkMatter},
		{name: "black hole", rolls: []int{0, 69}, want: ExpeditionBlackHole},
		{name: "delay", rolls: []int{0, 63}, want: ExpeditionDelay},
		{name: "accel", rolls: []int{0, 60}, want: ExpeditionAccel},
		{name: "resources", rolls: []int{0, 25}, want: ExpeditionResources},
		{name: "fleet", rolls: []int{0, 1}, want: ExpeditionFleet},
		{name: "trader", rolls: []int{0, 0}, want: ExpeditionTrader},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := expeditionEvent(settings, test.visit, 0, expeditionRandomSequence(t, test.rolls...))
			if err != nil || got != test.want {
				t.Fatalf("event=%v want=%v err=%v", got, test.want, err)
			}
		})
	}
	if ExpeditionDepletionChance(settings, 25) != 0 || ExpeditionDepletionChance(settings, 26) != 25 || ExpeditionDepletionChance(settings, 51) != 50 || ExpeditionDepletionChance(settings, 76) != 75 {
		t.Fatal("unexpected depletion thresholds")
	}
}

func TestResolveExpeditionDarkMatterTiers(t *testing.T) {
	settings := expeditionTestSettings()
	settings.DMFactor = 3
	for _, test := range []struct {
		name  string
		rolls []int
		tier  int
		want  int
	}{
		{name: "small", rolls: []int{0, 70, 0, 100, 0}, tier: 0, want: 600},
		{name: "medium", rolls: []int{0, 70, 90, 299, 0}, tier: 1, want: 1500},
		{name: "large", rolls: []int{0, 70, 99, 1575, 0}, tier: 2, want: 6228},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := ResolveExpedition(ExpeditionInput{Settings: settings}, expeditionRandomSequence(t, test.rolls...))
			if err != nil || outcome.Event != ExpeditionDarkMatter || outcome.Tier != test.tier || outcome.DarkMatter != test.want || outcome.ReturnSeconds != 0 {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
		})
	}
}

func TestResolveExpeditionResourcesUsesPointsAndCargo(t *testing.T) {
	settings := expeditionTestSettings()
	settings.PointLimits[0] = 9000
	settings.PointLimitMax = 12000
	input := ExpeditionInput{
		Settings: settings, FlightSeconds: 120,
		Fleet: FleetCounts{FleetLargeCargo: 20, FleetEspionageProbe: 1},
	}
	outcome, err := ResolveExpedition(input, expeditionRandomSequence(t, 0, 25, 0, 99, 49, 0, 0))
	if err != nil || outcome.Event != ExpeditionResources || outcome.ResourceType != 0 || outcome.ResourceAmount != 48200 || outcome.CargoLimited {
		t.Fatalf("uncapped resource outcome=%+v err=%v", outcome, err)
	}
	input.Loaded = Resources{Metal: 499900}
	outcome, err = ResolveExpedition(input, expeditionRandomSequence(t, 0, 25, 2, 99, 49, 0, 3, 0))
	if err != nil || outcome.ResourceAmount != 100 || !outcome.CargoLimited || outcome.FooterVariant != 3 {
		t.Fatalf("cargo-limited outcome=%+v err=%v", outcome, err)
	}
}

func TestResolveExpeditionTimingAndTrader(t *testing.T) {
	settings := expeditionTestSettings()
	delay, err := ResolveExpedition(ExpeditionInput{Settings: settings, HoldSeconds: 3600, FlightSeconds: 120}, expeditionRandomSequence(t, 0, 63, 99, 0))
	if err != nil || delay.Event != ExpeditionDelay || delay.ReturnSeconds != 18120 {
		t.Fatalf("delay=%+v err=%v", delay, err)
	}
	accel, err := ResolveExpedition(ExpeditionInput{Settings: settings, FlightSeconds: 120}, expeditionRandomSequence(t, 0, 60, 90, 0))
	if err != nil || accel.Event != ExpeditionAccel || accel.ReturnSeconds != 40 {
		t.Fatalf("accel=%+v err=%v", accel, err)
	}
	trader, err := ResolveExpedition(ExpeditionInput{Settings: settings}, expeditionRandomSequence(t, 0, 0, 1, 20, 90, 30, 1))
	if err != nil || trader.Event != ExpeditionTrader || !trader.UpdateTrader || trader.Trader.OfferID != 2 || trader.Trader.Metal != 3 || trader.Trader.Crystal != 2 || trader.Trader.Deut != 1 {
		t.Fatalf("trader=%+v err=%v", trader, err)
	}
	current := ExpeditionTraderState{OfferID: 1, Metal: 3, Crystal: 2, Deut: 1}
	trader, err = ResolveExpedition(ExpeditionInput{Settings: settings, Trader: current}, expeditionRandomSequence(t, 0, 0, 10, 0))
	if err != nil || trader.UpdateTrader {
		t.Fatalf("inferior trader offer must not replace current: %+v err=%v", trader, err)
	}
}

func TestResolveExpeditionBattleAndFleetDiscovery(t *testing.T) {
	settings := expeditionTestSettings()
	aliens, err := ResolveExpedition(ExpeditionInput{Settings: settings, FlightSeconds: 120, Fleet: FleetCounts{FleetSmallCargo: 10}}, expeditionRandomSequence(t, 0, 95, 99, 1, 18))
	if err != nil || aliens.Event != ExpeditionAliens || aliens.Tier != 2 || aliens.OpponentFleet[FleetSmallCargo] != 10 || aliens.OpponentFleet[FleetDestroyer] != 2 {
		t.Fatalf("aliens=%+v err=%v", aliens, err)
	}
	pirates, err := ResolveExpedition(ExpeditionInput{Settings: settings, Fleet: FleetCounts{FleetSmallCargo: 10}}, expeditionRandomSequence(t, 0, 85, 0, 0, 6))
	if err != nil || pirates.Event != ExpeditionPirates || pirates.OpponentFleet[FleetSmallCargo] != 3 || pirates.OpponentFleet[FleetLightFighter] != 5 {
		t.Fatalf("pirates=%+v err=%v", pirates, err)
	}

	discovery, err := ResolveExpedition(ExpeditionInput{
		Settings: settings, Fleet: FleetCounts{FleetEspionageProbe: 1}, TopScore: 1,
	}, expeditionRandomSequence(t, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0))
	if err != nil || discovery.Event != ExpeditionFleet || len(discovery.FoundFleet) == 0 || discovery.FleetPoints <= 0 || discovery.FleetUnits <= 0 {
		t.Fatalf("fleet discovery=%+v err=%v", discovery, err)
	}
}

func TestResolveExpeditionRejectsInvalidRandomSource(t *testing.T) {
	if _, err := ResolveExpedition(ExpeditionInput{}, nil); err == nil {
		t.Fatal("missing random source must fail")
	}
	if _, err := ResolveExpedition(ExpeditionInput{Settings: expeditionTestSettings()}, func(max int) int { return max }); err == nil {
		t.Fatal("out-of-range random source must fail")
	}
}

func TestExpeditionRemainingRandomizedBranches(t *testing.T) {
	t.Run("battle variants and opponents", func(t *testing.T) {
		tier, variant, err := expeditionBattleVariant(ExpeditionAliens, expeditionRandomSequence(t, 0, 3))
		if err != nil || tier != 0 || variant != 3 {
			t.Fatalf("weak alien variant: tier=%d variant=%d err=%v", tier, variant, err)
		}
		tier, variant, err = expeditionBattleVariant(ExpeditionPirates, expeditionRandomSequence(t, 90, 2))
		if err != nil || tier != 1 || variant != 2 {
			t.Fatalf("medium pirate variant: tier=%d variant=%d err=%v", tier, variant, err)
		}
		aliens, err := expeditionOpponentFleet(ExpeditionAliens, 1, FleetCounts{FleetSmallCargo: 10}, expeditionRandomSequence(t, 0))
		if err != nil || aliens[FleetSmallCargo] != 6 || aliens[FleetBattlecruiser] != 3 {
			t.Fatalf("medium aliens=%v err=%v", aliens, err)
		}
		pirates, err := expeditionOpponentFleet(ExpeditionPirates, 1, FleetCounts{FleetSmallCargo: 10}, expeditionRandomSequence(t, 0))
		if err != nil || pirates[FleetSmallCargo] != 4 || pirates[FleetCruiser] != 3 {
			t.Fatalf("medium pirates=%v err=%v", pirates, err)
		}
	})

	t.Run("timing and default dark matter factor", func(t *testing.T) {
		for _, test := range []struct {
			roll int
			want int
		}{{0, 2}, {90, 3}, {99, 5}} {
			got, err := expeditionTimingFactor(expeditionRandomSequence(t, test.roll))
			if err != nil || got != test.want {
				t.Fatalf("timing roll %d: got=%d err=%v", test.roll, got, err)
			}
		}
		_, amount, _, err := expeditionDarkMatter(0, expeditionRandomSequence(t, 0, 0, 0))
		if err != nil || amount != 100 {
			t.Fatalf("default DM factor amount=%d err=%v", amount, err)
		}
	})

	t.Run("resource types and tiers", func(t *testing.T) {
		input := ExpeditionInput{Settings: expeditionTestSettings(), Fleet: FleetCounts{FleetLargeCargo: 20}}
		for _, test := range []struct {
			rolls  []int
			typeID int
			tier   int
		}{{[]int{1, 90, 0, 0}, 1, 1}, {[]int{2, 0, 0, 0}, 2, 0}} {
			outcome := ExpeditionOutcome{}
			if err := resolveExpeditionResources(input, &outcome, expeditionRandomSequence(t, test.rolls...)); err != nil || outcome.ResourceType != test.typeID || outcome.Tier != test.tier || outcome.ResourceAmount <= 0 {
				t.Fatalf("resources=%+v err=%v", outcome, err)
			}
		}
	})

	t.Run("fleet selection edges", func(t *testing.T) {
		outcome := ExpeditionOutcome{FoundFleet: FleetCounts{}}
		input := ExpeditionInput{Settings: expeditionTestSettings(), Fleet: FleetCounts{FleetEspionageProbe: 1}}
		if err := resolveExpeditionFleet(input, &outcome, expeditionRandomSequence(t, 0, 0, 0, 0, 99, 99)); err != nil || len(outcome.FoundFleet) != 0 {
			t.Fatalf("empty selection=%+v err=%v", outcome, err)
		}
		outcome = ExpeditionOutcome{FoundFleet: FleetCounts{}}
		input.Fleet = FleetCounts{FleetLargeCargo: 1}
		rolls := []int{0, 0, 0, 4, 3, 2, 1, 99, 99, 99, 99, 0, 0}
		if err := resolveExpeditionFleet(input, &outcome, expeditionRandomSequence(t, rolls...)); err != nil || !outcome.CargoLimited || len(outcome.FoundFleet) != 0 {
			t.Fatalf("structure-limited selection=%+v err=%v", outcome, err)
		}
	})

	t.Run("trader offers", func(t *testing.T) {
		for _, test := range []struct {
			current ExpeditionTraderState
			rolls   []int
		}{{ExpeditionTraderState{OfferID: 1}, []int{10, 0}}, {ExpeditionTraderState{OfferID: 1}, []int{20, 0, 0, 0}}, {ExpeditionTraderState{OfferID: 2}, []int{20, 0, 0, 0}}, {ExpeditionTraderState{OfferID: 3}, []int{20, 0, 0, 0}}} {
			outcome := ExpeditionOutcome{}
			if err := resolveExpeditionTrader(test.current, &outcome, expeditionRandomSequence(t, test.rolls...)); err != nil || outcome.Trader.OfferID != test.current.OfferID || outcome.Trader.Metal <= 0 || outcome.Trader.Crystal <= 0 || outcome.Trader.Deut <= 0 {
				t.Fatalf("trader=%+v err=%v", outcome, err)
			}
		}
	})
}

func TestResolveExpeditionLogbookAndLoss(t *testing.T) {
	settings := expeditionTestSettings()
	blackHole, err := ResolveExpedition(ExpeditionInput{Settings: settings, FlightSeconds: 120}, expeditionRandomSequence(t, 0, 69, 3))
	if err != nil || blackHole.Event != ExpeditionBlackHole || blackHole.ReturnSeconds != 0 || blackHole.Variant != 3 {
		t.Fatalf("black hole=%+v err=%v", blackHole, err)
	}
	settings.ChanceSuccess = 0
	for _, visit := range []int{0, 26, 51, 76} {
		outcome, err := ResolveExpedition(ExpeditionInput{Settings: settings, VisitCounter: visit, Fleet: FleetCounts{FleetEspionageProbe: 1}}, expeditionRandomSequence(t, 0, 0, 0))
		if err != nil || !outcome.HasLogbook {
			t.Fatalf("visit %d outcome=%+v err=%v", visit, outcome, err)
		}
	}
}

func TestExpeditionHelpersRejectInvalidRandomResults(t *testing.T) {
	bad := func(maximum int) int { return maximum }
	settings := expeditionTestSettings()
	for name, call := range map[string]func() error{
		"battle variant": func() error { _, _, err := expeditionBattleVariant(ExpeditionAliens, bad); return err },
		"opponent": func() error {
			_, err := expeditionOpponentFleet(ExpeditionAliens, 0, FleetCounts{FleetSmallCargo: 1}, bad)
			return err
		},
		"dark matter": func() error { _, _, _, err := expeditionDarkMatter(1, bad); return err },
		"resources": func() error {
			return resolveExpeditionResources(ExpeditionInput{Settings: settings}, &ExpeditionOutcome{}, bad)
		},
		"fleet": func() error {
			return resolveExpeditionFleet(ExpeditionInput{Settings: settings}, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, bad)
		},
		"trader":  func() error { return resolveExpeditionTrader(ExpeditionTraderState{}, &ExpeditionOutcome{}, bad) },
		"shuffle": func() error { return expeditionShuffle([]int{1, 2}, bad) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid random result must fail")
			}
		})
	}
	if _, err := expeditionRoll(func(int) int { return 0 }, 0); err == nil {
		t.Fatal("zero random range must fail")
	}
}

func TestExpeditionDownstreamRandomFailures(t *testing.T) {
	settings := expeditionTestSettings()
	resourceInput := ExpeditionInput{Settings: settings, Fleet: FleetCounts{FleetLargeCargo: 20}}
	fleetInput := ExpeditionInput{Settings: settings, Fleet: FleetCounts{FleetEspionageProbe: 1}}
	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "dark matter amount", call: func() error { _, _, _, err := expeditionDarkMatter(1, expeditionRawSequence(0, -1)); return err }},
		{name: "resource chance", call: func() error {
			return resolveExpeditionResources(resourceInput, &ExpeditionOutcome{}, expeditionRawSequence(0, -1))
		}},
		{name: "resource amount", call: func() error {
			return resolveExpeditionResources(resourceInput, &ExpeditionOutcome{}, expeditionRawSequence(0, 0, -1))
		}},
		{name: "resource variant", call: func() error {
			return resolveExpeditionResources(resourceInput, &ExpeditionOutcome{}, expeditionRawSequence(0, 0, 0, -1))
		}},
		{name: "large fleet amount", call: func() error {
			return resolveExpeditionFleet(fleetInput, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, expeditionRawSequence(99, -1))
		}},
		{name: "medium fleet variant", call: func() error {
			return resolveExpeditionFleet(fleetInput, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, expeditionRawSequence(90, 0, -1))
		}},
		{name: "fleet shuffle", call: func() error {
			return resolveExpeditionFleet(fleetInput, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, expeditionRawSequence(0, 0, 0, -1))
		}},
		{name: "fleet selection", call: func() error {
			return resolveExpeditionFleet(fleetInput, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, expeditionRawSequence(0, 0, 0, 0, -1))
		}},
		{name: "fleet discovered amount", call: func() error {
			return resolveExpeditionFleet(fleetInput, &ExpeditionOutcome{FoundFleet: FleetCounts{}}, expeditionRawSequence(0, 0, 0, 0, 0, 0, -1))
		}},
		{name: "trader roll", call: func() error {
			return resolveExpeditionTrader(ExpeditionTraderState{OfferID: 1}, &ExpeditionOutcome{}, expeditionRawSequence(-1))
		}},
		{name: "trader rate", call: func() error {
			return resolveExpeditionTrader(ExpeditionTraderState{OfferID: 1}, &ExpeditionOutcome{}, expeditionRawSequence(20, -1))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("downstream invalid random result must fail")
			}
		})
	}
}

func TestExpeditionRemainingOpponentAndTraderBranches(t *testing.T) {
	pirates, err := expeditionOpponentFleet(ExpeditionPirates, 2, FleetCounts{FleetSmallCargo: 10}, expeditionRawSequence(0))
	if err != nil || pirates[FleetSmallCargo] != 7 || pirates[FleetBattleship] != 2 {
		t.Fatalf("strong pirates=%v err=%v", pirates, err)
	}
	for offer, wantRateTotal := range map[int]float64{2: 5.2, 3: 5.0} {
		outcome := ExpeditionOutcome{}
		if err := resolveExpeditionTrader(ExpeditionTraderState{OfferID: offer}, &outcome, expeditionRawSequence(10, 0)); err != nil {
			t.Fatalf("offer %d: %v", offer, err)
		}
		if outcome.Trader.Metal+outcome.Trader.Crystal+outcome.Trader.Deut != wantRateTotal {
			t.Fatalf("offer %d proposal=%+v", offer, outcome.Trader)
		}
	}
}

func expeditionTestSettings() ExpeditionSettings {
	return ExpeditionSettings{
		ChanceSuccess: 70, DepletedMin: 25, DepletedMed: 50, DepletedMax: 75,
		ChanceDepletedMin: 25, ChanceDepletedMed: 50, ChanceDepletedMax: 75,
		ChanceAlien: 95, ChancePirates: 85, ChanceDM: 70, ChanceLost: 69,
		ChanceDelay: 63, ChanceAccel: 60, ChanceRes: 25, ChanceFleet: 1,
	}
}

func expeditionRandomSequence(t *testing.T, values ...int) func(int) int {
	t.Helper()
	return func(maximum int) int {
		t.Helper()
		if len(values) == 0 {
			t.Fatalf("unexpected expedition random call with maximum %d", maximum)
		}
		value := values[0]
		values = values[1:]
		if value < 0 || value >= maximum {
			t.Fatalf("test roll %d outside [0,%d)", value, maximum)
		}
		return value
	}
}

func expeditionRawSequence(values ...int) func(int) int {
	return func(int) int {
		if len(values) == 0 {
			return -1
		}
		value := values[0]
		values = values[1:]
		return value
	}
}

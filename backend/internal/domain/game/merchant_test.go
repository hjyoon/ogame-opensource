package game

import "testing"

func TestNewMerchantBuildsRowsAndNormalizesOffer(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID: 1,
			Resources: Resources{
				Metal:             100.9,
				Crystal:           40.2,
				Deuterium:         7.8,
				MetalCapacity:     90,
				CrystalCapacity:   100,
				DeuteriumCapacity: 100,
			},
		},
		PlanetSwitcher: []PlanetSummary{{ID: 1, Current: true}},
	}
	merchant := NewMerchant(overview, MerchantUser{PaidDarkMatter: 3000}, MerchantResourceMetal, MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1})
	if merchant.Commander != "legor" || merchant.ActiveOfferID != MerchantResourceMetal || !merchant.Rows[0].Offered {
		t.Fatalf("unexpected merchant summary: %+v", merchant)
	}
	if merchant.Rows[0].Value != 100 || merchant.Rows[0].FreeStorage != 0 || merchant.Rows[1].FreeStorage != 60 {
		t.Fatalf("unexpected merchant rows: %+v", merchant.Rows)
	}

	merchant = NewMerchant(overview, MerchantUser{}, 99, MerchantRates{Metal: 3})
	if merchant.ActiveOfferID != 0 || merchant.Rates.Metal != 0 {
		t.Fatalf("invalid offer should clear active rates, got %+v", merchant)
	}
}

func TestMerchantDarkMatterSpending(t *testing.T) {
	spent, ok := SpendMerchantDarkMatter(MerchantUser{PaidDarkMatter: 3000, FreeDarkMatter: 10})
	if !ok || spent.PaidDarkMatter != 500 || spent.FreeDarkMatter != 10 {
		t.Fatalf("expected paid DM to be spent first, got %+v ok=%v", spent, ok)
	}
	spent, ok = SpendMerchantDarkMatter(MerchantUser{PaidDarkMatter: 1000, FreeDarkMatter: 2000})
	if !ok || spent.PaidDarkMatter != 0 || spent.FreeDarkMatter != 500 {
		t.Fatalf("expected free DM remainder spend, got %+v ok=%v", spent, ok)
	}
	spent, ok = SpendMerchantDarkMatter(MerchantUser{PaidDarkMatter: 1000, FreeDarkMatter: 1000})
	if ok || spent.PaidDarkMatter != 1000 || spent.FreeDarkMatter != 1000 {
		t.Fatalf("insufficient DM should preserve balances, got %+v ok=%v", spent, ok)
	}
}

func TestGenerateMerchantRates(t *testing.T) {
	if _, ok := GenerateMerchantRates(99, 0, 0, 0); ok {
		t.Fatalf("invalid offer should not generate rates")
	}
	rates, ok := GenerateMerchantRates(MerchantResourceMetal, 5, 0, 0)
	if !ok || rates.Metal != 3 || rates.Crystal != 2 || rates.Deuterium != 1 {
		t.Fatalf("unexpected perfect rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceCrystal, 15, 0, 0)
	if !ok || rates.Metal != 2.4 || rates.Crystal != 2 || rates.Deuterium != 0.8 {
		t.Fatalf("unexpected good crystal rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceMetal, 15, 0, 0)
	if !ok || rates.Metal != 3 || rates.Crystal != 1.6 || rates.Deuterium != 0.8 {
		t.Fatalf("unexpected good metal rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceDeuterium, 15, 0, 0)
	if !ok || rates.Metal != 2.4 || rates.Crystal != 1.6 || rates.Deuterium != 1 {
		t.Fatalf("unexpected good deuterium rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceMetal, 50, 999, -1)
	if !ok || rates.Metal != 3 || rates.Crystal != 2 || rates.Deuterium != 0.7 {
		t.Fatalf("unexpected clamped metal rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceCrystal, 50, 999, -1)
	if !ok || rates.Metal != 3 || rates.Crystal != 2 || rates.Deuterium != 0.7 {
		t.Fatalf("unexpected clamped crystal rates: %+v ok=%v", rates, ok)
	}
	rates, ok = GenerateMerchantRates(MerchantResourceDeuterium, 50, 999, -1)
	if !ok || rates.Metal != 3 || rates.Crystal != 1.4 || rates.Deuterium != 1 {
		t.Fatalf("unexpected clamped deuterium rates: %+v ok=%v", rates, ok)
	}
}

func TestResolveMerchantTradeCases(t *testing.T) {
	resources := Resources{
		Metal:             1000,
		Crystal:           1000,
		Deuterium:         1000,
		MetalCapacity:     2000,
		CrystalCapacity:   2000,
		DeuteriumCapacity: 2000,
	}
	rates := MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1}

	result, issue := ResolveMerchantTrade(MerchantResourceMetal, rates, resources, MerchantTradeValues{Crystal: 100, Deuterium: 50})
	if issue != nil || !result.Changed || result.Metal != 700 || result.Crystal != 1100 || result.Deuterium != 1050 {
		t.Fatalf("unexpected metal trade result=%+v issue=%+v", result, issue)
	}
	result, issue = ResolveMerchantTrade(MerchantResourceCrystal, rates, resources, MerchantTradeValues{Metal: -100, Deuterium: 50})
	if issue != nil || !result.Changed || result.Metal != 1100 || result.Crystal != 834 || result.Deuterium != 1050 {
		t.Fatalf("unexpected crystal trade result=%+v issue=%+v", result, issue)
	}
	result, issue = ResolveMerchantTrade(MerchantResourceDeuterium, rates, resources, MerchantTradeValues{Metal: 90, Crystal: 100})
	if issue != nil || !result.Changed || result.Metal != 1090 || result.Crystal != 1100 || result.Deuterium != 920 {
		t.Fatalf("unexpected deuterium trade result=%+v issue=%+v", result, issue)
	}

	_, issue = ResolveMerchantTrade(MerchantResourceMetal, rates, resources, MerchantTradeValues{Crystal: 10_000})
	if issue == nil || issue.Code != MerchantIssueNotEnoughResource {
		t.Fatalf("expected resource issue, got %+v", issue)
	}
	_, issue = ResolveMerchantTrade(MerchantResourceMetal, rates, Resources{
		Metal:             1000,
		Crystal:           1000,
		Deuterium:         1000,
		MetalCapacity:     1000,
		CrystalCapacity:   1000,
		DeuteriumCapacity: 2000,
	}, MerchantTradeValues{Crystal: 1})
	if issue == nil || issue.Code != MerchantIssueNotEnoughStorage {
		t.Fatalf("expected storage issue, got %+v", issue)
	}
	_, issue = ResolveMerchantTrade(MerchantResourceCrystal, rates, Resources{
		Metal:             1000,
		Crystal:           1000,
		Deuterium:         1000,
		MetalCapacity:     1000,
		CrystalCapacity:   2000,
		DeuteriumCapacity: 2000,
	}, MerchantTradeValues{Metal: 1})
	if issue == nil || issue.Code != MerchantIssueNotEnoughStorage {
		t.Fatalf("expected crystal storage issue, got %+v", issue)
	}
	_, issue = ResolveMerchantTrade(MerchantResourceDeuterium, rates, Resources{
		Metal:             1000,
		Crystal:           1000,
		Deuterium:         1000,
		MetalCapacity:     1000,
		CrystalCapacity:   2000,
		DeuteriumCapacity: 2000,
	}, MerchantTradeValues{Metal: 1})
	if issue == nil || issue.Code != MerchantIssueNotEnoughStorage {
		t.Fatalf("expected deuterium storage issue, got %+v", issue)
	}
	_, issue = ResolveMerchantTrade(MerchantResourceCrystal, rates, resources, MerchantTradeValues{Metal: 10_000})
	if issue == nil || issue.Code != MerchantIssueNotEnoughResource {
		t.Fatalf("expected crystal resource issue, got %+v", issue)
	}
	_, issue = ResolveMerchantTrade(MerchantResourceDeuterium, rates, resources, MerchantTradeValues{Metal: 10_000})
	if issue == nil || issue.Code != MerchantIssueNotEnoughResource {
		t.Fatalf("expected deuterium resource issue, got %+v", issue)
	}
	result, issue = ResolveMerchantTrade(MerchantResourceMetal, MerchantRates{Metal: 0, Crystal: 0}, resources, MerchantTradeValues{Crystal: 100})
	if issue != nil || result.Changed {
		t.Fatalf("zero rate trade should be ignored, result=%+v issue=%+v", result, issue)
	}
	result, issue = ResolveMerchantTrade(MerchantResourceCrystal, MerchantRates{Crystal: 0, Metal: 0}, resources, MerchantTradeValues{Metal: 100})
	if issue != nil || result.Changed {
		t.Fatalf("zero crystal rate trade should be ignored, result=%+v issue=%+v", result, issue)
	}
	result, issue = ResolveMerchantTrade(MerchantResourceDeuterium, MerchantRates{Deuterium: 0, Metal: 0}, resources, MerchantTradeValues{Metal: 100})
	if issue != nil || result.Changed {
		t.Fatalf("zero deuterium rate trade should be ignored, result=%+v issue=%+v", result, issue)
	}
	result, issue = ResolveMerchantTrade(99, rates, resources, MerchantTradeValues{Crystal: 100})
	if issue != nil || result.Changed {
		t.Fatalf("invalid offer should be ignored, result=%+v issue=%+v", result, issue)
	}
}

func TestMerchantIssueConstructors(t *testing.T) {
	if MerchantNotEnoughDarkMatterIssue().Code != MerchantIssueNotEnoughDarkMatter {
		t.Fatalf("unexpected DM issue")
	}
	if MerchantNotEnoughStorageIssue().Code != MerchantIssueNotEnoughStorage {
		t.Fatalf("unexpected storage issue")
	}
}

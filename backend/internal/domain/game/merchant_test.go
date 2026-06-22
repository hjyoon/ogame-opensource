package game

import "testing"

func TestSpendMerchantDarkMatterUsesPaidBeforeFree(t *testing.T) {
	user, ok := SpendMerchantDarkMatter(MerchantUser{PaidDarkMatter: 1000, FreeDarkMatter: 2000})
	if !ok {
		t.Fatal("expected dark matter to be spendable")
	}
	if user.PaidDarkMatter != 0 || user.FreeDarkMatter != 500 {
		t.Fatalf("unexpected dark matter after spend: %+v", user)
	}

	if _, ok := SpendMerchantDarkMatter(MerchantUser{PaidDarkMatter: 1000, FreeDarkMatter: 1000}); ok {
		t.Fatal("expected insufficient dark matter")
	}
}

func TestGenerateMerchantRatesMatchesLegacyBranches(t *testing.T) {
	rates, ok := GenerateMerchantRates(MerchantResourceMetal, 5, 140, 70)
	if !ok || rates.Metal != 3 || rates.Crystal != 2 || rates.Deuterium != 1 {
		t.Fatalf("unexpected common rates: %+v ok=%v", rates, ok)
	}

	rates, ok = GenerateMerchantRates(MerchantResourceCrystal, 15, 210, 70)
	if !ok || rates.Metal != 2.4 || rates.Crystal != 2 || rates.Deuterium != 0.8 {
		t.Fatalf("unexpected fixed crystal rates: %+v ok=%v", rates, ok)
	}

	rates, ok = GenerateMerchantRates(MerchantResourceDeuterium, 50, 225, 155)
	if !ok || rates.Metal != 2.25 || rates.Crystal != 1.55 || rates.Deuterium != 1 {
		t.Fatalf("unexpected random deuterium rates: %+v ok=%v", rates, ok)
	}
}

func TestResolveMerchantTradeAppliesMetalOffer(t *testing.T) {
	result, issue := ResolveMerchantTrade(
		MerchantResourceMetal,
		MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1},
		Resources{
			Metal:             1_000_000,
			Crystal:           100_000,
			Deuterium:         100_000,
			MetalCapacity:     2_000_000,
			CrystalCapacity:   2_000_000,
			DeuteriumCapacity: 2_000_000,
		},
		MerchantTradeValues{Crystal: 2000, Deuterium: 1000},
	)
	if issue != nil {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if !result.Changed || result.Metal != 994_000 || result.Crystal != 102_000 || result.Deuterium != 101_000 {
		t.Fatalf("unexpected trade result: %+v", result)
	}
}

func TestResolveMerchantTradeRejectsInsufficientResource(t *testing.T) {
	result, issue := ResolveMerchantTrade(
		MerchantResourceMetal,
		MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1},
		Resources{
			Metal:             1000,
			Crystal:           100_000,
			Deuterium:         100_000,
			MetalCapacity:     2_000_000,
			CrystalCapacity:   2_000_000,
			DeuteriumCapacity: 2_000_000,
		},
		MerchantTradeValues{Crystal: 2000, Deuterium: 1000},
	)
	if issue == nil || issue.Code != MerchantIssueNotEnoughResource {
		t.Fatalf("expected insufficient resource issue, got result=%+v issue=%+v", result, issue)
	}
	if result.Changed {
		t.Fatalf("trade should not change resources: %+v", result)
	}
}

package game

import "testing"

func TestSearchNormalizesLegacyRequestValues(t *testing.T) {
	if NormalizeSearchType("planetname") != SearchTypePlanetName ||
		NormalizeSearchType("allytag") != SearchTypeAllianceTag ||
		NormalizeSearchType("allyname") != SearchTypeAllianceName ||
		NormalizeSearchType("") != SearchTypePlayerName ||
		NormalizeSearchType("bad") != SearchTypePlayerName {
		t.Fatal("unexpected search type normalization")
	}
	if NormalizeSearchText("  legor  ") != "legor" {
		t.Fatal("search text should be trimmed")
	}
	if !SearchTextTooShort("a") || SearchTextTooShort("ab") || SearchTextTooShort("") {
		t.Fatal("unexpected short search text behavior")
	}
	if SearchOverLimitMessage(SearchTypeAllianceTag) != "more than 25 entries found" ||
		SearchOverLimitMessage(SearchTypePlayerName) != "More than 25 entries were found." {
		t.Fatal("unexpected over limit message")
	}
}

func TestSearchAllianceDisplayScoreMatchesLegacyPoints(t *testing.T) {
	row := SearchAllianceRow{Score: 950000000}
	if row.DisplayScore() != 950000 {
		t.Fatalf("unexpected display score: %d", row.DisplayScore())
	}
	row.Score = -1
	if row.DisplayScore() != 0 {
		t.Fatal("negative alliance score should display as zero")
	}
}

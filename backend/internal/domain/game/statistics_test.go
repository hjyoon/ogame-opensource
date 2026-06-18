package game

import "testing"

func TestStatisticsNormalizesLegacyRequestValues(t *testing.T) {
	if NormalizeStatisticsWho("ally") != StatisticsWhoAlly || NormalizeStatisticsWho("bad") != StatisticsWhoPlayer {
		t.Fatal("unexpected statistics who normalization")
	}
	if NormalizeStatisticsType("fleet") != StatisticsTypeFleet ||
		NormalizeStatisticsType("research") != StatisticsTypeResearch ||
		NormalizeStatisticsType("") != StatisticsTypeResources ||
		NormalizeStatisticsType("points") != StatisticsTypeResources {
		t.Fatal("unexpected statistics type normalization")
	}
	if NormalizeStatisticsStart(-1, 142) != 101 || NormalizeStatisticsStart(205, 1) != 201 || NormalizeStatisticsStart(0, 0) != 1 {
		t.Fatal("unexpected statistics start normalization")
	}
}

func TestStatisticsScoreColumnsAndDisplayScoreMatchLegacy(t *testing.T) {
	score, place, oldPlace := StatisticsScoreColumns(StatisticsTypeResources)
	if score != "score1" || place != "place1" || oldPlace != "oldplace1" {
		t.Fatalf("unexpected points columns: %s %s %s", score, place, oldPlace)
	}
	score, place, oldPlace = StatisticsScoreColumns(StatisticsTypeFleet)
	if score != "score2" || place != "place2" || oldPlace != "oldplace2" {
		t.Fatalf("unexpected fleet columns: %s %s %s", score, place, oldPlace)
	}
	score, place, oldPlace = StatisticsScoreColumns(StatisticsTypeResearch)
	if score != "score3" || place != "place3" || oldPlace != "oldplace3" {
		t.Fatalf("unexpected research columns: %s %s %s", score, place, oldPlace)
	}

	row := StatisticsRow{Place: 2, PreviousPlace: 5, Score: 950000000}
	if row.DisplayScore(StatisticsTypeResources) != 950000 || row.DisplayScore(StatisticsTypeFleet) != 950000000 || row.PlaceDelta() != -3 {
		t.Fatalf("unexpected row display values: %+v", row)
	}
	row.Score = -1
	if row.DisplayScore(StatisticsTypeResources) != 0 {
		t.Fatal("negative scores should display as zero")
	}
}

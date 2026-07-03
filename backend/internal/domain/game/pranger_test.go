package game

import "testing"

func TestNormalizePrangerFrom(t *testing.T) {
	if got := NormalizePrangerFrom(-10); got != 0 {
		t.Fatalf("expected negative offset to clamp to zero, got %d", got)
	}
	if got := NormalizePrangerFrom(75); got != 75 {
		t.Fatalf("expected positive offset to remain unchanged, got %d", got)
	}
}

func TestPrangerPagination(t *testing.T) {
	pranger := Pranger{From: 50, Entries: make([]PrangerEntry, PrangerPageLimit)}
	if !pranger.HasPrevious() || pranger.PreviousFrom() != 0 {
		t.Fatalf("expected previous page from 50, got has=%v from=%d", pranger.HasPrevious(), pranger.PreviousFrom())
	}
	if !pranger.HasNext() || pranger.NextFrom() != 100 {
		t.Fatalf("expected next page from 100, got has=%v from=%d", pranger.HasNext(), pranger.NextFrom())
	}

	pranger = Pranger{From: 0, Entries: []PrangerEntry{{}}}
	if pranger.HasPrevious() || pranger.PreviousFrom() != 0 || pranger.HasNext() {
		t.Fatalf("unexpected first short page pagination: %+v", pranger)
	}
}

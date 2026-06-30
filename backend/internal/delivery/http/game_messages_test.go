package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestGameMessagesSummaryMapsOperatorsAndComposeTarget(t *testing.T) {
	messages := domaingame.Messages{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:   99,
			Name: "Arakis",
		},
		PlanetSwitcher: []domaingame.PlanetSummary{{ID: 99, Name: "Arakis", Current: true}},
		Action:         domaingame.MessagesActionCompose,
		Rows: []domaingame.Message{{
			ID:         7,
			Type:       domaingame.MessageTypePM,
			From:       "Sender",
			Subject:    "Subject",
			Text:       "Body",
			Date:       123,
			Unread:     true,
			Reportable: true,
		}},
		Operators: []domaingame.MessageOperator{{
			PlayerID:  1,
			Name:      "Operator",
			Email:     "operator@example.test",
			HideEmail: true,
			Subject:   "Question from Legor",
		}},
		Compose: &domaingame.MessageCompose{
			Target: domaingame.MessageTarget{
				PlayerID: 77,
				Name:     "Target",
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			Subject:  "Re: Subject",
			MaxChars: 5000,
		},
	}

	payload := toGameMessagesSummary(messages)

	if payload.Commander != "legor" || payload.CurrentPlanet.ID != 99 || len(payload.PlanetSwitcher) != 1 {
		t.Fatalf("unexpected message summary identity: %+v", payload)
	}
	if len(payload.Rows) != 1 || !payload.Rows[0].Unread || !payload.Rows[0].Reportable {
		t.Fatalf("expected message rows to map: %+v", payload.Rows)
	}
	if len(payload.Operators) != 1 || payload.Operators[0].Name != "Operator" ||
		payload.Operators[0].Email != "operator@example.test" || !payload.Operators[0].HideEmail ||
		payload.Operators[0].Subject != "Question from Legor" {
		t.Fatalf("expected operator rows to map: %+v", payload.Operators)
	}
	if payload.Compose == nil || payload.Compose.Target.PlayerID != 77 ||
		payload.Compose.Target.Coordinates.System != 2 || payload.Compose.MaxChars != 5000 {
		t.Fatalf("expected compose target to map: %+v", payload.Compose)
	}
}

func TestSelectedMessageTargetIDHandlesLegacyQuery(t *testing.T) {
	targetID, err := selectedMessageTargetID(httptest.NewRequest(http.MethodGet, "/api/game/messages?messageziel=77", nil))
	if err != nil || targetID != 77 {
		t.Fatalf("expected target id 77, got id=%d err=%v", targetID, err)
	}
	targetID, err = selectedMessageTargetID(httptest.NewRequest(http.MethodGet, "/api/game/messages", nil))
	if err != nil || targetID != 0 {
		t.Fatalf("expected missing target id to be zero, got id=%d err=%v", targetID, err)
	}
	if _, err := selectedMessageTargetID(httptest.NewRequest(http.MethodGet, "/api/game/messages?messageziel=bad", nil)); err == nil {
		t.Fatal("expected invalid target id to fail")
	}
	if _, err := selectedMessageTargetID(httptest.NewRequest(http.MethodGet, "/api/game/messages?messageziel=-1", nil)); err == nil {
		t.Fatal("expected negative target id to fail")
	}
}

package configcatalog

import (
	"context"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestUniverseCatalogReturnsDefaultUniverse(t *testing.T) {
	catalog := UniverseCatalog{LegacyBaseURL: "http://localhost:8888"}

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 {
		t.Fatalf("expected one default universe, got %+v", universes)
	}
	if universes[0].Number != 1 || universes[0].BaseURL != "http://localhost:8888" || universes[0].Status != domain.UniverseOnline {
		t.Fatalf("unexpected default universe: %+v", universes[0])
	}
}

func TestUniverseCatalogParsesJSON(t *testing.T) {
	catalog := UniverseCatalog{RawJSON: `[
		{"number":2,"name":"Beta","baseUrl":"http://beta.local","language":"en","speed":128,"fleetSpeed":64,"status":"online"}
	]`}

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 || universes[0].Name != "Beta" || universes[0].Speed != 128 {
		t.Fatalf("unexpected parsed universes: %+v", universes)
	}
}

func TestUniverseCatalogRejectsInvalidJSON(t *testing.T) {
	catalog := UniverseCatalog{RawJSON: `{`}

	if _, err := catalog.ListUniverses(context.Background()); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

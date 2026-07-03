package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyAssetHandlersDirectBranches(t *testing.T) {
	assets := &fakeFrontendAssets{bodies: map[string]string{
		"public-assets/evolution/formate.css": "evolution css",
		"public-assets/game/img/planet.gif":   "GIF89a",
	}}
	handler := app{deps: Dependencies{Frontend: assets}}

	rec := httptest.NewRecorder()
	handler.handleLegacyEvolutionAsset(rec, httptest.NewRequest(http.MethodGet, "/game/img/planet.gif", nil))
	if rec.Code != http.StatusNotFound || len(assets.rels) != 0 {
		t.Fatalf("expected evolution wrong-prefix 404 without asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyEvolutionAsset(rec, httptest.NewRequest(http.MethodGet, "/evolution/formate.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "evolution css" {
		t.Fatalf("expected evolution asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyEvolutionAsset(rec, httptest.NewRequest(http.MethodGet, "/evolution/missing.css", nil))
	if rec.Code != http.StatusNotFound || assets.rels[len(assets.rels)-1] != "public-assets/evolution/missing.css" {
		t.Fatalf("expected missing evolution asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/js/app.js", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(strings.Join(assets.rels, ","), "game/js/app.js") {
		t.Fatalf("expected game static wrong-prefix 404 without asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/img/planet.gif", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "GIF89a" {
		t.Fatalf("expected game image asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/css/missing.css", nil))
	if rec.Code != http.StatusNotFound || assets.rels[len(assets.rels)-1] != "public-assets/game/css/missing.css" {
		t.Fatalf("expected missing game static asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}
}

type fakeFrontendAssets struct {
	bodies map[string]string
	rels   []string
}

func (f *fakeFrontendAssets) Serve(w http.ResponseWriter, _ *http.Request, rel string) bool {
	f.rels = append(f.rels, rel)
	body, ok := f.bodies[rel]
	if !ok {
		return false
	}
	_, _ = w.Write([]byte(body))
	return true
}

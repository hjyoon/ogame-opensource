package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyAssetHandlersDirectBranches(t *testing.T) {
	assets := &fakeFrontendAssets{bodies: map[string]string{
		"public-assets/evolution/formate.css":           "evolution css",
		"public-assets/img/overview_t.jpg":              "JPEG",
		"public-assets/game/img/planet.gif":             "GIF89a",
		"public-assets/game/js/go-game.js":              "function go(){}",
		"public-assets/game/mods/GalaxyTool/img/bg.png": "PNG",
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
	handler.handleLegacyPublicStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/img/overview_t.jpg", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "JPEG" {
		t.Fatalf("expected public image asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyEvolutionAsset(rec, httptest.NewRequest(http.MethodGet, "/evolution/missing.css", nil))
	if rec.Code != http.StatusNotFound || assets.rels[len(assets.rels)-1] != "public-assets/evolution/missing.css" {
		t.Fatalf("expected missing evolution asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyPublicStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/css/styles.css", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(strings.Join(assets.rels, ","), "css/styles.css") {
		t.Fatalf("expected unsupported public prefix 404 without asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/img/planet.gif", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "GIF89a" {
		t.Fatalf("expected game image asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/js/go-game.js", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "function go(){}" {
		t.Fatalf("expected game js asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/mods/GalaxyTool/img/bg.png", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "PNG" {
		t.Fatalf("expected game mod asset, code=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/css/missing.css", nil))
	if rec.Code != http.StatusNotFound || assets.rels[len(assets.rels)-1] != "public-assets/game/css/missing.css" {
		t.Fatalf("expected missing game static asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
	}

	rec = httptest.NewRecorder()
	handler.handleLegacyGameStaticAsset(rec, httptest.NewRequest(http.MethodGet, "/game/battle/file.cpp", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(strings.Join(assets.rels, ","), "game/battle/file.cpp") {
		t.Fatalf("expected unsupported game static prefix 404 without asset lookup, code=%d rels=%+v", rec.Code, assets.rels)
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

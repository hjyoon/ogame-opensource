package httpdelivery

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

func (a app) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	cleanPath := path.Clean("/" + r.URL.Path)
	rel := strings.TrimPrefix(cleanPath, "/")
	if rel == "." || rel == "" {
		rel = "index.html"
	}
	if a.deps.Frontend.Serve(w, r, rel) {
		return
	}
	if isLegacyPublicHTMLPath(cleanPath) || isLegacyGameHTMLPath(cleanPath) {
		if !a.deps.Frontend.Serve(w, r, "index.html") {
			http.Error(w, "frontend build is missing", http.StatusServiceUnavailable)
		}
		return
	}
	if strings.Contains(filepath.Base(rel), ".") {
		http.NotFound(w, r)
		return
	}
	if !a.deps.Frontend.Serve(w, r, "index.html") {
		http.Error(w, "frontend build is missing", http.StatusServiceUnavailable)
	}
}

func (a app) handleLegacyEvolutionAsset(w http.ResponseWriter, r *http.Request) {
	cleanPath := path.Clean("/" + r.URL.Path)
	if !strings.HasPrefix(cleanPath, "/evolution/") {
		http.NotFound(w, r)
		return
	}
	rel := "public-assets" + cleanPath
	if a.deps.Frontend.Serve(w, r, rel) {
		return
	}
	http.NotFound(w, r)
}

func (a app) handleLegacyGameStaticAsset(w http.ResponseWriter, r *http.Request) {
	cleanPath := path.Clean("/" + r.URL.Path)
	if !strings.HasPrefix(cleanPath, "/game/css/") && !strings.HasPrefix(cleanPath, "/game/img/") {
		http.NotFound(w, r)
		return
	}
	rel := "public-assets" + cleanPath
	if a.deps.Frontend.Serve(w, r, rel) {
		return
	}
	http.NotFound(w, r)
}

func isLegacyGameHTMLPath(cleanPath string) bool {
	return cleanPath == "/game/index.php"
}

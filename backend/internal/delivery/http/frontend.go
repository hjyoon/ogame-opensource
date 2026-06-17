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
	if isLegacyPublicHTMLPath(cleanPath) {
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

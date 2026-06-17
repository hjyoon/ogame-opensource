package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
)

type app struct {
	cfg config.Config
}

type healthResponse struct {
	Status            string `json:"status"`
	Service           string `json:"service"`
	Environment       string `json:"environment"`
	Runtime           string `json:"runtime"`
	GoTarget          string `json:"goTarget"`
	BunTarget         string `json:"bunTarget"`
	ReactTarget       string `json:"reactTarget"`
	StaticReady       bool   `json:"staticReady"`
	LegacyAssetsReady bool   `json:"legacyAssetsReady"`
	LegacyBaseURL     string `json:"legacyBaseUrl"`
}

func New(cfg config.Config) http.Handler {
	a := app{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", a.getOnly(a.handleHealthz))
	mux.Handle("/legacy-assets/", a.legacyAssets())
	mux.HandleFunc("/", a.handleFrontend)
	return securityHeaders(mux)
}

func (a app) getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func (a app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:            "ok",
		Service:           "ogame-go",
		Environment:       a.cfg.Environment,
		Runtime:           runtime.Version(),
		GoTarget:          config.GoTarget,
		BunTarget:         config.BunTarget,
		ReactTarget:       config.ReactTarget,
		StaticReady:       dirExists(a.cfg.StaticDir),
		LegacyAssetsReady: dirExists(a.cfg.LegacyAssetDir),
		LegacyBaseURL:     a.cfg.LegacyBaseURL,
	})
}

func (a app) legacyAssets() http.Handler {
	fs := noDirectoryListing{fs: http.Dir(a.cfg.LegacyAssetDir)}
	return http.StripPrefix("/legacy-assets/", http.FileServer(fs))
}

func (a app) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if rel == "." || rel == "" {
		rel = "index.html"
	}
	if serveStaticFile(w, r, a.cfg.StaticDir, rel) {
		return
	}
	if strings.Contains(filepath.Base(rel), ".") {
		http.NotFound(w, r)
		return
	}
	if !serveStaticFile(w, r, a.cfg.StaticDir, "index.html") {
		http.Error(w, "frontend build is missing", http.StatusServiceUnavailable)
	}
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, root string, rel string) bool {
	name := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return false
	}
	if strings.HasPrefix(rel, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, name)
	return true
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func dirExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

type noDirectoryListing struct {
	fs http.FileSystem
}

func (n noDirectoryListing) Open(name string) (http.File, error) {
	file, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, os.ErrNotExist
	}
	return file, nil
}

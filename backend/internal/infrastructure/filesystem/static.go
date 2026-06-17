package filesystem

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type StaticDir struct {
	Root string
}

func (s StaticDir) Serve(w http.ResponseWriter, r *http.Request, rel string) bool {
	name := filepath.Join(s.Root, filepath.FromSlash(rel))
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

func NewNoListingFS(root string) http.FileSystem {
	return noDirectoryListing{fs: http.Dir(root)}
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

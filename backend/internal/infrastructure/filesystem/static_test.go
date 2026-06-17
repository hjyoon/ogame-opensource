package filesystem

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeReady(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	writeFile(t, file, "data")

	probe := Probe{}
	if !probe.Ready(dir) {
		t.Fatal("expected directory to be ready")
	}
	if probe.Ready(file) {
		t.Fatal("expected regular file to be not ready")
	}
	if probe.Ready(filepath.Join(dir, "missing")) {
		t.Fatal("expected missing path to be not ready")
	}
}

func TestStaticDirServe(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "index.html"), "index")
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "assets", "main.js"), "js")

	static := StaticDir{Root: root}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !static.Serve(rec, req, "index.html") {
		t.Fatal("expected index to be served")
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Body.String() != "index" {
		t.Fatalf("unexpected index response: headers=%v body=%q", rec.Header(), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	if !static.Serve(rec, req, "assets/main.js") {
		t.Fatal("expected asset to be served")
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected asset cache header: %q", rec.Header().Get("Cache-Control"))
	}

	if static.Serve(httptest.NewRecorder(), req, "missing.js") {
		t.Fatal("expected missing file to return false")
	}
	if static.Serve(httptest.NewRecorder(), req, "assets") {
		t.Fatal("expected directory to return false")
	}
}

func TestNoListingFS(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "planet.jpg"), "jpeg")
	fs := NewNoListingFS(root)

	file, err := fs.Open("planet.jpg")
	if err != nil {
		t.Fatalf("expected file open: %v", err)
	}
	_ = file.Close()

	dir, err := fs.Open(".")
	if err == nil {
		_ = dir.Close()
		t.Fatal("expected directory open to fail")
	}
	if _, err := fs.Open("missing.jpg"); err == nil {
		t.Fatal("expected missing file to fail")
	}
}

func TestNoListingFSClosesFileWhenStatFails(t *testing.T) {
	file := &statErrorFile{}
	fs := noDirectoryListing{fs: fakeFS{file: file}}

	if opened, err := fs.Open("broken"); err == nil {
		_ = opened.Close()
		t.Fatal("expected stat error")
	}
	if !file.closed {
		t.Fatal("expected file to be closed when stat fails")
	}
}

type fakeFS struct {
	file http.File
}

func (f fakeFS) Open(string) (http.File, error) {
	return f.file, nil
}

type statErrorFile struct {
	closed bool
}

func (f *statErrorFile) Close() error {
	f.closed = true
	return nil
}

func (*statErrorFile) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (*statErrorFile) Seek(int64, int) (int64, error) {
	return 0, nil
}

func (*statErrorFile) Readdir(int) ([]os.FileInfo, error) {
	return nil, nil
}

func (*statErrorFile) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat failed")
}

func writeFile(t *testing.T, name string, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

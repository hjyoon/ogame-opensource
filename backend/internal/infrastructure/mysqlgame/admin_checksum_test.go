package mysqlgame

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestAdminRepositoryLoadsChecksumGroupsFromLegacyGameDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "temp"), 0o755); err != nil {
		t.Fatalf("create temp checksum dir: %v", err)
	}

	for _, group := range adminChecksumGroups {
		baseline := make(map[string]string, len(group.files))
		for _, name := range group.files {
			fullPath := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatalf("create source dir: %v", err)
			}
			if err := os.WriteFile(fullPath, []byte("legacy "+name), 0o644); err != nil {
				t.Fatalf("write source file: %v", err)
			}
			checksum, err := md5FileHex(fullPath)
			if err != nil {
				t.Fatalf("calculate checksum: %v", err)
			}
			baseline[name] = checksum
		}
		if group.title == "Engine" {
			delete(baseline, group.files[0])
		}
		if group.title == "Admin Area" {
			baseline[group.files[0]] = "00000000000000000000000000000000"
		}
		if err := os.WriteFile(filepath.Join(root, "temp", group.baselineFile), []byte(testPHPSerializedChecksumMap(baseline)), 0o644); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
	}

	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(root)
	groups, err := repository.loadAdminChecksumGroups(context.Background())

	if err != nil {
		t.Fatalf("loadAdminChecksumGroups returned error: %v", err)
	}
	if len(groups) != 4 || groups[0].Title != "Engine" || groups[1].Title != "Admin Area" {
		t.Fatalf("unexpected checksum groups: %+v", groups)
	}
	if groups[0].Rows[0].Status != "UNVERSIONED" {
		t.Fatalf("expected missing engine baseline to be UNVERSIONED, got %+v", groups[0].Rows[0])
	}
	if groups[1].Rows[0].Status != "BAD" {
		t.Fatalf("expected mismatched admin checksum to be BAD, got %+v", groups[1].Rows[0])
	}
	if groups[2].Rows[0].Status != "OK" || len(groups[2].Rows[0].Checksum) != 32 {
		t.Fatalf("expected valid game page checksum, got %+v", groups[2].Rows[0])
	}
}

func TestLoadPHPSerializedChecksumMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.md5")
	content := `a:2:{s:9:"ainfo.php";s:32:"0123456789abcdef0123456789abcdef";s:12:"core/acs.php";s:32:"abcdef0123456789abcdef0123456789";}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	result, err := loadPHPSerializedChecksumMap(path)

	if err != nil {
		t.Fatalf("loadPHPSerializedChecksumMap returned error: %v", err)
	}
	if result["ainfo.php"] != "0123456789abcdef0123456789abcdef" || result["core/acs.php"] != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected parsed checksums: %+v", result)
	}
}

func TestAdminChecksumFileErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.md5")
	if _, err := loadPHPSerializedChecksumMap(missing); err == nil {
		t.Fatal("expected missing baseline file error")
	}
	if _, err := md5FileHex(missing); err == nil {
		t.Fatal("expected missing source file error")
	}

	root := t.TempDir()
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(root)
	if _, err := repository.loadAdminChecksumGroups(context.Background()); err == nil {
		t.Fatal("expected missing checksum baseline to fail")
	}

	if err := os.MkdirAll(filepath.Join(root, "temp"), 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "temp", adminChecksumGroups[0].baselineFile), []byte(testPHPSerializedChecksumMap(map[string]string{})), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if _, err := repository.loadAdminChecksumGroups(context.Background()); err == nil {
		t.Fatal("expected missing checksum source to fail")
	}
}

func TestAdminRepositoryFixesChecksumBaselines(t *testing.T) {
	root := t.TempDir()
	writeAdminChecksumSources(t, root)
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(root)

	issue, err := repository.fixAdminChecksums()
	if err != nil {
		t.Fatalf("fixAdminChecksums returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.AdminIssueActionSaved {
		t.Fatalf("unexpected checksum issue: %+v", issue)
	}
	groups, err := repository.loadAdminChecksumGroups(context.Background())
	if err != nil {
		t.Fatalf("load fixed checksum groups: %v", err)
	}
	for _, group := range groups {
		for _, row := range group.Rows {
			if row.Status != "OK" {
				t.Fatalf("expected fixed checksum row, got %+v", row)
			}
		}
	}
}

func TestSerializePHPChecksumMapMatchesLegacyPHP(t *testing.T) {
	files := []string{"ainfo.php", "core/acs.php"}
	checksums := map[string]string{
		"ainfo.php":    "0123456789abcdef0123456789abcdef",
		"core/acs.php": "abcdef0123456789abcdef0123456789",
	}
	expected := `a:2:{s:9:"ainfo.php";s:32:"0123456789abcdef0123456789abcdef";s:12:"core/acs.php";s:32:"abcdef0123456789abcdef0123456789";}`
	if actual := string(serializePHPChecksumMap(files, checksums)); actual != expected {
		t.Fatalf("unexpected PHP serialization:\nwant %s\n got %s", expected, actual)
	}
}

func TestAdminRepositoryFixChecksumErrors(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_").WithLegacyGameDir(t.TempDir())
	if _, err := repository.fixAdminChecksums(); err == nil {
		t.Fatal("expected missing checksum source error")
	}

	root := t.TempDir()
	writeAdminChecksumSources(t, root)
	if err := os.WriteFile(filepath.Join(root, "temp"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write blocked temp path: %v", err)
	}
	repository = repository.WithLegacyGameDir(root)
	if _, err := repository.fixAdminChecksums(); err == nil {
		t.Fatal("expected checksum temp directory error")
	}
}

func writeAdminChecksumSources(t *testing.T, root string) {
	t.Helper()
	for _, group := range adminChecksumGroups {
		for _, name := range group.files {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create checksum source directory: %v", err)
			}
			if err := os.WriteFile(path, []byte("legacy "+name), 0o644); err != nil {
				t.Fatalf("write checksum source: %v", err)
			}
		}
	}
}

func testPHPSerializedChecksumMap(values map[string]string) string {
	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "a:%d:{", len(values))
	for key, value := range values {
		_, _ = fmt.Fprintf(&builder, `s:%d:"%s";s:%d:"%s";`, len(key), key, len(value), value)
	}
	builder.WriteByte('}')
	return builder.String()
}

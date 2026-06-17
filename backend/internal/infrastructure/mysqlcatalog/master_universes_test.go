package mysqlcatalog

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestDSN(t *testing.T) {
	dsn := DSN(MasterDBConfig{
		Host:     "db.example:3307",
		User:     "root",
		Password: "secret",
		Name:     "master",
	})

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "db.example:3307" || cfg.User != "root" || cfg.Passwd != "secret" || cfg.DBName != "master" {
		t.Fatalf("unexpected DSN config: %+v", cfg)
	}
}

func TestNewMasterUniverseCatalogUsesSQLQueryer(t *testing.T) {
	catalog := NewMasterUniverseCatalog(nil)

	if _, ok := catalog.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQLQueryer, got %T", catalog.queryer)
	}
}

func TestHostPort(t *testing.T) {
	cases := map[string]string{
		"":              "mysql:3306",
		"mysql":         "mysql:3306",
		"127.0.0.1":     "127.0.0.1:3306",
		"127.0.0.1:123": "127.0.0.1:123",
	}
	for input, expected := range cases {
		if got := hostPort(input); got != expected {
			t.Fatalf("%q: expected %q, got %q", input, expected, got)
		}
	}
}

func TestNormalizeLegacyURL(t *testing.T) {
	if normalizeLegacyURL("localhost:8888") != "http://localhost:8888" {
		t.Fatal("expected scheme to be added")
	}
	if normalizeLegacyURL("https://example.com") != "https://example.com" {
		t.Fatal("expected existing scheme to be preserved")
	}
	if normalizeLegacyURL(" ") != "" {
		t.Fatal("expected blank value to stay blank")
	}
}

func TestMasterUniverseCatalogListsUniverses(t *testing.T) {
	rows := &fakeRows{items: []fakeRow{
		{number: 1, rawURL: "localhost:8888"},
		{number: 2, rawURL: "https://uni2.example"},
	}}
	catalog := NewMasterUniverseCatalogWithQueryer(fakeQueryer{rows: rows})

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 2 {
		t.Fatalf("expected two universes, got %+v", universes)
	}
	if universes[0].Name != "Universe 1" || universes[0].BaseURL != "http://localhost:8888" {
		t.Fatalf("unexpected first universe: %+v", universes[0])
	}
	if universes[1].BaseURL != "https://uni2.example" {
		t.Fatalf("unexpected second universe: %+v", universes[1])
	}
	if !rows.closed {
		t.Fatal("expected rows to be closed")
	}
}

func TestMasterUniverseCatalogSkipsInvalidRows(t *testing.T) {
	catalog := NewMasterUniverseCatalogWithQueryer(fakeQueryer{rows: &fakeRows{items: []fakeRow{
		{number: 0, rawURL: "localhost:8888"},
		{number: 1, rawURL: " "},
		{number: 2, rawURL: "uni2.example"},
	}}})

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 || universes[0].Number != 2 || universes[0].BaseURL != "http://uni2.example" {
		t.Fatalf("expected only valid row, got %+v", universes)
	}
}

func TestMasterUniverseCatalogReturnsQueryError(t *testing.T) {
	wantErr := errors.New("query failed")
	catalog := NewMasterUniverseCatalogWithQueryer(fakeQueryer{err: wantErr})

	if _, err := catalog.ListUniverses(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestMasterUniverseCatalogReturnsScanError(t *testing.T) {
	catalog := NewMasterUniverseCatalogWithQueryer(fakeQueryer{rows: &fakeRows{
		items:   []fakeRow{{number: 1, rawURL: "localhost:8888"}},
		scanErr: errors.New("scan failed"),
	}})

	if _, err := catalog.ListUniverses(context.Background()); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestMasterUniverseCatalogReturnsRowsError(t *testing.T) {
	catalog := NewMasterUniverseCatalogWithQueryer(fakeQueryer{rows: &fakeRows{err: errors.New("rows failed")}})

	if _, err := catalog.ListUniverses(context.Background()); err == nil {
		t.Fatal("expected rows error")
	}
}

type fakeQueryer struct {
	rows Rows
	err  error
}

func (f fakeQueryer) QueryContext(context.Context, string, ...any) (Rows, error) {
	return f.rows, f.err
}

type fakeRow struct {
	number int
	rawURL string
}

type fakeRows struct {
	items   []fakeRow
	index   int
	closed  bool
	err     error
	scanErr error
}

func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.items)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	item := r.items[r.index]
	r.index++
	*(dest[0].(*int)) = item.number
	*(dest[1].(*string)) = item.rawURL
	return nil
}

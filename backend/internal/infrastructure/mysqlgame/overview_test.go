package mysqlgame

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
)

func TestOverviewRepositoryReadsLegacyOverview(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3, 12800, 19, 12, 163, 1234.5, 234.5, 12.0, 0, 1, 2})},
		{rows: fakeRowsFromValues(
			[]any{99, "Arakis", 1, 1, 2, 3},
			[]any{100, "Colony", 1, 1, 2, 4},
		)},
		{rows: fakeRowsFromValues([]any{2})},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.Commander != "legor" || overview.Score.RawScore != 123456 || overview.Score.Rank != 7 || overview.Score.DisplayPoints() != 123 {
		t.Fatalf("unexpected score overview: %+v", overview)
	}
	if overview.CurrentPlanet.ID != 99 || overview.CurrentPlanet.Name != "Arakis" || overview.CurrentPlanet.Coordinates.Position != 3 {
		t.Fatalf("unexpected current planet: %+v", overview.CurrentPlanet)
	}
	if overview.CurrentPlanet.Resources.Metal != 1234.5 || overview.CurrentPlanet.Resources.Crystal != 234.5 || overview.CurrentPlanet.Resources.Deuterium != 12 {
		t.Fatalf("unexpected resources: %+v", overview.CurrentPlanet.Resources)
	}
	if overview.CurrentPlanet.Resources.MetalCapacity != 100000 ||
		overview.CurrentPlanet.Resources.CrystalCapacity != 150000 ||
		overview.CurrentPlanet.Resources.DeuteriumCapacity != 200000 {
		t.Fatalf("unexpected resource capacities: %+v", overview.CurrentPlanet.Resources)
	}
	if overview.Score.UniversePlayers != 2 {
		t.Fatalf("expected universe player count, got %+v", overview.Score)
	}
	if len(overview.PlanetSwitcher) != 2 || !overview.PlanetSwitcher[0].Current || overview.PlanetSwitcher[1].Current {
		t.Fatalf("unexpected planet switcher: %+v", overview.PlanetSwitcher)
	}
	if !strings.Contains(queryer.calls[2].sql, "ORDER BY planet_id ASC, type DESC") {
		t.Fatalf("expected legacy default planet order, got %q", queryer.calls[2].sql)
	}
	if !strings.Contains(queryer.calls[0].sql, "`ogame_users`") || !strings.Contains(queryer.calls[1].sql, "`ogame_planets`") {
		t.Fatalf("expected prefixed table names, got %+v", queryer.calls)
	}
}

func TestNewOverviewRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewOverviewRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestSQLQueryerUsesDatabase(t *testing.T) {
	db := openOverviewTestDB(t)
	defer db.Close()

	rows, err := (SQLQueryer{DB: db}).QueryContext(context.Background(), "SELECT value")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected fake row")
	}
	var value int
	if err := rows.Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != 1 {
		t.Fatalf("unexpected fake value: %d", value)
	}
}

func TestOverviewRepositoryFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 999, 1, 1, 1})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 2, 3, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{1})},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 1 || overview.CurrentPlanet.Name != "Homeworld" {
		t.Fatalf("expected home planet fallback, got %+v", overview.CurrentPlanet)
	}
	if !strings.Contains(queryer.calls[3].sql, "ORDER BY g DESC, s DESC, p DESC, type DESC") {
		t.Fatalf("expected coordinate sort fallback, got %q", queryer.calls[3].sql)
	}
}

func TestOverviewRepositoryUsesRequestedPlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 2, 0})},
		{rows: fakeRowsFromValues([]any{100, "Colony", 1, 1, 2, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3}, []any{100, "Colony", 1, 1, 2, 4})},
		{rows: fakeRowsFromValues()},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 100))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 100 {
		t.Fatalf("expected requested planet, got %+v", overview.CurrentPlanet)
	}
	if got := queryer.calls[1].args[0]; got != 100 {
		t.Fatalf("expected requested planet id query arg, got %+v", queryer.calls[1].args)
	}
	if !strings.Contains(queryer.calls[2].sql, "ORDER BY name ASC, type DESC") {
		t.Fatalf("expected name sort order, got %q", queryer.calls[2].sql)
	}
	if overview.Score.UniversePlayers != 0 {
		t.Fatalf("expected missing universe player row to default to zero, got %+v", overview.Score)
	}
}

func TestOverviewRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:   "unsafe prefix",
			prefix: "bad-prefix_",
			queryer: &fakeQueryer{
				results: []fakeQueryResult{},
			},
			want: "invalid database table prefix",
		},
		{
			name:   "user query",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{err: errors.New("user query failed")},
			}},
			want: "user query failed",
		},
		{
			name:   "missing user",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues()},
			}},
			want: "overview user not found",
		},
		{
			name:   "missing current planet after fallback",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 9, 1, 0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
			}},
			want: "current planet not found",
		},
		{
			name:   "planet list",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{err: errors.New("planet list failed")},
			}},
			want: "planet list failed",
		},
		{
			name:   "universe",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{err: errors.New("universe failed")},
			}},
			want: "universe failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, tt.prefix)

			_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositoryPropagatesRowErrors(t *testing.T) {
	tests := []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name: "user rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsError(errors.New("user rows failed"))},
			}},
			want: "user rows failed",
		},
		{
			name: "user scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, int64(0), 0, 1, 1, 0, 0})},
			}},
			want: "expected string",
		},
		{
			name: "current planet query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{err: errors.New("current planet failed")},
			}},
			want: "current planet failed",
		},
		{
			name: "current planet rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsError(errors.New("current planet rows failed"))},
			}},
			want: "current planet rows failed",
		},
		{
			name: "current planet scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{"bad", "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			}},
			want: "expected int",
		},
		{
			name: "current planet post-scan rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("current planet post scan failed"), []any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			}},
			want: "current planet post scan failed",
		},
		{
			name: "planet list scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{"bad", "Homeworld", 1, 1, 1, 1})},
			}},
			want: "expected int",
		},
		{
			name: "planet list rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("planet list rows failed"), []any{1, "Homeworld", 1, 1, 1, 1})},
			}},
			want: "planet list rows failed",
		},
		{
			name: "universe scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "expected int",
		},
		{
			name: "universe rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{rows: fakeRowsFromValuesWithErr(errors.New("universe rows failed"), []any{1})},
			}},
			want: "universe rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")

			_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestPlanetOrder(t *testing.T) {
	tests := []struct {
		sortBy    int
		sortOrder int
		want      string
	}{
		{sortBy: 0, sortOrder: 0, want: " ORDER BY planet_id ASC, type DESC"},
		{sortBy: 0, sortOrder: 1, want: " ORDER BY planet_id DESC, type DESC"},
		{sortBy: 1, sortOrder: 0, want: " ORDER BY g ASC, s ASC, p ASC, type DESC"},
		{sortBy: 1, sortOrder: 1, want: " ORDER BY g DESC, s DESC, p DESC, type DESC"},
		{sortBy: 2, sortOrder: 0, want: " ORDER BY name ASC, type DESC"},
		{sortBy: 2, sortOrder: 1, want: " ORDER BY name DESC, type DESC"},
	}

	for _, tt := range tests {
		got := planetOrder(tt.sortBy, tt.sortOrder)
		if got != tt.want {
			t.Fatalf("planetOrder(%d, %d) = %q, want %q", tt.sortBy, tt.sortOrder, got, tt.want)
		}
	}
}

func overviewQuery(playerID int, planetID int) appgame.OverviewQuery {
	return appgame.OverviewQuery{PlayerID: playerID, PlanetID: planetID}
}

var registerOverviewTestDriver sync.Once

func openOverviewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerOverviewTestDriver.Do(func() {
		sql.Register("overview_queryer_test", overviewTestDriver{})
	})
	db, err := sql.Open("overview_queryer_test", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type overviewTestDriver struct{}

func (overviewTestDriver) Open(string) (driver.Conn, error) {
	return overviewTestConn{}, nil
}

type overviewTestConn struct{}

func (overviewTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (overviewTestConn) Close() error {
	return nil
}

func (overviewTestConn) Begin() (driver.Tx, error) {
	return overviewTestTx{}, nil
}

func (overviewTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &overviewTestRows{}, nil
}

type overviewTestTx struct{}

func (overviewTestTx) Commit() error {
	return nil
}

func (overviewTestTx) Rollback() error {
	return nil
}

type overviewTestRows struct {
	done bool
}

func (r *overviewTestRows) Columns() []string {
	return []string{"value"}
}

func (r *overviewTestRows) Close() error {
	return nil
}

func (r *overviewTestRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = int64(1)
	r.done = true
	return nil
}

type fakeQueryer struct {
	results []fakeQueryResult
	calls   []fakeQueryCall
}

type fakeQueryCall struct {
	sql  string
	args []any
}

type fakeQueryResult struct {
	rows *fakeRows
	err  error
}

func (f *fakeQueryer) QueryContext(_ context.Context, sql string, args ...any) (Rows, error) {
	f.calls = append(f.calls, fakeQueryCall{sql: sql, args: args})
	if len(f.results) == 0 {
		return nil, errors.New("unexpected query")
	}
	result := f.results[0]
	f.results = f.results[1:]
	if result.err != nil {
		return nil, result.err
	}
	return result.rows, nil
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
}

func fakeRowsFromValues(values ...[]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func fakeRowsFromValuesWithErr(err error, values ...[]any) *fakeRows {
	return &fakeRows{values: values, index: -1, err: err}
}

func fakeRowsError(err error) *fakeRows {
	return &fakeRows{index: -1, err: err}
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	if len(dest) != len(r.values[r.index]) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		if err := assign(dest[i], r.values[r.index][i]); err != nil {
			return err
		}
	}
	return nil
}

func assign(dest any, value any) error {
	switch target := dest.(type) {
	case *string:
		v, ok := value.(string)
		if !ok {
			return errors.New("expected string")
		}
		*target = v
	case *int:
		switch v := value.(type) {
		case int:
			*target = v
		case int64:
			*target = int(v)
		default:
			return errors.New("expected int")
		}
	case *int64:
		switch v := value.(type) {
		case int:
			*target = int64(v)
		case int64:
			*target = v
		default:
			return errors.New("expected int64")
		}
	case *float64:
		switch v := value.(type) {
		case float64:
			*target = v
		case int:
			*target = float64(v)
		default:
			return errors.New("expected float64")
		}
	default:
		return errors.New("unsupported scan destination")
	}
	return nil
}

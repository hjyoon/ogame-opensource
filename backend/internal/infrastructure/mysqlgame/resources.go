package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type ResourcesRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type transactionRunner interface {
	WithTransaction(context.Context, func(Queryer, Execer) error) error
}

type sqlTransactionRunner struct {
	tx      *sql.Tx
	dialect SQLDialect
}

func (q SQLQueryer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if detectSQLDialect(q.DB) == DialectSQLite {
		query = rewriteSQLiteLimitedMutation(query)
	}
	return q.DB.ExecContext(ctx, query, args...)
}

func (q SQLQueryer) WithTransaction(ctx context.Context, run func(Queryer, Execer) error) error {
	if q.DB == nil {
		return errors.New("game database unavailable")
	}
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	runner := sqlTransactionRunner{tx: tx, dialect: detectSQLDialect(q.DB)}
	if err := run(runner, runner); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r sqlTransactionRunner) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return r.tx.QueryContext(ctx, query, args...)
}

func (r sqlTransactionRunner) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r.dialect == DialectSQLite {
		query = rewriteSQLiteLimitedMutation(query)
	}
	return r.tx.ExecContext(ctx, query, args...)
}

func NewResourcesRepository(db *sql.DB, prefix string) ResourcesRepository {
	runner := SQLQueryer{DB: db}
	return ResourcesRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewResourcesReadRepository(db *sql.DB, prefix string) ResourcesRepository {
	return NewResourcesRepositoryWithRunner(SQLQueryer{DB: db}, nil, prefix, time.Now)
}

func NewResourcesRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) ResourcesRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewResourcesRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewResourcesRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) ResourcesRepository {
	if now == nil {
		now = time.Now
	}
	return ResourcesRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r ResourcesRepository) GetResources(ctx context.Context, query appgame.ResourcesQuery) (domaingame.ResourceProduction, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	levels, satellites, factors, err := r.loadProductionSettings(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	energyResearch, geologist, engineer, err := r.loadResourceUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	speed, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	return domaingame.BuildResourceProduction(overview, domaingame.ResourceProductionInputs{
		Levels:            levels,
		SolarSatellites:   satellites,
		ProductionFactors: factors,
		EnergyResearch:    energyResearch,
		UniverseSpeed:     speed,
		Geologist:         geologist,
		Engineer:          engineer,
	}), nil
}

func (r ResourcesRepository) GetMCPResourceProductionOptions(ctx context.Context, playerID int, planetID int) (domainmcp.ResourceProductionOptions, error) {
	if r.queryer == nil {
		return domainmcp.ResourceProductionOptions{}, errors.New("resource production reader unavailable")
	}
	reader := r
	reader.updateResources = false
	resources, err := reader.GetResources(ctx, appgame.ResourcesQuery{PlayerID: playerID, PlanetID: planetID})
	if err != nil {
		return domainmcp.ResourceProductionOptions{}, err
	}
	rows := make([]domainmcp.ResourceProductionRow, 0, len(resources.Rows))
	settings := make([]domainmcp.ResourceProductionSetting, 0, len(resources.Rows))
	for _, row := range resources.Rows {
		icons := make([]domainmcp.ResourceProductionBonusIcon, 0, len(row.BonusIcons))
		for _, icon := range row.BonusIcons {
			icons = append(icons, domainmcp.ResourceProductionBonusIcon{Image: icon.Image, Alt: icon.Alt})
		}
		rows = append(rows, domainmcp.ResourceProductionRow{
			ID:         row.ID,
			Name:       row.Name,
			Level:      row.Level,
			Percent:    row.Percent,
			Values:     mcpResourceProductionValues(row.Values),
			BonusIcons: icons,
		})
		settings = append(settings, domainmcp.ResourceProductionSetting{
			ID:      row.ID,
			Name:    row.Name,
			Percent: row.Percent,
		})
	}
	return domainmcp.ResourceProductionOptions{
		PlayerID: playerID,
		Planet: domainmcp.Planet{
			ID:       resources.CurrentPlanet.ID,
			Name:     resources.CurrentPlanet.Name,
			Type:     resources.CurrentPlanet.Type,
			TypeName: mcpPlanetTypeName(resources.CurrentPlanet.Type),
			Coordinates: domainmcp.Coordinates{
				Galaxy:   resources.CurrentPlanet.Coordinates.Galaxy,
				System:   resources.CurrentPlanet.Coordinates.System,
				Position: resources.CurrentPlanet.Coordinates.Position,
			},
			Current: true,
		},
		Factor:   resources.Factor,
		Natural:  mcpResourceProductionValues(resources.Natural),
		Rows:     rows,
		Storage:  mcpResourceProductionValues(resources.Storage),
		Totals:   mcpResourceProductionTotals(resources.Totals),
		Settings: settings,
	}, nil
}

func (r ResourcesRepository) UpdateProduction(ctx context.Context, query appgame.ResourcesUpdateQuery) (domaingame.ResourceProduction, error) {
	if r.execer == nil {
		return domaingame.ResourceProduction{}, errors.New("resource production updater unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	vacation, err := r.loadVacation(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	if !vacation {
		if err := r.updateProductionSettings(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID, query.Production); err != nil {
			return domaingame.ResourceProduction{}, err
		}
	}

	return r.GetResources(ctx, appgame.ResourcesQuery{
		PlayerID: query.PlayerID,
		PlanetID: overview.CurrentPlanet.ID,
	})
}

func (r ResourcesRepository) PreviewMCPUpdateResourceProduction(ctx context.Context, playerID int, command domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error) {
	if r.queryer == nil {
		return domainmcp.UpdateResourceProductionResult{}, errors.New("resource production reader unavailable")
	}
	production, err := mcpResourceProductionFactors(command.Production)
	if err != nil {
		return domainmcp.UpdateResourceProductionResult{}, err
	}
	result := domainmcp.UpdateResourceProductionResult{PlayerID: playerID, PlanetID: command.PlanetID}
	if len(production) == 0 {
		result.Issue = &domainmcp.ActionIssue{Code: "invalid_production", Message: "No supported resource production settings were provided."}
		return result, nil
	}
	preview := r
	preview.updateResources = false
	resources, err := preview.GetResources(ctx, appgame.ResourcesQuery{PlayerID: playerID, PlanetID: command.PlanetID})
	if err != nil {
		return domainmcp.UpdateResourceProductionResult{}, err
	}
	result.PlanetID = resources.CurrentPlanet.ID
	result.Settings = mcpResourceProductionSettings(resources, production)
	if len(result.Settings) == 0 {
		result.Issue = &domainmcp.ActionIssue{Code: "invalid_production", Message: "No supported resource production settings were provided."}
	}
	return result, nil
}

func (r ResourcesRepository) UpdateMCPResourceProduction(ctx context.Context, playerID int, command domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error) {
	if r.execer == nil {
		return domainmcp.UpdateResourceProductionResult{}, errors.New("resource production updater unavailable")
	}
	result, err := r.PreviewMCPUpdateResourceProduction(ctx, playerID, command)
	if err != nil || result.Issue != nil {
		return result, err
	}
	production, err := mcpResourceProductionFactors(command.Production)
	if err != nil {
		return domainmcp.UpdateResourceProductionResult{}, err
	}
	resources, err := r.UpdateProduction(ctx, appgame.ResourcesUpdateQuery{
		PlayerID:   playerID,
		PlanetID:   command.PlanetID,
		Production: production,
	})
	if err != nil {
		return domainmcp.UpdateResourceProductionResult{}, err
	}
	result.PlanetID = resources.CurrentPlanet.ID
	result.Executed = true
	return result, nil
}

func (r ResourcesRepository) loadProductionSettings(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.BuildingLevels, int, domaingame.ProductionFactors, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			planetsTable,
		),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, 0, nil, err
		}
		return nil, 0, nil, errors.New("resource production settings not found")
	}

	var metalMine, crystalMine, deuteriumSynth, solarPlant, fusionReactor, solarSatellites int
	var prodMetal, prodCrystal, prodDeuterium, prodSolar, prodFusion, prodSatellite float64
	if err := rows.Scan(
		&metalMine,
		&crystalMine,
		&deuteriumSynth,
		&solarPlant,
		&fusionReactor,
		&solarSatellites,
		&prodMetal,
		&prodCrystal,
		&prodDeuterium,
		&prodSolar,
		&prodFusion,
		&prodSatellite,
	); err != nil {
		return nil, 0, nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}

	return domaingame.BuildingLevels{
			domaingame.BuildingMetalMine:      metalMine,
			domaingame.BuildingCrystalMine:    crystalMine,
			domaingame.BuildingDeuteriumSynth: deuteriumSynth,
			domaingame.BuildingSolarPlant:     solarPlant,
			domaingame.BuildingFusionReactor:  fusionReactor,
		},
		solarSatellites,
		domaingame.ProductionFactors{
			domaingame.BuildingMetalMine:      prodMetal,
			domaingame.BuildingCrystalMine:    prodCrystal,
			domaingame.BuildingDeuteriumSynth: prodDeuterium,
			domaingame.BuildingSolarPlant:     prodSolar,
			domaingame.BuildingFusionReactor:  prodFusion,
			domaingame.FleetSolarSatellite:    prodSatellite,
		},
		nil
}

func mcpResourceProductionFactors(settings map[int]int) (domaingame.ProductionFactors, error) {
	return domaingame.NormalizeProductionSettings(domaingame.ProductionPercents(settings))
}

func mcpResourceProductionSettings(resources domaingame.ResourceProduction, production domaingame.ProductionFactors) []domainmcp.ResourceProductionSetting {
	names := map[int]string{}
	for _, row := range resources.Rows {
		names[row.ID] = row.Name
	}
	settings := make([]domainmcp.ResourceProductionSetting, 0, len(production))
	for _, id := range domaingame.ResourceProducerIDs() {
		factor, ok := production[id]
		if !ok {
			continue
		}
		name := names[id]
		if name == "" {
			name = fmt.Sprintf("prod%d", id)
		}
		settings = append(settings, domainmcp.ResourceProductionSetting{
			ID:      id,
			Name:    name,
			Percent: int(factor*100 + 0.5),
		})
	}
	return settings
}

func mcpResourceProductionValues(values domaingame.ResourceProductionValues) domainmcp.ResourceProductionValues {
	return domainmcp.ResourceProductionValues{
		Metal:        values.Metal,
		Crystal:      values.Crystal,
		Deuterium:    values.Deuterium,
		Energy:       values.Energy,
		EnergyRaw:    values.EnergyRaw,
		EnergyStored: values.EnergyStored,
	}
}

func mcpResourceProductionTotals(totals domaingame.ResourceProductionTotals) domainmcp.ResourceProductionTotals {
	return domainmcp.ResourceProductionTotals{
		Hour: mcpResourceProductionValues(totals.Hour),
		Day:  mcpResourceProductionValues(totals.Day),
		Week: mcpResourceProductionValues(totals.Week),
	}
}

func (r ResourcesRepository) loadResourceUser(ctx context.Context, usersTable string, playerID int) (int, bool, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT `%d`, geo_until, eng_until FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return 0, false, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, false, err
		}
		return 0, false, false, errors.New("resource user not found")
	}
	var energyResearch int
	var geologistUntil int64
	var engineerUntil int64
	if err := rows.Scan(&energyResearch, &geologistUntil, &engineerUntil); err != nil {
		return 0, false, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, false, false, err
	}
	now := r.now().Unix()
	return energyResearch, geologistUntil > now, engineerUntil > now, nil
}

func (r ResourcesRepository) loadVacation(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT vacation FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("resource vacation state not found")
	}
	var vacation int
	if err := rows.Scan(&vacation); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return vacation != 0, nil
}

func (r ResourcesRepository) updateProductionSettings(ctx context.Context, planetsTable string, playerID int, planetID int, production domaingame.ProductionFactors) error {
	assignments := make([]string, 0, len(production))
	args := make([]any, 0, len(production)+3)
	for _, id := range domaingame.ResourceProducerIDs() {
		factor, ok := production[id]
		if !ok {
			continue
		}
		assignments = append(assignments, fmt.Sprintf("prod%d = ?", id))
		args = append(args, factor)
	}
	if len(assignments) == 0 {
		return nil
	}
	args = append(args, planetID, playerID, planetTypeDebris)
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET %s WHERE planet_id = ? AND owner_id = ? AND type < ?", planetsTable, strings.Join(assignments, ", ")),
		args...,
	)
	return err
}

package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type MerchantRepository struct {
	queryer   Queryer
	execer    Execer
	overview  OverviewRepository
	prefix    string
	randomInt func(min int, max int) int
}

func NewMerchantRepository(db *sql.DB, prefix string) MerchantRepository {
	runner := SQLQueryer{DB: db}
	return MerchantRepository{
		queryer:   runner,
		execer:    runner,
		overview:  NewOverviewRepository(db, prefix),
		prefix:    prefix,
		randomInt: randomMerchantInt,
	}
}

func NewMerchantRepositoryWithQueryer(queryer Queryer, prefix string) MerchantRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewMerchantRepositoryWithRunner(queryer, execer, prefix, randomMerchantInt)
}

func NewMerchantRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, randomInt func(min int, max int) int) MerchantRepository {
	if randomInt == nil {
		randomInt = randomMerchantInt
	}
	return MerchantRepository{
		queryer:   queryer,
		execer:    execer,
		overview:  NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:    prefix,
		randomInt: randomInt,
	}
}

func (r MerchantRepository) GetMerchant(ctx context.Context, query appgame.MerchantQuery) (domaingame.Merchant, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Merchant{}, err
	}
	user, activeOfferID, rates, err := r.loadMerchantUser(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Merchant{}, err
	}
	return domaingame.NewMerchant(overview, user, activeOfferID, rates), nil
}

func (r MerchantRepository) MutateMerchant(ctx context.Context, query appgame.MerchantMutationQuery) (domaingame.Merchant, *domaingame.MerchantActionIssue, error) {
	if r.execer == nil {
		return domaingame.Merchant{}, nil, errors.New("merchant updater unavailable")
	}
	action := strings.ToLower(strings.TrimSpace(query.Mutation.Action))
	if action == "" {
		action = "call"
	}
	switch action {
	case "call":
		return r.callMerchant(ctx, query)
	case "trade":
		return r.tradeMerchant(ctx, query)
	default:
		merchant, err := r.GetMerchant(ctx, appgame.MerchantQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
		return merchant, nil, err
	}
}

func (r MerchantRepository) callMerchant(ctx context.Context, query appgame.MerchantMutationQuery) (domaingame.Merchant, *domaingame.MerchantActionIssue, error) {
	current, err := r.GetMerchant(ctx, appgame.MerchantQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	offerID := domaingame.NormalizeMerchantOfferID(query.Mutation.OfferID)
	if offerID == 0 {
		return current, nil, nil
	}
	spent, ok := domaingame.SpendMerchantDarkMatter(current.User)
	if !ok {
		return current, domaingame.MerchantNotEnoughDarkMatterIssue(), nil
	}
	rates, ok := r.generateRates(offerID)
	if !ok {
		return current, nil, nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET dm = ?, dmfree = ?, trader = ?, rate_m = ?, rate_k = ?, rate_d = ? WHERE player_id = ? LIMIT 1", usersTable),
		spent.PaidDarkMatter,
		spent.FreeDarkMatter,
		offerID,
		rates.Metal,
		rates.Crystal,
		rates.Deuterium,
		query.PlayerID,
	); err != nil {
		return domaingame.Merchant{}, nil, err
	}
	updated, err := r.GetMerchant(ctx, appgame.MerchantQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	return updated, nil, nil
}

func (r MerchantRepository) tradeMerchant(ctx context.Context, query appgame.MerchantMutationQuery) (domaingame.Merchant, *domaingame.MerchantActionIssue, error) {
	current, err := r.GetMerchant(ctx, appgame.MerchantQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	result, issue := domaingame.ResolveMerchantTrade(current.ActiveOfferID, current.Rates, current.CurrentPlanet.Resources, query.Mutation.Values)
	if issue != nil || !result.Changed {
		return current, issue, nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET trader = 0 WHERE player_id = ? LIMIT 1", usersTable), query.PlayerID); err != nil {
		return domaingame.Merchant{}, nil, err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = ?, `%d` = ?, `%d` = ? WHERE planet_id = ? AND owner_id = ? LIMIT 1", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium),
		result.Metal,
		result.Crystal,
		result.Deuterium,
		current.CurrentPlanet.ID,
		query.PlayerID,
	); err != nil {
		return domaingame.Merchant{}, nil, err
	}
	updated, err := r.GetMerchant(ctx, appgame.MerchantQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Merchant{}, nil, err
	}
	return updated, nil, nil
}

func (r MerchantRepository) loadMerchantUser(ctx context.Context, playerID int) (domaingame.MerchantUser, int, domaingame.MerchantRates, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(dm, 0), COALESCE(dmfree, 0), COALESCE(trader, 0), COALESCE(rate_m, 0), COALESCE(rate_k, 0), COALESCE(rate_d, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, err
		}
		return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, errors.New("merchant user not found")
	}
	var paidDarkMatter int
	var freeDarkMatter int
	var activeOfferID int
	var rateMetal float64
	var rateCrystal float64
	var rateDeuterium float64
	if err := rows.Scan(&paidDarkMatter, &freeDarkMatter, &activeOfferID, &rateMetal, &rateCrystal, &rateDeuterium); err != nil {
		return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.MerchantUser{}, 0, domaingame.MerchantRates{}, err
	}
	return domaingame.MerchantUser{PaidDarkMatter: paidDarkMatter, FreeDarkMatter: freeDarkMatter},
		activeOfferID,
		domaingame.MerchantRates{Metal: rateMetal, Crystal: rateCrystal, Deuterium: rateDeuterium},
		nil
}

func (r MerchantRepository) generateRates(offerID int) (domaingame.MerchantRates, bool) {
	roll := r.randomInt(0, 99)
	switch offerID {
	case domaingame.MerchantResourceMetal:
		return domaingame.GenerateMerchantRates(offerID, roll, r.randomInt(140, 200), r.randomInt(70, 100))
	case domaingame.MerchantResourceCrystal:
		return domaingame.GenerateMerchantRates(offerID, roll, r.randomInt(210, 300), r.randomInt(70, 100))
	case domaingame.MerchantResourceDeuterium:
		return domaingame.GenerateMerchantRates(offerID, roll, r.randomInt(210, 300), r.randomInt(140, 200))
	default:
		return domaingame.MerchantRates{}, false
	}
}

func randomMerchantInt(min int, max int) int {
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min+1)
}

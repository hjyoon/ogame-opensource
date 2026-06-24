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
)

type AllianceRepository struct {
	queryer  Queryer
	execer   Execer
	overview OverviewRepository
	prefix   string
	now      func() time.Time
}

func NewAllianceRepository(db *sql.DB, prefix string) AllianceRepository {
	runner := SQLQueryer{DB: db}
	return AllianceRepository{
		queryer:  runner,
		execer:   runner,
		overview: NewOverviewRepository(db, prefix),
		prefix:   prefix,
		now:      time.Now,
	}
}

func NewAllianceRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) AllianceRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewAllianceRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewAllianceRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) AllianceRepository {
	if now == nil {
		now = time.Now
	}
	return AllianceRepository{
		queryer:  queryer,
		execer:   execer,
		overview: NewOverviewRepositoryWithRunner(queryer, execer, prefix),
		prefix:   prefix,
		now:      now,
	}
}

func (r AllianceRepository) GetAlliance(ctx context.Context, query appgame.AllianceQuery) (domaingame.Alliance, error) {
	overview, err := r.overview.GetOverview(ctx, appgame.OverviewQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
	if err != nil {
		return domaingame.Alliance{}, err
	}
	viewer, err := r.loadAllianceViewer(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Alliance{}, err
	}
	alliance := domaingame.NewAlliance(overview, viewer, r.now()).WithView(query.View)
	if viewer.AllianceID <= 0 {
		return r.populateNoAlliance(ctx, alliance, query)
	}
	return r.populateOwnAlliance(ctx, alliance, query)
}

func (r AllianceRepository) MutateAlliance(ctx context.Context, query appgame.AllianceMutationQuery) (domaingame.Alliance, *domaingame.AllianceActionIssue, error) {
	if r.execer == nil {
		return domaingame.Alliance{}, nil, errors.New("alliance updater unavailable")
	}
	viewer, err := r.loadAllianceViewer(ctx, query.PlayerID)
	if err != nil {
		return domaingame.Alliance{}, nil, err
	}
	mutation := query.Mutation
	switch mutation.Action {
	case "create":
		issue, err := r.createAlliance(ctx, viewer, mutation)
		if err != nil || issue != nil && issue.Code != domaingame.AllianceIssueCreated {
			alliance, loadErr := r.GetAlliance(ctx, query.Query)
			if loadErr != nil {
				return domaingame.Alliance{}, nil, loadErr
			}
			return alliance, issue, err
		}
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID, View: domaingame.AllianceViewHome})
		return alliance, issue, loadErr
	case "apply":
		issue, err := r.applyAlliance(ctx, viewer, mutation)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID, View: domaingame.AllianceViewHome})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "withdraw":
		issue, err := r.withdrawApplication(ctx, viewer)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID, View: domaingame.AllianceViewHome})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "accept", "reject":
		issue, err := r.reviewApplication(ctx, viewer, mutation)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{
			PlayerID: query.PlayerID,
			PlanetID: query.PlanetID,
			View:     domaingame.AllianceViewApplications,
		})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "leave":
		issue, err := r.leaveAlliance(ctx, viewer)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID, View: domaingame.AllianceViewHome})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "save_text":
		issue, err := r.saveAllianceText(ctx, viewer, mutation)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{
			PlayerID: query.PlayerID,
			PlanetID: query.PlanetID,
			View:     domaingame.AllianceViewManagement,
			TextKind: domaingame.NormalizeAllianceTextKind(mutation.TextKind),
		})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "save_settings":
		issue, err := r.saveAllianceSettings(ctx, viewer, mutation)
		alliance, loadErr := r.GetAlliance(ctx, appgame.AllianceQuery{
			PlayerID: query.PlayerID,
			PlanetID: query.PlanetID,
			View:     domaingame.AllianceViewManagement,
		})
		if err != nil {
			return domaingame.Alliance{}, nil, err
		}
		return alliance, issue, loadErr
	case "", "search":
		alliance, err := r.GetAlliance(ctx, query.Query)
		return alliance, nil, err
	default:
		alliance, err := r.GetAlliance(ctx, query.Query)
		return alliance, domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), err
	}
}

func (r AllianceRepository) populateNoAlliance(ctx context.Context, alliance domaingame.Alliance, query appgame.AllianceQuery) (domaingame.Alliance, error) {
	pending, err := r.loadUserApplication(ctx, alliance.Viewer.PlayerID)
	if err != nil {
		return domaingame.Alliance{}, err
	}
	if pending != nil {
		alliance.Pending = pending
		target, err := r.loadAllianceInfo(ctx, pending.AllianceID)
		if err != nil {
			return domaingame.Alliance{}, err
		}
		alliance.Target = target
		alliance.View = domaingame.AllianceViewNoAlliance
		return alliance, nil
	}
	switch query.View {
	case domaingame.AllianceViewCreate:
		alliance.View = domaingame.AllianceViewCreate
	case domaingame.AllianceViewSearch:
		alliance.View = domaingame.AllianceViewSearch
		alliance.SearchText = strings.TrimSpace(query.SearchText)
		if alliance.SearchText != "" {
			results, err := r.searchAlliances(ctx, alliance.SearchText)
			if err != nil {
				return domaingame.Alliance{}, err
			}
			alliance.SearchResults = results
		}
	case domaingame.AllianceViewApply:
		target, err := r.loadAllianceInfo(ctx, query.AllianceID)
		if err != nil {
			return domaingame.Alliance{}, err
		}
		alliance.Target = target
		alliance.View = domaingame.AllianceViewApply
	default:
		alliance.View = domaingame.AllianceViewNoAlliance
	}
	return alliance, nil
}

func (r AllianceRepository) populateOwnAlliance(ctx context.Context, alliance domaingame.Alliance, query appgame.AllianceQuery) (domaingame.Alliance, error) {
	own, err := r.loadAllianceInfo(ctx, alliance.Viewer.AllianceID)
	if err != nil {
		return domaingame.Alliance{}, err
	}
	alliance.Own = own
	alliance.TextKind = domaingame.NormalizeAllianceTextKind(query.TextKind)
	switch query.View {
	case domaingame.AllianceViewApplications:
		alliance.View = domaingame.AllianceViewApplications
		applications, err := r.loadAllianceApplications(ctx, alliance.Viewer.AllianceID)
		if err != nil {
			return domaingame.Alliance{}, err
		}
		alliance.Applications = applications
		if query.ApplicationID > 0 {
			for index := range applications {
				if applications[index].ID == query.ApplicationID {
					app := applications[index]
					alliance.SelectedApp = &app
					break
				}
			}
		}
	case domaingame.AllianceViewMembers:
		alliance.View = domaingame.AllianceViewMembers
		if !alliance.Viewer.CanReadMembers() {
			return alliance, nil
		}
		members, err := r.loadAllianceMembers(ctx, alliance.Viewer.AllianceID)
		if err != nil {
			return domaingame.Alliance{}, err
		}
		alliance.Members = members
	case domaingame.AllianceViewManagement:
		alliance.View = domaingame.AllianceViewManagement
		if !alliance.Viewer.CanManageAlliance() {
			return alliance, nil
		}
		ranks, err := r.loadAllianceRanks(ctx, alliance.Viewer.AllianceID)
		if err != nil {
			return domaingame.Alliance{}, err
		}
		alliance.Ranks = ranks
	default:
		alliance.View = domaingame.AllianceViewHome
	}
	return alliance, nil
}

func (r AllianceRepository) createAlliance(ctx context.Context, viewer domaingame.AllianceViewer, mutation domaingame.AllianceMutation) (*domaingame.AllianceActionIssue, error) {
	if viewer.AllianceID > 0 {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	tag := domaingame.NormalizeAllianceTag(mutation.Tag)
	name := domaingame.NormalizeAllianceName(mutation.Name)
	if issue := domaingame.ValidateAllianceCreate(tag, name); issue != nil {
		return issue, nil
	}
	exists, err := r.allianceTagExists(ctx, tag)
	if err != nil {
		return nil, err
	}
	if exists {
		return domaingame.AllianceIssue(domaingame.AllianceIssueTagExists), nil
	}
	allianceID, err := r.insertAlliance(ctx, viewer.PlayerID, tag, name)
	if err != nil {
		return nil, err
	}
	if err := r.insertDefaultAllianceRanks(ctx, allianceID); err != nil {
		return nil, err
	}
	if err := r.updateFounderAlliance(ctx, viewer.PlayerID, allianceID); err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueCreated), nil
}

func (r AllianceRepository) applyAlliance(ctx context.Context, viewer domaingame.AllianceViewer, mutation domaingame.AllianceMutation) (*domaingame.AllianceActionIssue, error) {
	if viewer.AllianceID > 0 {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	if !viewer.Validated {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNotActivated), nil
	}
	if pending, err := r.loadUserApplication(ctx, viewer.PlayerID); err != nil {
		return nil, err
	} else if pending != nil {
		return domaingame.AllianceIssue(domaingame.AllianceIssueAlreadyApplied), nil
	}
	target, err := r.loadAllianceInfo(ctx, mutation.AllianceID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return domaingame.AllianceIssue(domaingame.AllianceIssueAllianceNotFound), nil
	}
	if !target.Open {
		return domaingame.AllianceIssue(domaingame.AllianceIssueApplicationsClosed), nil
	}
	if err := r.insertApplication(ctx, target.ID, viewer.PlayerID, mutation.Text); err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueApplied), nil
}

func (r AllianceRepository) withdrawApplication(ctx context.Context, viewer domaingame.AllianceViewer) (*domaingame.AllianceActionIssue, error) {
	pending, err := r.loadUserApplication(ctx, viewer.PlayerID)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return domaingame.AllianceIssue(domaingame.AllianceIssueApplicationNotFound), nil
	}
	if err := r.deleteApplication(ctx, pending.ID); err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueWithdrawn), nil
}

func (r AllianceRepository) reviewApplication(ctx context.Context, viewer domaingame.AllianceViewer, mutation domaingame.AllianceMutation) (*domaingame.AllianceActionIssue, error) {
	if viewer.AllianceID <= 0 || !viewer.CanWriteApplications() {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	app, err := r.loadApplication(ctx, mutation.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app == nil || app.AllianceID != viewer.AllianceID {
		return domaingame.AllianceIssue(domaingame.AllianceIssueApplicationNotFound), nil
	}
	if mutation.Action == "accept" {
		if err := r.acceptApplication(ctx, viewer.AllianceID, app.PlayerID); err != nil {
			return nil, err
		}
		if err := r.deleteApplication(ctx, app.ID); err != nil {
			return nil, err
		}
		return domaingame.AllianceIssue(domaingame.AllianceIssueAccepted), nil
	}
	if err := r.deleteApplication(ctx, app.ID); err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueRejected), nil
}

func (r AllianceRepository) leaveAlliance(ctx context.Context, viewer domaingame.AllianceViewer) (*domaingame.AllianceActionIssue, error) {
	if viewer.Founder {
		return domaingame.AllianceIssue(domaingame.AllianceIssueFounderCannotLeave), nil
	}
	if !viewer.CanLeaveAlliance() {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET ally_id = 0, allyrank = 0, joindate = 0 WHERE player_id = ? LIMIT 1", usersTable), viewer.PlayerID); err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueLeft), nil
}

func (r AllianceRepository) saveAllianceText(ctx context.Context, viewer domaingame.AllianceViewer, mutation domaingame.AllianceMutation) (*domaingame.AllianceActionIssue, error) {
	if viewer.AllianceID <= 0 || !viewer.CanManageAlliance() {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	text := domaingame.NormalizeAllianceText(mutation.Text)
	switch domaingame.NormalizeAllianceTextKind(mutation.TextKind) {
	case 2:
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET inttext = ? WHERE ally_id = ? LIMIT 1", allyTable), text, viewer.AllianceID)
	case 3:
		insertApp := 0
		if mutation.InsertApp {
			insertApp = 1
		}
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET apptext = ?, insertapp = ? WHERE ally_id = ? LIMIT 1", allyTable), text, insertApp, viewer.AllianceID)
	default:
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET exttext = ? WHERE ally_id = ? LIMIT 1", allyTable), text, viewer.AllianceID)
	}
	if err != nil {
		return nil, err
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueSaved), nil
}

func (r AllianceRepository) saveAllianceSettings(ctx context.Context, viewer domaingame.AllianceViewer, mutation domaingame.AllianceMutation) (*domaingame.AllianceActionIssue, error) {
	if viewer.AllianceID <= 0 || !viewer.CanManageAlliance() {
		return domaingame.AllianceIssue(domaingame.AllianceIssueNoPermission), nil
	}
	if issue := domaingame.ValidateAllianceRankName(mutation.FounderRankName); issue != nil {
		return issue, nil
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	ranksTable, err := tableName(r.prefix, "allyranks")
	if err != nil {
		return nil, err
	}
	open := 0
	if mutation.Open {
		open = 1
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET open = ?, homepage = ?, imglogo = ? WHERE ally_id = ? LIMIT 1", allyTable),
		open,
		domaingame.NormalizeAllianceURL(mutation.Homepage),
		domaingame.NormalizeAllianceURL(mutation.ImageLogo),
		viewer.AllianceID,
	); err != nil {
		return nil, err
	}
	if mutation.FounderRankName != "" {
		if _, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET name = ? WHERE ally_id = ? AND rank_id = ? LIMIT 1", ranksTable),
			mutation.FounderRankName,
			viewer.AllianceID,
			domaingame.AllianceRankFounder,
		); err != nil {
			return nil, err
		}
	}
	return domaingame.AllianceIssue(domaingame.AllianceIssueSaved), nil
}

func (r AllianceRepository) loadAllianceViewer(ctx context.Context, playerID int) (domaingame.AllianceViewer, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.AllianceViewer{}, err
	}
	ranksTable, err := tableName(r.prefix, "allyranks")
	if err != nil {
		return domaingame.AllianceViewer{}, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.validated, 0), COALESCE(u.ally_id, 0), COALESCE(u.allyrank, 0), COALESCE(r.name, ''), COALESCE(r.rights, 0) FROM %s u LEFT JOIN %s r ON u.ally_id = r.ally_id AND u.allyrank = r.rank_id WHERE u.player_id = ? LIMIT 1",
		usersTable,
		ranksTable,
	), playerID)
	if err != nil {
		return domaingame.AllianceViewer{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.AllianceViewer{}, err
		}
		return domaingame.AllianceViewer{}, errors.New("alliance viewer not found")
	}
	var viewer domaingame.AllianceViewer
	var validated int
	if err := rows.Scan(&viewer.PlayerID, &viewer.Name, &validated, &viewer.AllianceID, &viewer.RankID, &viewer.RankName, &viewer.RankRights); err != nil {
		return domaingame.AllianceViewer{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.AllianceViewer{}, err
	}
	viewer.Validated = validated != 0
	viewer.Founder = viewer.AllianceID > 0 && viewer.RankID == domaingame.AllianceRankFounder
	return viewer, nil
}

func (r AllianceRepository) loadAllianceInfo(ctx context.Context, allianceID int) (*domaingame.AllianceInfo, error) {
	if allianceID <= 0 {
		return nil, nil
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT a.ally_id, COALESCE(a.tag, ''), COALESCE(a.name, ''), COALESCE(a.owner_id, 0), COALESCE(a.homepage, ''), COALESCE(a.imglogo, ''), COALESCE(a.open, 0), COALESCE(a.insertapp, 0), COALESCE(a.exttext, ''), COALESCE(a.inttext, ''), COALESCE(a.apptext, ''), COALESCE(a.old_tag, ''), COALESCE(a.old_name, ''), COALESCE(a.tag_until, 0), COALESCE(a.name_until, 0), (SELECT COUNT(*) FROM %s u WHERE u.ally_id = a.ally_id), (SELECT COUNT(*) FROM %s aa WHERE aa.ally_id = a.ally_id) FROM %s a WHERE a.ally_id = ? LIMIT 1",
		usersTable,
		appsTable,
		allyTable,
	), allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var info domaingame.AllianceInfo
	var open int
	var insertApp int
	if err := rows.Scan(
		&info.ID,
		&info.Tag,
		&info.Name,
		&info.OwnerID,
		&info.Homepage,
		&info.ImageLogo,
		&open,
		&insertApp,
		&info.ExternalText,
		&info.InternalText,
		&info.ApplicationText,
		&info.OldTag,
		&info.OldName,
		&info.TagUntil,
		&info.NameUntil,
		&info.MemberCount,
		&info.ApplicationCount,
	); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	info.Open = open != 0
	info.InsertApp = insertApp != 0
	return &info, nil
}

func (r AllianceRepository) allianceTagExists(ctx context.Context, tag string) (bool, error) {
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return false, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT ally_id FROM %s WHERE tag = ? LIMIT 1", allyTable), tag)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (r AllianceRepository) searchAlliances(ctx context.Context, text string) ([]domaingame.AllianceSearchResult, error) {
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT a.ally_id, COALESCE(a.tag, ''), COALESCE(a.name, ''), COUNT(u.player_id) FROM %s a LEFT JOIN %s u ON u.ally_id = a.ally_id WHERE a.tag LIKE ? GROUP BY a.ally_id, a.tag, a.name ORDER BY a.ally_id ASC LIMIT 30",
		allyTable,
		usersTable,
	), "%"+text+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []domaingame.AllianceSearchResult{}
	for rows.Next() {
		var row domaingame.AllianceSearchResult
		if err := rows.Scan(&row.ID, &row.Tag, &row.Name, &row.MemberCount); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r AllianceRepository) loadUserApplication(ctx context.Context, playerID int) (*domaingame.AllianceApplication, error) {
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT aa.app_id, aa.ally_id, aa.player_id, COALESCE(u.oname, ''), COALESCE(aa.text, ''), COALESCE(aa.date, 0) FROM %s aa LEFT JOIN %s u ON u.player_id = aa.player_id WHERE aa.player_id = ? ORDER BY aa.app_id ASC LIMIT 1",
		appsTable,
		usersTable,
	), playerID)
	if err != nil {
		return nil, err
	}
	return scanOneAllianceApplication(rows)
}

func (r AllianceRepository) loadApplication(ctx context.Context, applicationID int) (*domaingame.AllianceApplication, error) {
	if applicationID <= 0 {
		return nil, nil
	}
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT aa.app_id, aa.ally_id, aa.player_id, COALESCE(u.oname, ''), COALESCE(aa.text, ''), COALESCE(aa.date, 0) FROM %s aa LEFT JOIN %s u ON u.player_id = aa.player_id WHERE aa.app_id = ? LIMIT 1",
		appsTable,
		usersTable,
	), applicationID)
	if err != nil {
		return nil, err
	}
	return scanOneAllianceApplication(rows)
}

func (r AllianceRepository) loadAllianceApplications(ctx context.Context, allianceID int) ([]domaingame.AllianceApplication, error) {
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT aa.app_id, aa.ally_id, aa.player_id, COALESCE(u.oname, ''), COALESCE(aa.text, ''), COALESCE(aa.date, 0) FROM %s aa LEFT JOIN %s u ON u.player_id = aa.player_id WHERE aa.ally_id = ? ORDER BY aa.date ASC, aa.app_id ASC",
		appsTable,
		usersTable,
	), allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applications := []domaingame.AllianceApplication{}
	for rows.Next() {
		var app domaingame.AllianceApplication
		if err := rows.Scan(&app.ID, &app.AllianceID, &app.PlayerID, &app.PlayerName, &app.Text, &app.Date); err != nil {
			return nil, err
		}
		applications = append(applications, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applications, nil
}

func (r AllianceRepository) loadAllianceMembers(ctx context.Context, allianceID int) ([]domaingame.AllianceMember, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	ranksTable, err := tableName(r.prefix, "allyranks")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT u.player_id, COALESCE(u.oname, ''), COALESCE(u.allyrank, 0), COALESCE(r.name, ''), COALESCE(u.score1, 0), COALESCE(u.joindate, 0), COALESCE(u.lastclick, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0) FROM %s u LEFT JOIN %s r ON u.ally_id = r.ally_id AND u.allyrank = r.rank_id LEFT JOIN %s p ON u.hplanetid = p.planet_id WHERE u.ally_id = ? ORDER BY u.player_id ASC",
		usersTable,
		ranksTable,
		planetsTable,
	), allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []domaingame.AllianceMember{}
	for rows.Next() {
		var member domaingame.AllianceMember
		if err := rows.Scan(&member.PlayerID, &member.Name, &member.RankID, &member.RankName, &member.Score, &member.JoinedAt, &member.LastClick, &member.Galaxy, &member.System, &member.Position); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func (r AllianceRepository) loadAllianceRanks(ctx context.Context, allianceID int) ([]domaingame.AllianceRank, error) {
	ranksTable, err := tableName(r.prefix, "allyranks")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT rank_id, COALESCE(name, ''), COALESCE(rights, 0) FROM %s WHERE ally_id = ? ORDER BY rank_id ASC", ranksTable), allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ranks := []domaingame.AllianceRank{}
	for rows.Next() {
		var rank domaingame.AllianceRank
		if err := rows.Scan(&rank.ID, &rank.Name, &rank.Rights); err != nil {
			return nil, err
		}
		ranks = append(ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ranks, nil
}

func scanOneAllianceApplication(rows Rows) (*domaingame.AllianceApplication, error) {
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var app domaingame.AllianceApplication
	if err := rows.Scan(&app.ID, &app.AllianceID, &app.PlayerID, &app.PlayerName, &app.Text, &app.Date); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &app, nil
}

func (r AllianceRepository) insertAlliance(ctx context.Context, ownerID int, tag string, name string) (int, error) {
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return 0, err
	}
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (tag, name, owner_id, homepage, imglogo, open, insertapp, exttext, inttext, apptext, nextrank, old_tag, old_name, tag_until, name_until, score1, score2, score3, place1, place2, place3, oldscore1, oldscore2, oldscore3, oldplace1, oldplace2, oldplace3, scoredate) VALUES (?, ?, ?, '', '', 1, 0, ?, '', '', 2, '', '', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)",
		allyTable,
	), tag, name, ownerID, "Welcome to the alliance page")
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r AllianceRepository) insertDefaultAllianceRanks(ctx context.Context, allianceID int) error {
	ranksTable, err := tableName(r.prefix, "allyranks")
	if err != nil {
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (rank_id, ally_id, name, rights) VALUES (?, ?, ?, ?), (?, ?, ?, ?)",
		ranksTable,
	), domaingame.AllianceRankFounder, allianceID, "Founder", domaingame.AllianceFounderRights, domaingame.AllianceRankNewcomer, allianceID, "Newcomer", 0)
	return err
}

func (r AllianceRepository) updateFounderAlliance(ctx context.Context, playerID int, allianceID int) error {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET ally_id = ?, joindate = ?, allyrank = 0 WHERE player_id = ? LIMIT 1", usersTable), allianceID, r.now().Unix(), playerID)
	return err
}

func (r AllianceRepository) insertApplication(ctx context.Context, allianceID int, playerID int, text string) error {
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (ally_id, player_id, text, date) VALUES (?, ?, ?, ?)", appsTable), allianceID, playerID, text, r.now().Unix())
	return err
}

func (r AllianceRepository) acceptApplication(ctx context.Context, allianceID int, playerID int) error {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET ally_id = ?, allyrank = ?, joindate = ? WHERE player_id = ? LIMIT 1", usersTable), allianceID, domaingame.AllianceRankNewcomer, r.now().Unix(), playerID)
	return err
}

func (r AllianceRepository) deleteApplication(ctx context.Context, applicationID int) error {
	appsTable, err := tableName(r.prefix, "allyapps")
	if err != nil {
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE app_id = ? LIMIT 1", appsTable), applicationID)
	return err
}

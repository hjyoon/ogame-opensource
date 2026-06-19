package mysqlgame

import (
	"context"
	"database/sql"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type ReportRepository struct {
	queryer Queryer
	prefix  string
}

func NewReportRepository(db *sql.DB, prefix string) ReportRepository {
	return ReportRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewReportRepositoryWithQueryer(queryer Queryer, prefix string) ReportRepository {
	return ReportRepository{queryer: queryer, prefix: prefix}
}

func (r ReportRepository) GetReport(ctx context.Context, query appgame.ReportQuery) (domaingame.Report, error) {
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return domaingame.Report{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Report{}, err
	}

	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT m.owner_id, m.pm, m.text, COALESCE(owner.ally_id, 0), COALESCE(viewer.ally_id, 0) FROM %s m LEFT JOIN %s owner ON owner.player_id = m.owner_id LEFT JOIN %s viewer ON viewer.player_id = ? WHERE m.msg_id = ? LIMIT 1",
			messagesTable,
			usersTable,
			usersTable,
		),
		query.PlayerID,
		query.ReportID,
	)
	if err != nil {
		return domaingame.Report{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.Report{}, err
		}
		return domaingame.NewReport(query.ReportID, domaingame.MessageTypeBattleReportText, "", false), nil
	}

	var ownerID int
	var messageType int
	var text string
	var ownerAllianceID int
	var viewerAllianceID int
	if err := rows.Scan(&ownerID, &messageType, &text, &ownerAllianceID, &viewerAllianceID); err != nil {
		return domaingame.Report{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.Report{}, err
	}

	allowed := ownerID == query.PlayerID ||
		(messageType == domaingame.MessageTypeSpyReport && ownerAllianceID != 0 && ownerAllianceID == viewerAllianceID)
	return domaingame.NewReport(query.ReportID, messageType, text, allowed), nil
}

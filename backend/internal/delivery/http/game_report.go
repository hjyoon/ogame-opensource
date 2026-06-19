package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type gameReportResponse struct {
	Authenticated bool                       `json:"authenticated"`
	Issues        []gameSessionIssueResponse `json:"issues"`
	Report        *gameReportSummary         `json:"report,omitempty"`
}

type gameReportSummary struct {
	ID      int    `json:"id"`
	Type    int    `json:"type"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Allowed bool   `json:"allowed"`
}

func (a app) handleGameReport(w http.ResponseWriter, r *http.Request) {
	if a.deps.GameReport == nil {
		http.Error(w, "game report unavailable", http.StatusServiceUnavailable)
		return
	}
	reportID, err := selectedReportID(r)
	if err != nil {
		http.Error(w, "invalid report id", http.StatusBadRequest)
		return
	}

	result, err := a.deps.GameReport.GetReport(r.Context(), appgame.ReportCommand{
		PublicSession:   r.URL.Query().Get("session"),
		PrivateSessions: cookieMap(r),
		RemoteAddr:      remoteIP(r.RemoteAddr),
		ReportID:        reportID,
	})
	if err != nil {
		http.Error(w, "game report unavailable", http.StatusServiceUnavailable)
		return
	}

	status := http.StatusOK
	var report *gameReportSummary
	if result.Authenticated {
		mapped := toGameReportSummary(result.Report)
		report = &mapped
	} else {
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(gameReportResponse{
		Authenticated: result.Authenticated,
		Issues:        toGameSessionIssueResponses(result.Issues),
		Report:        report,
	})
}

func selectedReportID(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("bericht")
	if raw == "" {
		raw = r.URL.Query().Get("report")
	}
	reportID, err := strconv.Atoi(raw)
	if err != nil || reportID <= 0 {
		return 0, strconv.ErrSyntax
	}
	return reportID, nil
}

func toGameReportSummary(report domaingame.Report) gameReportSummary {
	return gameReportSummary{
		ID:      report.ID,
		Type:    report.Type,
		Title:   report.Title,
		Text:    report.Text,
		Allowed: report.Allowed,
	}
}

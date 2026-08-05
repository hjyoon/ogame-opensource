package game

const (
	ReportTitleBattle = "Battle Report"
	ReportTitleSpy    = "Spy Report"
)

type Report struct {
	ID      int
	Type    int
	Title   string
	Text    string
	Date    int64
	Allowed bool
}

func NewReport(id int, messageType int, text string, allowed bool) Report {
	title := ReportTitleBattle
	if messageType == MessageTypeSpyReport {
		title = ReportTitleSpy
	}
	if !allowed {
		text = ""
	}
	return Report{ID: id, Type: messageType, Title: title, Text: text, Allowed: allowed}
}

func (r Report) WithDate(date int64) Report {
	r.Date = date
	return r
}

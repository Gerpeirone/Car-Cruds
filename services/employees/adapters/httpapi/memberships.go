package httpapi

import (
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"net/http"
	"net/url"
	"time"
)

type membershipInput struct {
	EmployeeID      string    `json:"employeeId"`
	Period          string    `json:"period"`
	SourceReference string    `json:"sourceReference"`
	Version         int64     `json:"version"`
	ObservedAt      time.Time `json:"observedAt"`
	Member          *bool     `json:"member"`
	Used            *bool     `json:"used"`
	CostMinor       *int64    `json:"costMinor"`
	SpendingMinor   *int64    `json:"spendingMinor"`
	Currency        string    `json:"currency"`
}

func (h *Handler) membershipEvidence(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "gym") {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}
	var input membershipInput
	if !decode(w, r, &input) {
		return
	}
	s := domain.MembershipSnapshot{EmployeeID: input.EmployeeID, Period: input.Period, SourceReference: input.SourceReference, Version: input.Version, ObservedAt: input.ObservedAt, Member: input.Member, Used: input.Used, CostMinor: input.CostMinor, SpendingMinor: input.SpendingMinor, Currency: input.Currency}
	applied, err := h.service.ReceiveMembership(r.Context(), platform.PrincipalFrom(r.Context()).App, s)
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, struct {
		Applied bool `json:"applied"`
	}{applied})
}

type membershipItem struct {
	EmployeeID  string           `json:"employeeId"`
	DisplayName string           `json:"displayName"`
	Active      bool             `json:"active"`
	SourceApp   string           `json:"sourceApp,omitempty"`
	Evidence    *membershipInput `json:"evidence"`
}
type membershipTotal struct {
	Currency        string `json:"currency"`
	CostMinor       *int64 `json:"costMinor"`
	SpendingMinor   *int64 `json:"spendingMinor"`
	CostRecords     int64  `json:"costRecords"`
	SpendingRecords int64  `json:"spendingRecords"`
}

func (h *Handler) membershipReport(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin", "service") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["period"]) != 1 {
		writeError(w, domain.ErrInvalidInput)
		return
	}
	period := query.Get("period")
	query.Del("period")
	pageURL := *r.URL
	pageURL.RawQuery = query.Encode()
	page, err := parsePage(&pageURL)
	if err != nil {
		writeError(w, err)
		return
	}
	report, err := h.service.MembershipReport(r.Context(), period, page)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]membershipItem, 0, len(report.Items))
	for _, row := range report.Items {
		item := membershipItem{EmployeeID: row.Employee.ID, DisplayName: row.Employee.DisplayName, Active: row.Employee.Active, SourceApp: row.SourceApp}
		if s := row.Evidence; s != nil {
			item.Evidence = &membershipInput{EmployeeID: s.EmployeeID, Period: s.Period, SourceReference: s.SourceReference, Version: s.Version, ObservedAt: s.ObservedAt, Member: s.Member, Used: s.Used, CostMinor: s.CostMinor, SpendingMinor: s.SpendingMinor, Currency: s.Currency}
		}
		items = append(items, item)
	}
	totals := make([]membershipTotal, 0, len(report.Totals))
	for _, t := range report.Totals {
		totals = append(totals, membershipTotal{t.Currency, t.CostMinor, t.SpendingMinor, t.CostRecords, t.SpendingRecords})
	}
	platform.WriteJSON(w, 200, struct {
		Period             string            `json:"period"`
		Page               int               `json:"page"`
		PageSize           int               `json:"pageSize"`
		Total              int64             `json:"total"`
		MissingEvidence    int64             `json:"missingEvidence"`
		IncompleteEvidence int64             `json:"incompleteEvidence"`
		Items              []membershipItem  `json:"items"`
		Totals             []membershipTotal `json:"totals"`
	}{period, page.Number, page.Size, report.Total, report.MissingEvidence, report.IncompleteEvidence, items, totals})
}

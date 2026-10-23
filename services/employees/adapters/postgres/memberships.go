package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5"
	"time"
)

var _ application.MembershipInbox = (*Repository)(nil)

func (r *Repository) ReceiveMembership(ctx context.Context, source string, s domain.MembershipSnapshot) (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	if !domain.ValidEvidenceKey(source) {
		return false, domain.ErrInvalidInput
	}
	s.ObservedAt = s.ObservedAt.UTC().Truncate(time.Microsecond)
	payload, err := json.Marshal(s)
	if err != nil {
		return false, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// Serialize versions across processes, including the initial insert.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,724104))`, s.EmployeeID+"\x1f"+s.Period); err != nil {
		return false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employees WHERE employee_id=$1)`, s.EmployeeID).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, domain.ErrNotFound
	}
	var version int64
	var app, reference string
	err = tx.QueryRow(ctx, `SELECT version,source_app,source_reference FROM membership_evidence WHERE employee_id=$1 AND period=$2`, s.EmployeeID, s.Period).Scan(&version, &app, &reference)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil && (app != source || reference != s.SourceReference) {
		return false, domain.ErrEvidenceConflict
	}
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT payload FROM membership_evidence_history WHERE employee_id=$1 AND period=$2 AND version=$3`, s.EmployeeID, s.Period, s.Version).Scan(&previous)
	if err == nil {
		if !bytes.Equal(previous, payload) {
			return false, domain.ErrEvidenceConflict
		}
		return false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO membership_evidence_history(employee_id,period,version,source_app,payload) VALUES($1,$2,$3,$4,$5)`, s.EmployeeID, s.Period, s.Version, source, payload); err != nil {
		return false, err
	}
	if s.Version <= version {
		return false, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `INSERT INTO membership_evidence(employee_id,period,source_app,source_reference,version,observed_at,member,used,cost_minor,spending_minor,currency)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
 ON CONFLICT(employee_id,period) DO UPDATE SET version=EXCLUDED.version,observed_at=EXCLUDED.observed_at,member=EXCLUDED.member,used=EXCLUDED.used,cost_minor=EXCLUDED.cost_minor,spending_minor=EXCLUDED.spending_minor,currency=EXCLUDED.currency`, s.EmployeeID, s.Period, source, s.SourceReference, s.Version, s.ObservedAt, s.Member, s.Used, s.CostMinor, s.SpendingMinor, s.Currency)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (r *Repository) MembershipReport(ctx context.Context, period string, page domain.Page) (domain.MembershipReport, error) {
	report := domain.MembershipReport{Period: period, Page: page, Items: []domain.MembershipRow{}, Totals: []domain.CurrencyTotal{}}
	if !domain.ValidEvidenceKey(period) {
		return report, domain.ErrInvalidInput
	}
	if err := page.Validate(); err != nil {
		return report, err
	}
	// One statement gives rows, provenance and totals the same MVCC snapshot.
	rows, err := r.pool.Query(ctx, `SELECT e.employee_id,e.display_name,e.active,m.source_app,m.source_reference,m.version,m.observed_at,m.member,m.used,m.cost_minor,m.spending_minor,m.currency
 FROM employees e LEFT JOIN membership_evidence m ON m.employee_id=e.employee_id AND m.period=$1 ORDER BY e.employee_id`, period)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var row domain.MembershipRow
		var app, reference, currency *string
		var s domain.MembershipSnapshot
		var version *int64
		var observed *time.Time
		if err = rows.Scan(&row.Employee.ID, &row.Employee.DisplayName, &row.Employee.Active, &app, &reference, &version, &observed, &s.Member, &s.Used, &s.CostMinor, &s.SpendingMinor, &currency); err != nil {
			return report, err
		}
		if version != nil {
			s.EmployeeID = row.Employee.ID
			s.Period = period
			s.SourceReference = *reference
			s.Version = *version
			s.ObservedAt = observed.UTC()
			s.Currency = *currency
			row.SourceApp = *app
			row.Evidence = &s
		}
		if err = report.Accumulate(row); err != nil {
			return domain.MembershipReport{}, err
		}
		index := report.Total - 1
		if index >= page.Offset() && len(report.Items) < page.Size {
			report.Items = append(report.Items, row)
		}
	}
	if err = rows.Err(); err != nil {
		return report, err
	}
	report.SortTotals()
	return report, nil
}

// Package clinic adapts the published Turnos para socios v1 contract pinned to
// Octi's commit c0b65f45867f884e0bdc8f7c23eb0f344f646bb2.
package clinic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Client struct {
	base   string
	apiKey string
	http   *http.Client
}

var _ application.Clinic = (*Client)(nil)

func New(base, apiKey string, allowHTTP bool, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) || timeout < 100*time.Millisecond || timeout > 5*time.Second {
		return nil, domain.ErrInvalidInput
	}
	basePath := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(basePath, "/v1") || path.Clean(basePath) != basePath || strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiKey) != apiKey {
		return nil, domain.ErrInvalidInput
	}
	for _, char := range apiKey {
		if char < 32 || char == 127 {
			return nil, domain.ErrInvalidInput
		}
	}
	client := platform.NewHTTPClient(timeout)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: strings.TrimRight(u.String(), "/"), apiKey: apiKey, http: client}, nil
}

type appointmentDTO struct {
	ID        string    `json:"id"`
	PatientID string    `json:"patientId"`
	DoctorID  string    `json:"doctorId"`
	StartAt   time.Time `json:"startAt"`
	EndAt     time.Time `json:"endAt"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
}

func (c *Client) Book(ctx context.Context, r domain.ClinicRequest) (domain.ClinicAppointment, error) {
	if r.ProviderContract != domain.ClinicProviderContract || !domain.ValidClinicIdempotencyKey(r.IdempotencyKey) {
		return domain.ClinicAppointment{}, domain.ErrInvalidInput
	}
	body, _ := json.Marshal(struct {
		PatientID string `json:"patientId"`
		DoctorID  string `json:"doctorId"`
		StartAt   string `json:"startAt"`
	}{r.PatientID, r.DoctorID, r.StartAt.UTC().Format(time.RFC3339)})
	return c.call(ctx, http.MethodPost, "/appointments", body, r.ID, r.IdempotencyKey)
}
func (c *Client) Lookup(ctx context.Context, id string) (domain.ClinicAppointment, error) {
	if !domain.ValidEvidenceKey(id) {
		return domain.ClinicAppointment{}, domain.ErrInvalidInput
	}
	return c.call(ctx, http.MethodGet, "/appointments/"+url.PathEscape(id), nil, id, "")
}
func (c *Client) call(ctx context.Context, method, path string, body []byte, correlation, idempotencyKey string) (domain.ClinicAppointment, error) {
	expected := 200
	if method == http.MethodPost {
		expected = 201
	}
	status, retryAfter, decode, err := c.send(ctx, method, path, body, correlation, idempotencyKey, 65536)
	if err != nil {
		return domain.ClinicAppointment{}, err
	}
	if status != expected {
		if code, ok := providerCode(decode); ok {
			switch {
			case status == 401 && code == "UNAUTHORIZED":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_UNAUTHORIZED"}
			case status == 429 && code == "RATE_LIMITED":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_RATE_LIMITED", RetryAfter: retryAfter}
			case method == http.MethodPost && status == 409 && code == "IDEMPOTENCY_CONFLICT":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_IDEMPOTENCY_CONFLICT"}
			case method == http.MethodGet && status == 404 && code == "NOT_FOUND":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_NOT_FOUND"}
			}
			if method == http.MethodPost {
				// OUTSIDE_SCHEDULE: the provider's main (73e9e28) rejects
				// times outside the doctor's weekly agenda (ART) with 422.
				allowed := map[string]int{"INVALID_REQUEST": 400, "INVALID_JSON": 400, "SLOT_UNAVAILABLE": 409, "PATIENT_NOT_ENABLED": 403, "UNKNOWN_DOCTOR": 404, "UNKNOWN_PATIENT": 404, "LEAD_TIME_TOO_SHORT": 422, "OUTSIDE_SCHEDULE": 422}
				if want, ok := allowed[code]; ok && want == status {
					return domain.ClinicAppointment{}, &domain.ClinicRejection{Code: code}
				}
			}
		}
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	var dto appointmentDTO
	if err = decode(&dto); err != nil {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	a := domain.ClinicAppointment{ID: dto.ID, PatientID: dto.PatientID, DoctorID: dto.DoctorID, StartAt: dto.StartAt.UTC(), EndAt: dto.EndAt.UTC(), Status: dto.Status, Version: dto.Version}
	return a, a.Validate()
}

// Doctors and Usage are read-only: any failure is reported as unavailable
// and never changes a stored request.
func (c *Client) Doctors(ctx context.Context) ([]domain.ClinicDoctor, error) {
	var dto struct {
		Doctors []struct {
			ID              string `json:"id"`
			DurationMinutes int    `json:"durationMinutes"`
		} `json:"doctors"`
	}
	if err := c.read(ctx, "/doctors", 65536, &dto); err != nil {
		return nil, err
	}
	doctors := make([]domain.ClinicDoctor, 0, len(dto.Doctors))
	for _, d := range dto.Doctors {
		doctor := domain.ClinicDoctor{ID: d.ID, DurationMinutes: d.DurationMinutes}
		if err := doctor.Validate(); err != nil {
			return nil, err
		}
		doctors = append(doctors, doctor)
	}
	return doctors, nil
}
func (c *Client) Usage(ctx context.Context, period string) (domain.ClinicUsage, error) {
	if !domain.ValidClinicPeriod(period) {
		return domain.ClinicUsage{}, domain.ErrInvalidInput
	}
	var dto struct {
		Period       string    `json:"period"`
		From         time.Time `json:"from"`
		To           time.Time `json:"to"`
		CurrentPrice struct {
			AmountCents int64  `json:"amountCents"`
			Currency    string `json:"currency"`
		} `json:"currentPrice"`
		Totals []struct {
			Currency    string `json:"currency"`
			Bookings    int64  `json:"bookings"`
			AmountCents int64  `json:"amountCents"`
		} `json:"totals"`
		Items []struct {
			AppointmentID string    `json:"appointmentId"`
			BookedAt      time.Time `json:"bookedAt"`
			PriceCents    int64     `json:"priceCents"`
			Currency      string    `json:"currency"`
			Status        string    `json:"status"`
		} `json:"items"`
	}
	// A busy month can list many charges; 1 MiB still bounds memory.
	if err := c.read(ctx, "/usage?period="+url.QueryEscape(period), 1<<20, &dto); err != nil {
		return domain.ClinicUsage{}, err
	}
	u := domain.ClinicUsage{Period: dto.Period, From: dto.From.UTC(), To: dto.To.UTC(), CurrentPrice: domain.ClinicMoney{AmountMinor: dto.CurrentPrice.AmountCents, Currency: dto.CurrentPrice.Currency}}
	for _, t := range dto.Totals {
		u.Totals = append(u.Totals, domain.ClinicUsageTotal{Currency: t.Currency, Bookings: t.Bookings, AmountMinor: t.AmountCents})
	}
	for _, i := range dto.Items {
		u.Items = append(u.Items, domain.ClinicUsageItem{AppointmentID: i.AppointmentID, BookedAt: i.BookedAt.UTC(), PriceMinor: i.PriceCents, Currency: i.Currency, Status: i.Status})
	}
	return u, u.Validate(period)
}
func (c *Client) read(ctx context.Context, path string, limit int64, out any) error {
	status, _, decode, err := c.send(ctx, http.MethodGet, path, nil, "", "", limit)
	if err != nil {
		return err
	}
	if status != 200 {
		if code, ok := providerCode(decode); ok && status == 400 && code == "INVALID_PERIOD" {
			return domain.ErrInvalidInput
		}
		return domain.ErrClinicUnavailable
	}
	if err = decode(out); err != nil {
		return domain.ErrClinicUnavailable
	}
	return nil
}

// send performs exactly one HTTP exchange and returns a strict decoder for a
// body of at most limit bytes: one JSON value, nothing after it.
func (c *Client) send(ctx context.Context, method, path string, body []byte, correlation, idempotencyKey string, limit int64) (int, time.Duration, func(any) error, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return 0, 0, nil, domain.ErrClinicUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	if correlation != "" {
		req.Header.Set("X-Correlation-ID", correlation)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		// Disable transport replay. Recovery is an explicit application action
		// using the same persisted key, never an invisible second POST.
		req.GetBody = nil
	}
	response, err := c.http.Do(req)
	if err != nil {
		return 0, 0, nil, domain.ErrClinicUnavailable
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return 0, 0, nil, domain.ErrClinicUnavailable
	}
	decode := func(v any) error {
		decoder := json.NewDecoder(bytes.NewReader(b))
		if err := decoder.Decode(v); err != nil {
			return err
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return domain.ErrClinicUnavailable
		}
		return nil
	}
	return response.StatusCode, retryDelay(response.Header.Get("Retry-After")), decode, nil
}
func providerCode(decode func(any) error) (string, bool) {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if decode(&envelope) != nil {
		return "", false
	}
	return envelope.Error.Code, true
}

// The published header is a number of seconds. Invalid/missing values use a
// conservative one-minute cooldown; no retry is performed by the HTTP client.
func retryDelay(value string) time.Duration {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
		return time.Minute
	}
	return time.Duration(seconds) * time.Second
}

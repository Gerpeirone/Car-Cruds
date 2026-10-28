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
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Correlation-ID", correlation)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		// Disable transport replay. Recovery is an explicit application action
		// using the same persisted key, never an invisible second POST.
		req.GetBody = nil
	}
	response, err := c.http.Do(req)
	if err != nil {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(b) > 65536 {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
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
	expected := 200
	if method == http.MethodPost {
		expected = 201
	}
	if response.StatusCode != expected {
		var envelope struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if decode(&envelope) == nil {
			code := envelope.Error.Code
			switch {
			case response.StatusCode == 401 && code == "UNAUTHORIZED":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_UNAUTHORIZED"}
			case response.StatusCode == 429 && code == "RATE_LIMITED":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_RATE_LIMITED", RetryAfter: retryDelay(response.Header.Get("Retry-After"))}
			case method == http.MethodPost && response.StatusCode == 409 && code == "IDEMPOTENCY_CONFLICT":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_IDEMPOTENCY_CONFLICT"}
			case method == http.MethodGet && response.StatusCode == 404 && code == "NOT_FOUND":
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_NOT_FOUND"}
			}
			if method == http.MethodPost {
				allowed := map[string]int{"INVALID_REQUEST": 400, "INVALID_JSON": 400, "SLOT_UNAVAILABLE": 409, "PATIENT_NOT_ENABLED": 403, "UNKNOWN_DOCTOR": 404, "UNKNOWN_PATIENT": 404, "LEAD_TIME_TOO_SHORT": 422}
				if status, ok := allowed[code]; ok && status == response.StatusCode {
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

// The published header is a number of seconds. Invalid/missing values use a
// conservative one-minute cooldown; no retry is performed by the HTTP client.
func retryDelay(value string) time.Duration {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
		return time.Minute
	}
	return time.Duration(seconds) * time.Second
}

// Package clinic adapts the appointment API observed in Octi's feature/entrega-2
// commit 5e4c2103e8c3b0ffaccd1e02a94d1dcf98f2f96d. It does not use Notifications.
package clinic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Client struct {
	base string
	http *http.Client
}

var _ application.Clinic = (*Client)(nil)

func New(base string, allowHTTP bool, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) || timeout < 100*time.Millisecond || timeout > 5*time.Second {
		return nil, domain.ErrInvalidInput
	}
	client := platform.NewHTTPClient(timeout)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: strings.TrimRight(u.String(), "/"), http: client}, nil
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
	body, _ := json.Marshal(struct {
		PatientID string `json:"patientId"`
		DoctorID  string `json:"doctorId"`
		StartAt   string `json:"startAt"`
	}{r.PatientID, r.DoctorID, r.StartAt.UTC().Format(time.RFC3339)})
	return c.call(ctx, http.MethodPost, "/appointments", body, r.ID)
}
func (c *Client) Lookup(ctx context.Context, id string) (domain.ClinicAppointment, error) {
	if !domain.ValidEvidenceKey(id) {
		return domain.ClinicAppointment{}, domain.ErrInvalidInput
	}
	return c.call(ctx, http.MethodGet, "/appointments/"+url.PathEscape(id), nil, id)
}
func (c *Client) call(ctx context.Context, method, path string, body []byte, correlation string) (domain.ClinicAppointment, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Correlation-ID", correlation)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.GetBody = nil
	}
	// No POST retry or invented Idempotency-Key: the provider does not implement it.
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
		if method == http.MethodPost && decode(&envelope) == nil {
			allowed := map[string]int{"INVALID_REQUEST": 400, "INVALID_JSON": 400, "SLOT_UNAVAILABLE": 409, "PATIENT_NOT_ENABLED": 403, "UNKNOWN_DOCTOR": 404, "UNKNOWN_PATIENT": 404, "LEAD_TIME_TOO_SHORT": 422}
			if status, ok := allowed[envelope.Error.Code]; ok && status == response.StatusCode {
				return domain.ClinicAppointment{}, &domain.ClinicRejection{Code: envelope.Error.Code}
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

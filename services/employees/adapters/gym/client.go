// Package gym implements the published 1.0.1 employee intake contract.
package gym

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Client struct {
	base, key string
	http      *http.Client
}

func New(base, key string, allowHTTP bool) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || !domain.GymText(key, 4096) {
		return nil, errors.New("configuración del gimnasio inválida")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && (local || allowHTTP)) {
		return nil, errors.New("gimnasio requiere HTTPS; HTTP remoto solo con autorización de desarrollo explícita")
	}
	client := platform.NewHTTPClient(2 * time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("redirección rechazada") }
	return &Client{base: strings.TrimRight(u.String(), "/"), key: key, http: client}, nil
}

func (c *Client) Enroll(ctx context.Context, e domain.GymEnrollment) domain.GymOutcome {
	unknown := func(code string) domain.GymOutcome { return domain.GymOutcome{State: "UNKNOWN", ProviderCode: code} }
	body, _ := json.Marshal(struct {
		ID    string `json:"empleadoExternoId"`
		DNI   string `json:"dni"`
		Name  string `json:"nombre"`
		Email string `json:"email"`
	}{e.EmployeeID, e.DNI, e.Name, e.Email})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/v1/integraciones/empleados", bytes.NewReader(body))
	if err != nil {
		return unknown("GYM_REQUEST_FAILED")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.key)
	req.Header.Set("Idempotency-Key", "carcruds-"+e.ID)
	resp, err := c.http.Do(req)
	if err != nil {
		return unknown("GYM_UNAVAILABLE")
	}
	defer resp.Body.Close()
	// The mock is useful for consumer tests, never evidence of a real intake.
	if resp.Header.Get("X-Mock") != "" {
		return unknown("GYM_MOCK_RESPONSE")
	}
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return unknown("GYM_INVALID_RESPONSE")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return unknown("GYM_INVALID_RESPONSE")
	}
	if resp.StatusCode == 201 {
		fields, err := object(raw)
		socio, solicitud := stringField(fields, "socioId"), stringField(fields, "solicitudId")
		pending := stringField(fields, "estadoSolicitud") == "PENDIENTE"
		disabled := bytes.Equal(bytes.TrimSpace(fields["membresiaHabilitada"]), []byte("false"))
		if err != nil || len(fields) != 4 || socio == "" || solicitud == "" || !pending || !disabled {
			return unknown("GYM_INVALID_RESPONSE")
		}
		return domain.GymOutcome{State: "ACCEPTED", SocioID: socio, SolicitudID: solicitud}
	}
	fields, err := object(raw)
	code, message := stringField(fields, "codigo"), stringField(fields, "mensaje")
	if err != nil || len(fields) != 2 || code == "" || message == "" {
		return unknown("GYM_INVALID_RESPONSE")
	}
	switch resp.StatusCode {
	case 400:
		if code == "PARAMETROS_INVALIDOS" {
			return domain.GymOutcome{State: "REJECTED", ProviderCode: code}
		}
	case 409:
		switch code {
		case "IDEMPOTENCIA_EN_CONFLICTO", "EMPLEADO_EXTERNO_EN_CONFLICTO", "DNI_EXISTENTE", "EMAIL_EXISTENTE":
			return domain.GymOutcome{State: "REJECTED", ProviderCode: code}
		}
	case 415:
		if code == "TIPO_CONTENIDO_NO_ADMITIDO" {
			return domain.GymOutcome{State: "REJECTED", ProviderCode: code}
		}
	case 401:
		if code == "NO_AUTENTICADO" {
			return unknown("GYM_AUTHENTICATION_FAILED")
		}
	case 500:
		if code == "ERROR_INTERNO" {
			return unknown("GYM_UNAVAILABLE")
		}
	}
	return unknown("GYM_INVALID_RESPONSE")
}

func stringField(fields map[string]json.RawMessage, key string) string {
	var value string
	if json.Unmarshal(fields[key], &value) != nil || strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

// Reject duplicate/unknown keys, trailing values and null bodies before decoding.
func object(raw []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, errors.New("invalid response")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("invalid response")
		}
		if _, exists := fields[key]; exists {
			return nil, errors.New("duplicate field")
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		fields[key] = value
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') {
		return nil, errors.New("invalid response")
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing response")
	}
	return fields, nil
}

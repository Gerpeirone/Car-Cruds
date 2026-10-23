package gym

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

const accepted = `{"socioId":"socio-fixture","solicitudId":"sol-fixture","estadoSolicitud":"PENDIENTE","membresiaHabilitada":false}`

var enrollment = domain.GymEnrollment{ID: "request-fixture", EmployeeID: "employee-fixture", DNI: "99900001", Email: "persona@example.invalid", Name: "Persona Ficticia"}

func TestConsumerMatchesPublishedRequestAndResponseContract(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/contracts/providers/gym-openapi-1.0.1.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Info       struct{ Version string }
		Components struct {
			Schemas map[string]struct {
				Required             []string
				AdditionalProperties bool
				Properties           map[string]json.RawMessage
			}
		}
	}
	if err = json.Unmarshal(raw, &contract); err != nil || contract.Info.Version != "1.0.1" {
		t.Fatal("provider contract missing or wrong version", err)
	}
	requestSchema := contract.Components.Schemas["EmpleadoExterno"]
	if len(requestSchema.Required) != 4 || requestSchema.AdditionalProperties {
		t.Fatal("provider request changed")
	}
	responseSchema := contract.Components.Schemas["AltaPendiente"]
	var response map[string]json.RawMessage
	if json.Unmarshal([]byte(accepted), &response) != nil || responseSchema.AdditionalProperties || len(response) != len(responseSchema.Required) {
		t.Fatal("provider response changed")
	}
	for _, field := range responseSchema.Required {
		if len(response[field]) == 0 {
			t.Fatal("missing provider response field", field)
		}
	}
	for _, field := range []string{"estadoSolicitud", "membresiaHabilitada"} {
		var property struct {
			Const json.RawMessage `json:"const"`
		}
		if json.Unmarshal(responseSchema.Properties[field], &property) != nil || !jsonEqual(property.Const, response[field]) {
			t.Fatal("provider acceptance semantics changed", field)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/integraciones/empleados" || r.Header.Get("X-API-Key") != "private-fixture-key" || r.Header.Get("Idempotency-Key") != "carcruds-request-fixture" {
			t.Error("wrong request or credentials")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("local identity leaked to provider")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body) != len(requestSchema.Required) {
			t.Error("unexpected personal fields")
		}
		for _, key := range requestSchema.Required {
			if body[key] == "" {
				t.Error("missing contract field", key)
			}
		}
		if body["empleadoExternoId"] != enrollment.EmployeeID || body["dni"] != enrollment.DNI || body["nombre"] != enrollment.Name || body["email"] != enrollment.Email {
			t.Error("incorrect mapping")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		fmt.Fprint(w, accepted)
	}))
	defer server.Close()
	client, err := New(server.URL, "private-fixture-key", false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result := client.Enroll(context.Background(), enrollment)
		if result.State != "ACCEPTED" || result.SocioID != "socio-fixture" || result.SolicitudID != "sol-fixture" {
			t.Fatal(result)
		}
	}
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && x == y
}

func TestProviderFailuresNeverBecomeAnEnabledMembership(t *testing.T) {
	for _, test := range []struct {
		name              string
		status            int
		body, state, code string
		mock              bool
	}{
		{"business conflict", 409, `{"codigo":"DNI_EXISTENTE","mensaje":"conflicto"}`, "REJECTED", "DNI_EXISTENTE", false},
		{"invalid input", 400, `{"codigo":"PARAMETROS_INVALIDOS","mensaje":"datos"}`, "REJECTED", "PARAMETROS_INVALIDOS", false},
		{"wrong credential", 401, `{"codigo":"NO_AUTENTICADO","mensaje":"private response"}`, "UNKNOWN", "GYM_AUTHENTICATION_FAILED", false},
		{"server failure", 500, `{"codigo":"ERROR_INTERNO","mensaje":"private response"}`, "UNKNOWN", "GYM_UNAVAILABLE", false},
		{"enabled membership", 201, strings.Replace(accepted, "false", "true", 1), "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"missing boolean", 201, `{"socioId":"a","solicitudId":"b","estadoSolicitud":"PENDIENTE"}`, "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"null boolean", 201, strings.Replace(accepted, "false", "null", 1), "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"additional property", 201, strings.Replace(accepted, `"socioId":`, `"dni":"private","socioId":`, 1), "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"duplicate key", 201, strings.Replace(accepted, `"socioId":`, `"socioId":"duplicate","socioId":`, 1), "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"trailing body", 201, accepted + ` {}`, "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"wrong error code", 409, `{"codigo":"NEW_UNKNOWN_CODE","mensaje":"error"}`, "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"oversized", 201, strings.Repeat(" ", 65537), "UNKNOWN", "GYM_INVALID_RESPONSE", false},
		{"mock", 201, accepted, "UNKNOWN", "GYM_MOCK_RESPONSE", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if test.mock {
					w.Header().Set("X-Mock", "true")
				}
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			client, _ := New(server.URL, "private-fixture-key", false)
			result := client.Enroll(context.Background(), enrollment)
			if result.State != test.state || result.ProviderCode != test.code || result.SocioID != "" {
				t.Fatal(result)
			}
		})
	}
}

func TestRedirectsAndSlowProviderPreserveUnknownResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer server.Close()
	client, _ := New(server.URL, "private-fixture-key", false)
	client.http.Timeout = 30 * time.Millisecond
	if result := client.Enroll(context.Background(), enrollment); result.State != "UNKNOWN" {
		t.Fatal(result)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, server.URL, 307) }))
	defer redirect.Close()
	client, _ = New(redirect.URL, "private-fixture-key", false)
	if result := client.Enroll(context.Background(), enrollment); result.State != "UNKNOWN" {
		t.Fatal(result)
	}
}

func TestConfigurationCannotLeakKeyOrUseImplicitRemoteHTTP(t *testing.T) {
	for _, base := range []string{"", "http://example.com", "https://user:secret@example.com", "https://example.com/api", "https://example.com?secret=value", "ftp://localhost"} {
		_, err := New(base, "private-fixture-key", false)
		if err == nil || strings.Contains(err.Error(), "private-fixture-key") {
			t.Fatal("unsafe configuration")
		}
	}
	if _, err := New("http://gym-gateway:8080", "private-fixture-key", true); err != nil {
		t.Fatal(err)
	}
}

package httpapi

import (
	"context"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/wire"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
	"mime"
	"net/http"
	"strings"
	"time"
)

type Handler struct{ Service *application.Service }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if pattern := routePattern(r); pattern != "" {
		r.Pattern = pattern
	}
	if !platform.RequireRole(w, r, "admin", "client", "service") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	p := platform.PrincipalFrom(ctx)
	actor := domain.Actor{App: p.App, Role: p.Role, CustomerID: p.CustomerID}
	switch {
	case r.URL.Path == "/v1/availability" && r.Method == "GET":
		q := r.URL.Query()
		for _, k := range []string{"vehicleId", "startsOn", "endsOn"} {
			if len(q[k]) != 1 {
				platform.Error(w, 400, "INVALID_INPUT", "Cada parámetro es obligatorio una sola vez.")
				return
			}
		}
		if !domain.Identifier.MatchString(q.Get("vehicleId")) {
			platform.Error(w, 400, "INVALID_INPUT", "Identificador de vehículo inválido.")
			return
		}
		v, err := h.Service.Availability(ctx, q.Get("vehicleId"), q.Get("startsOn"), q.Get("endsOn"))
		if err != nil {
			respondError(w, err)
			return
		}
		value := map[string]any{"vehicleId": v.VehicleID, "startsOn": v.Period.StartsOn, "endsOn": v.Period.EndsOn, "available": v.Available, "days": v.Period.Days, "dailyRateMinor": v.DailyRateMinor, "totalMinor": v.TotalMinor, "currency": v.Currency}
		if !v.Available {
			value["unavailableReason"] = v.Reason
		}
		platform.WriteJSON(w, 200, value)
	case r.URL.Path == "/v1/reservations" && r.Method == "POST":
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			platform.Error(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type debe ser application/json.")
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if len(r.Header.Values("Idempotency-Key")) != 1 || !validKey(key) {
			platform.Error(w, 400, "INVALID_INPUT", "Idempotency-Key requiere 1 a 128 caracteres ASCII visibles.")
			return
		}
		var input struct {
			VehicleID          string `json:"vehicleId"`
			CustomerID         string `json:"customerId"`
			StartsOn           string `json:"startsOn"`
			EndsOn             string `json:"endsOn"`
			ExpectedTotalMinor *int64 `json:"expectedTotalMinor"`
		}
		if err := platform.Decode(w, r, &input); err != nil {
			platform.Error(w, 400, "INVALID_INPUT", "Objeto JSON inválido.")
			return
		}
		if !domain.Identifier.MatchString(input.VehicleID) || !domain.Identifier.MatchString(input.CustomerID) || input.StartsOn == "" || input.EndsOn == "" || input.ExpectedTotalMinor == nil || *input.ExpectedTotalMinor < 0 {
			platform.Error(w, 400, "INVALID_INPUT", "Se requieren los cinco campos, identificadores válidos e importe entero no negativo.")
			return
		}
		result, err := h.Service.Create(ctx, actor, key, application.Create{VehicleID: input.VehicleID, CustomerID: input.CustomerID, StartsOn: input.StartsOn, EndsOn: input.EndsOn, ExpectedTotalMinor: *input.ExpectedTotalMinor})
		if err != nil {
			respondError(w, err)
			return
		}
		status, body, err := wire.Encode(result)
		if err != nil {
			respondError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	case strings.HasPrefix(r.URL.Path, "/v1/reservations/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/reservations/"), "/")
		if !domain.Identifier.MatchString(parts[0]) || len(parts) > 2 {
			platform.Error(w, 404, "NOT_FOUND", "Ruta inexistente.")
			return
		}
		var v domain.Reservation
		var err error
		if len(parts) == 1 && r.Method == "GET" {
			v, err = h.Service.Get(ctx, actor, parts[0])
		} else if len(parts) == 1 && r.Method == "DELETE" {
			v, err = h.Service.Cancel(ctx, actor, parts[0])
		} else if len(parts) == 2 && r.Method == "POST" && (parts[1] == "pickup" || parts[1] == "return") {
			if !platform.RequireRole(w, r, "admin", "service") {
				return
			}
			if parts[1] == "pickup" {
				v, err = h.Service.Pickup(ctx, actor, parts[0])
			} else {
				v, err = h.Service.Return(ctx, actor, parts[0])
			}
		} else {
			platform.Error(w, 405, "METHOD_NOT_ALLOWED", "Método no admitido.")
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, 200, wire.FromDomain(v))
	case r.URL.Path == "/v1/maintenance" && r.Method == "POST":
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		var input struct {
			VehicleID string `json:"vehicleId"`
			StartsOn  string `json:"startsOn"`
			EndsOn    string `json:"endsOn"`
		}
		if err := platform.Decode(w, r, &input); err != nil || !domain.Identifier.MatchString(input.VehicleID) {
			platform.Error(w, 400, "INVALID_INPUT", "Mantenimiento inválido.")
			return
		}
		period, p := domain.NewPeriod(input.StartsOn, input.EndsOn, h.Service.Now())
		if p != nil {
			respondError(w, p)
			return
		}
		if _, err := h.Service.Fleet.Quote(ctx, input.VehicleID); err != nil {
			respondError(w, err)
			return
		}
		id, err := h.Service.Repo.Maintenance(ctx, h.Service.NewID(), input.VehicleID, period)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, 201, map[string]string{"id": id, "vehicleId": input.VehicleID, "startsOn": input.StartsOn, "endsOn": input.EndsOn})
	default:
		platform.Error(w, 404, "NOT_FOUND", "Ruta inexistente.")
	}
}
func routePattern(r *http.Request) string {
	switch {
	case r.URL.Path == "/v1/availability" && r.Method == http.MethodGet:
		return "GET /v1/availability"
	case r.URL.Path == "/v1/reservations" && r.Method == http.MethodPost:
		return "POST /v1/reservations"
	case r.URL.Path == "/v1/maintenance" && r.Method == http.MethodPost:
		return "POST /v1/maintenance"
	case strings.HasPrefix(r.URL.Path, "/v1/reservations/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/reservations/"), "/")
		if !domain.Identifier.MatchString(parts[0]) {
			return ""
		}
		if len(parts) == 1 && r.Method == http.MethodGet {
			return "GET /v1/reservations/{id}"
		}
		if len(parts) == 1 && r.Method == http.MethodDelete {
			return "DELETE /v1/reservations/{id}"
		}
		if len(parts) == 2 && r.Method == http.MethodPost {
			switch parts[1] {
			case "pickup":
				return "POST /v1/reservations/{id}/pickup"
			case "return":
				return "POST /v1/reservations/{id}/return"
			}
		}
	}
	return ""
}
func validKey(key string) bool {
	if len(key) < 1 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if c < '!' || c > '~' {
			return false
		}
	}
	return true
}
func respondError(w http.ResponseWriter, err error) {
	var p *domain.Problem
	if errors.As(err, &p) {
		platform.Error(w, wire.Status(p.Code), p.Code, p.Message)
		return
	}
	platform.Error(w, 503, "DEPENDENCY_UNAVAILABLE", "La operación no pudo completarse.")
}

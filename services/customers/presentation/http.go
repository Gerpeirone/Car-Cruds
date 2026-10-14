package presentation

import (
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"net/http"
	"strings"
)

type Handler struct{ Service *business.Service }
type Customer struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Active            bool   `json:"active"`
	LicenseValidUntil string `json:"licenseValidUntil"`
}

func dto(c business.Customer) Customer { return Customer{c.ID, c.Name, c.Active, c.LicenseValidUntil} }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v1/customers" && r.Method == http.MethodGet:
		r.Pattern = "GET /v1/customers"
	case r.URL.Path == "/v1/customers" && r.Method == http.MethodPost:
		r.Pattern = "POST /v1/customers"
	case strings.HasPrefix(r.URL.Path, "/v1/customers/") && r.Method == http.MethodGet:
		r.Pattern = "GET /v1/customers/{id}"
	}
	if !platform.RequireRole(w, r, "admin", "client", "service") {
		return
	}
	switch {
	case r.URL.Path == "/v1/customers" && r.Method == "GET":
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		values, err := h.Service.List(r.Context())
		if err != nil {
			respondError(w, err)
			return
		}
		result := make([]Customer, 0, len(values))
		for _, c := range values {
			result = append(result, dto(c))
		}
		platform.WriteJSON(w, 200, map[string]any{"items": result})
	case r.URL.Path == "/v1/customers" && r.Method == "POST":
		if !platform.RequireRole(w, r, "admin") {
			return
		}
		var input struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Active  *bool  `json:"active"`
			License string `json:"licenseValidUntil"`
		}
		if err := platform.Decode(w, r, &input); err != nil || input.Active == nil {
			platform.Error(w, 400, "INVALID_INPUT", "Objeto JSON completo requerido.")
			return
		}
		c, err := h.Service.Create(r.Context(), business.Customer{ID: input.ID, Name: input.Name, Active: *input.Active, LicenseValidUntil: input.License})
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, 201, dto(c))
	case strings.HasPrefix(r.URL.Path, "/v1/customers/") && r.Method == "GET":
		id := strings.TrimPrefix(r.URL.Path, "/v1/customers/")
		p := platform.PrincipalFrom(r.Context())
		if p.Role == "client" && p.CustomerID != id {
			platform.Error(w, 404, "NOT_FOUND", "Cliente inexistente.")
			return
		}
		c, err := h.Service.Get(r.Context(), id)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, 200, dto(c))
	default:
		platform.Error(w, 404, "NOT_FOUND", "Ruta inexistente.")
	}
}
func respondError(w http.ResponseWriter, err error) {
	var p *business.Problem
	if errors.As(err, &p) {
		status := 503
		switch p.Code {
		case "INVALID_INPUT":
			status = 400
		case "NOT_FOUND":
			status = 404
		case "CONFLICT":
			status = 409
		}
		platform.Error(w, status, p.Code, p.Message)
		return
	}
	platform.Error(w, 503, "DEPENDENCY_UNAVAILABLE", "La operación no pudo completarse.")
}

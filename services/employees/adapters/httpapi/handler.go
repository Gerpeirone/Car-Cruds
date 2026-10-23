package httpapi

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Handler struct{ service *application.Service }

func New(service *application.Service) http.Handler {
	h := &Handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/employees", h.employees)
	mux.HandleFunc("/v1/employees/{id}", h.employee)
	mux.HandleFunc("/v1/employees/{id}/status", h.employeeStatus)
	mux.HandleFunc("/v1/admin/employees", h.adminEmployees)
	mux.HandleFunc("/v1/candidates", h.candidates)
	mux.HandleFunc("/v1/candidates/{id}", h.candidate)
	mux.HandleFunc("/v1/employees/integrations", h.integrations)
	mux.HandleFunc("/v1/employees/membership-evidence", h.membershipEvidence)
	mux.HandleFunc("/v1/employees/reports/memberships", h.membershipReport)
	mux.HandleFunc("/v1/employees/clinic/patient-links", h.clinicLinks)
	mux.HandleFunc("/v1/employees/clinic/requests", h.clinicRequests)
	mux.HandleFunc("/v1/employees/clinic/requests/{requestId}", h.clinicRequest)
	mux.HandleFunc("/v1/employees/clinic/requests/{requestId}/{action}", h.clinicAction)
	mux.HandleFunc("/v1/employees/reports/clinic", h.clinicReport)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { platform.Error(w, 404, "NOT_FOUND", "Ruta inexistente") })
	return mux
}

type employeeSummary struct {
	ID          string `json:"employeeId"`
	DisplayName string `json:"displayName"`
}

// This projection lets the gym verify an employment change without access to
// administrative details, candidates or another system's membership rules.
func (h *Handler) employeeStatus(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "gym", "admin", "service") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}
	item, err := h.service.GetEmployee(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, struct {
		ID     string `json:"employeeId"`
		Active bool   `json:"active"`
	}{item.ID, item.Active})
}

type employeeRecord struct {
	ID          string `json:"employeeId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

type candidateRecord struct {
	ID          string `json:"candidateId"`
	DisplayName string `json:"displayName"`
}

type pageResponse[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type employeeInput struct {
	DisplayName string `json:"displayName"`
	Active      *bool  `json:"active"`
}

type candidateInput struct {
	DisplayName string `json:"displayName"`
}

func (h *Handler) employees(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !platform.RequireRole(w, r, "admin", "gym", "service") {
			return
		}
		page, err := parsePage(r.URL)
		if err != nil {
			writeError(w, err)
			return
		}
		items, total, err := h.service.ListEmployees(r.Context(), page, true)
		if err != nil {
			writeError(w, err)
			return
		}
		projection := make([]employeeSummary, 0, len(items))
		for _, item := range items {
			projection = append(projection, employeeSummary{ID: item.ID, DisplayName: item.DisplayName})
		}
		platform.WriteJSON(w, 200, pageResponse[employeeSummary]{Items: projection, Page: page.Number, PageSize: page.Size, Total: total})
	case http.MethodPost:
		if !platform.RequireRole(w, r, "admin") {
			return
		}
		var input employeeInput
		if !decode(w, r, &input) {
			return
		}
		if input.Active == nil {
			writeError(w, domain.ErrInvalidInput)
			return
		}
		item, err := h.service.CreateEmployee(r.Context(), input.DisplayName, *input.Active)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Location", "/v1/employees/"+item.ID)
		platform.WriteJSON(w, 201, employeeDTO(item))
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) adminEmployees(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	page, err := parsePage(r.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	items, total, err := h.service.ListEmployees(r.Context(), page, false)
	if err != nil {
		writeError(w, err)
		return
	}
	projection := make([]employeeRecord, 0, len(items))
	for _, item := range items {
		projection = append(projection, employeeDTO(item))
	}
	platform.WriteJSON(w, 200, pageResponse[employeeRecord]{Items: projection, Page: page.Number, PageSize: page.Size, Total: total})
}

func (h *Handler) employee(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin") {
		return
	}
	var item domain.Employee
	var err error
	switch r.Method {
	case http.MethodGet:
		item, err = h.service.GetEmployee(r.Context(), r.PathValue("id"))
	case http.MethodPut:
		var input employeeInput
		if !decode(w, r, &input) {
			return
		}
		if input.Active == nil {
			writeError(w, domain.ErrInvalidInput)
			return
		}
		item, err = h.service.UpdateEmployee(r.Context(), r.PathValue("id"), input.DisplayName, *input.Active)
	case http.MethodDelete:
		item, err = h.service.DeactivateEmployee(r.Context(), r.PathValue("id"))
	default:
		methodNotAllowed(w, "GET, PUT, DELETE")
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, employeeDTO(item))
}

func (h *Handler) candidates(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin") {
		return
	}
	switch r.Method {
	case http.MethodGet:
		page, err := parsePage(r.URL)
		if err != nil {
			writeError(w, err)
			return
		}
		items, total, err := h.service.ListCandidates(r.Context(), page)
		if err != nil {
			writeError(w, err)
			return
		}
		projection := make([]candidateRecord, 0, len(items))
		for _, item := range items {
			projection = append(projection, candidateDTO(item))
		}
		platform.WriteJSON(w, 200, pageResponse[candidateRecord]{Items: projection, Page: page.Number, PageSize: page.Size, Total: total})
	case http.MethodPost:
		var input candidateInput
		if !decode(w, r, &input) {
			return
		}
		item, err := h.service.CreateCandidate(r.Context(), input.DisplayName)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Location", "/v1/candidates/"+item.ID)
		platform.WriteJSON(w, 201, candidateDTO(item))
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) candidate(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin") {
		return
	}
	var item domain.Candidate
	var err error
	switch r.Method {
	case http.MethodGet:
		item, err = h.service.GetCandidate(r.Context(), r.PathValue("id"))
	case http.MethodPut:
		var input candidateInput
		if !decode(w, r, &input) {
			return
		}
		item, err = h.service.UpdateCandidate(r.Context(), r.PathValue("id"), input.DisplayName)
	case http.MethodDelete:
		err = h.service.DeleteCandidate(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		methodNotAllowed(w, "GET, PUT, DELETE")
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, candidateDTO(item))
}

func (h *Handler) integrations(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin", "service") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	writeError(w, h.service.IntegrationStatus(r.Context()))
}

func employeeDTO(item domain.Employee) employeeRecord {
	return employeeRecord{ID: item.ID, DisplayName: item.DisplayName, Active: item.Active}
}
func candidateDTO(item domain.Candidate) candidateRecord {
	return candidateRecord{ID: item.ID, DisplayName: item.DisplayName}
}

func parsePage(location *url.URL) (domain.Page, error) {
	query, err := url.ParseQuery(location.RawQuery)
	if err != nil {
		return domain.Page{}, domain.ErrInvalidInput
	}
	page := domain.Page{Number: 1, Size: 20}
	for name, values := range query {
		if (name != "page" && name != "pageSize") || len(values) != 1 {
			return domain.Page{}, domain.ErrInvalidInput
		}
		number, err := strconv.Atoi(values[0])
		if err != nil {
			return domain.Page{}, domain.ErrInvalidInput
		}
		if name == "page" {
			page.Number = number
		} else {
			page.Size = number
		}
	}
	return page, page.Validate()
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		platform.Error(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Se requiere Content-Type application/json")
		return false
	}
	if err = platform.Decode(w, r, value); err != nil {
		writeError(w, domain.ErrInvalidInput)
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	platform.Error(w, 405, "METHOD_NOT_ALLOWED", "Método no admitido")
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrPatientLinkRequired):
		platform.Error(w, 409, "PATIENT_LINK_REQUIRED", "Registrar primero el vínculo con el paciente del hospital")
	case errors.Is(err, domain.ErrClinicUnavailable):
		platform.Error(w, 503, "CLINIC_UNAVAILABLE", "Hospital no disponible o respuesta incompatible; consultar la intención persistida")
	case errors.Is(err, domain.ErrEvidenceConflict):
		platform.Error(w, 409, "EVIDENCE_CONFLICT", "La referencia o versión contradice evidencia registrada")
	case errors.Is(err, domain.ErrInvalidInput):
		platform.Error(w, 400, "INVALID_INPUT", "Datos o paginación inválidos; consultar el contrato de la operación")
	case errors.Is(err, domain.ErrNotFound):
		platform.Error(w, 404, "NOT_FOUND", "Registro inexistente")
	case errors.Is(err, domain.ErrNotConfigured):
		platform.Error(w, 503, "NOT_CONFIGURED", "Gimnasio y hospital pendientes de contrato y configuración; no hay resultados confirmados")
	default:
		platform.Error(w, 503, "SERVICE_UNAVAILABLE", "Persistencia de personal no disponible")
	}
}

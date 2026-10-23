package httpapi

import (
	"net/http"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type gymResponse struct {
	ID                string `json:"enrollmentId"`
	EmployeeID        string `json:"employeeId"`
	State             string `json:"state"`
	Code              string `json:"code,omitempty"`
	SocioID           string `json:"socioId,omitempty"`
	SolicitudID       string `json:"solicitudId,omitempty"`
	RequestState      string `json:"providerRequestState,omitempty"`
	MembershipEnabled *bool  `json:"membershipEnabled,omitempty"`
}

func gymDTO(e domain.GymEnrollment) gymResponse {
	result := gymResponse{ID: e.ID, EmployeeID: e.EmployeeID, State: e.State, Code: e.ProviderCode, SocioID: e.SocioID, SolicitudID: e.SolicitudID}
	if e.State == "ACCEPTED" {
		enabled := false
		result.MembershipEnabled = &enabled
		result.RequestState = "PENDIENTE"
	}
	return result
}
func (h *Handler) gymEnrollments(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin", "service") {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	var input struct {
		DNI   string `json:"dni"`
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		writeError(w, domain.ErrInvalidInput)
		return
	}
	item, err := h.service.EnrollGym(r.Context(), platform.PrincipalFrom(r.Context()).App, r.PathValue("id"), keys[0], input.DNI, input.Email)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/employees/"+item.EmployeeID+"/gym-enrollments/"+item.ID)
	status := http.StatusAccepted
	switch item.State {
	case "ACCEPTED":
		status = 201
	case "REJECTED":
		status = 409
	case "UNKNOWN":
		status = 503
	}
	platform.WriteJSON(w, status, gymDTO(item))
}
func (h *Handler) gymEnrollment(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin", "service") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	item, err := h.service.GetGym(r.Context(), r.PathValue("id"), r.PathValue("enrollment"))
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, gymDTO(item))
}

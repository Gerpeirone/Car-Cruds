package httpapi

import (
	"net/http"
	"net/url"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type clinicAppointmentDTO struct {
	ID        string    `json:"id"`
	PatientID string    `json:"patientId"`
	DoctorID  string    `json:"doctorId"`
	StartAt   time.Time `json:"startAt"`
	EndAt     time.Time `json:"endAt"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
}
type clinicRequestDTO struct {
	ID             string                `json:"requestId"`
	LocalReference string                `json:"localReference"`
	PersonKind     string                `json:"personKind"`
	PersonID       string                `json:"personId"`
	PatientID      string                `json:"patientId"`
	DoctorID       string                `json:"doctorId"`
	StartAt        time.Time             `json:"requestedStartAt"`
	CreatedAt      time.Time             `json:"createdAt"`
	AttemptedAt    *time.Time            `json:"attemptedAt"`
	ObservedAt     *time.Time            `json:"observedAt"`
	NextAttemptAt  *time.Time            `json:"nextAttemptAt,omitempty"`
	State          string                `json:"state"`
	Classification string                `json:"classification"`
	FailureCode    string                `json:"failureCode,omitempty"`
	Appointment    *clinicAppointmentDTO `json:"lastKnownAppointment"`
}

func clinicDTO(s domain.ClinicRequest) clinicRequestDTO {
	d := clinicRequestDTO{ID: s.ID, LocalReference: s.LocalReference, PersonKind: s.PersonKind, PersonID: s.PersonID, PatientID: s.PatientID, DoctorID: s.DoctorID, StartAt: s.StartAt, CreatedAt: s.CreatedAt, AttemptedAt: s.AttemptedAt, ObservedAt: s.ObservedAt, State: s.State, Classification: s.Classification(), FailureCode: s.FailureCode}
	d.NextAttemptAt = s.NextAttemptAt
	if a := s.Appointment; a != nil {
		d.Appointment = &clinicAppointmentDTO{a.ID, a.PatientID, a.DoctorID, a.StartAt, a.EndAt, a.Status, a.Version}
	}
	return d
}
func clinicAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !platform.RequireRole(w, r, "admin") {
		return false
	}
	if r.URL.RawQuery != "" {
		writeError(w, domain.ErrInvalidInput)
		return false
	}
	return true
}
func (h *Handler) clinicLinks(w http.ResponseWriter, r *http.Request) {
	if !clinicAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	var input struct {
		PersonKind string `json:"personKind"`
		PersonID   string `json:"personId"`
		PatientID  string `json:"patientId"`
	}
	if !decode(w, r, &input) {
		return
	}
	err := h.service.LinkClinicPatient(r.Context(), domain.PatientLink{PersonKind: input.PersonKind, PersonID: input.PersonID, PatientID: input.PatientID, RegisteredBy: platform.PrincipalFrom(r.Context()).App})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) clinicRequests(w http.ResponseWriter, r *http.Request) {
	if !clinicAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	var input struct {
		PersonKind     string    `json:"personKind"`
		PersonID       string    `json:"personId"`
		LocalReference string    `json:"localReference"`
		DoctorID       string    `json:"doctorId"`
		StartAt        time.Time `json:"startAt"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := h.service.CreateClinicRequest(r.Context(), domain.ClinicRequest{PersonKind: input.PersonKind, PersonID: input.PersonID, LocalReference: input.LocalReference, DoctorID: input.DoctorID, StartAt: input.StartAt, RegisteredBy: platform.PrincipalFrom(r.Context()).App})
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/employees/clinic/requests/"+item.ID)
	platform.WriteJSON(w, 200, clinicDTO(item))
}
func (h *Handler) clinicRequest(w http.ResponseWriter, r *http.Request) {
	if !clinicAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	item, err := h.service.GetClinicRequest(r.Context(), r.PathValue("requestId"))
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, clinicDTO(item))
}
func (h *Handler) clinicAction(w http.ResponseWriter, r *http.Request) {
	if !clinicAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	var item domain.ClinicRequest
	var err error
	switch r.PathValue("action") {
	case "submit":
		// No body is accepted: the persisted intention is the only source.
		if r.ContentLength != 0 {
			writeError(w, domain.ErrInvalidInput)
			return
		}
		item, err = h.service.SubmitClinicRequest(r.Context(), r.PathValue("requestId"))
	case "refresh":
		var input struct {
			AppointmentID string `json:"appointmentId"`
		}
		if r.ContentLength != 0 && !decode(w, r, &input) {
			return
		}
		item, err = h.service.RefreshClinicRequest(r.Context(), r.PathValue("requestId"), input.AppointmentID)
	default:
		platform.Error(w, 404, "NOT_FOUND", "Operación inexistente")
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, clinicDTO(item))
}
func (h *Handler) clinicReport(w http.ResponseWriter, r *http.Request) {
	if !platform.RequireRole(w, r, "admin", "service") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["personKind"]) > 1 {
		writeError(w, domain.ErrInvalidInput)
		return
	}
	kind := "employee"
	if len(query["personKind"]) == 1 {
		kind = query.Get("personKind")
	}
	query.Del("personKind")
	location := *r.URL
	location.RawQuery = query.Encode()
	page, err := parsePage(&location)
	if err != nil {
		writeError(w, err)
		return
	}
	rows, total, err := h.service.ClinicReport(r.Context(), kind, page)
	if err != nil {
		writeError(w, err)
		return
	}
	type itemDTO struct {
		PersonKind     string            `json:"personKind"`
		PersonID       string            `json:"personId"`
		DisplayName    string            `json:"displayName"`
		Active         *bool             `json:"active"`
		Classification string            `json:"classification"`
		Request        *clinicRequestDTO `json:"request"`
	}
	items := make([]itemDTO, 0, len(rows))
	for _, row := range rows {
		item := itemDTO{PersonKind: row.PersonKind, PersonID: row.PersonID, DisplayName: row.DisplayName, Active: row.Active, Classification: "pending_verification"}
		if row.Request != nil {
			dto := clinicDTO(*row.Request)
			item.Request = &dto
			item.Classification = dto.Classification
		}
		items = append(items, item)
	}
	platform.WriteJSON(w, 200, pageResponse[itemDTO]{Items: items, Page: page.Number, PageSize: page.Size, Total: total})
}

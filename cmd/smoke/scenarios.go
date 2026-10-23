package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/google/uuid"
)

type employee struct {
	ID          string `json:"employeeId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}
type candidate struct {
	ID          string `json:"candidateId"`
	DisplayName string `json:"displayName"`
}
type vehicle struct {
	ID             string `json:"vehicleId"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	Category       string `json:"category"`
	Transmission   string `json:"transmission"`
	Seats          int    `json:"seats"`
	DailyRateMinor int64  `json:"dailyRateMinor"`
	Currency       string `json:"currency"`
	Active         bool   `json:"active"`
	Version        int64  `json:"version"`
}
type reservation struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	VehicleID      string `json:"vehicleId"`
	CustomerID     string `json:"customerId"`
	StartsOn       string `json:"startsOn"`
	EndsOn         string `json:"endsOn"`
	Days           int    `json:"days"`
	DailyRateMinor int64  `json:"dailyRateMinor"`
	TotalMinor     int64  `json:"totalMinor"`
	Currency       string `json:"currency"`
}
type reservationInput struct {
	VehicleID          string `json:"vehicleId"`
	CustomerID         string `json:"customerId"`
	StartsOn           string `json:"startsOn"`
	EndsOn             string `json:"endsOn"`
	ExpectedTotalMinor int64  `json:"expectedTotalMinor"`
}
type reservationIntent struct {
	key   string
	input reservationInput
	id    string
}

func (h *harness) run(ctx context.Context) error {
	if err := h.personnel(ctx); err != nil {
		return err
	}
	if err := h.fleet(ctx); err != nil {
		return err
	}
	if err := h.rentals(ctx); err != nil {
		return err
	}
	if h.history {
		return h.checkHistory(ctx)
	}
	h.log("SKIP historial PostgreSQL: ejecutar con -history para comprobar outbox y consumidor")
	return nil
}

func (h *harness) personnel(ctx context.Context) error {
	for _, check := range []struct {
		name, method, path, token string
		input                     any
		status                    int
	}{
		{"sin_token", "GET", "/v1/employees", "", nil, 401},
		{"cliente_sin_lista_personal", "GET", "/v1/employees", h.client, nil, 403},
		{"gimnasio_sin_lista_admin", "GET", "/v1/admin/employees", h.gym, nil, 403},
		{"gimnasio_sin_alta_personal", "POST", "/v1/employees", h.gym, map[string]any{"displayName": "smoke-denied-" + h.runID, "active": true}, 403},
		{"gimnasio_sin_candidatos", "GET", "/v1/candidates", h.gym, nil, 403},
		{"gimnasio_sin_alta_candidato", "POST", "/v1/candidates", h.gym, map[string]any{"displayName": "smoke-denied-" + h.runID}, 403},
	} {
		if _, err := h.request(ctx, check.name, check.method, check.path, check.token, check.input, "", check.status, nil); err != nil {
			return err
		}
	}
	name := "smoke-empleado-" + h.runID
	var person employee
	if _, err := h.request(ctx, "crear_empleado", "POST", "/v1/employees", h.admin, map[string]any{"displayName": name, "active": true}, "", 201, &person); err != nil {
		return err
	}
	if _, err := uuid.Parse(person.ID); err != nil || person.DisplayName != name {
		return errors.New("alta de empleado no coincide con la fixture")
	}
	h.employeeID = person.ID
	if !person.Active {
		return errors.New("empleado creado no está activo")
	}
	var applicant candidate
	if _, err := h.request(ctx, "crear_candidato", "POST", "/v1/candidates", h.admin, map[string]string{"displayName": "smoke-candidato-" + h.runID}, "", 201, &applicant); err != nil {
		return err
	}
	if _, err := uuid.Parse(applicant.ID); err != nil || applicant.ID == person.ID || applicant.DisplayName != "smoke-candidato-"+h.runID {
		return errors.New("alta de candidato no conserva su identidad separada")
	}
	h.candidateID = applicant.ID
	found, err := h.gymListContains(ctx, h.employeeID)
	if err != nil || !found {
		if err != nil {
			return err
		}
		return errors.New("el gimnasio no recibió el empleado activo creado")
	}
	for _, check := range []struct {
		name, method, path string
		input              any
	}{
		{"gimnasio_sin_ficha_personal", "GET", "/v1/employees/" + person.ID, nil},
		{"gimnasio_sin_edicion_personal", "PUT", "/v1/employees/" + person.ID, map[string]any{"displayName": name, "active": false}},
		{"gimnasio_sin_baja_personal", "DELETE", "/v1/employees/" + person.ID, nil},
		{"gimnasio_sin_ficha_candidato", "GET", "/v1/candidates/" + applicant.ID, nil},
		{"gimnasio_sin_edicion_candidato", "PUT", "/v1/candidates/" + applicant.ID, map[string]string{"displayName": "smoke-denied"}},
		{"gimnasio_sin_baja_candidato", "DELETE", "/v1/candidates/" + applicant.ID, nil},
	} {
		if _, err := h.request(ctx, check.name, check.method, check.path, h.gym, check.input, "", 403, nil); err != nil {
			return err
		}
	}
	h.log("PASS personal: proyección mínima y candidato excluido employeeID=%s candidateID=%s", person.ID, applicant.ID)
	return nil
}

func (h *harness) gymListContains(ctx context.Context, id string) (bool, error) {
	found := false
	for page := 1; page <= 1000; page++ {
		var result struct {
			Items    []map[string]json.RawMessage `json:"items"`
			Page     int                          `json:"page"`
			PageSize int                          `json:"pageSize"`
			Total    int64                        `json:"total"`
		}
		if _, err := h.request(ctx, "lista_gimnasio", "GET", fmt.Sprintf("/v1/employees?page=%d&pageSize=100", page), h.gym, nil, "", 200, &result); err != nil {
			return false, err
		}
		if result.Page != page || result.PageSize != 100 || result.Total < 0 || result.Items == nil {
			return false, errors.New("paginación de personal inválida")
		}
		for _, item := range result.Items {
			var employeeID, displayName string
			if len(item) != 2 || json.Unmarshal(item["employeeId"], &employeeID) != nil || json.Unmarshal(item["displayName"], &displayName) != nil || employeeID == "" || displayName == "" {
				return false, errors.New("lista de gimnasio expone campos adicionales o incompletos")
			}
			if employeeID == h.candidateID || displayName == "smoke-candidato-"+h.runID {
				return false, errors.New("un candidato apareció en la lista de empleados")
			}
			if employeeID == id {
				if displayName != "smoke-empleado-"+h.runID {
					return false, errors.New("identidad y nombre de empleado no coinciden")
				}
				found = true
			}
		}
		if int64(page*100) >= result.Total {
			return found, nil
		}
	}
	return false, errors.New("inventario de personal supera el límite acotado del harness")
}

func (h *harness) fleet(ctx context.Context) error {
	instances := make(map[string]bool)
	for i := 0; i < 12; i++ {
		var page struct {
			Items []vehicle `json:"items"`
			Page  int       `json:"page"`
		}
		result, err := h.request(ctx, "replica_flota", "GET", "/v1/vehicles?page=1&pageSize=1", h.admin, nil, "", 200, &page)
		if err != nil {
			return err
		}
		if result.instance == "" || result.instance == "gateway" || page.Page != 1 || page.Items == nil {
			return errors.New("Flota no informó una instancia y lista válidas")
		}
		instances[result.instance] = true
	}
	if len(instances) != 2 {
		return fmt.Errorf("se observaron %d réplicas distintas de Flota; se requieren dos", len(instances))
	}
	h.log("PASS balanceo: dos instancias distintas en doce respuestas del gateway")
	h.vehicle = vehicle{ID: h.vehicleID, Brand: "Smoke-" + h.runID, Model: "Fixture de integración", Category: "smoke", Transmission: "manual", Seats: 5, DailyRateMinor: 123456, Currency: "ARS", Active: true}
	h.vehicleIntent = true
	var stored vehicle
	if _, err := h.request(ctx, "crear_vehiculo", "POST", "/v1/vehicles", h.admin, h.vehicle, "", 201, &stored); err != nil {
		return err
	}
	if stored.ID != h.vehicleID || stored.Version != 1 || !stored.Active || stored.DailyRateMinor != h.vehicle.DailyRateMinor {
		return errors.New("vehículo creado no coincide con la fixture")
	}
	h.vehicle = stored
	// Fill the shared cache, then update the authoritative price and verify that
	// either replica returns the new version. A stale CAS must be rejected.
	if _, err := h.request(ctx, "ficha_cache", "GET", "/v1/vehicles/"+h.vehicleID, h.client, nil, "", 200, &stored); err != nil {
		return err
	}
	old := h.vehicle
	h.vehicle.DailyRateMinor++
	if _, err := h.request(ctx, "actualizar_vehiculo", "PUT", "/v1/vehicles/"+h.vehicleID, h.admin, h.vehicle, "", 200, &stored); err != nil {
		return err
	}
	h.vehicle = stored
	if stored.Version != 2 || stored.DailyRateMinor != old.DailyRateMinor+1 {
		return errors.New("escritura de Flota no avanzó versión y tarifa")
	}
	if _, err := h.request(ctx, "rechazar_version_anterior", "PUT", "/v1/vehicles/"+h.vehicleID, h.admin, old, "", 409, nil); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		var detail vehicle
		if _, err := h.request(ctx, "cache_version_actual", "GET", "/v1/vehicles/"+h.vehicleID, h.client, nil, "", 200, &detail); err != nil {
			return err
		}
		if detail.Version != 2 || detail.DailyRateMinor != h.vehicle.DailyRateMinor {
			return errors.New("ficha cacheada conservó una versión anterior a la escritura")
		}
	}
	return h.searchVisible(ctx, true)
}

func (h *harness) searchVisible(ctx context.Context, active bool) error {
	pollCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	for {
		var result struct {
			Items []vehicle `json:"items"`
			Total int64     `json:"total"`
		}
		path := "/v1/vehicles/search?brand=" + url.QueryEscape(h.vehicle.Brand) + "&page=1&pageSize=10&sort=price_asc"
		if _, err := h.request(pollCtx, "busqueda_eventual", "GET", path, h.client, nil, "", 200, &result); err != nil {
			return err
		}
		visible := len(result.Items) == 1 && result.Total == 1 && result.Items[0].ID == h.vehicleID && result.Items[0].Version == h.vehicle.Version && result.Items[0].DailyRateMinor == h.vehicle.DailyRateMinor && result.Items[0].Active
		if (active && visible) || (!active && result.Total == 0 && len(result.Items) == 0) {
			h.log("PASS índice active=%t visibilidadMs=%d", active, time.Since(started).Milliseconds())
			return nil
		}
		if err := pause(pollCtx, 200*time.Millisecond); err != nil {
			return errors.New("el índice no alcanzó la versión esperada dentro de cinco segundos de consultas")
		}
	}
}

func pause(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h *harness) rentals(ctx context.Context) error {
	branch, err := time.LoadLocation("America/Buenos_Aires")
	if err != nil {
		return errors.New("calendario de Buenos Aires no disponible")
	}
	today := time.Now().In(branch)
	start, end := today.Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := h.request(ctx, "crear_cliente_smoke", "POST", "/v1/customers", h.admin, map[string]any{"id": h.customerID, "name": "smoke-conductor-" + h.runID, "active": true, "licenseValidUntil": today.AddDate(1, 0, 0).Format("2006-01-02")}, "", 201, nil); err != nil {
		return err
	}
	input := reservationInput{h.vehicleID, h.customerID, start, end, h.vehicle.DailyRateMinor}
	if err := h.availability(ctx, input, true); err != nil {
		return err
	}
	h.intents = append(h.intents, reservationIntent{key: "smoke-" + h.runID + "-original", input: input})
	var original reservation
	created, err := h.request(ctx, "reserva_inicial", "POST", "/v1/reservations", h.admin, input, h.intents[0].key, 201, &original)
	if err != nil {
		return err
	}
	if err := h.validateReservation(original, input, "RESERVED"); err != nil {
		return err
	}
	h.intents[0].id = original.ID
	h.log("PASS reserva_creada reservationID=%s requestID=%s startsOn=%s endsOn=%s zona=America/Buenos_Aires", original.ID, created.requestID, start, end)
	var replay reservation
	if _, err = h.request(ctx, "reintento_idempotente", "POST", "/v1/reservations", h.admin, input, h.intents[0].key, 201, &replay); err != nil || !reflect.DeepEqual(original, replay) {
		return errors.New("reintento no recuperó la reserva original")
	}
	changed := input
	changed.ExpectedTotalMinor++
	reused, err := h.request(ctx, "clave_con_otro_payload", "POST", "/v1/reservations", h.admin, changed, h.intents[0].key, 409, nil)
	if err != nil {
		return err
	}
	if err = codeIs(reused, "IDEMPOTENCY_KEY_REUSED"); err != nil {
		return err
	}
	if err = h.availability(ctx, input, false); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		var cancelled reservation
		if _, err = h.request(ctx, "cancelacion_repetible", "DELETE", "/v1/reservations/"+original.ID, h.admin, nil, "", 200, &cancelled); err != nil || cancelled.Status != "CANCELLED" || cancelled.ID != original.ID {
			return errors.New("cancelación no conservó estado CANCELLED e identidad")
		}
	}
	if _, err = h.request(ctx, "reintento_despues_cancelacion", "POST", "/v1/reservations", h.admin, input, h.intents[0].key, 201, &replay); err != nil || !reflect.DeepEqual(original, replay) {
		return errors.New("cancelación modificó la respuesta idempotente original")
	}
	var current reservation
	if _, err = h.request(ctx, "estado_actual_cancelado", "GET", "/v1/reservations/"+original.ID, h.admin, nil, "", 200, &current); err != nil || current.Status != "CANCELLED" {
		return errors.New("GET no expuso estado actual CANCELLED")
	}
	if err = h.availability(ctx, input, true); err != nil {
		return err
	}
	// Two independent idempotency keys race for the same interval through the
	// real gateway; PostgreSQL must admit exactly one and reject the other.
	startIndex := len(h.intents)
	for i := 0; i < 2; i++ {
		h.intents = append(h.intents, reservationIntent{key: fmt.Sprintf("smoke-%s-race-%d", h.runID, i), input: input})
	}
	type outcome struct {
		index  int
		result response
		err    error
	}
	outcomes := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func(index int) {
			result, err := h.request(ctx, "reservas_concurrentes", "POST", "/v1/reservations", h.admin, input, h.intents[index].key, 0, nil)
			outcomes <- outcome{index, result, err}
		}(startIndex + i)
	}
	winners, conflicts := 0, 0
	var winner reservation
	var raceErr error
	for i := 0; i < 2; i++ {
		outcome := <-outcomes
		if outcome.err != nil {
			raceErr = outcome.err
			continue
		}
		if outcome.result.status == 201 {
			winners++
			if err := json.Unmarshal(outcome.result.body, &winner); err != nil {
				raceErr = errors.New("reserva concurrente con respuesta inválida")
			} else {
				if err := h.validateReservation(winner, input, "RESERVED"); err != nil {
					raceErr = err
				} else {
					h.intents[outcome.index].id = winner.ID
				}
				h.log("PASS reserva_creada reservationID=%s requestID=%s", winner.ID, outcome.result.requestID)
			}
		} else if err := codeIs(outcome.result, "VEHICLE_UNAVAILABLE"); err != nil {
			raceErr = err
		} else {
			conflicts++
		}
	}
	if raceErr != nil {
		return raceErr
	}
	if winners != 1 || conflicts != 1 {
		return errors.New("concurrencia no produjo una ganadora y un conflicto")
	}
	for _, operation := range []struct{ action, state string }{{"pickup", "ACTIVE"}, {"pickup", "ACTIVE"}, {"return", "COMPLETED"}, {"return", "COMPLETED"}} {
		var updated reservation
		if _, err := h.request(ctx, operation.action+"_repetible", "POST", "/v1/reservations/"+winner.ID+"/"+operation.action, h.admin, nil, "", 200, &updated); err != nil {
			return err
		}
		if err := h.validateReservation(updated, input, operation.state); err != nil {
			return err
		}
		if updated.ID != winner.ID {
			return errors.New("transición cambió identidad de reserva")
		}
	}
	h.completedID = winner.ID
	return h.availability(ctx, input, true)
}

func (h *harness) validateReservation(value reservation, input reservationInput, state string) error {
	if _, err := uuid.Parse(value.ID); err != nil || value.Status != state || value.VehicleID != input.VehicleID || value.CustomerID != input.CustomerID || value.StartsOn != input.StartsOn || value.EndsOn != input.EndsOn || value.Days != 1 || value.DailyRateMinor != h.vehicle.DailyRateMinor || value.TotalMinor != input.ExpectedTotalMinor || value.Currency != "ARS" {
		return errors.New("reserva no coincide con calendario, importe o estado esperado")
	}
	return nil
}

func (h *harness) availability(ctx context.Context, input reservationInput, expected bool) error {
	var result struct {
		VehicleID  string `json:"vehicleId"`
		Available  bool   `json:"available"`
		Days       int    `json:"days"`
		TotalMinor int64  `json:"totalMinor"`
		Currency   string `json:"currency"`
		Reason     string `json:"unavailableReason"`
	}
	query := url.Values{"vehicleId": {input.VehicleID}, "startsOn": {input.StartsOn}, "endsOn": {input.EndsOn}}
	if _, err := h.request(ctx, "disponibilidad", http.MethodGet, "/v1/availability?"+query.Encode(), h.client, nil, "", 200, &result); err != nil {
		return err
	}
	if result.VehicleID != input.VehicleID || result.Available != expected || result.Days != 1 || result.TotalMinor != input.ExpectedTotalMinor || result.Currency != "ARS" || (!expected && result.Reason != "OVERLAPPING_RESERVATION") {
		return errors.New("disponibilidad no coincide con la agenda y cotización esperadas")
	}
	return nil
}

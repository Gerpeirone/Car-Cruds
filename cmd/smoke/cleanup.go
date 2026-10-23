package main

import (
	"context"
	"encoding/json"
	"errors"
)

func (h *harness) cleanup(ctx context.Context) error {
	var failures []error
	seen := make(map[string]bool)
	for _, intent := range h.intents {
		id := intent.id
		if id == "" {
			// A timed-out POST may have committed. Recover its durable response
			// using the exact original key and payload, then release that fixture.
			result, err := h.request(ctx, "recuperar_reserva_para_limpieza", "POST", "/v1/reservations", h.admin, intent.input, intent.key, 0, nil)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if result.status == 409 {
				continue // A rejected key never created a reservation.
			}
			var recovered reservation
			if json.Unmarshal(result.body, &recovered) != nil || h.validateReservation(recovered, intent.input, "RESERVED") != nil {
				failures = append(failures, errors.New("no se pudo identificar con seguridad una reserva de esta ejecución"))
				continue
			}
			id = recovered.ID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var current reservation
		if _, err := h.request(ctx, "estado_para_limpieza", "GET", "/v1/reservations/"+id, h.admin, nil, "", 200, &current); err != nil {
			failures = append(failures, err)
			continue
		}
		if current.ID != id || current.VehicleID != h.vehicleID || current.CustomerID != h.customerID {
			failures = append(failures, errors.New("limpieza omitida: reserva ajena a la fixture"))
			continue
		}
		path, state := "", ""
		switch current.Status {
		case "RESERVED":
			path, state = "/v1/reservations/"+id, "CANCELLED"
		case "ACTIVE":
			path, state = "/v1/reservations/"+id+"/return", "COMPLETED"
		case "CANCELLED", "COMPLETED":
			continue
		default:
			failures = append(failures, errors.New("estado de reserva desconocido durante limpieza"))
			continue
		}
		method := "DELETE"
		if state == "COMPLETED" {
			method = "POST"
		}
		var released reservation
		if _, err := h.request(ctx, "liberar_reserva", method, path, h.admin, nil, "", 200, &released); err != nil {
			failures = append(failures, err)
		} else if released.ID != id || released.Status != state {
			failures = append(failures, errors.New("reserva no alcanzó estado liberado"))
		}
	}
	if h.employeeID != "" {
		var inactive employee
		if _, err := h.request(ctx, "desactivar_empleado_smoke", "DELETE", "/v1/employees/"+h.employeeID, h.admin, nil, "", 200, &inactive); err != nil {
			failures = append(failures, err)
		} else if inactive.ID != h.employeeID || inactive.Active {
			failures = append(failures, errors.New("empleado no quedó inactivo"))
		} else if err := h.verifyGymEmployeeStatus(ctx, h.employeeID, false); err != nil {
			failures = append(failures, err)
		} else if found, err := h.gymListContains(ctx, h.employeeID); err != nil {
			failures = append(failures, err)
		} else if found {
			failures = append(failures, errors.New("empleado inactivo sigue expuesto al gimnasio"))
		}
	}
	if h.candidateID != "" {
		if _, err := h.request(ctx, "eliminar_candidato_smoke", "DELETE", "/v1/candidates/"+h.candidateID, h.admin, nil, "", 204, nil); err != nil {
			failures = append(failures, err)
		} else if _, err := h.request(ctx, "candidato_eliminado", "GET", "/v1/candidates/"+h.candidateID, h.admin, nil, "", 404, nil); err != nil {
			failures = append(failures, err)
		}
	}
	if h.vehicleIntent {
		var quote struct {
			ID      string `json:"vehicleId"`
			Version int64  `json:"version"`
		}
		result, err := h.request(ctx, "version_autoritativa_para_limpieza", "GET", "/v1/vehicles/"+h.vehicleID+"/quote", h.admin, nil, "", -1, &quote)
		if err != nil {
			failures = append(failures, err)
		} else if result.status == 200 {
			if quote.ID != h.vehicleID || quote.Version < 1 {
				failures = append(failures, errors.New("no se pudo identificar con seguridad el vehículo propio"))
			} else {
				h.vehicle.Version, h.vehicle.Active = quote.Version, false
				var inactive vehicle
				if _, err := h.request(ctx, "desactivar_vehiculo_smoke", "PUT", "/v1/vehicles/"+h.vehicleID, h.admin, h.vehicle, "", 200, &inactive); err != nil {
					failures = append(failures, err)
				} else if inactive.ID != h.vehicleID || inactive.Active || inactive.Version != quote.Version+1 {
					failures = append(failures, errors.New("vehículo no quedó inactivo con nueva versión"))
				} else {
					h.vehicle = inactive
					if err := h.searchVisible(ctx, false); err != nil {
						failures = append(failures, err)
					}
				}
			}
		}
	}
	return errors.Join(failures...)
}

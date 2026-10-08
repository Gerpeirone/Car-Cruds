// Mock de contrato: todos los datos viven en memoria y se pierden al reiniciar.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
)

const dateLayout = "2006-01-02"

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type reservationRequest struct {
	VehicleID          string `json:"vehicleId"`
	CustomerID         string `json:"customerId"`
	StartsOn           string `json:"startsOn"`
	EndsOn             string `json:"endsOn"`
	ExpectedTotalMinor *int64 `json:"expectedTotalMinor"`
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

type availability struct {
	VehicleID         string `json:"vehicleId"`
	StartsOn          string `json:"startsOn"`
	EndsOn            string `json:"endsOn"`
	Available         bool   `json:"available"`
	Days              int    `json:"days"`
	DailyRateMinor    int64  `json:"dailyRateMinor"`
	TotalMinor        int64  `json:"totalMinor"`
	Currency          string `json:"currency"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type storedResponse struct {
	Canonical string
	Status    int
	Body      []byte
}

type api struct {
	mu           sync.Mutex
	token        string
	location     *time.Location
	rates        map[string]int64
	customers    map[string]bool
	reservations map[string]reservation
	idempotency  map[string]storedResponse
	nextID       int
}

func newAPI(token string) *api {
	location, err := time.LoadLocation("America/Buenos_Aires")
	if err != nil {
		panic(err) // tzdata se incluye en el ejecutable para no depender del host.
	}
	return &api{
		token: token, location: location,
		rates:        map[string]int64{"auto-001": 5000000, "auto-002": 6500000},
		customers:    map[string]bool{"cli-001": true, "cli-blocked": false},
		reservations: make(map[string]reservation),
		idempotency:  make(map[string]storedResponse),
	}
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.token)) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, apiError{"UNAUTHORIZED", "Se requiere un token Bearer válido."})
		return
	}
	switch {
	case r.URL.Path == "/v1/availability":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		a.getAvailability(w, r)
	case r.URL.Path == "/v1/reservations":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		a.createReservation(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/reservations/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/reservations/")
		if !identifierPattern.MatchString(id) {
			writeJSON(w, http.StatusNotFound, apiError{"NOT_FOUND", "Reserva inexistente."})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			methodNotAllowed(w, "GET, DELETE")
			return
		}
		a.reservationByID(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, apiError{"NOT_FOUND", "Ruta inexistente."})
	}
}

func methodNotAllowed(w http.ResponseWriter, methods string) {
	w.Header().Set("Allow", methods)
	writeJSON(w, http.StatusMethodNotAllowed, apiError{"METHOD_NOT_ALLOWED", "Método no admitido para esta ruta."})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	writeEncodedJSON(w, status, body)
}

func writeEncodedJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (a *api) calendarDays(startsOn, endsOn string) (int, error) {
	start, err := time.ParseInLocation(dateLayout, startsOn, a.location)
	if err != nil || start.Format(dateLayout) != startsOn {
		return 0, fmt.Errorf("startsOn debe ser una fecha válida YYYY-MM-DD")
	}
	end, err := time.ParseInLocation(dateLayout, endsOn, a.location)
	if err != nil || end.Format(dateLayout) != endsOn {
		return 0, fmt.Errorf("endsOn debe ser una fecha válida YYYY-MM-DD")
	}
	// Convertir solo componentes de calendario a UTC evita cobrar horas de DST.
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	days := int(endDay.Sub(startDay) / (24 * time.Hour))
	if days < 1 || days > 30 {
		return 0, fmt.Errorf("el período [startsOn, endsOn) debe durar entre 1 y 30 días calendario")
	}
	return days, nil
}

// Caller mantiene mu: comprobar y ocupar el período constituye una sola operación.
func (a *api) isAvailable(vehicleID, startsOn, endsOn string) bool {
	for _, existing := range a.reservations {
		if (existing.Status == "RESERVED" || existing.Status == "ACTIVE") && existing.VehicleID == vehicleID &&
			startsOn < existing.EndsOn && existing.StartsOn < endsOn {
			return false
		}
	}
	return true
}

func (a *api) getAvailability(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for _, key := range []string{"vehicleId", "startsOn", "endsOn"} {
		if len(query[key]) != 1 {
			writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "Se requiere un valor único para vehicleId, startsOn y endsOn."})
			return
		}
	}
	vehicleID, startsOn, endsOn := query.Get("vehicleId"), query.Get("startsOn"), query.Get("endsOn")
	if !identifierPattern.MatchString(vehicleID) {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "vehicleId tiene un formato inválido."})
		return
	}
	days, err := a.calendarDays(startsOn, endsOn)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", err.Error()})
		return
	}
	a.mu.Lock()
	rate, exists := a.rates[vehicleID]
	if !exists {
		a.mu.Unlock()
		writeJSON(w, http.StatusNotFound, apiError{"NOT_FOUND", "Vehículo inexistente."})
		return
	}
	available := a.isAvailable(vehicleID, startsOn, endsOn)
	a.mu.Unlock()
	result := availability{
		VehicleID: vehicleID, StartsOn: startsOn, EndsOn: endsOn, Available: available,
		Days: days, DailyRateMinor: rate, TotalMinor: rate * int64(days), Currency: "ARS",
	}
	if !available {
		result.UnavailableReason = "OVERLAPPING_RESERVATION"
	}
	writeJSON(w, http.StatusOK, result)
}

func validIdempotencyKey(key string) bool {
	if len(key) < 1 || len(key) > 128 {
		return false
	}
	for _, character := range key {
		if character < '!' || character > '~' {
			return false
		}
	}
	return true
}

func (a *api) createReservation(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !validIdempotencyKey(key) {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "Idempotency-Key es obligatorio (1 a 128 caracteres ASCII sin espacios)."})
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, apiError{"UNSUPPORTED_MEDIA_TYPE", "Content-Type debe ser application/json."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input reservationRequest
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "El cuerpo debe ser un objeto JSON válido con los cinco campos documentados."})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "Se admite un único objeto JSON."})
		return
	}
	if !identifierPattern.MatchString(input.VehicleID) || !identifierPattern.MatchString(input.CustomerID) ||
		input.StartsOn == "" || input.EndsOn == "" || input.ExpectedTotalMinor == nil || *input.ExpectedTotalMinor < 0 {
		writeJSON(w, http.StatusBadRequest, apiError{"INVALID_INPUT", "Los cinco campos son obligatorios; IDs válidos e importe entero no negativo."})
		return
	}
	canonical, _ := json.Marshal(input) // Mismo contenido, independientemente del orden y espacios del JSON.
	a.mu.Lock()
	defer a.mu.Unlock()
	if previous, exists := a.idempotency[key]; exists {
		if previous.Canonical != string(canonical) {
			writeJSON(w, http.StatusConflict, apiError{"IDEMPOTENCY_KEY_REUSED", "La clave ya se usó con otro contenido."})
			return
		}
		writeEncodedJSON(w, previous.Status, previous.Body)
		return
	}
	respond := func(status int, value any) {
		body, _ := json.Marshal(value)
		a.idempotency[key] = storedResponse{Canonical: string(canonical), Status: status, Body: body}
		writeEncodedJSON(w, status, body)
	}
	days, err := a.calendarDays(input.StartsOn, input.EndsOn)
	if err != nil {
		respond(http.StatusBadRequest, apiError{"INVALID_INPUT", err.Error()})
		return
	}
	rate, exists := a.rates[input.VehicleID]
	if !exists {
		respond(http.StatusNotFound, apiError{"NOT_FOUND", "Vehículo inexistente."})
		return
	}
	eligible, exists := a.customers[input.CustomerID]
	if !exists {
		respond(http.StatusNotFound, apiError{"NOT_FOUND", "Cliente inexistente."})
		return
	}
	if !eligible {
		respond(http.StatusUnprocessableEntity, apiError{"CUSTOMER_NOT_ELIGIBLE", "El cliente está bloqueado para reservar."})
		return
	}
	total := rate * int64(days)
	if *input.ExpectedTotalMinor != total {
		respond(http.StatusConflict, apiError{"PRICE_CHANGED", "El importe esperado no coincide con la tarifa vigente; consulte disponibilidad y use una nueva clave."})
		return
	}
	if !a.isAvailable(input.VehicleID, input.StartsOn, input.EndsOn) {
		respond(http.StatusConflict, apiError{"VEHICLE_UNAVAILABLE", "El vehículo ya tiene una reserva que se solapa con el período."})
		return
	}
	a.nextID++
	result := reservation{
		ID: fmt.Sprintf("res-%06d", a.nextID), Status: "RESERVED",
		VehicleID: input.VehicleID, CustomerID: input.CustomerID, StartsOn: input.StartsOn, EndsOn: input.EndsOn,
		Days: days, DailyRateMinor: rate, TotalMinor: total, Currency: "ARS",
	}
	a.reservations[result.ID] = result
	respond(http.StatusCreated, result)
}

func (a *api) reservationByID(w http.ResponseWriter, r *http.Request, id string) {
	a.mu.Lock()
	result, exists := a.reservations[id]
	if exists && r.Method == http.MethodDelete {
		if result.Status != "RESERVED" && result.Status != "CANCELLED" {
			a.mu.Unlock()
			writeJSON(w, http.StatusConflict, apiError{"INVALID_STATE", "Solo se pueden cancelar reservas en estado RESERVED; CANCELLED admite repetición."})
			return
		}
		result.Status = "CANCELLED"
		a.reservations[id] = result
	}
	a.mu.Unlock()
	if !exists {
		writeJSON(w, http.StatusNotFound, apiError{"NOT_FOUND", "Reserva inexistente."})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func main() {
	address := os.Getenv("MOCK_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	token := os.Getenv("MOCK_API_TOKEN")
	if token == "" {
		token = "demo-token" // Fixture pública, únicamente para el mock local.
	}
	server := &http.Server{
		Addr: address, Handler: newAPI(token), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("Mock en memoria en http://%s; Ctrl+C detiene y descarta los datos.", address)
	log.Fatal(server.ListenAndServe())
}

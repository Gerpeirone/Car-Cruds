package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const demoBody = `{"vehicleId":"auto-001","customerId":"cli-001","startsOn":"2026-10-15","endsOn":"2026-10-18","expectedTotalMinor":15000000}`

func request(handler http.Handler, method, path, body, key, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	return result
}

func decodeObject(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var value map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("respuesta JSON inválida: %v", err)
	}
	return value
}

func expectCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP %d, se esperaba %d: %s", response.Code, status, response.Body.String())
	}
	value := decodeObject(t, response)
	if value["code"] != code || value["message"] == "" || len(value) != 2 {
		t.Fatalf("error inesperado: %#v", value)
	}
}

func TestAvailabilityAndCalendarPricing(t *testing.T) {
	api := newAPI("demo-token")
	for _, test := range []struct {
		vehicle, start, end string
		days                int
		rate                int64
	}{
		{"auto-001", "2026-10-15", "2026-10-18", 3, 5000000},
		{"auto-002", "2028-02-28", "2028-03-01", 2, 6500000},
		{"auto-001", "2026-12-31", "2027-01-01", 1, 5000000},
		// El mock permite pasado para mantener las fixtures reproducibles.
		{"auto-001", "2020-01-01", "2020-01-31", 30, 5000000},
	} {
		t.Run(test.start+test.vehicle, func(t *testing.T) {
			path := fmt.Sprintf("/v1/availability?vehicleId=%s&startsOn=%s&endsOn=%s", test.vehicle, test.start, test.end)
			response := request(api, "GET", path, "", "", "demo-token")
			if response.Code != http.StatusOK {
				t.Fatal(response.Body.String())
			}
			value := decodeObject(t, response)
			if value["available"] != true || value["days"] != float64(test.days) ||
				value["dailyRateMinor"] != float64(test.rate) || value["totalMinor"] != float64(test.rate*int64(test.days)) || value["currency"] != "ARS" {
				t.Fatalf("cotización incorrecta: %#v", value)
			}
			if _, exists := value["unavailableReason"]; exists {
				t.Fatal("unavailableReason no corresponde cuando available=true")
			}
		})
	}
	for _, dates := range [][2]string{
		{"2026-02-29", "2026-03-01"}, {"2026-10-15", "2026-10-15"},
		{"2026-10-18", "2026-10-15"}, {"2026-10-01", "2026-11-01"},
		{"2026-10-15T00:00:00Z", "2026-10-18"}, {"2026-1-01", "2026-01-02"},
	} {
		path := fmt.Sprintf("/v1/availability?vehicleId=auto-001&startsOn=%s&endsOn=%s", dates[0], dates[1])
		expectCode(t, request(api, "GET", path, "", "", "demo-token"), 400, "INVALID_INPUT")
	}
}

func TestIdempotencyReplayCollisionAndCancellation(t *testing.T) {
	api := newAPI("demo-token")
	created := request(api, "POST", "/v1/reservations", demoBody, "same-key", "demo-token")
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	id := decodeObject(t, created)["id"].(string)
	// Normalizar orden de propiedades y espacios debe conservar identidad.
	reordered := ` { "expectedTotalMinor":15000000, "endsOn":"2026-10-18", "startsOn":"2026-10-15", "customerId":"cli-001", "vehicleId":"auto-001" } `
	replayed := request(api, "POST", "/v1/reservations", reordered, "same-key", "demo-token")
	if replayed.Code != created.Code || replayed.Body.String() != created.Body.String() {
		t.Fatalf("replay no coincide: %s", replayed.Body.String())
	}
	changed := strings.Replace(demoBody, "15000000", "14999999", 1)
	expectCode(t, request(api, "POST", "/v1/reservations", changed, "same-key", "demo-token"), 409, "IDEMPOTENCY_KEY_REUSED")
	busy := request(api, "GET", "/v1/availability?vehicleId=auto-001&startsOn=2026-10-17&endsOn=2026-10-19", "", "", "demo-token")
	busyValue := decodeObject(t, busy)
	if busyValue["available"] != false || busyValue["unavailableReason"] != "OVERLAPPING_RESERVATION" {
		t.Fatalf("la reserva no ocupó su período: %s", busy.Body.String())
	}
	// La frontera endsOn es exclusiva: una reserva adyacente no se solapa.
	adjacent := request(api, "GET", "/v1/availability?vehicleId=auto-001&startsOn=2026-10-18&endsOn=2026-10-19", "", "", "demo-token")
	if decodeObject(t, adjacent)["available"] != true {
		t.Fatal("una reserva adyacente debería estar disponible")
	}
	path := "/v1/reservations/" + id
	cancelled := request(api, "DELETE", path, "", "", "demo-token")
	if cancelled.Code != 200 || decodeObject(t, cancelled)["status"] != "CANCELLED" {
		t.Fatal(cancelled.Body.String())
	}
	secondCancel := request(api, "DELETE", path, "", "", "demo-token")
	if secondCancel.Code != 200 || secondCancel.Body.String() != cancelled.Body.String() {
		t.Fatal("cancelación repetida no fue idempotente")
	}
	current := request(api, "GET", path, "", "", "demo-token")
	if decodeObject(t, current)["status"] != "CANCELLED" {
		t.Fatal("GET no mostró el estado actual")
	}
	afterCancelReplay := request(api, "POST", "/v1/reservations", demoBody, "same-key", "demo-token")
	if afterCancelReplay.Code != 201 || afterCancelReplay.Body.String() != created.Body.String() {
		t.Fatal("el replay debe preservar la respuesta original incluso después de cancelar")
	}
	free := request(api, "GET", "/v1/availability?vehicleId=auto-001&startsOn=2026-10-15&endsOn=2026-10-18", "", "", "demo-token")
	if decodeObject(t, free)["available"] != true {
		t.Fatal("cancelar no liberó el período")
	}
	newReservation := request(api, "POST", "/v1/reservations", demoBody, "new-key", "demo-token")
	if newReservation.Code != 201 || decodeObject(t, newReservation)["id"] == id {
		t.Fatal("una nueva clave debe crear una nueva reserva en el período liberado")
	}
}

func TestConcurrentDistinctKeysAllowExactlyOneReservation(t *testing.T) {
	api := newAPI("demo-token")
	server := httptest.NewServer(api)
	defer server.Close()
	const clients = 24
	ready := make(chan struct{})
	statuses := make(chan int, clients)
	errors := make(chan error, clients)
	var workers sync.WaitGroup
	for i := 0; i < clients; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-ready
			req, err := http.NewRequest("POST", server.URL+"/v1/reservations", strings.NewReader(demoBody))
			if err != nil {
				errors <- err
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer demo-token")
			req.Header.Set("Idempotency-Key", fmt.Sprintf("concurrent-%d", i))
			response, err := server.Client().Do(req)
			if err != nil {
				errors <- err
				return
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				errors <- err
				return
			}
			if response.StatusCode == 409 {
				var value apiError
				if err := json.Unmarshal(body, &value); err != nil || value.Code != "VEHICLE_UNAVAILABLE" {
					errors <- fmt.Errorf("conflicto inesperado: %s", body)
					return
				}
			}
			statuses <- response.StatusCode
		}(i)
	}
	close(ready)
	workers.Wait()
	close(statuses)
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	winners, conflicts := 0, 0
	for status := range statuses {
		switch status {
		case 201:
			winners++
		case 409:
			conflicts++
		default:
			t.Errorf("HTTP inesperado: %d", status)
		}
	}
	if winners != 1 || conflicts != clients-1 || len(api.reservations) != 1 {
		t.Fatalf("ganadoras=%d conflictos=%d reservas=%d", winners, conflicts, len(api.reservations))
	}
}

func TestInputAuthAndBusinessErrors(t *testing.T) {
	for _, test := range []struct {
		name, body, key, token, code string
		status                       int
	}{
		{"missing-token", demoBody, "key", "", "UNAUTHORIZED", 401},
		{"wrong-token", demoBody, "key", "wrong", "UNAUTHORIZED", 401},
		{"missing-key", demoBody, "", "demo-token", "INVALID_INPUT", 400},
		{"malformed-json", "{", "key", "demo-token", "INVALID_INPUT", 400},
		{"trailing-json", demoBody + " {}", "key", "demo-token", "INVALID_INPUT", 400},
		{"unknown-field", strings.Replace(demoBody, "{", `{"extra":true,`, 1), "key", "demo-token", "INVALID_INPUT", 400},
		{"missing-price", strings.Replace(demoBody, `,"expectedTotalMinor":15000000`, "", 1), "key", "demo-token", "INVALID_INPUT", 400},
		{"negative-price", strings.Replace(demoBody, "15000000", "-1", 1), "key", "demo-token", "INVALID_INPUT", 400},
		{"fraction-price", strings.Replace(demoBody, "15000000", "15000000.5", 1), "key", "demo-token", "INVALID_INPUT", 400},
		{"invalid-date", strings.Replace(demoBody, "2026-10-15", "2026-02-30", 1), "key", "demo-token", "INVALID_INPUT", 400},
		{"different-price", strings.Replace(demoBody, "15000000", "14999999", 1), "key", "demo-token", "PRICE_CHANGED", 409},
		{"blocked-client", strings.Replace(demoBody, "cli-001", "cli-blocked", 1), "key", "demo-token", "CUSTOMER_NOT_ELIGIBLE", 422},
		{"missing-client", strings.Replace(demoBody, "cli-001", "cli-missing", 1), "key", "demo-token", "NOT_FOUND", 404},
		{"missing-vehicle", strings.Replace(demoBody, "auto-001", "auto-missing", 1), "key", "demo-token", "NOT_FOUND", 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectCode(t, request(newAPI("demo-token"), "POST", "/v1/reservations", test.body, test.key, test.token), test.status, test.code)
		})
	}
	api := newAPI("configured-demo")
	expectCode(t, request(api, "GET", "/v1/reservations/res-missing", "", "", "demo-token"), 401, "UNAUTHORIZED")
	expectCode(t, request(api, "GET", "/v1/reservations/res-missing", "", "", "configured-demo"), 404, "NOT_FOUND")
	response := request(api, "PUT", "/v1/reservations/res-missing", "", "", "configured-demo")
	expectCode(t, response, 405, "METHOD_NOT_ALLOWED")
	if response.Header().Get("Allow") != "GET, DELETE" {
		t.Fatal("Allow incorrecto")
	}
}

func TestRejectedRequestReplayIsStable(t *testing.T) {
	api := newAPI("demo-token")
	request(api, "POST", "/v1/reservations", demoBody, "winner", "demo-token")
	rejected := request(api, "POST", "/v1/reservations", demoBody, "loser", "demo-token")
	expectCode(t, rejected, 409, "VEHICLE_UNAVAILABLE")
	request(api, "DELETE", "/v1/reservations/res-000001", "", "", "demo-token")
	replay := request(api, "POST", "/v1/reservations", demoBody, "loser", "demo-token")
	if replay.Code != rejected.Code || replay.Body.String() != rejected.Body.String() || len(api.reservations) != 1 {
		t.Fatal("un rechazo se conserva para esa clave aunque cambie la disponibilidad; reintentar requiere clave nueva")
	}
}

func TestCancellationProtectsFutureStates(t *testing.T) {
	for _, status := range []string{"ACTIVE", "COMPLETED"} {
		t.Run(status, func(t *testing.T) {
			api := newAPI("demo-token")
			created := request(api, "POST", "/v1/reservations", demoBody, "create", "demo-token")
			id := decodeObject(t, created)["id"].(string)
			// Estado sintético exclusivo del test: no es una fixture pública del mock.
			stored := api.reservations[id]
			stored.Status = status
			api.reservations[id] = stored
			expectCode(t, request(api, "DELETE", "/v1/reservations/"+id, "", "", "demo-token"), 409, "INVALID_STATE")
			if api.reservations[id].Status != status {
				t.Fatal("el rechazo alteró el estado de la reserva")
			}
		})
	}
}

func TestMalformedJSONDoesNotConsumeIdempotencyKey(t *testing.T) {
	api := newAPI("demo-token")
	expectCode(t, request(api, "POST", "/v1/reservations", "{", "retry", "demo-token"), 400, "INVALID_INPUT")
	valid := request(api, "POST", "/v1/reservations", demoBody, "retry", "demo-token")
	if valid.Code != 201 {
		t.Fatal("JSON malformado no debe vincular la clave:", valid.Body.String())
	}
}

// Validador acotado a las palabras clave usadas por este contrato. No reemplaza
// un validador completo de OAS; detecta refs rotas y divergencias de respuestas.
func resolveReference(document map[string]any, reference string) (map[string]any, error) {
	if !strings.HasPrefix(reference, "#/") {
		return nil, fmt.Errorf("ref externa no soportada: %s", reference)
	}
	var node any = document
	for _, segment := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		object, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ref no resuelta: %s", reference)
		}
		node = object[strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")]
	}
	object, ok := node.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ref no resuelta: %s", reference)
	}
	return object, nil
}

func validateSchema(document, schema map[string]any, value any) error {
	if reference, ok := schema["$ref"].(string); ok {
		resolved, err := resolveReference(document, reference)
		if err != nil {
			return err
		}
		return validateSchema(document, resolved, value)
	}
	if alternatives, ok := schema["oneOf"].([]any); ok {
		matches := 0
		for _, alternative := range alternatives {
			if validateSchema(document, alternative.(map[string]any), value) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("oneOf coincide con %d alternativas", matches)
		}
	}
	if values, ok := schema["enum"].([]any); ok {
		found := false
		for _, option := range values {
			found = found || reflect.DeepEqual(option, value)
		}
		if !found {
			return fmt.Errorf("valor fuera de enum: %v", value)
		}
	}
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("se esperaba objeto")
		}
		for _, field := range schema["required"].([]any) {
			if _, ok := object[field.(string)]; !ok {
				return fmt.Errorf("falta campo %s", field)
			}
		}
		properties := schema["properties"].(map[string]any)
		for field, item := range object {
			property, declared := properties[field]
			if !declared {
				if schema["additionalProperties"] == false {
					return fmt.Errorf("campo no declarado: %s", field)
				}
				continue
			}
			if err := validateSchema(document, property.(map[string]any), item); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("se esperaba string")
		}
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(text) {
			return fmt.Errorf("string no cumple pattern")
		}
		if minimum, ok := schema["minLength"].(float64); ok && len(text) < int(minimum) {
			return fmt.Errorf("string demasiado corto")
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("se esperaba entero")
		}
		if minimum, ok := schema["minimum"].(float64); ok && number < minimum {
			return fmt.Errorf("entero menor al mínimo")
		}
		if maximum, ok := schema["maximum"].(float64); ok && number > maximum {
			return fmt.Errorf("entero mayor al máximo")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("se esperaba boolean")
		}
	}
	return nil
}

func TestOpenAPIReferencesAndResponseContract(t *testing.T) {
	bytes, err := os.ReadFile("../../docs/contracts/openapi-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(bytes, &document); err != nil {
		t.Fatal(err)
	}
	if document["openapi"] != "3.0.3" || document["info"].(map[string]any)["version"] != "1.0.0" {
		t.Fatal("versiones de contrato incorrectas")
	}
	var visit func(any)
	visit = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			if reference, ok := node["$ref"].(string); ok {
				if _, err := resolveReference(document, reference); err != nil {
					t.Error(err)
				}
			}
			for _, child := range node {
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(document)
	check := func(method, contractPath string, response *httptest.ResponseRecorder) {
		t.Helper()
		operation := document["paths"].(map[string]any)[contractPath].(map[string]any)[method].(map[string]any)
		definition, exists := operation["responses"].(map[string]any)[strconv.Itoa(response.Code)]
		if !exists {
			t.Fatalf("HTTP %d no está en %s %s", response.Code, method, contractPath)
		}
		responseDefinition := definition.(map[string]any)
		if reference, ok := responseDefinition["$ref"].(string); ok {
			responseDefinition, err = resolveReference(document, reference)
			if err != nil {
				t.Fatal(err)
			}
		}
		schema := responseDefinition["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		if err := validateSchema(document, schema, decodeObject(t, response)); err != nil {
			t.Fatalf("%s %s HTTP%d viola el contrato: %v", method, contractPath, response.Code, err)
		}
	}
	api := newAPI("demo-token")
	availabilityPath := "/v1/availability?vehicleId=auto-001&startsOn=2026-10-15&endsOn=2026-10-18"
	check("get", "/v1/availability", request(api, "GET", availabilityPath, "", "", "demo-token"))
	check("get", "/v1/availability", request(api, "GET", availabilityPath, "", "", ""))
	check("get", "/v1/availability", request(api, "GET", "/v1/availability", "", "", "demo-token"))
	check("post", "/v1/reservations", request(api, "POST", "/v1/reservations", demoBody, "contract-key", "demo-token"))
	check("get", "/v1/availability", request(api, "GET", availabilityPath, "", "", "demo-token"))
	check("post", "/v1/reservations", request(api, "POST", "/v1/reservations", demoBody, "overlap", "demo-token"))
	check("post", "/v1/reservations", request(api, "POST", "/v1/reservations", strings.Replace(demoBody, "cli-001", "cli-blocked", 1), "blocked", "demo-token"))
	check("get", "/v1/reservations/{id}", request(api, "GET", "/v1/reservations/res-000001", "", "", "demo-token"))
	check("delete", "/v1/reservations/{id}", request(api, "DELETE", "/v1/reservations/res-000001", "", "", "demo-token"))
	future := api.reservations["res-000001"]
	future.Status = "ACTIVE"
	api.reservations[future.ID] = future
	check("delete", "/v1/reservations/{id}", request(api, "DELETE", "/v1/reservations/res-000001", "", "", "demo-token"))
	check("delete", "/v1/reservations/{id}", request(api, "DELETE", "/v1/reservations/res-missing", "", "", "demo-token"))
	check("get", "/v1/reservations/{id}", request(api, "GET", "/v1/reservations/res-missing", "", "", "demo-token"))
}

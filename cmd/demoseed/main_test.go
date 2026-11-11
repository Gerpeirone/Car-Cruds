package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const adminToken = "admin-fixture-not-a-real-secret"
const clientToken = "client-fixture-not-a-real-secret"

// fakeGateway keeps just enough of each service to observe what the seed does.
type fakeGateway struct {
	mu           sync.Mutex
	customers    map[string]remoteCustomer
	employees    []remoteEmployee
	vehicles     map[string]remoteVehicle
	reservations []remoteReservation
	writes       []string
}

func newFake() *fakeGateway {
	return &fakeGateway{
		customers:    map[string]remoteCustomer{"cli-001": {ID: "cli-001", Name: "Cliente demo", Active: true, LicenseValidUntil: "2030-12-31"}},
		employees:    []remoteEmployee{{ID: "employee-001", DisplayName: "Persona ficticia de demostración Uno", Active: true}},
		vehicles:     map[string]remoteVehicle{"auto-001": {VehicleID: "auto-001", Brand: "Marca Demo", Model: "Sedán Uno", Active: true, Version: 1}},
		reservations: []remoteReservation{{ID: "res-previa", Status: "RESERVED", VehicleID: "auto-001", CustomerID: "cli-001"}},
	}
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != adminToken && token != clientToken {
		w.WriteHeader(401)
		return
	}
	write := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if r.Method != "GET" {
		f.writes = append(f.writes, r.Method+" "+r.URL.Path)
	}
	path := r.URL.Path
	switch {
	case r.Method == "POST" && path == "/v1/customers":
		id := body["id"].(string)
		if _, ok := f.customers[id]; ok {
			write(409, map[string]string{"code": "CONFLICT"})
			return
		}
		f.customers[id] = remoteCustomer{ID: id, Name: body["name"].(string), Active: body["active"].(bool), LicenseValidUntil: body["licenseValidUntil"].(string)}
		write(201, f.customers[id])
	case r.Method == "GET" && strings.HasPrefix(path, "/v1/customers/"):
		write(200, f.customers[strings.TrimPrefix(path, "/v1/customers/")])
	case r.Method == "GET" && path == "/v1/admin/employees":
		write(200, map[string]any{"items": f.employees, "total": len(f.employees)})
	case r.Method == "POST" && path == "/v1/employees":
		e := remoteEmployee{ID: fmt.Sprintf("emp-%d", len(f.employees)), DisplayName: body["displayName"].(string), Active: body["active"].(bool)}
		f.employees = append(f.employees, e)
		write(201, e)
	case r.Method == "DELETE" && strings.HasPrefix(path, "/v1/employees/"):
		for i := range f.employees {
			if f.employees[i].ID == strings.TrimPrefix(path, "/v1/employees/") {
				f.employees[i].Active = false
			}
		}
		write(200, map[string]any{})
	case r.Method == "POST" && path == "/v1/vehicles":
		id := body["vehicleId"].(string)
		if _, ok := f.vehicles[id]; ok {
			write(409, map[string]string{"code": "CONFLICT"})
			return
		}
		f.vehicles[id] = remoteVehicle{VehicleID: id, Brand: body["brand"].(string), Model: body["model"].(string), Active: true, Version: 1}
		write(201, f.vehicles[id])
	case r.Method == "GET" && path == "/v1/vehicles/search":
		_, ok := f.vehicles["demo-renault-kangoo"]
		write(200, map[string]any{"total": map[bool]int{true: 1}[ok]})
	case r.Method == "GET" && strings.HasPrefix(path, "/v1/vehicles/"):
		v, ok := f.vehicles[strings.TrimPrefix(path, "/v1/vehicles/")]
		if !ok {
			write(404, map[string]string{"code": "NOT_FOUND"})
			return
		}
		write(200, v)
	case r.Method == "PUT" && strings.HasPrefix(path, "/v1/vehicles/"):
		id := strings.TrimPrefix(path, "/v1/vehicles/")
		v := f.vehicles[id]
		v.Active, v.Version = body["active"].(bool), v.Version+1
		f.vehicles[id] = v
		write(200, v)
	case r.Method == "GET" && path == "/v1/reservations":
		var items []remoteReservation
		for _, x := range f.reservations {
			if x.CustomerID == r.URL.Query().Get("customerId") {
				items = append(items, x)
			}
		}
		write(200, map[string]any{"items": items, "total": len(items)})
	case r.Method == "GET" && path == "/v1/availability":
		write(200, map[string]any{"available": true, "totalMinor": 1000})
	case r.Method == "POST" && path == "/v1/reservations":
		if r.Header.Get("Idempotency-Key") == "" {
			write(400, map[string]string{"code": "INVALID_INPUT"})
			return
		}
		x := remoteReservation{ID: fmt.Sprintf("res-%d", len(f.reservations)), Status: "RESERVED", VehicleID: body["vehicleId"].(string), CustomerID: body["customerId"].(string)}
		f.reservations = append(f.reservations, x)
		write(201, x)
	case r.Method == "POST" && strings.HasSuffix(path, "/pickup"), r.Method == "POST" && strings.HasSuffix(path, "/return"), r.Method == "DELETE" && strings.HasPrefix(path, "/v1/reservations/"):
		id := strings.Split(strings.TrimPrefix(path, "/v1/reservations/"), "/")[0]
		next := map[string]string{"POST pickup": "ACTIVE", "POST return": "COMPLETED", "DELETE": "CANCELLED"}
		key := r.Method
		if r.Method == "POST" {
			key += " " + path[strings.LastIndex(path, "/")+1:]
		}
		for i := range f.reservations {
			if f.reservations[i].ID == id {
				f.reservations[i].Status = next[key]
			}
		}
		write(200, map[string]any{})
	default:
		write(404, map[string]string{"code": "NOT_FOUND"})
	}
}

func runSeed(t *testing.T, f *fakeGateway, args ...string) (int, string) {
	t.Helper()
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	t.Setenv("AUTH_ADMIN_TOKEN", adminToken)
	t.Setenv("AUTH_CLIENT_TOKEN", clientToken)
	t.Setenv("AUTH_CLIENT_CUSTOMER_ID", "cli-001")
	var out bytes.Buffer
	code := run(append([]string{"-env", "", "-url", server.URL, "-timeout", "40s"}, args...), &out)
	if strings.Contains(out.String(), adminToken) || strings.Contains(out.String(), clientToken) {
		t.Fatal("la salida contiene un token")
	}
	return code, out.String()
}

func TestSeedIsIdempotentAndNeverTouchesExistingRecords(t *testing.T) {
	f := newFake()
	if code, out := runSeed(t, f, "-apply"); code != 0 {
		t.Fatalf("primera carga=%d\n%s", code, out)
	}
	if len(f.customers) != 1+len(customers) || len(f.employees) != 1+len(employees) || len(f.vehicles) != 1+len(vehicles) {
		t.Fatalf("cantidades: %d clientes, %d empleados, %d vehículos", len(f.customers), len(f.employees), len(f.vehicles))
	}
	active := 0
	for _, r := range f.reservations {
		if r.Status == "ACTIVE" {
			active++
		}
	}
	if len(f.reservations) != 1+len(reservations)+len(credentialReservations) || active != 2 {
		t.Fatalf("reservas=%+v", f.reservations)
	}
	writes := len(f.writes)
	code, out := runSeed(t, f, "-apply")
	if code != 0 || strings.Contains(out, "CREADO") || strings.Contains(out, "RETIRO") {
		t.Fatalf("segunda carga=%d\n%s", code, out)
	}
	for _, w := range f.writes[writes:] {
		if !strings.HasPrefix(w, "POST /v1/customers") && !strings.HasPrefix(w, "POST /v1/vehicles") {
			t.Fatalf("la segunda carga escribió %s", w)
		}
	}
	if len(f.reservations) != 1+len(reservations)+len(credentialReservations) {
		t.Fatal("la segunda carga duplicó reservas")
	}
	for _, w := range f.writes {
		if strings.Contains(w, "auto-001") || strings.Contains(w, "employee-001") || strings.Contains(w, "res-previa") || strings.HasPrefix(w, "PUT /v1/customers") {
			t.Fatalf("se modificó un registro existente: %s", w)
		}
	}
}

func TestCleanupRetiresOnlyDemoRecords(t *testing.T) {
	f := newFake()
	if code, out := runSeed(t, f, "-apply"); code != 0 {
		t.Fatal(out)
	}
	code, out := runSeed(t, f, "-cleanup")
	if code != 1 || !strings.Contains(out, "se conserva (usar -return-active") {
		t.Fatalf("sin -return-active los alquileres activos deben conservarse: %d\n%s", code, out)
	}
	for _, r := range f.reservations {
		switch {
		case r.ID == "res-previa" && r.Status != "RESERVED":
			t.Fatal("se canceló una reserva previa")
		case r.ID != "res-previa" && r.Status == "RESERVED":
			t.Fatalf("reserva demo sin cancelar: %+v", r)
		}
	}
	for id, v := range f.vehicles {
		if v.Active != (id == "auto-001") {
			t.Fatalf("vehículo %s activo=%t", id, v.Active)
		}
	}
	for _, e := range f.employees {
		if e.ID == "employee-001" && !e.Active {
			t.Fatal("se dio de baja un empleado de las integraciones")
		}
		if strings.HasPrefix(e.DisplayName, "Demo ·") && e.Active {
			t.Fatalf("empleado demo activo: %+v", e)
		}
	}
	if code, out := runSeed(t, f, "-cleanup", "-return-active"); code != 0 {
		t.Fatalf("limpieza con devolución=%d\n%s", code, out)
	}
	for _, r := range f.reservations {
		if r.ID != "res-previa" && (r.Status == "ACTIVE" || r.Status == "RESERVED") {
			t.Fatalf("quedó abierta %+v", r)
		}
	}
}

func TestSeedRefusesRemoteOriginsAndRunsNothingByDefault(t *testing.T) {
	for _, u := range []string{"https://57-156-56-201.sslip.io", "http://10.0.0.4:8080", "http://127.0.0.1:8080/v1", "ftp://127.0.0.1"} {
		if _, err := gatewayURL(u, false); err == nil {
			t.Fatalf("%s aceptado", u)
		}
	}
	if _, err := gatewayURL("http://127.0.0.1:8080", false); err != nil {
		t.Fatal(err)
	}
	f := newFake()
	if code, out := runSeed(t, f); code != 0 || !strings.Contains(out, "Plan (sin cambios") || len(f.writes) != 0 {
		t.Fatalf("sin -apply no debe escribir: %d %v\n%s", code, f.writes, out)
	}
}

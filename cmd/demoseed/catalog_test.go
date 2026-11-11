package main

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// azureLike mirrors the state to preserve: three earlier vehicles (one with
// the same brand and model as a demo vehicle, one inactive), the employees of
// the integrations plus one already named like a demo employee, and the five
// reservations of cli-001 created by the operator. All data is fictitious.
func azureLike() *fakeGateway {
	f := newFake()
	f.vehicles = map[string]remoteVehicle{
		"auto-001": {VehicleID: "auto-001", Brand: "Marca Demo", Model: "Sedán Uno", Category: "sedan", Transmission: "manual", Seats: 5, DailyRateMinor: 4500000, Currency: "ARS", Active: true, Version: 3},
		"auto-002": {VehicleID: "auto-002", Brand: "Toyota", Model: "Corolla", Category: "sedan", Transmission: "automatic", Seats: 5, DailyRateMinor: 5000000, Currency: "ARS", Active: true, Version: 1},
		"auto-003": {VehicleID: "auto-003", Brand: "Fiat", Model: "Uno", Category: "hatchback", Transmission: "manual", Seats: 5, DailyRateMinor: 2500000, Currency: "ARS", Active: false, Version: 2},
	}
	f.employees = []remoteEmployee{
		{ID: "employee-001", DisplayName: "Persona ficticia de demostración Uno", Active: true},
		{ID: "employee-002", DisplayName: "Persona ficticia de demostración Dos", Active: true},
		{ID: "employee-077", DisplayName: "demo · ana torres  (atención al cliente)", Active: false},
	}
	f.reservations = []remoteReservation{
		{ID: "res-1", Status: "RESERVED", VehicleID: "auto-001", CustomerID: "cli-001"},
		{ID: "res-2", Status: "RESERVED", VehicleID: "auto-002", CustomerID: "cli-001"},
		{ID: "res-3", Status: "RESERVED", VehicleID: "auto-001", CustomerID: "cli-001"},
		{ID: "res-4", Status: "RESERVED", VehicleID: "auto-002", CustomerID: "cli-001"},
		{ID: "res-5", Status: "CANCELLED", VehicleID: "auto-001", CustomerID: "cli-001"},
	}
	return f
}

type snapshot struct {
	customers    map[string]remoteCustomer
	vehicles     map[string]remoteVehicle
	employees    []remoteEmployee
	reservations []remoteReservation
}

func take(f *fakeGateway) snapshot {
	return snapshot{maps.Clone(f.customers), maps.Clone(f.vehicles), slices.Clone(f.employees), slices.Clone(f.reservations)}
}

func TestCatalogPreviewReadsOnly(t *testing.T) {
	f := azureLike()
	before := take(f)
	code, out := runSeed(t, f, "-catalog")
	if code != 0 || len(f.writes) != 0 || !reflect.DeepEqual(before, take(f)) {
		t.Fatalf("la vista previa escribió: %d %v\n%s", code, f.writes, out)
	}
	for _, want := range []string{
		"Vista previa del catálogo demo",
		"OMITIDO  vehículo demo-toyota-corolla", // same brand and model as auto-002
		"CREAR    vehículo demo-peugeot-2008",
		"EXISTE   empleado Demo · Ana Torres (Atención al cliente) (employee-077",
		"CREAR    empleado Demo · Matías Rojas (Seguridad)",
		"vehículos: 3 existentes, 15 a crear · empleados: 3 existentes, 9 a crear",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q en\n%s", want, out)
		}
	}
}

func TestCatalogApplyIsIdempotentAndPreservesEverything(t *testing.T) {
	f := azureLike()
	before := take(f)
	code, out := runSeed(t, f, "-catalog", "-apply")
	if code != 0 {
		t.Fatalf("carga=%d\n%s", code, out)
	}
	// Only vehicle and employee creations: no customers, reservations,
	// rentals, edits, deactivations, Hospital or Gimnasio.
	for _, w := range f.writes {
		if w != "POST /v1/vehicles" && w != "POST /v1/employees" {
			t.Fatalf("escritura no permitida: %s", w)
		}
	}
	if !reflect.DeepEqual(before.customers, f.customers) || !reflect.DeepEqual(before.reservations, f.reservations) {
		t.Fatal("cambiaron clientes o reservas")
	}
	for id, v := range before.vehicles {
		if f.vehicles[id] != v {
			t.Fatalf("se modificó el vehículo previo %s", id)
		}
	}
	if !reflect.DeepEqual(before.employees, f.employees[:len(before.employees)]) {
		t.Fatal("se modificó un empleado previo")
	}
	// No duplicates by brand and model or by name, at least 14 models, and
	// 8 to 12 demo employees.
	models := map[string]int{}
	for _, v := range f.vehicles {
		models[sameKey(v.Brand, v.Model)]++
	}
	for model, n := range models {
		if n != 1 {
			t.Fatalf("modelo duplicado %s ×%d", model, n)
		}
	}
	names, demo := map[string]int{}, 0
	for _, e := range f.employees {
		names[sameKey(e.DisplayName)]++
		if strings.HasPrefix(strings.ToLower(e.DisplayName), "demo ·") {
			demo++
		}
	}
	for name, n := range names {
		if n != 1 {
			t.Fatalf("empleado duplicado %s ×%d", name, n)
		}
	}
	if len(f.vehicles) != 3+15 || len(models) < 14 || demo < 8 || demo > 12 || len(f.employees) != 3+9 {
		t.Fatalf("vehículos=%d modelos=%d empleados demo=%d total=%d", len(f.vehicles), len(models), demo, len(f.employees))
	}
	for _, want := range []string{"VERIFICADO índice Solr", "modelos distintos", "VERIFICADO búsqueda de texto", "VERIFICADO filtro brand=Toyota", "VERIFICADO filtro seats=7", "VERIFICADO orden price_asc", "VERIFICADO orden price_desc", "VERIFICADO paginado", "TERMINADO sin observaciones"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q en\n%s", want, out)
		}
	}
	// Second run: nothing new.
	writes := len(f.writes)
	after := take(f)
	code, out = runSeed(t, f, "-catalog", "-apply")
	if code != 0 || len(f.writes) != writes || strings.Contains(out, "CREADO") || !reflect.DeepEqual(after, take(f)) {
		t.Fatalf("la segunda carga cambió algo: %d %v\n%s", code, f.writes[writes:], out)
	}
}

func TestCatalogVerifyReportsAnIndexBehindFlota(t *testing.T) {
	f := azureLike()
	if code, out := runSeed(t, f, "-catalog", "-apply"); code != 0 {
		t.Fatal(out)
	}
	writes := len(f.writes)
	code, out := runSeed(t, f, "-verify")
	if code != 0 || len(f.writes) != writes || !strings.Contains(out, "VERIFICADO índice Solr: los 17 vehículos activos") {
		t.Fatalf("verificación=%d %v\n%s", code, f.writes[writes:], out)
	}
	// An active vehicle in Flota that the search does not return yet.
	defer func(previous time.Duration) { indexWait = previous }(indexWait)
	indexWait = 0
	f.unindexed = map[string]bool{"demo-nissan-kicks": true}
	code, out = runSeed(t, f, "-verify")
	if code != 1 || len(f.writes) != writes || !strings.Contains(out, "AVISO índice Solr: faltan 1 vehículos (demo-nissan-kicks)") {
		t.Fatalf("índice atrasado=%d\n%s", code, out)
	}
}

func TestCatalogFlagsCannotBeCombinedWithOtherModes(t *testing.T) {
	for _, args := range [][]string{{"-catalog", "-cleanup"}, {"-verify", "-apply"}, {"-verify", "-catalog"}} {
		f := azureLike()
		if code, out := runSeed(t, f, args...); code != 2 || len(f.writes) != 0 {
			t.Fatalf("%v aceptado: %d\n%s", args, code, out)
		}
	}
}

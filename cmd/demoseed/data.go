package main

// Fictitious demonstration data. Every identifier starts with "demo-" (or the
// employee name with "Demo ·") so the seed recognizes its own records and
// never touches pre-existing ones such as cli-001, auto-001 or employee-001.

type customer struct {
	ID, Name, License string
	Active            bool
	Scenario          string
}

type employee struct {
	Name   string
	Active bool
}

type vehicle struct {
	ID, Brand, Model, Category, Transmission string
	Seats                                    int
	DailyRateMinor                           int64
}

// reservation describes a scenario relative to the day the seed runs, in the
// branch calendar. Active rentals start today and are picked up by the operator.
type reservation struct {
	Customer, Vehicle string
	StartInDays, Days int
	Pickup            bool
	Scenario          string
}

var customers = []customer{
	{"demo-cli-habilitada", "Lucía Fernández", "2029-06-30", true, "activa y habilitada"},
	{"demo-cli-inactiva", "Martín Gómez", "2028-11-15", false, "inactiva"},
	{"demo-cli-licencia-vencida", "Sofía Ramírez", "2025-03-31", true, "licencia vencida"},
	{"demo-cli-alquiler-activo", "Diego Herrera", "2030-01-31", true, "con alquiler activo"},
	{"demo-cli-reserva-futura", "Valentina Ruiz", "2029-09-30", true, "con reserva futura"},
	{"demo-cli-sin-reservas", "Tomás Acosta", "2031-05-31", true, "sin reservas"},
}

var employees = []employee{
	{"Demo · Ana Torres (Atención al cliente)", true},
	{"Demo · Bruno Medina (Taller)", true},
	{"Demo · Camila Ortiz (Administración)", true},
	{"Demo · Federico Sosa (Logística)", true},
	{"Demo · Gabriela Paz (Ventas, baja)", false},
}

var vehicles = []vehicle{
	{"demo-toyota-corolla", "Toyota", "Corolla", "sedan", "automatic", 5, 5200000},
	{"demo-toyota-yaris", "Toyota", "Yaris", "hatchback", "manual", 5, 3900000},
	{"demo-vw-polo", "Volkswagen", "Polo", "hatchback", "manual", 5, 4100000},
	{"demo-vw-golf", "Volkswagen", "Golf", "hatchback", "automatic", 5, 5600000},
	{"demo-ford-ranger", "Ford", "Ranger", "pickup", "manual", 5, 8200000},
	{"demo-toyota-hilux", "Toyota", "Hilux", "pickup", "automatic", 5, 8900000},
	{"demo-peugeot-208", "Peugeot", "208", "hatchback", "manual", 5, 3800000},
	{"demo-chevrolet-onix", "Chevrolet", "Onix", "sedan", "manual", 5, 3700000},
	{"demo-renault-sandero", "Renault", "Sandero", "hatchback", "manual", 5, 3400000},
	{"demo-honda-civic", "Honda", "Civic", "sedan", "automatic", 5, 6100000},
	{"demo-jeep-renegade", "Jeep", "Renegade", "suv", "automatic", 5, 6900000},
	{"demo-fiat-cronos", "Fiat", "Cronos", "sedan", "manual", 5, 3600000},
	{"demo-nissan-kicks", "Nissan", "Kicks", "suv", "automatic", 5, 6200000},
	{"demo-renault-kangoo", "Renault", "Kangoo", "van", "manual", 7, 5400000},
}

// Rentals for the fictitious drivers (operator view) plus, for the driver of
// the demo rental credential, one future reservation and one active rental so
// that "Mis alquileres" has content. The credential customer comes from
// AUTH_CLIENT_CUSTOMER_ID (cli-001 by default) and is only read, never edited.
var reservations = []reservation{
	{"demo-cli-reserva-futura", "demo-vw-golf", 14, 3, false, "reserva futura"},
	{"demo-cli-alquiler-activo", "demo-toyota-hilux", 0, 4, true, "alquiler activo"},
}

var credentialReservations = []reservation{
	{"", "demo-honda-civic", 21, 2, false, "reserva futura de la credencial"},
	{"", "demo-jeep-renegade", 0, 3, true, "alquiler activo de la credencial"},
}

// Command demoseed loads (or retires) fictitious demonstration data through the
// gateway and the services' own APIs, so every domain validation, version
// check, idempotency key and the Flota → Solr indexing path apply as usual.
//
// It never runs by itself: without -apply or -cleanup it only prints the plan.
// -catalog limits it to vehicles and employees (a read-only preview against
// the target, or with -apply only the missing ones); -verify only checks the
// published catalog.
// It only creates records whose identifiers start with "demo-" (employees:
// "Demo ·"), never edits or deletes existing ones, and refuses non-loopback
// URLs unless -allow-remote is given. Tokens are read from the environment or
// the local .env and are never printed.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/devconfig"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

type seeder struct {
	base, admin, client, credentialCustomer string
	http                                    *http.Client
	out                                     io.Writer
	now                                     func() time.Time
	problems                                int
}

func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("demoseed", flag.ContinueOnError)
	fs.SetOutput(out)
	envFile := fs.String("env", ".env", "archivo .env local (las variables del entorno tienen prioridad)")
	target := fs.String("url", "", "origen del gateway; predeterminado http://127.0.0.1:${APP_PORT:-8080}")
	apply := fs.Bool("apply", false, "cargar los datos de demostración")
	cleanup := fs.Bool("cleanup", false, "retirar los datos de demostración (cancelar reservas, desactivar autos y empleados demo)")
	returnActive := fs.Bool("return-active", false, "con -cleanup, registrar también la devolución de los alquileres demo activos")
	allowRemote := fs.Bool("allow-remote", false, "permitir un origen que no sea loopback (requiere decisión explícita del equipo)")
	catalog := fs.Bool("catalog", false, "solo vehículos y empleados demo: sin -apply es una vista previa de solo lectura contra el destino")
	verify := fs.Bool("verify", false, "solo verificar el catálogo publicado (Solr, filtros, orden, paginado) sin escribir")
	timeout := fs.Duration("timeout", 2*time.Minute, "límite total")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || (*apply && *cleanup) || (*returnActive && !*cleanup) ||
		(*catalog && *cleanup) || (*verify && (*apply || *cleanup || *catalog)) {
		fmt.Fprintln(out, "Uso: go run ./cmd/demoseed [-apply | -cleanup [-return-active] | -catalog [-apply] | -verify] [-url http://127.0.0.1:8080] [-env .env]")
		return 2
	}
	if err := devconfig.Load(*envFile); err != nil {
		fmt.Fprintln(out, "FALLA:", err)
		return 2
	}
	base, err := gatewayURL(*target, *allowRemote)
	if err != nil {
		fmt.Fprintln(out, "FALLA:", err)
		return 2
	}
	s := &seeder{base: base, admin: os.Getenv("AUTH_ADMIN_TOKEN"), client: os.Getenv("AUTH_CLIENT_TOKEN"),
		credentialCustomer: envOr("AUTH_CLIENT_CUSTOMER_ID", "cli-001"), out: out, now: time.Now,
		http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if !*apply && !*cleanup && !*catalog && !*verify {
		s.plan()
		return 0
	}
	if s.admin == "" {
		fmt.Fprintln(out, "FALLA: falta AUTH_ADMIN_TOKEN en el entorno o en el .env")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	switch {
	case *verify:
		err = s.verifyCatalog(ctx)
	case *catalog && *apply:
		err = s.applyCatalog(ctx)
	case *catalog:
		err = s.previewCatalog(ctx)
	case *apply:
		err = s.seed(ctx)
	default:
		err = s.retire(ctx, *returnActive)
	}
	if err != nil {
		fmt.Fprintln(out, "FALLA:", err)
		return 1
	}
	if s.problems > 0 {
		fmt.Fprintf(out, "TERMINADO con %d observaciones (ver líneas AVISO).\n", s.problems)
		return 1
	}
	fmt.Fprintln(out, "TERMINADO sin observaciones.")
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func gatewayURL(value string, allowRemote bool) (string, error) {
	if value == "" {
		value = "http://127.0.0.1:" + envOr("APP_PORT", "8080")
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("-url debe ser un origen http(s) sin ruta, usuario ni parámetros")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if !(host == "localhost" || (ip != nil && ip.IsLoopback())) && !allowRemote {
		return "", errors.New("el origen no es loopback; usar -allow-remote solo con autorización explícita del equipo")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func (s *seeder) logf(format string, args ...any) { fmt.Fprintf(s.out, format+"\n", args...) }
func (s *seeder) warnf(format string, args ...any) {
	s.problems++
	fmt.Fprintf(s.out, "AVISO "+format+"\n", args...)
}

func (s *seeder) plan() {
	s.logf("Plan (sin cambios; usar -apply para cargar o -cleanup para retirar) contra %s:", s.base)
	for _, c := range customers {
		s.logf("  cliente   %-28s %-20s %s", c.ID, c.Name, c.Scenario)
	}
	for _, e := range employees {
		s.logf("  empleado  %s (activo=%t)", e.Name, e.Active)
	}
	for _, v := range vehicles {
		s.logf("  vehículo  %-22s %s %s · %s", v.ID, v.Brand, v.Model, v.Category)
	}
	for _, r := range reservations {
		s.logf("  reserva   %s en %s: %s", r.Customer, r.Vehicle, r.Scenario)
	}
	for _, r := range credentialReservations {
		s.logf("  reserva   %s (credencial demo) en %s: %s", s.credentialCustomer, r.Vehicle, r.Scenario)
	}
}

// call performs one request. The token travels only in the Authorization
// header; responses are decoded into out when the status is 2xx.
func (s *seeder) call(ctx context.Context, method, path, token string, body any, key string, out any) (int, string, error) {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, reader)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := s.http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("%s %s: sin respuesta del gateway", method, path)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var problem struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &problem)
	if res.StatusCode >= 200 && res.StatusCode < 300 && out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return res.StatusCode, "", fmt.Errorf("%s %s: respuesta ilegible", method, path)
		}
	}
	return res.StatusCode, problem.Code, nil
}

type remoteCustomer struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Active            bool   `json:"active"`
	LicenseValidUntil string `json:"licenseValidUntil"`
}
type remoteEmployee struct {
	ID          string `json:"employeeId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}
type remoteVehicle struct {
	VehicleID      string `json:"vehicleId"`
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
type remoteReservation struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	VehicleID  string `json:"vehicleId"`
	CustomerID string `json:"customerId"`
}

func (s *seeder) seed(ctx context.Context) error {
	s.logf("Cargando datos de demostración en %s", s.base)
	for _, c := range customers {
		status, code, err := s.call(ctx, "POST", "/v1/customers", s.admin, map[string]any{"id": c.ID, "name": c.Name, "active": c.Active, "licenseValidUntil": c.License}, "", nil)
		switch {
		case err != nil:
			return err
		case status == 201:
			s.logf("CREADO   cliente %s (%s)", c.ID, c.Scenario)
		case status == 409:
			var current remoteCustomer
			if st, _, err := s.call(ctx, "GET", "/v1/customers/"+url.PathEscape(c.ID), s.admin, nil, "", &current); err != nil || st != 200 {
				return fmt.Errorf("no se pudo leer el cliente existente %s", c.ID)
			}
			if current.Name != c.Name || current.Active != c.Active || current.LicenseValidUntil != c.License {
				s.warnf("cliente %s ya existe con otros datos; no se modifica", c.ID)
			} else {
				s.logf("EXISTE   cliente %s", c.ID)
			}
		default:
			return fmt.Errorf("alta del cliente %s: HTTP %d %s", c.ID, status, code)
		}
	}
	existing, err := s.employees(ctx)
	if err != nil {
		return err
	}
	for _, e := range employees {
		if current, ok := existing[e.Name]; ok {
			if current.Active != e.Active {
				s.warnf("empleado %q ya existe con activo=%t; no se modifica", e.Name, current.Active)
			} else {
				s.logf("EXISTE   empleado %s (%s)", e.Name, current.ID)
			}
			continue
		}
		var created remoteEmployee
		status, code, err := s.call(ctx, "POST", "/v1/employees", s.admin, map[string]any{"displayName": e.Name, "active": e.Active}, "", &created)
		if err != nil {
			return err
		}
		if status != 201 {
			return fmt.Errorf("alta del empleado %q: HTTP %d %s", e.Name, status, code)
		}
		s.logf("CREADO   empleado %s (%s)", e.Name, created.ID)
	}
	active := true
	for _, v := range vehicles {
		body := map[string]any{"vehicleId": v.ID, "brand": v.Brand, "model": v.Model, "category": v.Category, "transmission": v.Transmission, "seats": v.Seats, "dailyRateMinor": v.DailyRateMinor, "currency": "ARS", "active": active}
		status, code, err := s.call(ctx, "POST", "/v1/vehicles", s.admin, body, "", nil)
		switch {
		case err != nil:
			return err
		case status == 201:
			s.logf("CREADO   vehículo %s (%s %s)", v.ID, v.Brand, v.Model)
		case status == 409:
			var current remoteVehicle
			if st, _, err := s.call(ctx, "GET", "/v1/vehicles/"+url.PathEscape(v.ID), s.admin, nil, "", &current); err != nil || st != 200 {
				return fmt.Errorf("no se pudo leer el vehículo existente %s", v.ID)
			}
			if current.Brand != v.Brand || current.Model != v.Model {
				s.warnf("vehículo %s ya existe con otros datos; no se modifica", v.ID)
			} else {
				s.logf("EXISTE   vehículo %s (activo=%t)", v.ID, current.Active)
			}
		default:
			return fmt.Errorf("alta del vehículo %s: HTTP %d %s", v.ID, status, code)
		}
	}
	for _, r := range reservations {
		if err := s.reserve(ctx, r, r.Customer, s.admin); err != nil {
			return err
		}
	}
	if s.client == "" {
		s.warnf("sin AUTH_CLIENT_TOKEN: no se crean las reservas de la credencial %s", s.credentialCustomer)
	} else {
		for _, r := range credentialReservations {
			if err := s.reserve(ctx, r, s.credentialCustomer, s.client); err != nil {
				return err
			}
		}
	}
	return s.verifySearch(ctx)
}

func (s *seeder) employees(ctx context.Context) (map[string]remoteEmployee, error) {
	found := map[string]remoteEmployee{}
	for page := 1; ; page++ {
		var res struct {
			Items []remoteEmployee `json:"items"`
			Total int64            `json:"total"`
		}
		status, code, err := s.call(ctx, "GET", fmt.Sprintf("/v1/admin/employees?page=%d&pageSize=100", page), s.admin, nil, "", &res)
		if err != nil {
			return nil, err
		}
		if status != 200 {
			return nil, fmt.Errorf("lista de empleados: HTTP %d %s", status, code)
		}
		for _, e := range res.Items {
			found[e.DisplayName] = e
		}
		if len(res.Items) == 0 || int64(page*100) >= res.Total {
			return found, nil
		}
	}
}

func (s *seeder) reservationsOf(ctx context.Context, customer string) ([]remoteReservation, error) {
	var all []remoteReservation
	for page := 1; ; page++ {
		var res struct {
			Items []remoteReservation `json:"items"`
			Total int64               `json:"total"`
		}
		status, code, err := s.call(ctx, "GET", fmt.Sprintf("/v1/reservations?customerId=%s&page=%d&pageSize=50", url.QueryEscape(customer), page), s.admin, nil, "", &res)
		if err != nil {
			return nil, err
		}
		if status != 200 {
			return nil, fmt.Errorf("reservas de %s: HTTP %d %s (¿Alquileres actualizado con GET /v1/reservations?)", customer, status, code)
		}
		all = append(all, res.Items...)
		if len(res.Items) == 0 || int64(page*50) >= res.Total {
			return all, nil
		}
	}
}

// reserve creates the scenario once: an existing RESERVED/ACTIVE reservation of
// the same driver and vehicle means it is already loaded. Quotes come from
// Rentals; the total is never computed here.
func (s *seeder) reserve(ctx context.Context, r reservation, customer, token string) error {
	current, err := s.reservationsOf(ctx, customer)
	if err != nil {
		return err
	}
	for _, c := range current {
		if c.VehicleID == r.Vehicle && (c.Status == "RESERVED" || c.Status == "ACTIVE") {
			if r.Pickup && c.Status == "RESERVED" {
				return s.pickup(ctx, c.ID, r)
			}
			s.logf("EXISTE   %s: %s (%s)", r.Scenario, c.ID, c.Status)
			return nil
		}
	}
	branch, _ := time.LoadLocation("America/Buenos_Aires")
	today := s.now().In(branch)
	start := today.AddDate(0, 0, r.StartInDays).Format("2006-01-02")
	end := today.AddDate(0, 0, r.StartInDays+r.Days).Format("2006-01-02")
	var quote struct {
		Available  bool  `json:"available"`
		TotalMinor int64 `json:"totalMinor"`
	}
	q := url.Values{"vehicleId": {r.Vehicle}, "startsOn": {start}, "endsOn": {end}}
	if status, code, err := s.call(ctx, "GET", "/v1/availability?"+q.Encode(), s.admin, nil, "", &quote); err != nil || status != 200 {
		if err == nil {
			err = fmt.Errorf("disponibilidad de %s: HTTP %d %s", r.Vehicle, status, code)
		}
		return err
	}
	if !quote.Available {
		s.warnf("%s: %s no está disponible del %s al %s; se omite", r.Scenario, r.Vehicle, start, end)
		return nil
	}
	var created remoteReservation
	key := fmt.Sprintf("demo-seed-%s-%s-%s", customer, r.Vehicle, start)
	status, code, err := s.call(ctx, "POST", "/v1/reservations", token, map[string]any{"vehicleId": r.Vehicle, "customerId": customer, "startsOn": start, "endsOn": end, "expectedTotalMinor": quote.TotalMinor}, key, &created)
	if err != nil {
		return err
	}
	if status != 201 {
		s.warnf("%s para %s: HTTP %d %s; se omite", r.Scenario, customer, status, code)
		return nil
	}
	s.logf("CREADO   %s: %s del %s al %s", r.Scenario, created.ID, start, end)
	if r.Pickup {
		return s.pickup(ctx, created.ID, r)
	}
	return nil
}

func (s *seeder) pickup(ctx context.Context, id string, r reservation) error {
	status, code, err := s.call(ctx, "POST", "/v1/reservations/"+url.PathEscape(id)+"/pickup", s.admin, nil, "", nil)
	if err != nil {
		return err
	}
	if status != 200 {
		s.warnf("retiro de %s (%s): HTTP %d %s", id, r.Scenario, status, code)
		return nil
	}
	s.logf("RETIRO   %s: %s queda ACTIVE", r.Scenario, id)
	return nil
}

// verifySearch waits for the indexer: the vehicles must be findable through
// the same Solr search the catalog uses.
func (s *seeder) verifySearch(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		var page struct {
			Total int64 `json:"total"`
		}
		status, _, err := s.call(ctx, "GET", "/v1/vehicles/search?q=Kangoo&page=1&pageSize=6", s.admin, nil, "", &page)
		if err == nil && status == 200 && page.Total > 0 {
			s.logf("BÚSQUEDA los vehículos demo ya figuran en el índice")
			return nil
		}
		if time.Now().After(deadline) {
			s.warnf("la búsqueda todavía no muestra los vehículos demo; revisar el indexador o ejecutar POST /v1/vehicles/reindex")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *seeder) retire(ctx context.Context, returnActive bool) error {
	s.logf("Retirando datos de demostración en %s (no se borra nada)", s.base)
	drivers := []string{s.credentialCustomer}
	for _, c := range customers {
		drivers = append(drivers, c.ID)
	}
	for _, customer := range drivers {
		current, err := s.reservationsOf(ctx, customer)
		if err != nil {
			return err
		}
		for _, r := range current {
			if !strings.HasPrefix(r.VehicleID, "demo-") {
				continue
			}
			switch {
			case r.Status == "RESERVED":
				if status, code, err := s.call(ctx, "DELETE", "/v1/reservations/"+url.PathEscape(r.ID), s.admin, nil, "", nil); err != nil || status != 200 {
					s.warnf("no se pudo cancelar %s: HTTP %d %s", r.ID, status, code)
				} else {
					s.logf("CANCELADA reserva %s", r.ID)
				}
			case r.Status == "ACTIVE" && returnActive:
				if status, code, err := s.call(ctx, "POST", "/v1/reservations/"+url.PathEscape(r.ID)+"/return", s.admin, nil, "", nil); err != nil || status != 200 {
					s.warnf("no se pudo registrar la devolución de %s: HTTP %d %s", r.ID, status, code)
				} else {
					s.logf("DEVUELTO alquiler %s", r.ID)
				}
			case r.Status == "ACTIVE":
				s.warnf("alquiler activo %s en %s: se conserva (usar -return-active para registrar la devolución)", r.ID, r.VehicleID)
			}
		}
	}
	for _, v := range vehicles {
		var current remoteVehicle
		status, _, err := s.call(ctx, "GET", "/v1/vehicles/"+url.PathEscape(v.ID), s.admin, nil, "", &current)
		if err != nil {
			return err
		}
		if status == 404 || (status == 200 && !current.Active) {
			continue
		}
		if status != 200 {
			s.warnf("no se pudo leer %s: HTTP %d", v.ID, status)
			continue
		}
		off := false
		body := map[string]any{"brand": current.Brand, "model": current.Model, "category": current.Category, "transmission": current.Transmission, "seats": current.Seats, "dailyRateMinor": current.DailyRateMinor, "currency": current.Currency, "active": off, "version": current.Version}
		if st, code, err := s.call(ctx, "PUT", "/v1/vehicles/"+url.PathEscape(v.ID), s.admin, body, "", nil); err != nil || st != 200 {
			s.warnf("no se pudo desactivar %s: HTTP %d %s", v.ID, st, code)
		} else {
			s.logf("INACTIVO vehículo %s", v.ID)
		}
	}
	existing, err := s.employees(ctx)
	if err != nil {
		return err
	}
	for _, e := range employees {
		current, ok := existing[e.Name]
		if !ok || !current.Active {
			continue
		}
		if st, code, err := s.call(ctx, "DELETE", "/v1/employees/"+url.PathEscape(current.ID), s.admin, nil, "", nil); err != nil || st != 200 {
			s.warnf("no se pudo dar de baja %q: HTTP %d %s", e.Name, st, code)
		} else {
			s.logf("BAJA     empleado %s", e.Name)
		}
	}
	s.logf("Los clientes demo-cli-* se conservan: Clientes no expone baja de conductores. Quedan inertes y sin reservas abiertas.")
	return nil
}

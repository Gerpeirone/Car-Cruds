package main

// Catalog mode (-catalog): only the demo vehicles and employees. It never
// touches customers, reservations, Hospital or Gimnasio, and it never edits
// or deactivates an existing record. Without -apply it is a read-only preview
// of what would be created; -verify checks the published catalog (Solr
// search, filters, ordering and paging) without writing anything.

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// catalogPlan is the difference between the demo data and the target.
type catalogPlan struct {
	vehicles        []vehicle
	employees       []employee
	currentVehicles int
	currentStaff    int
}

// sameKey compares names the way a person would: letter case and surrounding
// or repeated spaces do not make a different model or employee.
func sameKey(parts ...string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(parts, " ")), " "))
}

// allVehicles reads the whole fleet, inactive vehicles included, from the
// source of truth (Flota/MongoDB), not from the search index.
func (s *seeder) allVehicles(ctx context.Context) ([]remoteVehicle, error) {
	var all []remoteVehicle
	for page := 1; ; page++ {
		var res struct {
			Items []remoteVehicle `json:"items"`
			Total int64           `json:"total"`
		}
		status, code, err := s.call(ctx, "GET", fmt.Sprintf("/v1/vehicles?includeInactive=true&page=%d&pageSize=100", page), s.admin, nil, "", &res)
		if err != nil {
			return nil, err
		}
		if status != 200 {
			return nil, fmt.Errorf("lista de vehículos: HTTP %d %s", status, code)
		}
		all = append(all, res.Items...)
		if len(res.Items) == 0 || int64(page*100) >= res.Total {
			return all, nil
		}
	}
}

// planCatalog decides, record by record, what is missing. A demo vehicle is
// skipped when its identifier exists or when another vehicle already has the
// same brand and model; an employee is skipped when the same name exists.
func (s *seeder) planCatalog(ctx context.Context) (catalogPlan, error) {
	current, err := s.allVehicles(ctx)
	if err != nil {
		return catalogPlan{}, err
	}
	byID := map[string]remoteVehicle{}
	byModel := map[string]remoteVehicle{}
	for _, v := range current {
		byID[v.VehicleID] = v
		byModel[sameKey(v.Brand, v.Model)] = v
	}
	p := catalogPlan{currentVehicles: len(current)}
	for _, v := range vehicles {
		key := sameKey(v.Brand, v.Model)
		if cur, ok := byID[v.ID]; ok {
			if sameKey(cur.Brand, cur.Model) == key {
				s.logf("EXISTE   vehículo %-24s %s %s (activo=%t)", v.ID, v.Brand, v.Model, cur.Active)
			} else {
				s.warnf("vehículo %s ya existe como %s %s; no se modifica", v.ID, cur.Brand, cur.Model)
			}
			continue
		}
		if other, ok := byModel[key]; ok {
			s.logf("OMITIDO  vehículo %-24s %s %s ya existe como %s; no se duplica", v.ID, v.Brand, v.Model, other.VehicleID)
			continue
		}
		s.logf("CREAR    vehículo %-24s %s %s · %s · %s · %d plazas · $ %d/día", v.ID, v.Brand, v.Model, v.Category, v.Transmission, v.Seats, v.DailyRateMinor/100)
		p.vehicles = append(p.vehicles, v)
	}
	staff, err := s.employees(ctx)
	if err != nil {
		return catalogPlan{}, err
	}
	p.currentStaff = len(staff)
	names := map[string]remoteEmployee{}
	for _, e := range staff {
		names[sameKey(e.DisplayName)] = e
	}
	for _, e := range employees {
		if cur, ok := names[sameKey(e.Name)]; ok {
			s.logf("EXISTE   empleado %s (%s, activo=%t)", e.Name, cur.ID, cur.Active)
			continue
		}
		s.logf("CREAR    empleado %s (activo=%t)", e.Name, e.Active)
		p.employees = append(p.employees, e)
	}
	return p, nil
}

func (s *seeder) previewCatalog(ctx context.Context) error {
	s.logf("Vista previa del catálogo demo en %s (solo lectura: no se crea ni modifica nada)", s.base)
	p, err := s.planCatalog(ctx)
	if err != nil {
		return err
	}
	s.logf("RESUMEN  vehículos: %d existentes, %d a crear · empleados: %d existentes, %d a crear", p.currentVehicles, len(p.vehicles), p.currentStaff, len(p.employees))
	s.logf("RESUMEN  sin clientes, reservas, alquileres, turnos ni membresías. Para cargar: agregar -apply")
	return nil
}

func (s *seeder) applyCatalog(ctx context.Context) error {
	s.logf("Cargando solo vehículos y empleados demo en %s", s.base)
	p, err := s.planCatalog(ctx)
	if err != nil {
		return err
	}
	for _, v := range p.vehicles {
		body := map[string]any{"vehicleId": v.ID, "brand": v.Brand, "model": v.Model, "category": v.Category, "transmission": v.Transmission, "seats": v.Seats, "dailyRateMinor": v.DailyRateMinor, "currency": "ARS", "active": true}
		status, code, err := s.call(ctx, "POST", "/v1/vehicles", s.admin, body, "", nil)
		switch {
		case err != nil:
			return err
		case status == 201:
			s.logf("CREADO   vehículo %s (%s %s)", v.ID, v.Brand, v.Model)
		case status == 409:
			s.logf("EXISTE   vehículo %s (creado por otra ejecución)", v.ID)
		default:
			return fmt.Errorf("alta del vehículo %s: HTTP %d %s", v.ID, status, code)
		}
	}
	for _, e := range p.employees {
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
	s.logf("RESUMEN  %d vehículos y %d empleados creados", len(p.vehicles), len(p.employees))
	return s.verifyCatalog(ctx)
}

// indexWait bounds how long the verification waits for the event-driven
// indexer to publish recent changes to Solr.
var indexWait = 60 * time.Second

type searchPage struct {
	Items    []remoteVehicle `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

func (s *seeder) search(ctx context.Context, q url.Values) (searchPage, error) {
	var page searchPage
	status, code, err := s.call(ctx, "GET", "/v1/vehicles/search?"+q.Encode(), s.admin, nil, "", &page)
	if err == nil && status != 200 {
		err = fmt.Errorf("búsqueda %s: HTTP %d %s", q.Encode(), status, code)
	}
	return page, err
}

// verifyCatalog checks, read-only, the catalog as the public site sees it:
// every active vehicle of Flota is in the Solr index (waiting for the event
// driven indexer), and text search, filters, ordering and paging behave.
func (s *seeder) verifyCatalog(ctx context.Context) error {
	s.logf("Verificando el catálogo publicado (solo lectura)")
	all, err := s.allVehicles(ctx)
	if err != nil {
		return err
	}
	active := map[string]remoteVehicle{}
	models := map[string]bool{}
	brands := map[string]bool{}
	for _, v := range all {
		if v.Active {
			active[v.VehicleID] = v
			models[sameKey(v.Brand, v.Model)] = true
			brands[v.Brand] = true
		}
	}
	// 1. Index in sync with Flota: same active total and every active vehicle
	//    findable by its brand filter.
	deadline := time.Now().Add(indexWait)
	for {
		missing, err := s.missingFromIndex(ctx, active, brands)
		if err != nil {
			return err
		}
		if len(missing) == 0 {
			s.logf("VERIFICADO índice Solr: los %d vehículos activos de Flota figuran en la búsqueda", len(active))
			break
		}
		if time.Now().After(deadline) {
			s.warnf("índice Solr: faltan %d vehículos (%s); revisar el indexador o POST /v1/vehicles/reindex", len(missing), strings.Join(missing, ", "))
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if len(models) >= 14 {
		s.logf("VERIFICADO catálogo: %d modelos distintos de %d marcas", len(models), len(brands))
	} else {
		s.warnf("catálogo: solo %d modelos distintos activos (se esperaban al menos 14)", len(models))
	}
	// 2. Text search through Solr.
	if page, err := s.search(ctx, url.Values{"q": {"Kangoo"}, "pageSize": {"20"}}); err != nil {
		return err
	} else if !slices.ContainsFunc(page.Items, func(v remoteVehicle) bool { return v.Model == "Kangoo" }) {
		s.warnf("búsqueda de texto: «Kangoo» sin resultados")
	} else {
		s.logf("VERIFICADO búsqueda de texto: «Kangoo» → %d resultado(s)", page.Total)
	}
	// 3. Filters: every result matches.
	for _, f := range []struct{ key, value string }{{"brand", "Toyota"}, {"category", "suv"}, {"transmission", "automatic"}, {"seats", "7"}} {
		page, err := s.search(ctx, url.Values{f.key: {f.value}, "pageSize": {"100"}})
		if err != nil {
			return err
		}
		ok := page.Total > 0 && int64(len(page.Items)) == page.Total
		for _, v := range page.Items {
			ok = ok && map[string]bool{"brand": v.Brand == f.value, "category": v.Category == f.value, "transmission": v.Transmission == f.value, "seats": fmt.Sprint(v.Seats) == f.value}[f.key]
		}
		if ok {
			s.logf("VERIFICADO filtro %s=%s → %d vehículos, todos coinciden", f.key, f.value, page.Total)
		} else {
			s.warnf("filtro %s=%s: resultados vacíos o que no coinciden", f.key, f.value)
		}
	}
	// 4. Ordering by price, both directions.
	for _, order := range []string{"price_asc", "price_desc"} {
		page, err := s.search(ctx, url.Values{"sort": {order}, "pageSize": {"100"}})
		if err != nil {
			return err
		}
		sorted := slices.IsSortedFunc(page.Items, func(a, b remoteVehicle) int {
			if order == "price_desc" {
				a, b = b, a
			}
			return int(min(max(a.DailyRateMinor-b.DailyRateMinor, -1), 1))
		})
		if sorted && len(page.Items) > 1 {
			s.logf("VERIFICADO orden %s: %d vehículos, de $ %d a $ %d por día", order, len(page.Items), page.Items[0].DailyRateMinor/100, page.Items[len(page.Items)-1].DailyRateMinor/100)
		} else {
			s.warnf("orden %s: resultado desordenado o insuficiente", order)
		}
	}
	// 5. Paging: two consecutive pages, disjoint, with a stable total.
	first, err := s.search(ctx, url.Values{"page": {"1"}, "pageSize": {"6"}})
	if err != nil {
		return err
	}
	second, err := s.search(ctx, url.Values{"page": {"2"}, "pageSize": {"6"}})
	if err != nil {
		return err
	}
	overlap := slices.ContainsFunc(second.Items, func(v remoteVehicle) bool {
		return slices.ContainsFunc(first.Items, func(w remoteVehicle) bool { return w.VehicleID == v.VehicleID })
	})
	if first.Total > 6 && len(first.Items) == 6 && len(second.Items) > 0 && !overlap && first.Total == second.Total {
		s.logf("VERIFICADO paginado: %d vehículos en %d páginas de 6, sin repetidos", first.Total, (first.Total+5)/6)
	} else {
		s.warnf("paginado: páginas 1 y 2 inconsistentes (total %d/%d, %d+%d ítems)", first.Total, second.Total, len(first.Items), len(second.Items))
	}
	staff, err := s.employees(ctx)
	if err != nil {
		return err
	}
	demo, on := 0, 0
	for _, e := range staff {
		if strings.HasPrefix(e.DisplayName, "Demo ·") {
			demo++
			if e.Active {
				on++
			}
		}
	}
	s.logf("VERIFICADO panel de empleados: %d empleados demo (%d activos) de %d en total", demo, on, len(staff))
	return nil
}

func (s *seeder) missingFromIndex(ctx context.Context, active map[string]remoteVehicle, brands map[string]bool) ([]string, error) {
	indexed := map[string]bool{}
	for brand := range brands {
		page, err := s.search(ctx, url.Values{"brand": {brand}, "pageSize": {"100"}})
		if err != nil {
			return nil, err
		}
		for _, v := range page.Items {
			indexed[v.VehicleID] = true
		}
	}
	var missing []string
	for id := range active {
		if !indexed[id] {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)
	return missing, nil
}

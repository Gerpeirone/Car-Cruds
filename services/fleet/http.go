package fleet

import (
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"net/http"
	"strconv"
	"strings"
)

func Handler(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/vehicles", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "client", "service") {
			return
		}
		q, err := readQuery(r)
		if err != nil {
			respondError(w, err)
			return
		}
		q.IncludeInactive = r.URL.Query().Get("includeInactive") == "true" && platform.PrincipalFrom(r.Context()).Role == "admin"
		result, err := service.List(r.Context(), q)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /v1/vehicles/search", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "client", "service") {
			return
		}
		q, err := readQuery(r)
		if err != nil {
			respondError(w, err)
			return
		}
		result, err := service.Find(r.Context(), q)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /v1/vehicles/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "client", "service") {
			return
		}
		v, err := service.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /v1/vehicles/{id}/quote", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		v, err := service.Quote(r.Context(), r.PathValue("id"))
		if err != nil {
			respondError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		platform.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("POST /v1/vehicles", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		var input Input
		if err := platform.Decode(w, r, &input); err != nil {
			platform.Error(w, 400, "INVALID_JSON", "Datos de vehículo inválidos")
			return
		}
		v, err := service.Create(r.Context(), input)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusCreated, v)
	})
	mux.HandleFunc("PUT /v1/vehicles/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		var input Input
		if err := platform.Decode(w, r, &input); err != nil {
			platform.Error(w, 400, "INVALID_JSON", "Datos de vehículo inválidos")
			return
		}
		v, err := service.Update(r.Context(), r.PathValue("id"), input)
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("POST /v1/vehicles/reindex", func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "service") {
			return
		}
		count, err := service.Rebuild(r.Context())
		if err != nil {
			respondError(w, err)
			return
		}
		platform.WriteJSON(w, http.StatusAccepted, map[string]any{"status": "pending", "enqueued": count})
	})
	return mux
}
func readQuery(r *http.Request) (Query, error) {
	values := r.URL.Query()
	q := Query{Page: 1, PageSize: 20, Brand: strings.TrimSpace(values.Get("brand")), Category: strings.TrimSpace(values.Get("category")), Transmission: values.Get("transmission"), Sort: values.Get("sort"), Text: strings.TrimSpace(values.Get("q"))}
	var err error
	if values.Has("page") {
		q.Page, err = strconv.Atoi(values.Get("page"))
		if err != nil {
			return q, ErrInvalid
		}
	}
	if values.Has("pageSize") {
		q.PageSize, err = strconv.Atoi(values.Get("pageSize"))
		if err != nil {
			return q, ErrInvalid
		}
	}
	if values.Has("seats") {
		q.Seats, err = strconv.Atoi(values.Get("seats"))
		if err != nil {
			return q, ErrInvalid
		}
	}
	return q, q.Validate()
}
func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		platform.Error(w, 400, "INVALID_VEHICLE", "Vehículo o consulta inválidos")
	case errors.Is(err, ErrNotFound):
		platform.Error(w, 404, "VEHICLE_NOT_FOUND", "Vehículo inexistente")
	case errors.Is(err, ErrConflict):
		platform.Error(w, 409, "VERSION_CONFLICT", "El vehículo cambió o ya existe; consultar su versión")
	case errors.Is(err, ErrOutboxFull):
		platform.Error(w, 503, "OUTBOX_FULL", "Cambios pendientes de publicar; reintentar más adelante")
	case errors.Is(err, ErrBusy):
		platform.Error(w, 503, "SOURCE_BUSY", "Capacidad de lectura temporalmente agotada")
	case errors.Is(err, ErrSearchUnavailable):
		platform.Error(w, 503, "SEARCH_UNAVAILABLE", "Búsqueda temporalmente no disponible")
	default:
		platform.Error(w, 503, "FLEET_UNAVAILABLE", "Flota temporalmente no disponible")
	}
}

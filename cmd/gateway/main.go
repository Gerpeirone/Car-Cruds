package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
)

func proxy(target string) (http.Handler, error) {
	base, err := url.Parse(target)
	if err != nil || base.Host == "" || base.Scheme != "http" || base.User != nil {
		return nil, errors.New("URL de servicio inválida")
	}
	p := httputil.NewSingleHostReverseProxy(base)
	p.Transport = platform.NewHTTPClient(5 * time.Second).Transport
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Service-Instance", platform.Env("INSTANCE_ID", "gateway"))
		platform.Error(w, 503, "SERVICE_UNAVAILABLE", "El servicio requerido no está disponible")
	}
	p.ModifyResponse = func(resp *http.Response) error {
		if resp.Request != nil {
			resp.Header.Set("X-Request-ID", resp.Request.Header.Get("X-Request-ID"))
		}
		if resp.StatusCode == 502 || resp.StatusCode == 504 {
			_ = resp.Body.Close()
			body := `{"code":"SERVICE_UNAVAILABLE","message":"Dependencia temporalmente no disponible"}`
			resp.StatusCode = 503
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Set("Cache-Control", "no-store")
			resp.Header.Set("X-Service-Instance", platform.Env("INSTANCE_ID", "gateway"))
			resp.Body = io.NopCloser(strings.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Set("Content-Length", "")
			resp.Header.Del("Content-Length")
		}
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ReverseProxy agrega encabezados del backend. Se retiran los valores
		// locales para que la respuesta identifique una sola instancia atendida.
		w.Header().Del("X-Service-Instance")
		w.Header().Del("X-Request-ID")
		p.ServeHTTP(w, r)
	}), nil
}

// maxInFlight lee el límite de admisión del gateway (D13). Se mantiene 64 por
// defecto para conservar la línea base medida.
func maxInFlight() (int, error) {
	raw := platform.Env("GATEWAY_MAX_IN_FLIGHT", "64")
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 4096 {
		return 0, errors.New("GATEWAY_MAX_IN_FLIGHT debe ser un entero entre 1 y 4096")
	}
	return limit, nil
}

func handler() (http.Handler, func(context.Context) error, error) {
	mux := http.NewServeMux()
	services := map[string]string{"customers": platform.Env("CUSTOMERS_URL", "http://customers:8080"), "fleet": platform.Env("FLEET_URL", "http://fleet-lb:8080"), "rentals": platform.Env("RENTALS_URL", "http://rentals:8080"), "employees": platform.Env("EMPLOYEES_URL", "http://employees:8080")}
	routes := map[string]string{"/v1/customers": "customers", "/v1/vehicles": "fleet", "/v1/reservations": "rentals", "/v1/availability": "rentals", "/v1/maintenance": "rentals", "/v1/employees": "employees", "/v1/admin/employees": "employees"}
	for path, name := range routes {
		p, err := proxy(services[name])
		if err != nil {
			return nil, nil, err
		}
		mux.Handle(path, p)
		mux.Handle(path+"/", p)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		platform.Error(w, 404, "NOT_FOUND", "Ruta no disponible")
	})
	limit, err := maxInFlight()
	if err != nil {
		return nil, nil, err
	}
	slots := make(chan struct{}, limit)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			mux.ServeHTTP(w, r)
		default:
			platform.Error(w, 503, "SERVICE_UNAVAILABLE", "Capacidad de entrada temporalmente agotada")
		}
	})
	ready := func(ctx context.Context) error {
		client := platform.NewHTTPClient(time.Second)
		var group sync.WaitGroup
		failures := make(chan error, len(services))
		for _, address := range services {
			group.Add(1)
			go func(address string) {
				defer group.Done()
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/internal/ready", nil)
				if err != nil {
					failures <- err
					return
				}
				request.Header.Set("Authorization", "Bearer "+os.Getenv("AUTH_SERVICE_TOKEN"))
				response, err := client.Do(request)
				if err != nil {
					failures <- errors.New("servicio requerido no disponible")
					return
				}
				response.Body.Close()
				if response.StatusCode != 200 {
					failures <- errors.New("servicio requerido no disponible")
					return
				}
			}(address)
		}
		group.Wait()
		close(failures)
		for err := range failures {
			return err
		}
		return nil
	}
	return h, ready, nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	h, ready, err := handler()
	if err == nil {
		err = platform.Run(ctx, "gateway", h, ready)
	}
	if err != nil {
		slog.Error("gateway_stopped", "error", err)
		os.Exit(1)
	}
}

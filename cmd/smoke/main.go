// smoke exercises the running system exclusively through its local gateway.
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
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/Gerpeirone/Car-Cruds/internal/devconfig"
	"github.com/google/uuid"
)

type settings struct {
	gateway, admin, client, gym string
	timeout                     time.Duration
	history                     bool
}

type response struct {
	body                []byte
	requestID, instance string
	status              int
}

type harness struct {
	settings
	clientHTTP    *http.Client
	outputMu      sync.Mutex
	runID         string
	vehicleID     string
	vehicle       vehicle
	vehicleIntent bool
	employeeID    string
	candidateID   string
	customerID    string
	intents       []reservationIntent
	completedID   string
	requestCount  int
}

func main() {
	os.Exit(mainExit())
}

func mainExit() int {
	envFile := flag.String("env", ".env", "archivo local de configuración; vacío omite su lectura")
	gateway := flag.String("gateway", "", "URL del gateway local (también SMOKE_GATEWAY_URL)")
	timeout := flag.Duration("timeout", 120*time.Second, "presupuesto total de escenarios, sin incluir limpieza")
	history := flag.Bool("history", false, "verificar outbox e historial con PostgreSQL local, sólo lectura")
	flag.Parse()
	if err := devconfig.Load(*envFile); err != nil {
		fmt.Printf("FAIL configuración: %s\n", err)
		return 2
	}
	address := *gateway
	if address == "" {
		address = os.Getenv("SMOKE_GATEWAY_URL")
	}
	if address == "" {
		port := os.Getenv("APP_PORT")
		if port == "" {
			port = "8080"
		}
		address = "http://127.0.0.1:" + port
	}
	base, err := localURL(address, "http", "https")
	if err != nil || *timeout <= 0 {
		fmt.Println("FAIL configuración: gateway local o presupuesto inválido")
		return 2
	}
	cfg := settings{gateway: strings.TrimRight(base.String(), "/"), admin: os.Getenv("AUTH_ADMIN_TOKEN"), client: os.Getenv("AUTH_CLIENT_TOKEN"), gym: os.Getenv("AUTH_GYM_TOKEN"), timeout: *timeout, history: *history}
	if len(cfg.admin) < 24 || len(cfg.client) < 24 || len(cfg.gym) < 24 || cfg.admin == cfg.client || cfg.admin == cfg.gym || cfg.client == cfg.gym {
		fmt.Println("FAIL configuración: se requieren credenciales diferentes para admin, client y gym")
		return 2
	}
	if cfg.history {
		for _, service := range []string{"customers", "rentals"} {
			if _, err := localDatabaseURL(service); err != nil {
				fmt.Printf("FAIL configuración: %s\n", err)
				return 2
			}
		}
	}
	runID := strings.ReplaceAll(uuid.NewString(), "-", "")
	h := &harness{settings: cfg, runID: runID, vehicleID: "smoke-" + runID, customerID: "smoke-cli-" + runID,
		clientHTTP: &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, MaxIdleConnsPerHost: 8}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}}
	defer h.clientHTTP.CloseIdleConnections()
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, cfg.timeout)
	defer cancel()
	h.log("RUN smoke=%s vehículo=%s cliente=%s", runID, h.vehicleID, h.customerID)
	err = h.run(ctx)
	if err != nil {
		h.log("FAIL escenario: %s", err)
	}
	// Cleanup has a separate bounded context so Ctrl+C or a scenario deadline
	// still releases this run's reservations and disables its own fixtures.
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 45*time.Second)
	cleanErr := h.cleanup(cleanCtx)
	cleanCancel()
	if cleanErr != nil {
		h.log("FAIL limpieza: %s; conservar identificador smoke=%s para recuperar fixtures", cleanErr, runID)
	}
	if err != nil || cleanErr != nil {
		h.log("FAIL smoke=%s solicitudes=%d", runID, h.requestCount)
		return 1
	}
	h.log("PASS smoke=%s solicitudes=%d reservas liberadas y fixtures desactivadas", runID, h.requestCount)
	return 0
}

func localURL(value string, schemes ...string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("URL local inválida")
	}
	allowed := false
	for _, scheme := range schemes {
		allowed = allowed || u.Scheme == scheme
	}
	ip := net.ParseIP(u.Hostname())
	if !allowed || (u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) {
		return nil, errors.New("URL local inválida")
	}
	return u, nil
}

func (h *harness) log(format string, args ...any) {
	h.outputMu.Lock()
	defer h.outputMu.Unlock()
	fmt.Printf(format+"\n", args...)
}

var safeCode = regexp.MustCompile(`^[A-Z_]{1,80}$`)

func (h *harness) request(ctx context.Context, name, method, path, token string, input any, key string, want int, out any) (response, error) {
	id := uuid.NewString()
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return response{}, errors.New("fixture JSON inválida")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, h.gateway+path, bytes.NewReader(body))
	if err != nil {
		return response{}, errors.New("ruta de escenario inválida")
	}
	req.Header.Set("X-Request-ID", id)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	h.outputMu.Lock()
	h.requestCount++
	h.outputMu.Unlock()
	resp, err := h.clientHTTP.Do(req)
	if err != nil {
		h.log("FAIL %s requestID=%s transporte no disponible", name, id)
		return response{requestID: id}, errors.New("solicitud al gateway fallida; consultar su requestID")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	result := response{body: raw, requestID: id, instance: resp.Header.Get("X-Service-Instance"), status: resp.StatusCode}
	if err != nil || len(raw) > 1<<20 {
		h.log("FAIL %s requestID=%s respuesta inválida", name, id)
		return result, errors.New("respuesta ilegible o demasiado grande")
	}
	statusMatches := resp.StatusCode == want || (want == 0 && (resp.StatusCode == 201 || resp.StatusCode == 409)) || (want == -1 && (resp.StatusCode == 200 || resp.StatusCode == 404))
	if !statusMatches {
		var problem struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &problem)
		if !safeCode.MatchString(problem.Code) {
			problem.Code = "UNEXPECTED_RESPONSE"
		}
		h.log("FAIL %s status=%d esperado=%d code=%s requestID=%s", name, resp.StatusCode, want, problem.Code, id)
		return result, errors.New("estado HTTP inesperado en " + name)
	}
	if len(resp.Header.Values("X-Request-ID")) != 1 || resp.Header.Get("X-Request-ID") != id {
		h.log("FAIL %s requestID=%s correlación no preservada", name, id)
		return result, errors.New("correlación de solicitud no preservada")
	}
	if len(resp.Header.Values("X-Service-Instance")) != 1 || result.instance == "" {
		return result, errors.New("identidad de instancia ausente o ambigua en " + name)
	}
	if want != http.StatusNoContent {
		if !json.Valid(raw) {
			return result, errors.New("respuesta JSON inválida en " + name)
		}
		if out != nil && json.Unmarshal(raw, out) != nil {
			return result, errors.New("contrato de respuesta inválido en " + name)
		}
	}
	h.log("PASS %s status=%d requestID=%s", name, resp.StatusCode, id)
	return result, nil
}

func codeIs(result response, expected string) error {
	var value struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(result.body, &value) != nil || value.Code != expected {
		return errors.New("código de negocio inesperado; se esperaba " + expected)
	}
	return nil
}

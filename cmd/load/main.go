// Herramienta local de carga de lectura. No genera escrituras ni carga a terceros.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/devconfig"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
)

type sample struct {
	latency time.Duration
	status  int
	invalid bool
}
type stage struct {
	Concurrency     int            `json:"concurrency"`
	Seconds         float64        `json:"elapsedSeconds"`
	Requests        int            `json:"requests"`
	RPS             float64        `json:"requestsPerSecond"`
	P50             float64        `json:"p50Ms"`
	P95             float64        `json:"p95Ms"`
	P99             float64        `json:"p99Ms"`
	SuccessfulRPS   float64        `json:"successfulRequestsPerSecond"`
	SuccessfulP95   float64        `json:"successfulP95Ms"`
	Statuses        map[string]int `json:"httpStatuses"`
	TransportErrors int            `json:"transportErrors"`
	InvalidJSON     int            `json:"invalidJSON"`
}
type report struct {
	StartedAt    string  `json:"startedAt"`
	Revision     string  `json:"revision"`
	Dirty        bool    `json:"workingTreeDirty"`
	Go           string  `json:"goVersion"`
	OS           string  `json:"generatorOS"`
	CPUs         int     `json:"generatorLogicalCPUs"`
	Route        string  `json:"route"`
	Role         string  `json:"role"`
	Model        string  `json:"model"`
	StageSeconds float64 `json:"requestedStageSeconds"`
	Stages       []stage `json:"stages"`
}

func percentile(values []time.Duration, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(len(values)))) - 1
	if i < 0 {
		i = 0
	}
	return float64(values[i].Microseconds()) / 1000
}

func measure(ctx context.Context, target, token string, concurrency int, duration time.Duration) stage {
	transport := &http.Transport{MaxIdleConns: concurrency, MaxIdleConnsPerHost: concurrency, MaxConnsPerHost: concurrency, IdleConnTimeout: time.Minute}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	results := make(chan sample, concurrency*2)
	var workers sync.WaitGroup
	started := time.Now()
	until := started.Add(duration)
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for time.Now().Before(until) && ctx.Err() == nil {
				r, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
				if err != nil {
					return
				}
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("X-Request-ID", "load-"+platform.UUID())
				begin := time.Now()
				response, err := client.Do(r)
				result := sample{latency: time.Since(begin)}
				if err == nil {
					body, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
					response.Body.Close()
					result.latency = time.Since(begin)
					result.status = response.StatusCode
					result.invalid = readErr != nil || len(body) > 1<<20 || !json.Valid(body)
				}
				results <- result
			}
		}()
	}
	go func() { workers.Wait(); close(results) }()
	out := stage{Concurrency: concurrency, Statuses: map[string]int{}}
	latencies := make([]time.Duration, 0, 10000)
	successLatencies := make([]time.Duration, 0, 10000)
	for result := range results {
		out.Requests++
		latencies = append(latencies, result.latency)
		if result.status == 200 && !result.invalid {
			successLatencies = append(successLatencies, result.latency)
		}
		if result.status == 0 {
			out.TransportErrors++
		} else {
			out.Statuses[strconv.Itoa(result.status)]++
		}
		if result.invalid {
			out.InvalidJSON++
		}
	}
	out.Seconds = time.Since(started).Seconds()
	out.RPS = float64(out.Requests) / out.Seconds
	out.SuccessfulRPS = float64(len(successLatencies)) / out.Seconds
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	sort.Slice(successLatencies, func(i, j int) bool { return successLatencies[i] < successLatencies[j] })
	out.SuccessfulP95 = percentile(successLatencies, .95)
	out.P50, out.P95, out.P99 = percentile(latencies, .5), percentile(latencies, .95), percentile(latencies, .99)
	return out
}

func run() error {
	gateway := flag.String("gateway", "", "URL local del gateway")
	path := flag.String("path", "", "Ruta GET; predeterminado: disponibilidad para mañana")
	role := flag.String("role", "client", "client, gym o admin")
	levels := flag.String("concurrency", "1,8,32,64,128", "Concurrencias por etapa, entre 1 y 256")
	duration := flag.Duration("duration", 5*time.Second, "Duración de cada etapa, entre 1s y 30s")
	flag.Parse()
	if err := devconfig.Load(".env"); err != nil {
		return err
	}
	if *gateway == "" {
		*gateway = "http://127.0.0.1:" + platform.Env("APP_PORT", "8080")
	}
	base, err := url.Parse(*gateway)
	if err != nil || base.User != nil || base.Scheme != "http" || (base.Hostname() != "localhost" && base.Hostname() != "127.0.0.1" && base.Hostname() != "::1") {
		return errors.New("la carga sólo admite un gateway HTTP local")
	}
	if *duration < time.Second || *duration > 30*time.Second {
		return errors.New("duración permitida: 1s a 30s")
	}
	key := map[string]string{"client": "AUTH_CLIENT_TOKEN", "gym": "AUTH_GYM_TOKEN", "admin": "AUTH_ADMIN_TOKEN"}[*role]
	if key == "" || os.Getenv(key) == "" {
		return errors.New("rol inválido o credencial local ausente")
	}
	if *path == "" {
		zone, err := time.LoadLocation("America/Buenos_Aires")
		if err != nil {
			return errors.New("zona horaria no disponible")
		}
		start := time.Now().In(zone).AddDate(0, 0, 1)
		*path = "/v1/availability?vehicleId=auto-001&startsOn=" + start.Format("2006-01-02") + "&endsOn=" + start.AddDate(0, 0, 1).Format("2006-01-02")
		if *role == "gym" {
			*path = "/v1/employees"
		}
	}
	route, err := url.Parse(*path)
	if err != nil || route.IsAbs() || route.Host != "" || !strings.HasPrefix(route.Path, "/v1/") {
		return errors.New("se requiere una ruta relativa /v1/")
	}
	var concurrencies []int
	for _, text := range strings.Split(*levels, ",") {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 256 {
			return errors.New("concurrencia permitida: 1 a 256")
		}
		concurrencies = append(concurrencies, n)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	target := base.ResolveReference(route).String()
	preflight := &http.Client{Transport: &http.Transport{}, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer preflight.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return errors.New("ruta inválida")
	}
	request.Header.Set("Authorization", "Bearer "+os.Getenv(key))
	response, err := preflight.Do(request)
	if err != nil {
		return errors.New("gateway local no disponible para preflight")
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	response.Body.Close()
	if response.StatusCode != 200 || readErr != nil || len(body) > 1<<20 || !json.Valid(body) {
		return fmt.Errorf("preflight requiere JSON HTTP200; estado recibido %d", response.StatusCode)
	}
	sha, _ := exec.Command("git", "rev-parse", "HEAD").Output()
	dirty, _ := exec.Command("git", "status", "--porcelain").Output()
	out := report{StartedAt: time.Now().UTC().Format(time.RFC3339), Revision: strings.TrimSpace(string(sha)), Dirty: len(dirty) > 0, Go: runtime.Version(), OS: runtime.GOOS, CPUs: runtime.NumCPU(), Route: *path, Role: *role, Model: "closed-loop GET, sin espera entre solicitudes; duración incluye finalización de solicitudes en vuelo; percentiles incluyen todos los resultados", StageSeconds: duration.Seconds()}
	for _, concurrency := range concurrencies {
		if ctx.Err() != nil {
			break
		}
		result := measure(ctx, target, os.Getenv(key), concurrency, *duration)
		out.Stages = append(out.Stages, result)
		fmt.Fprintf(os.Stderr, "etapa concurrencia=%d solicitudes=%d p95=%.2fms erroresTransporte=%d HTTP503=%d\n", concurrency, result.Requests, result.P95, result.TransportErrors, result.Statuses["503"])
		// Evita seguir aumentando presión después de degradación sostenida.
		if result.TransportErrors > 0 || result.InvalidJSON > 0 || result.P95 > 2000 || result.Statuses["503"]*5 > result.Requests {
			break
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

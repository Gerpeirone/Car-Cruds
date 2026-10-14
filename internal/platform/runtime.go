package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func OpenPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("falta DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("DATABASE_URL inválida")
	}
	cfg.MaxConns = 20
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("no se pudo crear el pool PostgreSQL")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, errors.New("PostgreSQL no disponible")
	}
	return pool, nil
}

func InitTelemetry(ctx context.Context, service string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, errors.New("no se pudo configurar el exportador de trazas")
	}
	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", service), attribute.String("service.instance.id", Env("INSTANCE_ID", service))))
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)
var metricMu sync.Mutex
var extraMetrics []prometheus.Collector

// RegisterMetrics conecta las métricas del componente a su endpoint privado.
func RegisterMetrics(collectors ...prometheus.Collector) {
	metricMu.Lock()
	defer metricMu.Unlock()
	extraMetrics = append(extraMetrics, collectors...)
}

func Run(ctx context.Context, service string, handler http.Handler, ready func(context.Context) error) error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	shutdown, err := InitTelemetry(ctx, service)
	if err != nil {
		return err
	}
	defer func() {
		flush, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(flush)
	}()
	tokens, err := TokensFromEnv()
	if err != nil {
		return err
	}
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "car_cruds_http_requests_total", Help: "Solicitudes por ruta y resultado."}, []string{"service", "instance", "method", "route", "status"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "car_cruds_http_duration_seconds", Help: "Duración por ruta.", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 5}}, []string{"service", "route"})
	registry.MustRegister(requests, latency, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metricMu.Lock()
	for _, collector := range extraMetrics {
		if err := registry.Register(collector); err != nil {
			metricMu.Unlock()
			return errors.New("métrica duplicada o inválida")
		}
	}
	metricMu.Unlock()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/ready", func(w http.ResponseWriter, r *http.Request) {
		if !RequireRole(w, r, "admin", "service") {
			return
		}
		probe, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if ready != nil {
			if err := ready(probe); err != nil {
				Error(w, 503, "SERVICE_UNAVAILABLE", "Dependencia no disponible")
				return
			}
		}
		WriteJSON(w, 200, map[string]string{"status": "ok", "service": service, "instance": Env("INSTANCE_ID", service)})
	})
	mux.Handle("GET /internal/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequireRole(w, r, "admin", "service") {
			promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
		}
	}))
	mux.Handle("/", handler)
	secured := AuthMiddleware(mux, tokens)
	observed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			id = UUID()
		}
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		deadline, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		*r = *r.WithContext(context.WithValue(deadline, requestIDKey{}, id))
		w.Header().Set("X-Service-Instance", Env("INSTANCE_ID", service))
		result := httpsnoop.CaptureMetrics(secured, w, r)
		route := r.Pattern
		if route == "" || route == "/" {
			route = "unmatched"
		}
		instance := Env("INSTANCE_ID", service)
		requests.WithLabelValues(service, instance, r.Method, route, fmt.Sprint(result.Code)).Inc()
		latency.WithLabelValues(service, route).Observe(result.Duration.Seconds())
		span := trace.SpanFromContext(r.Context())
		span.SetName(service + " " + r.Method + " " + route)
		span.SetAttributes(attribute.String("http.route", route), attribute.String("request.id", id))
		slog.Info("http_request", "service", service, "instance", instance, "method", r.Method, "route", route, "status", result.Code, "duration_ms", float64(result.Duration.Microseconds())/1000, "request_id", id, "trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String())
	})
	server := &http.Server{Addr: Env("HTTP_ADDR", ":"+Env("PORT", "8080")), Handler: otelhttp.NewHandler(observed, service), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 6 * time.Second, WriteTimeout: 6 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	slog.Info("service_started", "service", service, "instance", Env("INSTANCE_ID", service), "address", server.Addr)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(stop)
}

package main

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/Gerpeirone/Car-Cruds/services/customers/data"
	"github.com/Gerpeirone/Car-Cruds/services/customers/presentation"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := platform.OpenPostgres(ctx, platform.Env("DATABASE_URL", ""))
	if err != nil {
		log.Fatal("No se pudo abrir el almacenamiento de Clientes.")
	}
	defer pool.Close()
	if err = data.Migrate(ctx, pool); err != nil {
		log.Fatal("No se pudo inicializar Clientes.")
	}
	if platform.Env("SEED_DEMO", "false") == "true" {
		if err = data.SeedDemo(ctx, pool); err != nil {
			log.Fatal("No se pudo crear el perfil de demostración.")
		}
	}
	repo := &data.Repository{Pool: pool}
	service := &business.Service{Repo: repo}
	if broker := platform.Env("AMQP_URL", ""); broker != "" {
		go repo.ConsumeRentals(ctx, broker)
	}
	ttl, err := time.ParseDuration(platform.Env("CUSTOMER_SESSION_TTL", "12h"))
	if err != nil || ttl < time.Minute || ttl > 7*24*time.Hour {
		log.Fatal("CUSTOMER_SESSION_TTL debe estar entre 1m y 168h.")
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/auth/", presentation.NewAccountHandler(business.Accounts{Store: repo, TTL: ttl}))
	mux.Handle("/", &presentation.Handler{Service: service})
	if err = platform.Run(ctx, "customers", mux, pool.Ping, platform.AccountRoute); err != nil {
		log.Fatal("El servidor de Clientes finalizó con error.")
	}
}

package main

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/dependencies"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/messaging"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"log"
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
		log.Fatal("No se pudo abrir el almacenamiento de Alquileres.")
	}
	defer pool.Close()
	if err = postgres.Migrate(ctx, pool); err != nil {
		log.Fatal("No se pudo inicializar el almacenamiento de Alquileres.")
	}
	repo := &postgres.Repository{Pool: pool, NewID: platform.UUID}
	deps := &dependencies.HTTP{Client: platform.NewHTTPClient(time.Second), CustomersURL: platform.Env("CUSTOMERS_URL", "http://customers:8080"), FleetURL: platform.Env("FLEET_URL", "http://load-balancer:8080"), Token: platform.Env("SERVICE_TOKEN", platform.Env("AUTH_SERVICE_TOKEN", ""))}
	service := &application.Service{Repo: repo, Customers: deps, Fleet: deps, Now: time.Now, NewID: platform.UUID}
	if broker := platform.Env("AMQP_URL", ""); broker != "" {
		go messaging.Run(ctx, pool, broker)
	}
	if err = platform.Run(ctx, "rentals", &httpapi.Handler{Service: service}, pool.Ping); err != nil {
		log.Fatal("El servidor de Alquileres finalizó con error.")
	}
}

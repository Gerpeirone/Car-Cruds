package main

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/Gerpeirone/Car-Cruds/services/customers/data"
	"github.com/Gerpeirone/Car-Cruds/services/customers/presentation"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	if err = platform.Run(ctx, "customers", &presentation.Handler{Service: service}, pool.Ping); err != nil {
		log.Fatal("El servidor de Clientes finalizó con error.")
	}
}

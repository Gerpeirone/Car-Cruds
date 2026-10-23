package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if _, err := platform.TokensFromEnv(); err != nil || os.Getenv("AUTH_GYM_TOKEN") == "" {
		slog.Error("inicio de Empleados fallido", "cause", "se requieren credenciales válidas de administración, servicio y gimnasio")
		os.Exit(1)
	}
	startup, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platform.OpenPostgres(startup, os.Getenv("DATABASE_URL"))
	if err != nil {
		startupCancel()
		slog.Error("inicio de Empleados fallido", "cause", "base de datos no disponible o configuración inválida")
		os.Exit(1)
	}
	defer pool.Close()
	repository := postgres.New(pool)
	if err = repository.Migrate(startup); err != nil {
		startupCancel()
		slog.Error("inicio de Empleados fallido", "cause", "migración de almacenamiento")
		os.Exit(1)
	}
	seed, err := strconv.ParseBool(platform.Env("SEED_DEMO", "false"))
	if err != nil {
		startupCancel()
		slog.Error("inicio de Empleados fallido", "cause", "SEED_DEMO debe ser booleano")
		os.Exit(1)
	}
	if seed {
		if err = repository.SeedDemo(startup); err != nil {
			startupCancel()
			slog.Error("inicio de Empleados fallido", "cause", "datos ficticios de demostración")
			os.Exit(1)
		}
	}
	startupCancel()
	service := application.New(repository, platform.UUID).WithMembershipInbox(repository)
	if err = platform.Run(ctx, "employees", httpapi.New(service), pool.Ping); err != nil {
		slog.Error("Empleados finalizó con error", "cause", err)
		os.Exit(1)
	}
}

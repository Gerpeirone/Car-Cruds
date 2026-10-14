package main

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/fleet"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fleet.RegisterMetrics()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if platform.Env("RABBITMQ_URL", "") == "" || platform.Env("SOLR_URL", "") == "" {
		slog.Error("indexer dependencies not configured")
		os.Exit(1)
	}
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	repo, err := fleet.OpenMongo(startup, platform.Env("MONGODB_URI", "mongodb://localhost:27017"), platform.Env("MONGODB_DATABASE", "fleet"))
	cancel()
	if err != nil {
		slog.Error("indexer Mongo startup failed")
		os.Exit(1)
	}
	defer repo.Close(context.Background())
	search := fleet.NewSolr(platform.Env("SOLR_URL", ""), platform.NewHTTPClient(2*time.Second))
	worker := fleet.NewWorker(repo, search, platform.Env("RABBITMQ_URL", ""))
	go worker.Run(ctx)
	if err = platform.Run(ctx, "indexer", http.NewServeMux(), worker.Ready); err != nil {
		slog.Error("indexer stopped")
		os.Exit(1)
	}
}

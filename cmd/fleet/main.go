package main

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/fleet"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fleet.RegisterMetrics()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	repo, err := fleet.OpenMongo(startup, platform.Env("MONGODB_URI", "mongodb://localhost:27017"), platform.Env("MONGODB_DATABASE", "fleet"))
	if err != nil {
		cancel()
		slog.Error("fleet startup failed")
		os.Exit(1)
	}
	defer repo.Close(context.Background())
	if platform.Env("SEED_DEMO", "false") == "true" {
		err = fleet.SeedDemo(startup, repo)
	}
	cancel()
	if err != nil {
		slog.Error("fleet seed failed")
		os.Exit(1)
	}
	var cache *fleet.RedisCache
	var optionalCache fleet.Cache
	if addr := platform.Env("REDIS_ADDR", ""); addr != "" {
		cache = fleet.NewRedisCache(addr, platform.Env("REDIS_PASSWORD", ""), "fleet:v1:"+platform.Env("MONGODB_DATABASE", "fleet"))
		optionalCache = cache
		defer cache.Close()
	}
	var search fleet.SearchEngine
	if url := platform.Env("SOLR_URL", ""); url != "" {
		search = fleet.NewSolr(url, platform.NewHTTPClient(2*time.Second))
	}
	service := fleet.NewService(repo, optionalCache, search)
	if err = platform.Run(ctx, "fleet", fleet.Handler(service), repo.Ready); err != nil {
		slog.Error("fleet stopped")
		os.Exit(1)
	}
}

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/nut"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/runtime"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/web"
)

func main() {
	cfgPath := flag.String("config", "/etc/ecoflow-ble-nutd.conf", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store := state.New()
	manager := runtime.NewProviderManager(cfg, store)
	go func() {
		if err := manager.Run(ctx); err != nil {
			slog.Error("provider manager stopped", "error", err)
		}
	}()
	go func() {
		if err := web.New(cfg, store, manager).Run(ctx); err != nil {
			slog.Error("web server stopped", "error", err)
			stop()
		}
	}()

	if err := nut.New(cfg, store).Run(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

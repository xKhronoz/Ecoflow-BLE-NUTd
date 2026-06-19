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
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/provider"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
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
	p := provider.New(cfg)
	go func() {
		if err := p.Run(ctx, store); err != nil {
			slog.Error("provider stopped", "error", err)
			stop()
		}
	}()

	if err := nut.New(cfg, store).Run(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

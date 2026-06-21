package provider

import (
	"context"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type Provider interface {
	Run(ctx context.Context, store *state.Store) error
}

func New(cfg *config.Config) Provider {
	switch cfg.Provider.Type {
	case "json-dir":
		return NewJSONDir(cfg)
	case "eco-ble":
		return NewEcoBLE(cfg)
	default:
		return NewMock(cfg)
	}
}

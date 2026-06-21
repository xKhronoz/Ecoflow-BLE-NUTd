package provider

import (
	"context"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type EcoBLE struct {
	cfg *config.Config
}

func NewEcoBLE(cfg *config.Config) *EcoBLE {
	return &EcoBLE{cfg: cfg}
}

func (e *EcoBLE) Run(ctx context.Context, store *state.Store) error {
	return ecoflow.NewProvider(e.cfg).Run(ctx, store)
}

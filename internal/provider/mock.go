package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type Mock struct{ cfg *config.Config }

func NewMock(cfg *config.Config) *Mock { return &Mock{cfg: cfg} }

func (m *Mock) Run(ctx context.Context, store *state.Store) error {
	ticker := time.NewTicker(time.Duration(m.cfg.Provider.PollSeconds) * time.Second)
	defer ticker.Stop()
	charge := 95
	publish := func() {
		charge--
		if charge < 10 {
			charge = 95
		}
		for _, d := range m.cfg.Devices {
			status := "OB"
			runtime := charge * 60
			if charge <= d.LowBatteryPercent || runtime <= d.LowRuntimeSeconds {
				status = "OB LB"
			}
			vars := baseVars(d, status)
			vars["battery.charge"] = fmt.Sprintf("%d", charge)
			vars["battery.runtime"] = fmt.Sprintf("%d", runtime)
			vars["input.power"] = "0"
			vars["output.power"] = "85"
			store.Upsert(d.Name, d.Description, vars)
		}
	}
	publish()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			publish()
		}
	}
}

func baseVars(d config.Device, status string) map[string]string {
	model := d.Model
	if model == "" {
		model = "EcoFlow"
	}
	serial := d.Serial
	if serial == "" {
		serial = d.MAC
	}
	return map[string]string{
		"device.mfr":            "EcoFlow",
		"device.model":          model,
		"device.serial":         serial,
		"ups.mfr":               "EcoFlow",
		"ups.model":             model,
		"ups.serial":            serial,
		"ups.status":            status,
		"ups.realpower.nominal": "0",
	}
}

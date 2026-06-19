package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type JSONDir struct{ cfg *config.Config }

func NewJSONDir(cfg *config.Config) *JSONDir { return &JSONDir{cfg: cfg} }

type DeviceReading struct {
	BatteryCharge  int  `json:"battery_charge"`
	BatteryRuntime int  `json:"battery_runtime"`
	InputPower     int  `json:"input_power"`
	OutputPower    int  `json:"output_power"`
	ACInputPresent bool `json:"ac_input_present"`
}

func (j *JSONDir) Run(ctx context.Context, store *state.Store) error {
	if j.cfg.Provider.JSONDir == "" {
		return fmt.Errorf("provider.json_dir is required")
	}
	ticker := time.NewTicker(time.Duration(j.cfg.Provider.PollSeconds) * time.Second)
	defer ticker.Stop()
	poll := func() {
		for _, d := range j.cfg.Devices {
			path := filepath.Join(j.cfg.Provider.JSONDir, d.Name+".json")
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var r DeviceReading
			if json.Unmarshal(b, &r) != nil {
				continue
			}
			status := "OB"
			if r.ACInputPresent {
				status = "OL"
			}
			if r.BatteryCharge <= d.LowBatteryPercent || r.BatteryRuntime <= d.LowRuntimeSeconds {
				status = status + " LB"
			}
			vars := baseVars(d, status)
			vars["battery.charge"] = state.FormatInt(r.BatteryCharge)
			vars["battery.runtime"] = state.FormatInt(r.BatteryRuntime)
			vars["input.power"] = state.FormatInt(r.InputPower)
			vars["output.power"] = state.FormatInt(r.OutputPower)
			store.Upsert(d.Name, d.Description, vars)
		}
	}
	poll()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			poll()
		}
	}
}

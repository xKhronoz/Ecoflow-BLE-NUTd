package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

func TestProvidersPublishWattsWithoutInventingVA(t *testing.T) {
	for _, kind := range []string{"mock", "json-dir"} {
		t.Run(kind, func(t *testing.T) {
			cfg := &config.Config{Provider: config.ProviderCfg{PollSeconds: 60, JSONDir: t.TempDir()},
				Devices: []config.Device{{Name: "river3"}}}
			if err := os.WriteFile(filepath.Join(cfg.Provider.JSONDir, "river3.json"),
				[]byte(`{"battery_charge":75,"battery_runtime":1800,"input_power":0,"output_power":85}`), 0o600); err != nil {
				t.Fatal(err)
			}
			run := NewMock(cfg).Run
			if kind == "json-dir" {
				run = NewJSONDir(cfg).Run
			}
			ctx, cancel := context.WithCancel(context.Background())
			store := state.New()
			done := make(chan error, 1)
			go func() { done <- run(ctx, store) }()
			t.Cleanup(func() {
				cancel()
				if err := <-done; err != nil {
					t.Error(err)
				}
			})
			deadline := time.Now().Add(time.Second)
			for {
				if ups, ok := store.Get("river3"); ok {
					if ups.Vars["experimental.input.realpower.total"] != "0" || ups.Vars["ups.realpower"] != "85" {
						t.Fatalf("incorrect watts: %#v", ups.Vars)
					}
					for _, key := range []string{"input.power", "output.power"} {
						if _, present := ups.Vars[key]; present {
							t.Errorf("invented apparent power: %s", key)
						}
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("provider did not publish")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}

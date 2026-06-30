package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type fakeRunner struct {
	run func(context.Context, *state.Store) error
}

func (f fakeRunner) Run(ctx context.Context, store *state.Store) error {
	return f.run(ctx, store)
}

func testConfig() *config.Config {
	return &config.Config{
		Provider: config.ProviderCfg{
			Type:                  "eco-ble",
			ReconnectDelaySeconds: 1,
		},
		Devices: []config.Device{
			{Name: "delta2", Description: "Delta 2", Model: "DELTA 2", MAC: "AA:BB:CC:DD:EE:01"},
		},
	}
}

func TestProviderManagerDisable(t *testing.T) {
	t.Parallel()

	store := state.New()
	manager := NewProviderManager(testConfig(), store)
	started := make(chan struct{}, 1)
	manager.newRun = func(cfg *config.Config) Runner {
		return fakeRunner{run: func(ctx context.Context, store *state.Store) error {
			store.Upsert("delta2", "Delta 2", map[string]string{"ups.status": "OL"})
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- manager.Run(ctx)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("provider did not start")
	}

	manager.Disable()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if !status.Enabled && status.State == "disabled" {
			ups, ok := store.Get("delta2")
			if !ok {
				t.Fatalf("missing device state")
			}
			if ups.Vars["ups.status"] != "WAIT" {
				t.Fatalf("ups.status = %q", ups.Vars["ups.status"])
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("manager stopped with error: %v", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("manager never reached disabled state: %#v", manager.Status())
}

func TestProviderManagerRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	store := state.New()
	manager := NewProviderManager(testConfig(), store)
	manager.retryGap = 20 * time.Millisecond

	attempts := 0
	retried := make(chan struct{}, 1)
	manager.newRun = func(cfg *config.Config) Runner {
		return fakeRunner{run: func(ctx context.Context, store *state.Store) error {
			attempts++
			if attempts == 1 {
				return errors.New("bootstrap failed")
			}
			retried <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = manager.Run(ctx)
	}()

	select {
	case <-retried:
	case <-time.After(2 * time.Second):
		t.Fatalf("manager did not retry, status = %#v", manager.Status())
	}

	status := manager.Status()
	if !status.Enabled || !status.Running || status.State != "running" {
		t.Fatalf("unexpected status after retry: %#v", status)
	}
}

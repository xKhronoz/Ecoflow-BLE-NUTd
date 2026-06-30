package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/provider"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type ProviderStatus struct {
	Type        string    `json:"type"`
	Enabled     bool      `json:"enabled"`
	Running     bool      `json:"running"`
	State       string    `json:"state"`
	LastError   string    `json:"last_error,omitempty"`
	LastErrorAt time.Time `json:"last_error_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Runner interface {
	Run(ctx context.Context, store *state.Store) error
}

type RunnerFactory func(cfg *config.Config) Runner

type ProviderManager struct {
	cfg      *config.Config
	store    *state.Store
	newRun   RunnerFactory
	retryGap time.Duration
	wakeCh   chan struct{}

	mu        sync.RWMutex
	status    ProviderStatus
	runCancel context.CancelFunc
}

func NewProviderManager(cfg *config.Config, store *state.Store) *ProviderManager {
	provider.MarkDevicesWaiting(store, cfg.Devices)
	now := time.Now()
	return &ProviderManager{
		cfg:      cfg,
		store:    store,
		newRun:   func(cfg *config.Config) Runner { return provider.New(cfg) },
		retryGap: time.Duration(cfg.Provider.ReconnectDelaySeconds) * time.Second,
		wakeCh:   make(chan struct{}, 1),
		status: ProviderStatus{
			Type:      cfg.Provider.Type,
			Enabled:   true,
			Running:   false,
			State:     "starting",
			UpdatedAt: now,
		},
	}
}

func (m *ProviderManager) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			m.setStopped()
			return nil
		}
		if !m.isEnabled() {
			if err := m.waitForWake(ctx, 0); err != nil {
				m.setStopped()
				return nil
			}
			continue
		}

		err := m.runActive(ctx)
		if ctx.Err() != nil {
			m.setStopped()
			return nil
		}
		if !m.isEnabled() {
			m.transition(false, false, "disabled", "", time.Time{})
			continue
		}

		if err == nil || errors.Is(err, context.Canceled) {
			err = fmt.Errorf("provider stopped unexpectedly")
		}
		now := time.Now()
		m.transition(true, false, "retrying", err.Error(), now)
		provider.MarkDevicesWaiting(m.store, m.cfg.Devices)

		if err := m.waitForWake(ctx, m.retryDelay()); err != nil {
			m.setStopped()
			return nil
		}
	}
}

func (m *ProviderManager) Enable() {
	m.mu.Lock()
	if m.status.Enabled {
		m.mu.Unlock()
		return
	}
	m.status.Enabled = true
	m.status.Running = false
	m.status.State = "starting"
	m.status.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.signalWake()
}

func (m *ProviderManager) Disable() {
	var cancel context.CancelFunc
	m.mu.Lock()
	if !m.status.Enabled {
		m.mu.Unlock()
		return
	}
	m.status.Enabled = false
	if m.status.Running {
		m.status.State = "stopping"
		cancel = m.runCancel
	} else {
		m.status.State = "disabled"
	}
	m.status.Running = false
	m.status.UpdatedAt = time.Now()
	m.mu.Unlock()
	provider.MarkDevicesWaiting(m.store, m.cfg.Devices)
	if cancel != nil {
		cancel()
	}
	m.signalWake()
}

func (m *ProviderManager) Status() ProviderStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *ProviderManager) runActive(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.runCancel = cancel
	m.status.Enabled = true
	m.status.Running = true
	m.status.State = "running"
	m.status.UpdatedAt = time.Now()
	m.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.newRun(m.cfg).Run(runCtx, m.store)
	}()

	err := <-errCh

	m.mu.Lock()
	m.runCancel = nil
	m.status.Running = false
	m.status.UpdatedAt = time.Now()
	m.mu.Unlock()
	return err
}

func (m *ProviderManager) isEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.Enabled
}

func (m *ProviderManager) retryDelay() time.Duration {
	if m.retryGap <= 0 {
		return 5 * time.Second
	}
	return m.retryGap
}

func (m *ProviderManager) waitForWake(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.wakeCh:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.wakeCh:
		return nil
	case <-timer.C:
		return nil
	}
}

func (m *ProviderManager) signalWake() {
	select {
	case m.wakeCh <- struct{}{}:
	default:
	}
}

func (m *ProviderManager) transition(enabled, running bool, state, lastError string, lastErrorAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Enabled = enabled
	m.status.Running = running
	m.status.State = state
	m.status.LastError = lastError
	m.status.LastErrorAt = lastErrorAt
	m.status.UpdatedAt = time.Now()
}

func (m *ProviderManager) setStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Running = false
	m.status.State = "stopped"
	m.status.UpdatedAt = time.Now()
	m.runCancel = nil
}

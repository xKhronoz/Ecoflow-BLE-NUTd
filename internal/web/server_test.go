package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/runtime"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type fakeController struct {
	status   runtime.ProviderStatus
	enables  int
	disables int
}

func (f *fakeController) Enable() {
	f.enables++
	f.status.Enabled = true
	f.status.Running = true
	f.status.State = "running"
}

func (f *fakeController) Disable() {
	f.disables++
	f.status.Enabled = false
	f.status.Running = false
	f.status.State = "disabled"
}

func (f *fakeController) Status() runtime.ProviderStatus {
	return f.status
}

func TestStatusEndpoint(t *testing.T) {
	t.Parallel()

	store := state.New()
	store.Upsert("delta2", "Delta 2", map[string]string{"ups.status": "OL", "battery.charge": "77"})

	controller := &fakeController{status: runtime.ProviderStatus{
		Type:      "eco-ble",
		Enabled:   true,
		Running:   true,
		State:     "running",
		UpdatedAt: time.Unix(1700000000, 0).UTC(),
	}}
	server := New(&config.Config{
		Listen: "127.0.0.1:3493",
		Web:    config.WebConfig{Enable: true, Listen: "127.0.0.1:8080"},
	}, store, controller)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}

	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider.State != "running" {
		t.Fatalf("provider state = %q", resp.Provider.State)
	}
	if len(resp.Devices) != 1 || resp.Devices[0].Vars["battery.charge"] != "77" {
		t.Fatalf("devices = %#v", resp.Devices)
	}
}

func TestDisableEndpointRequiresPost(t *testing.T) {
	t.Parallel()

	controller := &fakeController{status: runtime.ProviderStatus{Type: "eco-ble", Enabled: true, Running: true, State: "running"}}
	server := New(&config.Config{}, state.New(), controller)

	req := httptest.NewRequest(http.MethodPost, "/api/ble/disable", nil)
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	if controller.disables != 1 {
		t.Fatalf("disable count = %d", controller.disables)
	}
}

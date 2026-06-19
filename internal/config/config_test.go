package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.conf")
	body := `listen: 0.0.0.0:3493
auth:
  username: monuser
  password: secret
provider:
  type: mock
  poll_seconds: 15
  json_dir: /run/ecoflow-ble-nutd
devices:
  - name: delta2
    description: EcoFlow Delta 2 BLE
    model: DELTA 2
    mac: AA:BB:CC:DD:EE:01
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Listen != "0.0.0.0:3493" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
	if cfg.Auth.Username != "monuser" || cfg.Auth.Password != "secret" {
		t.Fatalf("auth = %#v", cfg.Auth)
	}
	if cfg.Provider.Type != "mock" || cfg.Provider.PollSeconds != 15 {
		t.Fatalf("provider = %#v", cfg.Provider)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0].Name != "delta2" {
		t.Fatalf("devices = %#v", cfg.Devices)
	}
}

func TestLoadJSONConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.json")
	body := `{
  "provider": {
    "type": "mock",
    "poll_seconds": 12,
    "json_dir": "/run/ecoflow-ble-nutd"
  },
  "devices": [
    {
      "name": "river3",
      "description": "EcoFlow River 3 BLE",
      "mac": "AA:BB:CC:DD:EE:02"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Listen != "127.0.0.1:3493" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
	if cfg.Provider.PollSeconds != 12 {
		t.Fatalf("poll_seconds = %d", cfg.Provider.PollSeconds)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0].Name != "river3" {
		t.Fatalf("devices = %#v", cfg.Devices)
	}
	if cfg.Devices[0].LowBatteryPercent != 20 || cfg.Devices[0].LowRuntimeSeconds != 300 {
		t.Fatalf("defaults = %#v", cfg.Devices[0])
	}
}

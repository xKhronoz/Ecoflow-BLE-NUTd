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

func TestLoadEcoBLEUserIDConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.conf")
	body := `provider:
  type: eco-ble
  adapter: hci0
  auth:
    user_id: "12345"
devices:
  - name: delta2
    mac: AA:BB:CC:DD:EE:01
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Provider.Type != "eco-ble" {
		t.Fatalf("provider type = %q", cfg.Provider.Type)
	}
	if cfg.Provider.Auth.UserID != "12345" {
		t.Fatalf("user_id = %q", cfg.Provider.Auth.UserID)
	}
	if cfg.Provider.ScanTimeoutSeconds != 8 || cfg.Provider.ConnectTimeoutSeconds != 20 || cfg.Provider.ReconnectDelaySeconds != 10 {
		t.Fatalf("provider defaults = %#v", cfg.Provider)
	}
}

func TestLoadEcoBLEEmailPasswordConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.conf")
	body := `provider:
  type: eco-ble
  auth:
    email: user@example.com
    password: secret
devices:
  - name: river3
    mac: AA:BB:CC:DD:EE:02
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Provider.Auth.Email != "user@example.com" || cfg.Provider.Auth.Password != "secret" {
		t.Fatalf("provider auth = %#v", cfg.Provider.Auth)
	}
	if cfg.Provider.Auth.Region != "auto" {
		t.Fatalf("region = %q", cfg.Provider.Auth.Region)
	}
}

func TestLoadEcoBLERequiresAuth(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.conf")
	body := `provider:
  type: eco-ble
devices:
  - name: delta3
    mac: AA:BB:CC:DD:EE:03
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected auth validation error")
	}
}

func TestLoadEcoBLERequiresMAC(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "ecoflow-ble-nutd.conf")
	body := `provider:
  type: eco-ble
  auth:
    user_id: "12345"
devices:
  - name: delta2
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected MAC validation error")
	}
}

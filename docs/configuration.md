# Configuration and providers

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

The daemon accepts YAML-oriented `.conf` files and JSON configuration files. See the [BLE authentication guide](ble-authentication.md) for account and MAC setup.

## Daemon configuration

Example daemon config:

```yaml
listen: 0.0.0.0:3493
auth:
  username: monuser
  password: secret
provider:
  type: eco-ble
  adapter: hci0
  scan_timeout_seconds: 8
  connect_timeout_seconds: 20
  reconnect_delay_seconds: 10
  poll_seconds: 15
  auth:
    user_id: ""
    email: user@example.com
    password: change-me
    region: auto
web:
  enable: true
  listen: 127.0.0.1:8080
devices:
  - name: delta2
    description: EcoFlow Delta 2 BLE
    model: DELTA 2
    mac: AA:BB:CC:DD:EE:01
    low_battery_percent: 20
    low_runtime_seconds: 300
    stale_timeout_seconds: 60
```

The [native configuration example](../examples/ecoflow-ble-nutd.conf) and [Docker configuration example](../deploy/docker/ecoflow-ble-nutd.conf.example) contain the same daemon settings with different web bind addresses.

## Web status and control

For the optional web server:

- set `web.enable: true` to enable the local status/control UI
- set `web.listen` to choose the bind address, for example `127.0.0.1:8080` or `0.0.0.0:8080`
- if `web.enable` is `true` and `web.listen` is omitted, it defaults to `127.0.0.1:8080`
- if NUT `auth.username` / `auth.password` are set, the same credentials are required as HTTP Basic auth for the web UI and API
- `POST /api/ble/disable` pauses this daemon's BLE collector and keeps the NUT server running with `WAIT` status data
- `POST /api/ble/enable` starts the collector again
- `GET /api/status` returns daemon, collector, and device JSON status
- disabling the collector does not toggle Bluetooth on the EcoFlow device itself; it only releases this daemon's BLE session

## BLE provider

For `eco-ble`:

- `devices[].mac` is required
- `devices[].model` is optional metadata only
- packet/profile selection is based on the BLE advertisement serial prefix
- if `provider.auth.user_id` is set, it is used directly and `email/password` are ignored
- otherwise `provider.auth.email` and `provider.auth.password` must both be set
- `region` defaults to `auto`
- cloud login is used once at startup to resolve `user_id`; normal telemetry stays local after that
- keep config file permissions tight if you store credentials inline, for example `0600`
- supported runtime target is Linux with BlueZ and a working BLE adapter such as `hci0`
- once a device passes `stale_timeout_seconds`, the daemon serves only identity vars plus `ups.status=WAIT`

## JSON directory provider

Set:

```json
{
  "provider": {
    "type": "json-dir",
    "json_dir": "/run/ecoflow-ble-nutd"
  }
}
```

Then write files like:

```json
{
  "battery_charge": 74,
  "battery_runtime": 3600,
  "input_power": 0,
  "output_power": 85,
  "ac_input_present": false
}
```

File path:

```text
/run/ecoflow-ble-nutd/delta2.json
```

This lets you develop the BLE collector separately while keeping the NUT server stable.

The [sample reading](../examples/delta2.json) provides a starting point for integration testing.

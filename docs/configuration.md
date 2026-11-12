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
    rated_power_watts: 1800
    low_runtime_seconds: 300
    low_runtime_max_percent: 50
    low_runtime_grace_seconds: 60
    stale_timeout_seconds: 60
```

The [native configuration example](../examples/ecoflow-ble-nutd.conf) and [Docker configuration example](../deploy/docker/ecoflow-ble-nutd.conf.example) contain the same daemon settings with different web bind addresses.

NUT discovery and telemetry reads are available without authentication, including when `auth.username` and `auth.password` are set. These credentials protect NUT session/control commands and the web UI/API. Clients such as pfSense `upsmon` should use these top-level credentials; `provider.auth` is only for EcoFlow authentication. See [NUT protocol coverage](nut-protocol.md) for the supported commands.

### Device monitoring settings

Set these values separately for each entry under `devices`. The table describes the `eco-ble` provider. They control the telemetry and status reported to monitoring clients; client configuration determines notifications and shutdown actions.

| Setting | Default | Effect on monitoring | Effect of changing the value |
| --- | --- | --- | --- |
| `low_battery_percent` | `20` (%) | Immediately adds `LB` (low battery) to `ups.status` when a reported charge is at or below this percentage, including while charging. Also published as `battery.charge.low`. The runtime grace period does not delay this threshold. | A higher percentage warns earlier and leaves more charge available for shutdown. A lower percentage delays the warning. |
| `rated_power_watts` | `0` (unset) | Fallback output rating for `ups.realpower.nominal` when the device reports no positive rating. Used to calculate `ups.load` as output watts divided by the rating, multiplied by 100 and rounded. A positive device rating takes priority. | Set it to the unit's rated AC output. `1800` is an example, not a universal default: 900 W output with a 1800 W rating reports 50% load. A rating that is too high understates load; one that is too low overstates it. Without either rating, `ups.load` is absent. |
| `low_runtime_seconds` | `300` (5 minutes) | Runtime at or below this value can trigger `LB`, but only while discharging, within the charge ceiling below, and after the grace period. The reported `battery.runtime` remains the device's estimate. | A higher value allows an earlier runtime warning. A lower value waits until less estimated runtime remains. |
| `low_runtime_max_percent` | `50` (%) | Allows runtime to trigger `LB` only when the reported charge is at or below this percentage. This rejects implausibly short runtime estimates at high charge. The ceiling is not applied until a charge reading is known. | Raising it allows runtime warnings at higher charge levels. Lowering it restricts runtime warnings to a more depleted battery. It does not change the immediate `low_battery_percent` threshold. |
| `low_runtime_grace_seconds` | `60` (1 minute) | Requires the runtime warning conditions to hold continuously for this duration before setting `LB`. Charging, resting, a runtime above the threshold, or a known charge above the ceiling resets the timer. | A longer grace period filters more transient estimates but delays the warning. A shorter period responds faster to low runtime. |
| `stale_timeout_seconds` | `60` (1 minute) | After this long without fresh decoded telemetry, reports `ups.status=WAIT` and removes dynamic readings such as charge, runtime, and watts. Checked on `provider.poll_seconds` ticks; a disconnection can cause `WAIT` sooner. | A shorter timeout identifies missing telemetry sooner but is more sensitive to brief gaps. A longer timeout tolerates gaps while retaining older readings for longer. |

For the five threshold and timeout settings, an omitted, zero, or negative value selects the default; zero does not disable the setting. `rated_power_watts` supplies a fallback only when positive.

With the example values, discharging at 40% charge with an estimated 240 seconds remaining sets `LB` after those conditions persist for 60 seconds. At 80% charge, that same runtime estimate does not trigger `LB`. At 20% charge or below, the charge threshold sets `LB` immediately, even while charging.

Discharge is determined from the battery's own power reading when available, else the balance of total input and output power, including solar and car input. AC presence is the fallback when power readings are missing. The discharge floor set in the EcoFlow app is a separate device limit, exposed as `experimental.battery.charge.minimum` when reported.

The `mock` and `json-dir` providers use `low_battery_percent` and `low_runtime_seconds`, applying either threshold immediately. They do not use the BLE runtime guards, rating fallback, or stale timeout described above.

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

The Enable and Disable buttons send requests in the background, show an inline result, and update the collector and device status without leaving the page. While the page is visible, status refreshes every five seconds. Green badges indicate active states, red indicates inactive states, and amber indicates starting, stopping, or retrying; the text labels remain visible.

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

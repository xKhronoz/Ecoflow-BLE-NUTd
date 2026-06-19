# ecoflow-ble-nutd

Small NUT-compatible daemon for exposing EcoFlow-like battery readings as virtual UPS devices.

This is an appliance-oriented skeleton for low-RAM SBCs. It keeps runtime state in memory and does not require a database, Home Assistant, Docker, MQTT, or persistent logs.

## Current status

Implemented:

- TCP NUT-like server on port `3493`
- multiple virtual UPS devices
- `upsc`-style commands:
  - `VER`
  - `LIST UPS`
  - `LIST VAR <ups>`
  - `GET VAR <ups> <var>`
  - `GET UPSDESC <ups>`
- simple `USERNAME` / `PASSWORD`
- mock provider
- JSON directory provider for integration testing

Not implemented yet:

- direct EcoFlow BLE protocol implementation
- full NUT protocol coverage
- TLS
- verified `upsmon` compatibility in every shutdown mode

For production safe-shutdown today, the safer path is still: BLE collector -> NUT `dummy-ups` -> real `upsd`. This daemon is the start of a standalone replacement.

## Build

```sh
go build -o ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd
```

## Dev Container

The repo includes a `.devcontainer/` setup for Go 1.26 development with `nc` and NUT client tools installed.

Open the project in a dev container, then build normally:

```sh
go build ./...
```

The container is useful for daemon and protocol work, mock-provider testing, and JSON-dir integration work. Direct host BLE access may still depend on the host platform and container runtime.

## Run

```sh
./ecoflow-ble-nutd -config examples/ecoflow-ble-nutd.conf
```

The daemon config is YAML-oriented and uses `.conf` by default. Existing JSON config files are still accepted for compatibility.

Test with netcat:

```sh
printf 'USERNAME monuser\nPASSWORD secret\nLIST UPS\n' | nc 127.0.0.1 3493
```

Or with NUT client tools:

```sh
upsc delta2@127.0.0.1
```

## Config format

Example daemon config:

```yaml
listen: 0.0.0.0:3493
auth:
  username: monuser
  password: secret
provider:
  type: mock
  poll_seconds: 10
  json_dir: /run/ecoflow-ble-nutd
devices:
  - name: delta2
    description: EcoFlow Delta 2 BLE
    model: DELTA 2
    mac: AA:BB:CC:DD:EE:01
    low_battery_percent: 20
    low_runtime_seconds: 300
    stale_timeout_seconds: 60
```

The shipped example lives at `examples/ecoflow-ble-nutd.conf`.

## JSON provider

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

## NUT variable mapping

| EcoFlow reading             | NUT variable      |
| --------------------------- | ----------------- |
| battery percent             | `battery.charge`  |
| estimated seconds remaining | `battery.runtime` |
| AC input present            | `ups.status=OL`   |
| no AC input                 | `ups.status=OB`   |
| low battery/runtime         | append `LB`       |
| input watts                 | `input.power`     |
| output watts                | `output.power`    |

## NUT Protocol Coverage

Level 1: upsc compatible (Current)

- LIST UPS
- LIST VAR
- GET VAR
- GET UPSDESC

Level 2: upsmon compatible (Target)

- Level 1
- USERNAME / PASSWORD
- LOGIN
- GET NUMLOGINS
- PRIMARY / MASTER
- FSD
- LOGOUT

Level 3: admin/tool compatible (Not Planned for this daemon)

- GET TYPE / DESC
- LIST CMD / RW / ENUM / RANGE
- SET VAR
- INSTCMD

Level 4: full protocol (Not Planned for this daemon)

- STARTTLS
- TRACKING
- strict quoting/parsing
- complete NUT error behaviour

## SBC notes

Recommended runtime layout:

- config: `/etc/ecoflow-ble-nutd.conf`
- runtime data: `/run/ecoflow-ble-nutd`
- logs: volatile journald
- no database
- no persistent metrics

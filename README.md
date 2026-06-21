# ecoflow-ble-nutd

Small NUT-compatible daemon for exposing EcoFlow power stations as virtual UPS devices over direct BLE.

This is an appliance-oriented daemon for low-RAM SBCs. It keeps runtime state in memory and does not require a database, Home Assistant, Docker, MQTT, or persistent logs.

## Current status

Implemented:

- TCP NUT-like server on port `3493`
- multiple virtual UPS devices
- direct `eco-ble` provider for modern EcoFlow power stations
- `upsc`-style commands:
  - `VER`
  - `LIST UPS`
  - `LIST VAR <ups>`
  - `GET VAR <ups> <var>`
  - `GET UPSDESC <ups>`
- simple `USERNAME` / `PASSWORD`
- mock provider
- JSON directory provider for integration testing

Implemented `eco-ble` scope:

- Linux/BlueZ runtime via `tinygo.org/x/bluetooth`
- configured-device discovery by `devices[].mac`
- auth with direct `user_id` or one-time cloud bootstrap from `email/password`
- session modes `0`, `1`, and `7`
- V2 families:
  - DELTA 2
  - DELTA 2 Max / Max S
  - RIVER 2 / Max / Pro
- V3 / `0x13` families:
  - DELTA 3 family
  - DELTA Pro 3 family
  - DELTA Pro Ultra
  - RIVER 3 family
- stale-device handling with `ups.status=WAIT`

Still not implemented:

- legacy V1 devices
- V4 protocol devices
- non-power-station product classes such as WAVE, generators, SHP, STREAM, PowerOcean, and similar
- device control writes such as AC/DC switching or charge-limit updates
- full NUT protocol coverage
- TLS
- verified `upsmon` compatibility in every shutdown mode

This is still an early standalone replacement, but it now includes a direct read-only EcoFlow BLE path instead of requiring an external collector.

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

The daemon config is YAML-oriented and uses `.conf` by default. JSON config files are also accepted for compatibility.

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

## Finding `user_id` and `mac`

You have two auth options for `eco-ble`:

- easiest: leave `provider.auth.user_id` empty and use `email/password`
- stricter/local after bootstrap: resolve your EcoFlow `user_id` once, then store that instead of the cloud password

To obtain `user_id` directly:

1. Base64-encode your EcoFlow password:

   ```sh
   printf '%s' 'YOUR_PASSWORD' | base64
   ```

2. Call the same login API shape used by this daemon and extract `data.user.userId`:

   ```sh
   curl -s https://api.ecoflow.com/auth/login \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{
       "scene":"IOT_APP",
       "appVersion":"1.0.0",
       "password":"BASE64_PASSWORD_HERE",
       "oauth":{"bundleId":"com.ef.EcoFlow"},
       "userType":"ECOFLOW",
       "email":"you@example.com"
     }'
   ```

For accounts that need another region, change the host from `api.ecoflow.com` to the matching region such as `api-e.ecoflow.com` or `api-cn.ecoflow.com`. The returned JSON includes `data.user.userId`.

To obtain the BLE `mac` address:

1. On Linux with BlueZ, scan while the unit is awake:

   ```sh
   bluetoothctl
   scan on
   ```

   Look for an EcoFlow advertisement name and note the address shown next to it, for example `AA:BB:CC:DD:EE:FF`.

2. If scanning is noisy, match by the device name or serial prefix:

- Delta/River units often advertise names like `EF-DP3`, `EF-RIVER3`, or similar
- the MAC can also usually be cross-checked in the EcoFlow app or on the device label

Use that address as `devices[].mac` in the config.

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

Current command coverage:

| Command / feature                     | Status          | Notes                                                                                      |
| ------------------------------------- | --------------- | ------------------------------------------------------------------------------------------ |
| `VER`                                 | implemented     | Returns daemon version string                                                              |
| `USERNAME`                            | implemented     | Simple username check                                                                      |
| `PASSWORD`                            | implemented     | Simple password check                                                                      |
| `LOGIN`                               | implemented     | Returns `OK` after auth                                                                    |
| `PRIMARY` / `MASTER`                  | implemented     | Returns `OK` after auth                                                                    |
| `FSD`                                 | implemented     | Returns `OK` after auth                                                                    |
| `LOGOUT` / `QUIT`                     | implemented     | Closes connection                                                                          |
| `PING`                                | implemented     | Returns `PONG`                                                                             |
| `LIST UPS`                            | implemented     | Basic `upsc` discovery                                                                     |
| `LIST VAR <ups>`                      | implemented     | Returns current variables                                                                  |
| `GET VAR <ups> <var>`                 | implemented     | Returns a single variable                                                                  |
| `GET UPSDESC <ups>`                   | implemented     | Returns configured description                                                             |
| `GET NUMLOGINS <ups>`                 | implemented     | Returns a fixed `1` today                                                                  |
| `GET TYPE / DESC`                     | not implemented |                                                                                            |
| `LIST CMD / RW / ENUM / RANGE`        | not implemented |                                                                                            |
| `SET VAR`                             | not implemented | Read-only daemon in this phase                                                             |
| `INSTCMD`                             | not implemented |                                                                                            |
| `STARTTLS`                            | not implemented |                                                                                            |
| `TRACKING`                            | not implemented |                                                                                            |
| strict NUT quoting/parsing edge cases | partial         | Good enough for current simple clients, not full protocol parity                           |
| full `upsmon` shutdown semantics      | not verified    | Command stubs exist, but end-to-end shutdown compatibility still needs hardware validation |

Level 1: upsc compatible (discovery and read-only variables) [*Currently Implemented*]

- LIST UPS
- LIST VAR
- GET VAR
- GET UPSDESC

Level 2: upsmon compatible (authentication and session management) [*Currently Implemented*]

- USERNAME / PASSWORD
- LOGIN
- GET NUMLOGINS
- PRIMARY / MASTER
- FSD
- LOGOUT

Level 3: admin/tool compatible (full read-only protocol) [*Yet to Implement*]

- GET TYPE / DESC
- LIST CMD / RW / ENUM / RANGE
- SET VAR
- INSTCMD

Level 4: full protocol (including TLS and tracking) [*Yet to Implement*]

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

The daemon is designed for low-RAM SBCs and keeps all state in memory, so it does not require a database or persistent logs. Use journald for logs and monitor with `journalctl -u ecoflow-ble-nutd` or similar.

## Credits

The direct BLE provider in this repo was built from protocol study and interoperability work based heavily on [`rabits/ha-ef-ble`](https://github.com/rabits/ha-ef-ble).

In particular, this project follows that repo's public research for:

- EcoFlow manufacturer-data parsing
- session/auth flows including `encrypt_type` `0`, `1`, and `7`
- packet framing and CRC behavior
- modern Delta/River device-family mapping
- V2 and V3 telemetry structure selection

If you use this daemon, you should also star and review the upstream `ha-ef-ble` project because a lot of the protocol groundwork came from there.

## License

This repo is licensed under `GNU GPL v3`. See `LICENSE`.

Why `GPLv3` fits this project:

- it keeps redistributed derivatives and modifications under the same copyleft license
- it matches the goal of keeping protocol and interoperability improvements open
- it is still compatible with the project informed by Apache-2.0 licensed upstream: `rabits/ha-ef-ble`, as long as the required attribution and notices are preserved

This is a clean Go implementation, but it was clearly informed by the public protocol work in `ha-ef-ble`, so keeping attribution and license notices intact matters.

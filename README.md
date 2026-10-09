# Ecoflow-BLE-NUTd

A small NUT-compatible daemon that exposes EcoFlow power stations as virtual UPS devices over local Bluetooth LE.

It runs on Linux with BlueZ, keeps runtime state in memory, and supports low-RAM hosts such as Raspberry Pi. The optional web UI shows device status and can pause or resume the collector. No database, Home Assistant, or MQTT service is required.

## Get started

| Task                                         | Guide                                            |
| -------------------------------------------- | ------------------------------------------------ |
| Deploy with Docker Compose                   | [Docker deployment](deploy/docker/README.md)     |
| Install a native binary or systemd service   | [Native deployment](docs/deployment.md)          |
| Configure the daemon and web UI              | [Configuration](docs/configuration.md)           |
| Find the EcoFlow user ID and BLE MAC address | [BLE authentication](docs/ble-authentication.md) |

Use the EcoFlow app account associated with the device. A Developer account is not required. Docker setup is under [`deploy/docker/`](deploy/docker/README.md); native installation scripts and service files remain under `scripts/` and `systemd/`.

## Supported scope

The daemon supports the DELTA 2 and RIVER 2 families, DELTA 3 family, DELTA Pro 3, DELTA Pro Ultra, and RIVER 3. Device profiles are selected from the advertised serial prefix. It implements authentication types `0`, `1`, and `7`, read-only telemetry, multiple virtual UPS devices, and stale-device reporting with `ups.status=WAIT`.

The NUT server supports discovery, variable queries, and basic authentication/session commands. Full `upsmon` shutdown compatibility remains unverified. Legacy V1 devices, V4 devices, control writes, TLS, and full NUT protocol parity are not implemented.

See [supported devices and current scope](docs/supported-devices.md) and [NUT protocol coverage](docs/nut-protocol.md) for details.

## Development

From a source checkout:

```sh
go build -o ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd
go test ./...
```

The `mock` and `json-dir` providers allow development without BLE hardware. See the [development guide](docs/development.md) for local runs, the dev container, and container smoke tests. See [CI and releases](docs/releases.md) for packaging and architecture targets.

## Documentation

Browse the [documentation index](docs/README.md) or [deployment index](deploy/README.md). Authentication errors, are covered in [BLE troubleshooting](docs/ble-authentication.md#authentication-troubleshooting).

Read the [security policy](SECURITY.md) for vulnerability reporting, trust boundaries, and known protocol limitations.

## Credits and license

The BLE implementation is informed by the public protocol and interoperability research in [`rabits/ha-ef-ble`](https://github.com/rabits/ha-ef-ble), including manufacturer data, session authentication, framing, device families, and telemetry structures.

This project is licensed under GNU GPL v3. See [LICENSE](LICENSE). Preserve the upstream attribution and license notices when redistributing.

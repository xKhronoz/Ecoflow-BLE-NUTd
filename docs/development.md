# Development and verification

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

Run the following commands from a repository checkout. Use the Go version declared in `go.mod`.

## Build

```sh
go build -o ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd
```

For release bundles described in the [release guide](releases.md):

```sh
./scripts/build-release-artifacts.sh v0.1.0
```

## Development container

The repo includes a `.devcontainer/` setup for Go 1.26 development with `nc` and NUT client tools installed.

Open the project in a dev container, then build normally:

```sh
go build ./...
```

The container is useful for daemon and protocol work, mock-provider testing, and JSON-dir integration work. Direct host BLE access may still depend on the host platform and container runtime.

## Run locally

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

For a real BLE session, edit the configuration first and run on Linux with BlueZ. The `mock` and `json-dir` providers support development without a power station. `upsc` read-only access requires a configuration that permits unauthenticated reads; the shown netcat exchange supplies credentials for the shipped example.

## Checks

```sh
go vet ./...
go test -race ./...
```

The CI workflow also checks Go formatting and cross-builds Linux AMD64, ARM64, and ARMv7 binaries. Its container job validates Compose, builds the production image, and checks NUT telemetry, web status, health, and clean shutdown.

For the container smoke test, run from the repository root:

```sh
docker build -f deploy/docker/Dockerfile -t ecoflow-ble-nutd:local .
./deploy/docker/test-image.sh ecoflow-ble-nutd:local
```

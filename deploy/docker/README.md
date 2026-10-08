# Docker deployment

[Project overview](../../README.md) · [Documentation index](../../docs/README.md) · [Deployment options](../README.md)

This directory contains the production [Dockerfile](Dockerfile), [Compose configuration](compose.yaml), [daemon configuration example](ecoflow-ble-nutd.conf.example), and [environment example](.env.example).

## Requirements

Use a source checkout on a Linux host with BlueZ, a powered BLE adapter such as `hci0`, a rootful Docker engine, and Docker Compose. Docker image builds require the Go source at the repository root; native release tarballs provide precompiled binaries instead.

The image supports Linux AMD64, ARM64, and ARMv7. A Raspberry Pi 5 with a 64-bit OS builds the ARM64 image automatically. Docker Desktop on macOS/Windows can run the mock provider for testing, but does not expose the physical host's BlueZ service through this setup.

On the Linux Docker host, verify the adapter and system bus:

```sh
bluetoothctl show
test -S /run/dbus/system_bus_socket
```

## Prepare configuration

From the repository root:

```sh
cd deploy/docker
cp ecoflow-ble-nutd.conf.example ecoflow-ble-nutd.conf
chmod 600 ecoflow-ble-nutd.conf
nano ecoflow-ble-nutd.conf
```

Set the device's BLE MAC address and change the NUT/web password. Either set `provider.auth.user_id` or enter the EcoFlow app account's email and password. See [BLE authentication and discovery](../../docs/ble-authentication.md) and [configuration](../../docs/configuration.md).

The example listens on `0.0.0.0:3493` and `0.0.0.0:8080` inside the container. Compose controls how those listeners are exposed on the host.

## Start and verify

Run these commands from `deploy/docker/`:

```sh
docker compose config --quiet
docker compose up -d --build
docker compose ps
docker compose logs -f --tail=100
```

Compose builds from the repository root using this directory's Dockerfile. The configuration file and host D-Bus directory must already exist; Compose does not create missing bind sources.

The health check verifies that the NUT server responds. Check logs for `eco-ble authenticated` and the web UI for fresh telemetry to verify BLE operation. A container can be healthy while the device is disconnected or authentication is retrying.

## Ports and web access

The NUT service is published on host port `3493`. The web UI is available at `http://127.0.0.1:8080` on the Docker host and requires the configured NUT credentials.

To open that local web port from another computer:

```sh
ssh -L 8080:127.0.0.1:8080 pi@raspberrypi
```

Then open `http://127.0.0.1:8080` on that computer.

## Environment overrides

Run from `deploy/docker/`:

```sh
cp .env.example .env
nano .env
```

| Variable | Default | Purpose |
| --- | --- | --- |
| `TZ` | `UTC` | Container timezone, such as `Asia/Singapore` |
| `ECOFLOW_CONFIG` | `./ecoflow-ble-nutd.conf` | Host configuration file; relative paths resolve from this directory |
| `DBUS_HOST_PATH` | `/run/dbus` | Host system D-Bus directory |
| `NUT_BIND_ADDRESS` | `0.0.0.0` | Host interface for NUT clients |
| `NUT_PORT` | `3493` | Published host NUT port |
| `WEB_BIND_ADDRESS` | `127.0.0.1` | Host interface for the web UI |
| `WEB_PORT` | `8080` | Published host web port |

The port variables change host ports; keep the example's container listeners at `3493` and `8080`. The local configuration and `.env` are excluded from Git and the Docker build context.

## Host integration and runtime

Compose uses the host's BlueZ service through a read-only `/run/dbus` directory mount. Keep Bluetooth and the system bus running on the host. The backend talks over D-Bus and does not require raw Bluetooth device mounts or host networking.

The image defaults to a non-root user. The BLE Compose service uses UID 0 to read a host-owned `0600` configuration and work with restrictive host D-Bus policies. It drops all Linux capabilities, prevents privilege escalation, and uses a read-only filesystem with a small temporary filesystem.

The service restarts unless explicitly stopped. Logs rotate at 5 MB per file with two files retained. Runtime state stays in memory; no database or persistent data volume is required.

## Updating and stopping

After updating the source, run from `deploy/docker/` to rebuild and include code or embedded key-table fixes:

```sh
docker compose up -d --build --force-recreate
```

After editing configuration or `.env`, recreate the container:

```sh
docker compose up -d --force-recreate
```

Stop the deployment:

```sh
docker compose down
```

The configuration and `.env` files stay on the host. If migrating from the earlier repository-root layout, an existing root configuration can be reused by setting `ECOFLOW_CONFIG=../../ecoflow-ble-nutd.conf` in this directory's `.env`.

## Image verification without BLE hardware

Run from the repository root:

```sh
docker build -f deploy/docker/Dockerfile -t ecoflow-ble-nutd:local .
./deploy/docker/test-image.sh ecoflow-ble-nutd:local
```

The [smoke-test script](test-image.sh) runs a temporary mock-provider container, checks health, NUT telemetry, web status, and clean SIGTERM shutdown, then removes it. It does not validate a physical BLE connection.

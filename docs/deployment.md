# Deployment methods

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment files](../deploy/README.md)

Choose how to run the daemon on your host. The `eco-ble` provider requires Linux with BlueZ and a working Bluetooth LE adapter, such as `hci0`. Docker Desktop on macOS or Windows can run the mock provider for testing, but this deployment does not expose the host’s Bluetooth adapter.

| Method | When to use it | Guide |
| --- | --- | --- |
| Native binary with systemd | Run directly on a Linux host, start automatically at boot, and manage the service with systemctl and journald | [Native and systemd deployment](../deploy/systemd/README.md) |
| Docker Compose | Manage the daemon as a container using a published GHCR/Docker Hub image or a local source build | [Docker deployment](../deploy/docker/README.md) |
| Native binary without systemd | Run in the foreground for troubleshooting, or use your own service manager on a Linux host without systemd | [Run without systemd](../deploy/systemd/README.md#run-without-systemd) |

The native guide covers both precompiled release archives and source builds, installation, service operation, and removal. The Docker guide covers configuration, host Bluetooth integration, published images, local builds, updates, and checks.

## Before deployment

1. Set up your EcoFlow account user ID and device BLE addresses using the [BLE authentication and discovery guide](ble-authentication.md).
2. Review the [configuration guide](configuration.md) for device settings, NUT credentials, and the optional web UI.
3. Choose the native release architecture or container image tag using the [release guide](releases.md). Linux AMD64, ARM64, and ARMv7 targets are available through the release workflow.
4. Follow the installation and verification steps in your chosen deployment guide. Confirm authentication and fresh telemetry; a running service or healthy container alone does not confirm the BLE connection.

For development without BLE hardware, see the mock and JSON-directory providers in the [development guide](development.md).

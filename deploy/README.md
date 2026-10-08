# Deployment

[Project overview](../README.md) · [Documentation index](../docs/README.md)

The direct BLE provider runs on Linux with BlueZ and a working adapter such as `hci0`.

| Method | Guide | Files |
| --- | --- | --- |
| Docker Compose | [Docker deployment](docker/README.md) | [Dockerfile](docker/Dockerfile), [Compose](docker/compose.yaml), [configuration example](docker/ecoflow-ble-nutd.conf.example), [environment example](docker/.env.example) |
| Native binary / systemd | [Native deployment](../docs/deployment.md) | [Configuration example](../examples/ecoflow-ble-nutd.conf), [service unit](../systemd/ecoflow-ble-nutd.service), [installer](../scripts/install-systemd.sh) |

Set up the account and device using the [BLE authentication guide](../docs/ble-authentication.md). See [configuration](../docs/configuration.md) for common daemon settings and [release targets](../docs/releases.md#published-release-targets) for CPU architectures.

Docker image builds require a source checkout. Native release archives contain the precompiled binary, installation files, and these guides.

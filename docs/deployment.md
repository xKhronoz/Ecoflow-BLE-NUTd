# Native Linux deployment

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

Use a Linux host with BlueZ and a working BLE adapter for the `eco-ble` provider. This guide covers native binaries and systemd; the [Docker guide](../deploy/docker/README.md) covers container deployment.

## Install from a source checkout

Run from the repository root:

```sh
go build -o ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd
cp examples/ecoflow-ble-nutd.conf ecoflow-ble-nutd.conf
chmod 600 ecoflow-ble-nutd.conf
nano ecoflow-ble-nutd.conf
sudo ./scripts/install-systemd.sh --binary ./ecoflow-ble-nutd --config ./ecoflow-ble-nutd.conf
```

Set the account credentials and device MAC addresses before installation. The installer keeps an existing `/etc/ecoflow-ble-nutd.conf`. See [configuration](configuration.md) and [BLE authentication](ble-authentication.md).

## Install a release binary

Choose the architecture from the [release target table](releases.md#published-release-targets). This example uses AMD64; substitute `arm64` for a Raspberry Pi 5 with a 64-bit OS, or `armv7` for a 32-bit ARMv7 host.

```sh
wget https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-amd64.tar.gz
mkdir -p ecoflow-ble-nutd-install
tar -xzf ecoflow-ble-nutd-linux-amd64.tar.gz -C ecoflow-ble-nutd-install --strip-components=1
cd ecoflow-ble-nutd-install
cp ecoflow-ble-nutd.conf.example ecoflow-ble-nutd.conf
chmod 600 ecoflow-ble-nutd.conf
nano ecoflow-ble-nutd.conf
sudo ./install-systemd.sh --config ./ecoflow-ble-nutd.conf
```

The rolling `latest` asset names remain stable across releases. For a fixed version, download a versioned asset from [GitHub Releases](https://github.com/xkhronoz/ecoflow-ble-nutd/releases).

The bundled installer detects the binary, service unit, and example configuration in the extracted release directory. Available flags include `--binary PATH`, `--config PATH`, `--skip-config`, and `--no-start`.

## Run without systemd

From an extracted release directory with an edited configuration:

```sh
./ecoflow-ble-nutd -config ./ecoflow-ble-nutd.conf
```

Non-systemd hosts can run the binary directly; the systemd installer is for systemd hosts.

## Service operation

```sh
sudo systemctl status ecoflow-ble-nutd
sudo journalctl -u ecoflow-ble-nutd -f
sudo systemctl restart ecoflow-ble-nutd
```

After updating the binary or configuration, restart the service. Check for `eco-ble authenticated` and fresh telemetry in the web UI.

The native runtime uses `/usr/local/bin/ecoflow-ble-nutd` and `/etc/ecoflow-ble-nutd.conf`. Runtime state stays in memory. `/run/ecoflow-ble-nutd` is useful for optional JSON-provider inputs; it is not a database or required BLE data directory. Logs go to journald; the host's journald settings determine retention.

## Uninstall

From a source checkout:

```sh
sudo ./scripts/uninstall-systemd.sh
```

From an extracted release directory, use `sudo ./uninstall-systemd.sh`. Add `--remove-binary --remove-config` only when you also want to delete the installed binary and configuration.

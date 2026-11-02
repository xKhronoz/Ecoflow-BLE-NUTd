# Native Linux deployment

[Project overview](../../README.md) · [Documentation index](../../docs/README.md) · [Deployment options](../README.md)

Use a Linux host with BlueZ and a working BLE adapter for the `eco-ble` provider. This guide covers native binaries and systemd; the [Docker guide](../docker/README.md) covers container deployment.

The service requires systemd 247 or newer (`systemctl --version`). The config stays root-owned with mode `0600`; systemd's [LoadCredential](https://www.freedesktop.org/software/systemd/man/247/systemd.exec.html#Credentials) gives the service's dynamic user a private, read-only copy at startup.

The [service unit](ecoflow-ble-nutd.service), [installer](install-systemd.sh), and [uninstaller](uninstall-systemd.sh) live under `deploy/systemd/`. Release archives also bundle these files at the archive root.

## Install from a source checkout

Run from the repository root:

```sh
go build -o ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd
cp examples/ecoflow-ble-nutd.conf ecoflow-ble-nutd.conf
chmod 600 ecoflow-ble-nutd.conf
nano ecoflow-ble-nutd.conf
sudo ./deploy/systemd/install-systemd.sh --binary ./ecoflow-ble-nutd --config ./ecoflow-ble-nutd.conf
```

Set the account credentials and device MAC addresses before installation. The installer keeps an existing `/etc/ecoflow-ble-nutd.conf`. See [configuration](../../docs/configuration.md) and [BLE authentication](../../docs/ble-authentication.md).

## Install a release binary

Choose the architecture from the [release target table](../../docs/releases.md#published-release-targets). This example uses AMD64; substitute `arm64` for a Raspberry Pi 5 with a 64-bit OS, or `armv7` for a 32-bit ARMv7 host.

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

Both source and release installations keep an existing `/etc/ecoflow-ble-nutd.conf`. Use `--skip-config` to install only the binary and service unit. Use `--no-start` to install and enable the service without starting or restarting it.

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

### Fix config permission errors from an earlier installation

If the journal reports `open /etc/ecoflow-ble-nutd.conf: permission denied`, install the updated service unit from this checkout:

```sh
sudo install -m 0644 deploy/systemd/ecoflow-ble-nutd.service /etc/systemd/system/ecoflow-ble-nutd.service
sudo systemctl daemon-reload
sudo systemctl restart ecoflow-ble-nutd
sudo systemctl --no-pager --full status ecoflow-ble-nutd
```

From an updated release archive, use `ecoflow-ble-nutd.service` as the source path. Alternatively, rerun the updated installer with `--skip-config`; it replaces the service unit and preserves the existing config. Keep `/etc/ecoflow-ble-nutd.conf` root-owned with mode `0600`, since it contains credentials.

The native runtime uses `/usr/local/bin/ecoflow-ble-nutd` and `/etc/ecoflow-ble-nutd.conf`. Runtime state stays in memory. `/run/ecoflow-ble-nutd` is useful for optional JSON-provider inputs; it is not a database or required BLE data directory. Logs go to journald; the host's journald settings determine retention.

## Uninstall

From a source checkout:

```sh
sudo ./deploy/systemd/uninstall-systemd.sh
```

From an extracted release directory, use `sudo ./uninstall-systemd.sh`. By default, uninstalling disables and stops the service and removes its unit; the binary and configuration are retained. Add `--remove-binary --remove-config` only when you also want to delete the installed binary and configuration.

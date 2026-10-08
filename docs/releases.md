# CI and releases

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

This repo now expects two GitHub Actions workflows:

- `CI`: runs `gofmt` checks, `go vet`, `go test ./...`, Linux cross-builds for `amd64`, `arm64`, and `armv7`, and a production Docker build/smoke test
- `Release`: runs on tags like `v0.1.0` or manually, builds release tarballs, generates `SHA256SUMS`, and publishes a GitHub Release

The release archives include:

- `ecoflow-ble-nutd`
- `ecoflow-ble-nutd.service`
- `ecoflow-ble-nutd.conf.example`
- `install-systemd.sh`
- `uninstall-systemd.sh`
- `README.md`, `SECURITY.md`, `docs/`, and deployment guides under `deploy/`
- source-layout examples and native installer/service files for documentation links
- `LICENSE`

## Published release targets

| Artifact suffix | Linux CPU target        | Typical use case                   |
| --------------- | ----------------------- | ---------------------------------- |
| `linux-amd64`   | x86_64 / amd64          | PCs, servers, Intel/AMD mini PCs   |
| `linux-arm64`   | aarch64 / arm64         | 64-bit ARM SBCs                    |
| `linux-armv7`   | 32-bit ARMv7 hard-float | Raspberry Pi OS 32-bit class hosts |

Notes:

- the tarballs are for Linux only
- the bundled install script expects a `systemd`-based Linux host
- the `eco-ble` provider expects Linux with BlueZ and a working BLE adapter such as `hci0`
- non-`systemd` Linux hosts can still run the binary manually, but should not use the bundled `systemd` installer
- versioned `v*` releases are immutable; the `latest` tag and release are intentionally moved forward on each new release

To cut a release from GitHub, push a tag such as:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Or run the `Release` workflow manually and pass a version starting with `v`.

For automation that should always pull the newest published release, use the rolling `latest` release with stable asset names:

```text
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-amd64.tar.gz
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-arm64.tar.gz
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-armv7.tar.gz
```

Those `latest` asset names stay stable across releases. The versioned releases continue to use versioned filenames such as `ecoflow-ble-nutd-v0.1.0-linux-amd64.tar.gz`.

Docker builds require a source checkout. Release tarballs provide precompiled native binaries; follow [native deployment](deployment.md) to install them. The included Docker guide explains the source-checkout deployment separately.

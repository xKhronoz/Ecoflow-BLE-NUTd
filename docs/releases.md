# CI and releases

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

This repo now expects two GitHub Actions workflows:

- `CI`: runs `gofmt` checks, `go vet`, `go test ./...`, Linux cross-builds for `amd64`, `arm64`, and `armv7`, and a production Docker build/smoke test
- `Release`: runs on tags like `v0.1.0` or manually, builds release tarballs, generates `SHA256SUMS`, publishes GitHub Releases, then smoke-tests and publishes multi-platform images to GHCR and Docker Hub

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

Or run the `Release` workflow manually and pass a version such as `v0.1.0`. Versions must use `vMAJOR.MINOR.PATCH`, optionally followed by a prerelease suffix such as `-rc.1`. Manual runs release the selected ref; select the intended release commit or tag.

## Container image publishing

The `publish-images` job runs after the native release job succeeds. It builds and smoke-tests the Linux AMD64 image, then builds and pushes manifests for `linux/amd64`, `linux/arm64`, and `linux/arm/v7` to both registries.

Configure these repository secrets under **Settings → Secrets and variables → Actions** before releasing:

| Setting | Purpose |
| --- | --- |
| Secret `DOCKERHUB_USERNAME` | Docker Hub login username |
| Secret `DOCKERHUB_TOKEN` | Docker Hub access token with write access to the image repository |
| Optional variable `DOCKERHUB_NAMESPACE` | Docker Hub organization or namespace; defaults to the login username |

Create the Docker Hub repository `<namespace>/ecoflow-ble-nutd` and grant the login account write access. GHCR authentication uses `GITHUB_TOKEN` with job-scoped `packages: write`; no additional GHCR token is required. If a GHCR package already exists, ensure this repository has Actions access to it. Make the GHCR package public if users should pull without authentication.

Browse release images on [GHCR container package](https://github.com/xKhronoz/Ecoflow-BLE-NUTd/pkgs/container/ecoflow-ble-nutd) and [Docker Hub](https://hub.docker.com/r/khronoz/ecoflow-ble-nutd). Native binary archives are available from [GitHub Releases](https://github.com/xKhronoz/Ecoflow-BLE-NUTd/releases).

Published image names and example version tags:

```text
ghcr.io/xkhronoz/ecoflow-ble-nutd:v0.2.0
docker.io/khronoz/ecoflow-ble-nutd:v0.2.0
```

GHCR uses the lowercase GitHub repository name, so forks publish under their own owner. Stable versions also update `latest` in both registries. Prereleases publish only their explicit version tag and do not replace the image `latest` tag. The existing rolling native `latest` release still follows every release, including prereleases.

Missing Docker Hub credentials fail the image job with a setup message; the native release will already have been published. After correcting credentials or registry permissions, rerun the failed image job. CI and pull requests build/test containers without publishing.

The workflow follows Docker's [multi-platform build](https://docs.docker.com/build/ci/github-actions/multi-platform/) and [test-before-push](https://docs.docker.com/build/ci/github-actions/test-before-push/) patterns and GitHub's [GHCR authentication guidance](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

For automation that should always pull the newest published release, use the rolling `latest` release with stable asset names:

```text
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-amd64.tar.gz
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-arm64.tar.gz
https://github.com/xkhronoz/ecoflow-ble-nutd/releases/download/latest/ecoflow-ble-nutd-linux-armv7.tar.gz
```

Those `latest` asset names stay stable across releases. The versioned releases continue to use versioned filenames such as `ecoflow-ble-nutd-v0.1.0-linux-amd64.tar.gz`.

Docker builds require a source checkout. Release tarballs provide precompiled native binaries; follow [native deployment](../deploy/systemd/README.md) to install them. The included Docker guide explains the source-checkout deployment separately.

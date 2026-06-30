#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "" ]; then
  echo "usage: $0 <version>" >&2
  exit 1
fi

version="$1"
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
dist_dir="${repo_root}/dist/release"

rm -rf "${dist_dir}"
mkdir -p "${dist_dir}"

build_target() {
  local goarch="$1"
  local suffix="$2"
  local goarm="${3:-}"
  local stage_dir="${dist_dir}/ecoflow-ble-nutd-${version}-linux-${suffix}"
  local binary_path="${stage_dir}/ecoflow-ble-nutd"

  mkdir -p "${stage_dir}"

  if [ -n "${goarm}" ]; then
    GOOS=linux GOARCH="${goarch}" GOARM="${goarm}" \
      go build -trimpath -ldflags="-s -w" \
      -o "${binary_path}" ./cmd/ecoflow-ble-nutd
  else
    GOOS=linux GOARCH="${goarch}" \
      go build -trimpath -ldflags="-s -w" \
      -o "${binary_path}" ./cmd/ecoflow-ble-nutd
  fi

  install -m 0644 "${repo_root}/README.md" "${stage_dir}/README.md"
  install -m 0644 "${repo_root}/LICENSE" "${stage_dir}/LICENSE"
  install -m 0644 "${repo_root}/examples/ecoflow-ble-nutd.conf" "${stage_dir}/ecoflow-ble-nutd.conf.example"
  install -m 0644 "${repo_root}/systemd/ecoflow-ble-nutd.service" "${stage_dir}/ecoflow-ble-nutd.service"
  install -m 0755 "${repo_root}/scripts/install-systemd.sh" "${stage_dir}/install-systemd.sh"
  install -m 0755 "${repo_root}/scripts/uninstall-systemd.sh" "${stage_dir}/uninstall-systemd.sh"

  tar -C "${dist_dir}" -czf "${dist_dir}/ecoflow-ble-nutd-${version}-linux-${suffix}.tar.gz" \
    "ecoflow-ble-nutd-${version}-linux-${suffix}"
  rm -rf "${stage_dir}"
}

cd "${repo_root}"
build_target amd64 amd64
build_target arm64 arm64
build_target arm armv7 7

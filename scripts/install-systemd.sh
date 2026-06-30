#!/usr/bin/env bash
set -euo pipefail

service_name="ecoflow-ble-nutd"
script_dir="$(cd "$(dirname "$0")" && pwd)"
binary_src="./ecoflow-ble-nutd"
config_src=""
install_config=1
start_service=1

usage() {
  cat <<'EOF'
usage: install-systemd.sh [options]

options:
  --binary PATH       Path to the ecoflow-ble-nutd binary to install.
  --config PATH       Path to the config file to install.
  --skip-config       Do not install the config file.
  --no-start          Enable the service but do not start or restart it.
  --help              Show this help text.

This script installs:
  /usr/local/bin/ecoflow-ble-nutd
  /etc/systemd/system/ecoflow-ble-nutd.service
  /etc/ecoflow-ble-nutd.conf
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --binary)
      binary_src="$2"
      shift 2
      ;;
    --config)
      config_src="$2"
      shift 2
      ;;
    --skip-config)
      install_config=0
      shift
      ;;
    --no-start)
      start_service=0
      shift
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  echo "please run as root, for example with sudo" >&2
  exit 1
fi

if [ ! -f "${binary_src}" ] && [ -f "${script_dir}/ecoflow-ble-nutd" ]; then
  binary_src="${script_dir}/ecoflow-ble-nutd"
fi

if [ ! -f "${binary_src}" ]; then
  echo "binary not found: ${binary_src}" >&2
  exit 1
fi

if [ -z "${config_src}" ]; then
  if [ -f "${script_dir}/ecoflow-ble-nutd.conf.example" ]; then
    config_src="${script_dir}/ecoflow-ble-nutd.conf.example"
  else
    config_src="${script_dir}/../examples/ecoflow-ble-nutd.conf"
  fi
fi

if [ -f "${script_dir}/${service_name}.service" ]; then
  service_src="${script_dir}/${service_name}.service"
else
  service_src="${script_dir}/../systemd/${service_name}.service"
fi

if [ ! -f "${service_src}" ]; then
  echo "service file not found: ${service_src}" >&2
  exit 1
fi

install -D -m 0755 "${binary_src}" "/usr/local/bin/${service_name}"
install -D -m 0644 "${service_src}" "/etc/systemd/system/${service_name}.service"

if [ "${install_config}" -eq 1 ]; then
  if [ ! -f "${config_src}" ]; then
    echo "config file not found: ${config_src}" >&2
    exit 1
  fi
  if [ -e "/etc/${service_name}.conf" ]; then
    echo "keeping existing config: /etc/${service_name}.conf"
  else
    install -D -m 0600 "${config_src}" "/etc/${service_name}.conf"
  fi
fi

systemctl daemon-reload
systemctl enable "${service_name}.service"

if [ "${start_service}" -eq 1 ]; then
  systemctl restart "${service_name}.service"
else
  echo "service enabled but not started"
fi

systemctl --no-pager --full status "${service_name}.service" || true

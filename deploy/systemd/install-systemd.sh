#!/usr/bin/env bash
set -euo pipefail

service_name="ecoflow-ble-nutd"
script_dir="$(cd "$(dirname "$0")" && pwd)"
binary_src="./ecoflow-ble-nutd"
config_src=""
install_config=1
replace_config=0
start_service=1
config_dst="/etc/${service_name}.conf"
config_tmp=""

cleanup() {
  if [ -n "${config_tmp}" ]; then
    rm -f "${config_tmp}"
  fi
}
trap cleanup EXIT

usage() {
  cat <<'EOF'
usage: install-systemd.sh [options]

options:
  --binary PATH       Path to the ecoflow-ble-nutd binary to install.
  --config PATH       Path to the config file to install.
  --replace-config    Replace an existing config with --config PATH, keeping a backup.
  --skip-config       Do not install the config file.
  --no-start          Enable the service but do not start or restart it.
  --help              Show this help text.

This script installs:
  /usr/local/bin/ecoflow-ble-nutd
  /etc/systemd/system/ecoflow-ble-nutd.service
  /etc/ecoflow-ble-nutd.conf

Requires systemd 247 or newer and systemd-sysusers.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --binary)
      if [ $# -lt 2 ] || [ -z "$2" ]; then
        echo "--binary requires a path" >&2
        exit 1
      fi
      binary_src="$2"
      shift 2
      ;;
    --config)
      if [ $# -lt 2 ] || [ -z "$2" ]; then
        echo "--config requires a path" >&2
        exit 1
      fi
      config_src="$2"
      shift 2
      ;;
    --replace-config)
      replace_config=1
      shift
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

if [ "${replace_config}" -eq 1 ]; then
  if [ "${install_config}" -eq 0 ]; then
    echo "--replace-config cannot be combined with --skip-config" >&2
    exit 1
  fi
  if [ -z "${config_src}" ]; then
    echo "--replace-config requires --config PATH" >&2
    exit 1
  fi
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "please run as root, for example with sudo" >&2
  exit 1
fi

systemd_version="$(systemctl --version | awk 'NR == 1 { print $2 }')"
case "${systemd_version}" in
  ''|*[!0-9]*)
    echo "could not determine systemd version" >&2
    exit 1
    ;;
esac
if [ "${systemd_version}" -lt 247 ]; then
  echo "systemd 247 or newer is required for LoadCredential (found ${systemd_version})" >&2
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
    config_src="${script_dir}/../../examples/ecoflow-ble-nutd.conf"
  fi
fi

service_src="${script_dir}/${service_name}.service"

if [ ! -f "${service_src}" ]; then
  echo "service file not found: ${service_src}" >&2
  exit 1
fi

if [ "${install_config}" -eq 1 ] && [ ! -f "${config_src}" ]; then
  echo "config file not found: ${config_src}" >&2
  exit 1
fi

# A static account is resolvable by dbus-daemon even when it bypasses
# systemd's dynamic-user NSS lookups. sysusers preserves existing accounts.
systemd-sysusers - <<'EOF'
u ecoflow-ble-nutd - "EcoFlow BLE NUT daemon" /nonexistent
EOF

install -D -m 0755 "${binary_src}" "/usr/local/bin/${service_name}"
install -D -m 0644 "${service_src}" "/etc/systemd/system/${service_name}.service"

if [ "${install_config}" -eq 1 ]; then
  if [ -e "${config_dst}" ] && [ "${replace_config}" -eq 0 ]; then
    echo "keeping existing config: ${config_dst} (use --config PATH --replace-config to replace it)"
  elif [ "${config_src}" -ef "${config_dst}" ]; then
    echo "config already installed at ${config_dst}"
  else
    if [ -e "${config_dst}" ]; then
      config_backup="$(mktemp "${config_dst}.bak.XXXXXX")"
      install -m 0600 "${config_dst}" "${config_backup}"
      echo "config backup: ${config_backup}"
    fi
    # systemd passes a private copy to the service user via LoadCredential.
    config_tmp="$(mktemp "${config_dst}.tmp.XXXXXX")"
    install -m 0600 "${config_src}" "${config_tmp}"
    mv -f "${config_tmp}" "${config_dst}"
    config_tmp=""
    echo "installed config: ${config_dst}"
  fi
fi

systemctl daemon-reload
systemctl enable "${service_name}.service"

if [ "${start_service}" -eq 1 ]; then
  systemctl restart "${service_name}.service"
else
  echo "service enabled but not restarted; configuration changes apply on the next start or restart"
fi

systemctl --no-pager --full status "${service_name}.service" || true

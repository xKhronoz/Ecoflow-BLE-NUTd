#!/usr/bin/env bash
set -euo pipefail

service_name="ecoflow-ble-nutd"
remove_binary=0
remove_config=0

usage() {
  cat <<'EOF'
usage: uninstall-systemd.sh [options]

options:
  --remove-binary     Remove /usr/local/bin/ecoflow-ble-nutd
  --remove-config     Remove /etc/ecoflow-ble-nutd.conf
  --help              Show this help text.

By default this script removes only the systemd unit and disables the service.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --remove-binary)
      remove_binary=1
      shift
      ;;
    --remove-config)
      remove_config=1
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

systemctl disable --now "${service_name}.service" 2>/dev/null || true
rm -f "/etc/systemd/system/${service_name}.service"
systemctl daemon-reload

if [ "${remove_binary}" -eq 1 ]; then
  rm -f "/usr/local/bin/${service_name}"
fi

if [ "${remove_config}" -eq 1 ]; then
  rm -f "/etc/${service_name}.conf"
fi

echo "removed ${service_name} systemd unit"

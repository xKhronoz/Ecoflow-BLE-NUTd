#!/usr/bin/env bash
set -euo pipefail

image="${1:-ecoflow-ble-nutd:local}"
test_dir="$(mktemp -d)"
container_name="ecoflow-smoke-$(basename "${test_dir}" | tr '[:upper:]' '[:lower:]')"
cleanup() {
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
  rm -rf "${test_dir}"
}
trap cleanup EXIT

cat > "${test_dir}/ecoflow-ble-nutd.conf" <<'EOF'
listen: 0.0.0.0:3493
web:
  enable: true
  listen: 0.0.0.0:8080
provider:
  type: mock
  poll_seconds: 1
devices:
  - name: testups
EOF
chmod 644 "${test_dir}/ecoflow-ble-nutd.conf"

# Exercise the non-root image default and the same restrictions as Compose.
docker run -d --name "${container_name}" \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --pids-limit 128 --health-interval 1s --health-start-period 0s \
  --mount "type=bind,source=${test_dir}/ecoflow-ble-nutd.conf,target=/etc/ecoflow-ble-nutd.conf,readonly" \
  "${image}" >/dev/null

for attempt in $(seq 1 20); do
  if [ "$(docker inspect --format '{{.State.Health.Status}}' "${container_name}")" = "healthy" ]; then
    break
  fi
  if [ "${attempt}" = 20 ]; then
    docker logs "${container_name}"
    echo "container did not become healthy" >&2
    exit 1
  fi
  sleep 1
done

docker exec "${container_name}" sh -c \
  "printf 'GET VAR testups ups.status\n' | nc -w 3 127.0.0.1 3493 | grep -q '^VAR testups ups.status \"OB\"'"
docker exec "${container_name}" wget -q -O - http://127.0.0.1:8080/api/status \
  | grep -q '"battery.charge"'
docker stop --time 10 "${container_name}" >/dev/null
if [ "$(docker inspect --format '{{.State.ExitCode}}' "${container_name}")" != 0 ]; then
  docker logs "${container_name}"
  echo "container did not shut down cleanly" >&2
  exit 1
fi
echo "Docker smoke test passed: health, NUT telemetry, web status, and SIGTERM."

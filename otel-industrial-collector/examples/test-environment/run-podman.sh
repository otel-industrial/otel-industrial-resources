#!/usr/bin/env bash
# Runs the test environment with plain Podman, for machines without a compose
# tool. Same containers, names and ports as compose.yaml.
#
#   cd otel-industrial-collector/examples/test-environment
#   bash run-podman.sh          # build and start
#   bash run-podman.sh down     # stop and remove everything
#
# Grafana:    http://localhost:3000
# Prometheus: http://localhost:9090

set -euo pipefail
cd "$(dirname "$0")"
REPO_ROOT="$(cd ../../.. && pwd)"
NET=otel-industrial-demo
IMAGE=otel-industrial-demo:local
CONTAINERS=(grafana gateway prometheus enip-sim modbus-sim)

down() {
  for c in "${CONTAINERS[@]}"; do podman rm -f "$NET-$c" >/dev/null 2>&1 || true; done
  podman network rm "$NET" >/dev/null 2>&1 || true
}

if [ "${1:-}" = "down" ]; then down; echo "Stopped."; exit 0; fi
down

echo "==> Building $IMAGE (the first build downloads Go modules and takes a few minutes)"
podman build -t "$IMAGE" -f Dockerfile "$REPO_ROOT"

podman network create "$NET" >/dev/null

# --network-alias gives each container the same hostname it has in compose.yaml.
run() { local name=$1; shift; podman run -d --name "$NET-$name" --network "$NET" --network-alias "$name" "$@" >/dev/null; }

echo "==> Starting containers"
run modbus-sim "$IMAGE" modbus-simulator -addr 0.0.0.0:5020
run enip-sim "$IMAGE" enip-simulator
run prometheus -p 127.0.0.1:9090:9090 \
  -v "$PWD/prometheus.yaml:/etc/prometheus/prometheus.yaml:ro" \
  prom/prometheus:v3.5.0 \
  --config.file=/etc/prometheus/prometheus.yaml --storage.tsdb.path=/prometheus --web.enable-otlp-receiver
run gateway -v "$PWD/collector.yaml:/etc/otelcol/collector.yaml:ro" \
  "$IMAGE" otelcol-industrial-gateway --config /etc/otelcol/collector.yaml
run grafana -p 127.0.0.1:3000:3000 \
  -e GF_AUTH_ANONYMOUS_ENABLED=true -e GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer \
  -e GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/var/lib/grafana/dashboards/industrial-demo.json \
  -e GF_ANALYTICS_REPORTING_ENABLED=false -e GF_ANALYTICS_CHECK_FOR_UPDATES=false \
  -e GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES=false \
  -v "$PWD/grafana/datasource.yaml:/etc/grafana/provisioning/datasources/datasource.yaml:ro" \
  -v "$PWD/grafana/dashboards.yaml:/etc/grafana/provisioning/dashboards/dashboards.yaml:ro" \
  -v "$PWD/grafana/dashboards:/var/lib/grafana/dashboards:ro" \
  grafana/grafana:11.6.0

echo "==> Running. Grafana: http://localhost:3000  Prometheus: http://localhost:9090"
echo "    Collector logs: podman logs -f $NET-gateway"
echo "    Stop:           bash run-podman.sh down"

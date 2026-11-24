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
CONTAINERS=(grafana sparkplug-sim gateway opcua-sim loki prometheus enip-sim modbus-sim)

# The OPC UA receiver project's own test server (C#), built from its repo at
# the same commit as the OCB manifests.
OPCUA_REPO=https://github.com/bruegth/opentelemetry-collector-opcua-receiver.git
OPCUA_COMMIT=91d3129184f0439fb1a744e41096e68b1bc37392
OPCUA_IMAGE=otel-industrial-demo-opcua-sim:local
OPCUA_SRC="${TMPDIR:-/tmp}/otel-industrial-demo-opcua"

down() {
  for c in "${CONTAINERS[@]}"; do podman rm -f "$NET-$c" >/dev/null 2>&1 || true; done
  podman network rm "$NET" >/dev/null 2>&1 || true
}

if [ "${1:-}" = "down" ]; then down; echo "Stopped."; exit 0; fi
down

echo "==> Building $IMAGE (the first build downloads Go modules and takes a few minutes)"
podman build -t "$IMAGE" -f Dockerfile "$REPO_ROOT"

echo "==> Building $OPCUA_IMAGE (the first build downloads the .NET SDK image)"
if [ "$(git -C "$OPCUA_SRC" rev-parse HEAD 2>/dev/null)" != "$OPCUA_COMMIT" ]; then
  rm -rf "$OPCUA_SRC"
  git clone -q "$OPCUA_REPO" "$OPCUA_SRC"
  git -C "$OPCUA_SRC" checkout -q "$OPCUA_COMMIT"
fi
podman build -t "$OPCUA_IMAGE" "$OPCUA_SRC/testserver"

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
run loki grafana/loki:3.5.0
run opcua-sim "$OPCUA_IMAGE"

# The OPC UA receiver aborts collector startup if the server isn't listening
# yet, so wait for port 4840 before starting the gateway.
echo "==> Waiting for the OPC UA test server"
ready=
for _ in $(seq 1 60); do
  if podman exec "$NET-opcua-sim" bash -c 'echo > /dev/tcp/127.0.0.1/4840' 2>/dev/null; then ready=1; break; fi
  sleep 1
done
[ -n "$ready" ] || { echo "OPC UA test server did not start; see: podman logs $NET-opcua-sim"; exit 1; }

run gateway -v "$PWD/collector.yaml:/etc/otelcol/collector.yaml:ro" \
  "$IMAGE" otelcol-industrial-gateway --config /etc/otelcol/collector.yaml
run sparkplug-sim "$IMAGE" sparkplug-simulator -broker tcp://gateway:1883
run grafana -p 127.0.0.1:3000:3000 \
  -e GF_AUTH_ANONYMOUS_ENABLED=true -e GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer \
  -e GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/var/lib/grafana/dashboards/industrial-demo.json \
  -e GF_ANALYTICS_REPORTING_ENABLED=false -e GF_ANALYTICS_CHECK_FOR_UPDATES=false \
  -e GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES=false \
  -e GF_SECURITY_DISABLE_GRAVATAR=true -e GF_NEWS_NEWS_FEED_ENABLED=false \
  -e GF_SNAPSHOTS_EXTERNAL_ENABLED=false \
  -v "$PWD/grafana/datasource.yaml:/etc/grafana/provisioning/datasources/datasource.yaml:ro" \
  -v "$PWD/grafana/dashboards.yaml:/etc/grafana/provisioning/dashboards/dashboards.yaml:ro" \
  -v "$PWD/grafana/dashboards:/var/lib/grafana/dashboards:ro" \
  grafana/grafana:11.6.0

echo "==> Running. Grafana: http://localhost:3000  Prometheus: http://localhost:9090"
echo "    Collector logs: podman logs -f $NET-gateway"
echo "    Stop:           bash run-podman.sh down"

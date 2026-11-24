# Image for the test environment: the otelcol-industrial-gateway distribution,
# built from this checkout, plus the Modbus, EtherNet/IP and Sparkplug B
# simulators.
# Build context is the repo root (compose.yaml and run-podman.sh set this).

# ---- build stage: needs internet for Go modules -----------------------------
FROM golang:1.26-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends make \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
# Only what the build needs, so editing the demo files in examples/ does not
# invalidate the cache and rebuild the collector.
COPY otel-industrial-collector/manifests/ ./otel-industrial-collector/manifests/
COPY otel-industrial-collector/receiver/ ./otel-industrial-collector/receiver/
COPY otel-industrial-collector/industrialscraper/ ./otel-industrial-collector/industrialscraper/
COPY otel-industrial-collector/internal/ ./otel-industrial-collector/internal/

# LOCAL=1 builds the in-repo receivers from this checkout instead of their
# release tags, so the demo always shows the code on your branch.
RUN make -C otel-industrial-collector/manifests clean \
 && make -C otel-industrial-collector/manifests LOCAL=1 industrial-gateway-full \
 && mkdir -p /out \
 && cp otel-industrial-collector/manifests/otelcol-industrial-gateway/otelcol-industrial-gateway /out/

RUN cd otel-industrial-collector/receiver/modbusreceiver \
 && go build -o /out/modbus-simulator ./cmd/simulator
RUN cd otel-industrial-collector/receiver/ethernetipreceiver \
 && go build -o /out/enip-simulator ./cmd/simulator

# Sparkplug B edge-node simulator. It has no go.mod in this repo: it's built in
# a throwaway module against the same sparkplug commit as the OCB manifests.
COPY otel-industrial-collector/examples/test-environment/sparkplug-simulator/main.go /sim/sparkplug/
RUN cd /sim/sparkplug \
 && go mod init sparkplug-simulator \
 && go get github.com/jmacd/opentelemetry-mqtt-sparkplug@v0.0.0-20260526170218-8540b400c6d8 \
 && go mod tidy \
 && go build -o /out/sparkplug-simulator .

# ---- run stage: no Go toolchain, unprivileged user --------------------------
FROM debian:bookworm-slim
RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin demo
COPY --from=build /out/ /usr/local/bin/
USER demo

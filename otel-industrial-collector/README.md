# OpenTelemetry Industrial Collector

OpenTelemetry Collector distributions for industrial, OT, IIoT, and legacy environments, built with the [OpenTelemetry Collector Builder (OCB)](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder), plus the receivers and shared helpers developed in this org.

📦 **Latest release:** [v0.1.0](https://github.com/otel-industrial/otel-industrial-resources/releases/tag/v0.1.0), built against OpenTelemetry Collector v0.163.0.

> [!NOTE]
> Everything here is experimental and not yet validated against real hardware. Use a lab or non-production environment only.

## Distributions

| Manifest | Binary | For |
|---|---|---|
| [`modbus-edge-gateway.yaml`](manifests/modbus-edge-gateway.yaml) | `otelcol-modbus-edge` | Modbus TCP edge gateways |
| [`rockwell-ethernetip.yaml`](manifests/rockwell-ethernetip.yaml) | `otelcol-ethernetip` | Rockwell/Allen-Bradley EtherNet/IP cells |
| [`opcua-plant-floor.yaml`](manifests/opcua-plant-floor.yaml) | `otelcol-opcua` | OPC UA plant-floor servers |
| [`iiot-sparkplug.yaml`](manifests/iiot-sparkplug.yaml) | `otelcol-sparkplug` | MQTT Sparkplug B / IIoT |
| [`industrial-gateway-full.yaml`](manifests/industrial-gateway-full.yaml) | `otelcol-industrial-gateway` | All four protocols in one multi-protocol gateway |

Prebuilt linux/amd64 and linux/arm64 binaries are attached to each [release](https://github.com/otel-industrial/otel-industrial-resources/releases). To build one yourself:

```sh
make -C otel-industrial-collector/manifests industrial-gateway-full          # from release tags
make -C otel-industrial-collector/manifests LOCAL=1 industrial-gateway-full  # from this checkout
```

## Components

| Path | What it is |
|---|---|
| [`receiver/modbusreceiver`](receiver/modbusreceiver) | Modbus TCP receiver, with a built-in simulator |
| [`receiver/ethernetipreceiver`](receiver/ethernetipreceiver) | EtherNet/IP (CIP) receiver, with a built-in simulator |
| [`industrialscraper`](industrialscraper) | Shared helper module: reconnect backoff, per-device poll limiter, busy-device backoff |
| [`internal/tools`](internal/tools) | Pinned `mdatagen` for generated code (`make generate`) |

The OPC UA and Sparkplug B receivers come from external projects: [bruegth/opentelemetry-collector-opcua-receiver](https://github.com/bruegth/opentelemetry-collector-opcua-receiver) and [jmacd/opentelemetry-mqtt-sparkplug](https://github.com/jmacd/opentelemetry-mqtt-sparkplug).

An end-to-end demo with simulated devices, Prometheus and Grafana is in progress ([#53](https://github.com/otel-industrial/otel-industrial-resources/issues/53)).

## Contributing

Ideas, use cases, requirements, and feedback are welcome, especially:

- Industrial use cases, deployment patterns and reference architectures
- New protocol integrations and distributions
- Security, networking, and operational considerations
- Feedback from manufacturers, system integrators, vendors, and end users

Open an issue to start a discussion, or pick one labeled [help wanted](https://github.com/otel-industrial/otel-industrial-resources/issues?q=is%3Aopen+label%3A%22help+wanted%22).

## Releasing

The Go modules here are versioned and released together. See [docs/release.md](../docs/release.md).

# 📚 OpenTelemetry Industrial Resources

Documentation, reference materials, and implementation guidance for OpenTelemetry adoption in industrial, OT, IoT, and legacy environments, plus the OpenTelemetry Industrial Collector and its receivers.

📦 **Latest release:** [v0.1.0](https://github.com/otel-industrial/otel-industrial-resources/releases/tag/v0.1.0): five OpenTelemetry Industrial Collector distributions (linux/amd64 + arm64), the Modbus and EtherNet/IP receivers, and the shared `industrialscraper` module, all versioned together.

> [!NOTE]
> Everything here is experimental and not yet validated against real hardware. Use a lab or non-production environment only.

## Repository Structure

### [`/otel-industrial-collector`](otel-industrial-collector)

The OpenTelemetry Industrial Collector: OCB manifests for five distributions, the in-repo receivers, and shared helpers.

| Path | What it is |
|---|---|
| [`manifests/`](otel-industrial-collector/manifests) | OCB manifests: `modbus-edge-gateway`, `rockwell-ethernetip`, `opcua-plant-floor`, `iiot-sparkplug`, `industrial-gateway-full` |
| [`receiver/modbusreceiver`](otel-industrial-collector/receiver/modbusreceiver) | Modbus TCP receiver, with a built-in simulator |
| [`receiver/ethernetipreceiver`](otel-industrial-collector/receiver/ethernetipreceiver) | EtherNet/IP (CIP) receiver for Rockwell/Allen-Bradley controllers, with a built-in simulator |
| [`industrialscraper`](otel-industrial-collector/industrialscraper) | Shared helper module: reconnect backoff, per-device poll limiter, busy-device backoff |

Prebuilt binaries are attached to each [release](https://github.com/otel-industrial/otel-industrial-resources/releases). Receivers can also be added to your own collector build with OCB:

```yaml
receivers:
  - gomod: github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/modbusreceiver v0.1.0
```

An end-to-end demo with simulated devices, Prometheus and Grafana is in progress ([#53](https://github.com/otel-industrial/otel-industrial-resources/issues/53)).

### [`/docs/otel-industrial-environments`](docs/otel-industrial-environments)

Documentation and guidance for OpenTelemetry in industrial and operational technology environments, such as [device availability alerts](docs/otel-industrial-environments/security/device-availability.md).

The long-term goal is to develop content that may eventually be proposed to the OpenTelemetry documentation under [Platforms and Environments](https://opentelemetry.io/docs/platforms/).

### [`/otel-industrial-projects`](otel-industrial-projects)

Community-maintained catalog of OpenTelemetry projects, integrations, collectors, receivers, bridges, and related efforts focused on industrial, OT, IoT, and legacy environments.

The goal is to help practitioners discover existing work, identify collaboration opportunities, avoid duplicated efforts, and better understand the current state of industrial observability within the OpenTelemetry ecosystem.

###

# Industrial Receiver Catalog

This directory is a discovery point for OpenTelemetry Collector receivers relevant to industrial, OT, and IIoT environments. Some receivers live directly in this repository; others are maintained in separate projects and are referenced here with a short summary and a link.

The goal is to make it easy to find existing work on a given protocol before starting something new. All four receivers below are included in the [OpenTelemetry Industrial Collector distributions](../manifests).

## Receivers

| Protocol | Status | Location | Distributions |
|---|---|---|---|
| EtherNet/IP | In this repository, [v0.1.0](https://github.com/otel-industrial/otel-industrial-resources/releases/tag/v0.1.0) (development: metrics) | [ethernetipreceiver](ethernetipreceiver) | `rockwell-ethernetip`, `industrial-gateway-full` |
| Modbus | In this repository, [v0.1.0](https://github.com/otel-industrial/otel-industrial-resources/releases/tag/v0.1.0) (development: metrics) | [modbusreceiver](modbusreceiver) | `modbus-edge-gateway`, `industrial-gateway-full` |
| OPC UA | External project (logs only, alpha) | [bruegth/opentelemetry-collector-opcua-receiver](https://github.com/bruegth/opentelemetry-collector-opcua-receiver) | `opcua-plant-floor`, `industrial-gateway-full` |
| MQTT / Sparkplug B | External project | [jmacd/opentelemetry-mqtt-sparkplug](https://github.com/jmacd/opentelemetry-mqtt-sparkplug) | `iiot-sparkplug`, `industrial-gateway-full` |

## Contributing

If you know of an OpenTelemetry receiver, exporter, or other component relevant to industrial protocols that isn't listed here, please open an issue or a PR adding it. A short pointer README with a link to the external project is enough — the implementation doesn't need to live in this repository.

A receiver that lives in this repository also needs its module in `versions.yaml` and in at least one manifest, or CI fails. See "Adding a module" in [docs/release.md](../../docs/release.md).

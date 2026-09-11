# Industrial Receiver Catalog

This directory is a discovery point for OpenTelemetry Collector receivers relevant to industrial, OT, and IIoT environments. Some receivers live directly in this repository; others are maintained in separate projects and are referenced here with a short summary and a link.

The goal is to make it easy to find existing work on a given protocol before starting something new, and to give a sense of what components might eventually be considered for an OpenTelemetry Industrial Collector distribution.

## Receivers

| Protocol | Status | Location |
|---|---|---|
| EtherNet/IP | Experimental — implementation pending merge | [PR #5](https://github.com/otel-industrial/otel-industrial-resources/pull/5) (`add-ethernetipreceiver` branch) |
| [Modbus](./modbusreceiver/README.md) | External project | [lukaszciukaj/modbusreceiver](https://github.com/lukaszciukaj/modbusreceiver) |
| [OPC UA](./opcuareceiver/README.md) | External project (logs only, alpha) | [bruegth/opentelemetry-collector-opcua-receiver](https://github.com/bruegth/opentelemetry-collector-opcua-receiver) |
| [MQTT / Sparkplug B](./mqttsparkplugreceiver/README.md) | External project | [jmacd/opentelemetry-mqtt-sparkplug](https://github.com/jmacd/opentelemetry-mqtt-sparkplug) |

## Contributing

If you know of an OpenTelemetry receiver, exporter, or other component relevant to industrial protocols that isn't listed here, please open an issue or a PR adding it. A short pointer README with a link to the external project is enough — the implementation doesn't need to live in this repository.

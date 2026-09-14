# OpenTelemetry Industrial Projects

This directory maintains a community-curated catalog of OpenTelemetry projects, integrations, collectors, receivers, bridges, and related efforts focused on industrial, operational technology (OT), IoT, and legacy environments.

The goal is to help practitioners discover existing work, identify collaboration opportunities, avoid duplicated efforts, and better understand the current state of industrial observability within the OpenTelemetry ecosystem.

## Projects

🟡 Experimental (early, may still change)<br>🔵 Active Development (evolving, with a real project depending on it)

| Project                                                                               | Description                                                                                     | Status                | Author                                          |
| ------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | --------------------- | ----------------------------------------------- |
| [EtherNet/IP Receiver](https://github.com/otel-industrial/otel-industrial-resources/tree/main/otel-industrial-collector/receiver/ethernetipreceiver) | OpenTelemetry Collector receiver for EtherNet/IP (CIP) enabled industrial devices, e.g. Rockwell PLCs. Developed in this org. | 🟡 Experimental       | [lukaszciukaj](https://github.com/lukaszciukaj) |
| [Modbus Receiver](https://github.com/lukaszciukaj/modbusreceiver)                     | OpenTelemetry Collector receiver for Modbus devices and industrial equipment.                   | 🟡 Experimental       | [lukaszciukaj](https://github.com/lukaszciukaj) |
| [OPC UA Receiver](https://github.com/bruegth/opentelemetry-collector-opcua-receiver)  | OpenTelemetry Collector receiver for OPC UA servers and industrial systems.                     | 🟡 Experimental       | [bruegth](https://github.com/bruegth)           |
| [OpenTelemetry MQTT Sparkplug](https://github.com/jmacd/opentelemetry-mqtt-sparkplug) | MQTT Sparkplug protocol support for OpenTelemetry Collector.                                    | 🔵 Active Development | [jmacd](https://github.com/jmacd)               |
| [mqtt2otel](https://github.com/OSgAgA/mqtt2otel)                                      | MQTT-to-OpenTelemetry bridge that transforms MQTT messages into OpenTelemetry metrics and logs. | 🟡 Experimental            | [OSgAgA](https://github.com/OSgAgA)             |

## Contributing

Are you working on an industrial OpenTelemetry project that is not listed here?

Please open an issue and include:

* Project name
* Repository URL
* Brief description
* Current status
* Primary maintainer or organization

Community contributions, corrections, and updates are welcome.

## Disclaimer

This catalog is maintained by the OpenTelemetry Industrial community and is intended as an informational resource. Inclusion in this list does not imply endorsement, official OpenTelemetry project status, or production readiness.

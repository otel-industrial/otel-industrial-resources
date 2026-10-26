# Modbus Receiver — Design Notes

This document captures the goals, design decisions and roadmap behind the
Modbus receiver. User-facing documentation lives in [README.md](./README.md);
development workflow lives in [CONTRIBUTING.md](./CONTRIBUTING.md).

## Goals

- Read telemetry from Modbus TCP devices using the standard read function codes.
- Map register and coil values to OpenTelemetry metrics with predictable,
  bounded cardinality.
- Stay read-only and conservative towards devices on OT networks.
- Follow OpenTelemetry Collector receiver design patterns.

## Design Considerations

- **Modbus client library.** Decision: using `goburrow/modbus`. It's
  BSD-licensed, widely used, and supports Modbus TCP as well as RTU and ASCII,
  which leaves room for serial transports later.
- **Read-only scope.** The client library supports writes, but the receiver
  only uses FC01–FC04, consistent with a non-invasive design for OT environments.
- **Explicit register map.** Modbus has no discovery or type information on the
  wire, so every register is declared in config with its address, register
  type, data type and byte order. The register count per read is derived from
  the data type rather than configured separately.
- **Configurable byte order.** Multi-register values have no standard layout,
  so `ABCD`, `DCBA`, `BADC` and `CDAB` are supported per register.
- **Lazy connection.** The receiver connects on the first poll instead of in
  `Start`, so an unreachable device never blocks collector startup.
- **Shared connection handling.** The connection, the reconnect backoff, the
  per-device poll limit, byte-order decoding and partial scrape errors come
  from the shared `industrialscraper` module, so they behave the same in every
  industrial receiver.
- **Error classification.** Transport errors drop the connection and end the
  poll, and the next poll reconnects. Modbus exception responses and responses
  that can't be decoded only skip the affected register, so one misconfigured
  address doesn't force a reconnect on every poll.
- **Testability.** Dialing is injectable, and the tests use an in-memory fake
  device to cover decoding, error handling and receiver-level recovery.

## Known limitations

- One request per register; consecutive registers are not batched.
- One device and unit ID per receiver instance.
- No scaling or engineering units; raw decoded values are reported with unit `1`.
- `int64` and `uint64` values beyond 2^53 lose precision when reported as doubles.

## Roadmap

The items below are follow-up milestones, not commitments with dates. Feedback
and contributions on any of these are welcome; they are tracked as issues in the
[Modbus receiver v0.0.2](https://github.com/otel-industrial/otel-industrial-resources/milestone/1) milestone.

- **Batched reads.** Group consecutive registers of the same type into one
  request to reduce round-trips per poll.
- **Scale factor and units per register.** For example raw → °C.
- **Multiple unit IDs per endpoint.** Poll several devices behind one gateway
  from a single receiver instance.
- **Modbus RTU and ASCII (serial).**
- **Shared helpers.** Move byte-order and connection handling onto the shared
  `industrialscraper` module
  ([#19](https://github.com/otel-industrial/otel-industrial-resources/issues/19)).

# EtherNet/IP Receiver

> **Status:** Initial Implementation In Progress

The EtherNet/IP Receiver is an experimental community effort to explore collecting telemetry from EtherNet/IP (CIP) enabled industrial devices using the OpenTelemetry Collector.

The long-term goal is to provide a vendor-neutral receiver capable of collecting telemetry from PLCs, remote I/O, drives, and other industrial devices that implement the EtherNet/IP protocol.

## Goals

- Read telemetry from EtherNet/IP devices using standard CIP services.
- Map industrial telemetry to OpenTelemetry Metrics.
- Support multiple vendors implementing the EtherNet/IP specification.
- Follow OpenTelemetry Collector receiver design patterns.

## Potential Features

- Periodic polling of tags or CIP objects
- Device metadata collection
- Resource attribute population
- Health and communication metrics
- Multiple device support

## Initial Scope

The initial focus is to establish the receiver architecture and demonstrate telemetry collection from EtherNet/IP devices.

The first implementation milestone is a read-only, single-device scraper that validates the full pipeline (config → CIP client → scraper → OpenTelemetry metrics), starting with a single metric (device connectivity/health) before expanding to a fixed list of polled tags.

Future milestones may include:

- Batched multi-service reads
- Tag/UDT discovery instead of a static tag list
- Multiple vendor support
- Multiple device support in a single receiver instance
- Performance optimization
- Configuration improvements
- Additional CIP object support

## Design Considerations

- **CIP client library.** Decision: using `danomagnum/gologix`. It's MIT licensed, actively maintained with regular releases, and by far the most widely adopted Go option for this protocol (imported by ~26 other modules vs. 0-1 for alternatives). It provides typed tag read/write, multi-tag batching, tag/program discovery, and UDT decoding out of the box, modeled after the established Python `pylogix` library. Scope is Rockwell ControlLogix/CompactLogix/Micro820 for v1 (not older PCCC-based PLC5/SLC/MicroLogix).
  - Other options considered: `iceisfun/goindustrial` (MIT, broader low-level feature set, far less adoption) and `loki-os/go-ethernet-ip` (WTFPL, likely needs legal review for upstream OTel inclusion, largely unmaintained).
- **Generic CIP vs. vendor-specific tag access.** CIP's object model is broad (vendor-specific objects, assemblies, tag-based addressing). `gologix` leans toward Rockwell/Logix-style tag access rather than generic CIP objects; the initial implementation will follow that path, with broader vendor-neutral support as a future milestone.
- **Read-only scope.** `gologix` supports writing tags, but the receiver will only use read paths, consistent with the read-only, non-invasive design goal for OT environments.
- **Polling model.** The first implementation uses a fixed, configured list of tags per device polled on an interval via `scraperhelper`, rather than a full subscription/discovery model. Discovery-based polling is a future milestone.
- **Batched reads.** CIP supports multi-service (batched) read requests, which can significantly reduce round-trips when polling many tags. Deferred until basic single-tag polling is working end-to-end.
- **OT network safety.** Polling behavior (frequency, request pacing, connection handling) should be designed conservatively to avoid impacting devices that may be sensitive to unexpected load.

## Status

Design phase complete; moving into initial implementation. Work in progress:

- [ ] Receiver scaffolding (config, factory, metadata.yaml)
- [ ] `gologix` client wrapper
- [ ] First end-to-end metric (device connectivity)
- [ ] Static tag list polling
- [ ] Tests and example config

Contributions, feedback, use cases, and implementation ideas are welcome.
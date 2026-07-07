# EtherNet/IP Receiver

> **Status:** Proposal / Early Design

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

Future milestones may include:

- Multiple vendor support
- Device discovery
- Performance optimization
- Configuration improvements
- Additional CIP object support

## Design Considerations

A few open questions to work through during design, before implementation begins:

- **CIP client library.** Several Go options exist, with varying maturity:
  - `danomagnum/gologix` — currently the strongest candidate: MIT licensed, actively maintained with regular releases, and by far the most widely adopted (imported by ~26 other modules vs. 0-1 for alternatives). Supports typed tag read/write, multi-tag reads, tag/program discovery, UDT decoding, and can act as both a client and a class 1/3 server. Modeled after the established Python `pylogix` library. Scope is Rockwell ControlLogix/CompactLogix/Micro820 only (not older PCCC-based PLC5/SLC/MicroLogix).
  - `iceisfun/goindustrial` — MIT licensed, broader low-level feature set (explicit + implicit/UDP I/O), but far less adoption; notably, its author also maintains a fork of `gologix`, suggesting these two projects may be related and worth comparing directly rather than evaluating independently.
  - `loki-os/go-ethernet-ip` — WTFPL licensed (likely needs legal review for upstream OTel inclusion) and largely unmaintained.
  - Given adoption and maintenance signals, `gologix` is the leading candidate pending a closer trial integration.
- **Generic CIP vs. vendor-specific tag access.** CIP's object model is broad (vendor-specific objects, assemblies, tag-based addressing). All strong library candidates so far lean toward Rockwell/Logix-style tag access rather than generic CIP objects, which may push the initial implementation in that direction even though the long-term goal is vendor neutrality.
- **Read-only scope.** Some candidate libraries support writing tags as well as reading. The receiver should only use read paths, consistent with the read-only, non-invasive design goal for OT environments.
- **Polling model.** Rather than polling every discoverable tag continuously, consider polling only tags with an active subscription/configuration, using a one-time discovery pass to populate available tags and metadata. This limits load on PLCs, especially over high-latency links.
- **Batched reads.** CIP supports multi-service (batched) read requests, which can significantly reduce round-trips when polling many tags — worth designing for from the start rather than adding later.
- **OT network safety.** Polling behavior (frequency, request pacing, connection handling) should be designed conservatively to avoid impacting devices that may be sensitive to unexpected load.

## Status

This receiver is currently under active design.

Contributions, feedback, use cases, and implementation ideas are welcome.
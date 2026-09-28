# EtherNet/IP Receiver — Design Notes

This document captures the goals, design decisions and roadmap behind the
EtherNet/IP receiver. User-facing documentation lives in [README.md](./README.md);
development workflow lives in [CONTRIBUTING.md](./CONTRIBUTING.md).

## Goals

- Read telemetry from EtherNet/IP devices using standard CIP services.
- Map industrial telemetry to OpenTelemetry Metrics.
- Support multiple vendors implementing the EtherNet/IP specification.
- Follow OpenTelemetry Collector receiver design patterns.

## Initial Scope

The initial focus is to establish the receiver architecture and demonstrate telemetry collection from EtherNet/IP devices.

The first implementation milestone is a read-only, single-device scraper that validates the full pipeline (config → CIP client → scraper → OpenTelemetry metrics), starting with a single metric (device connectivity/health) before expanding to a fixed list of polled tags.

## Design Considerations

- **CIP client library.** Decision: using `danomagnum/gologix`. It's MIT licensed, actively maintained with regular releases, and by far the most widely adopted Go option for this protocol (imported by ~26 other modules vs. 0-1 for alternatives). It provides typed tag read/write, multi-tag batching, tag/program discovery, and UDT decoding out of the box, modeled after the established Python `pylogix` library. Scope is Rockwell ControlLogix/CompactLogix/Micro820 for v1 (not older PCCC-based PLC5/SLC/MicroLogix).
  - Other options considered: `iceisfun/goindustrial` (MIT, broader low-level feature set, far less adoption) and `loki-os/go-ethernet-ip` (WTFPL, likely needs legal review for upstream OTel inclusion, largely unmaintained).
- **Generic CIP vs. vendor-specific tag access.** `gologix` leans toward Rockwell/Logix-style tag access rather than generic CIP objects; the initial implementation follows that path. See [Roadmap](#roadmap) below for the plan to reach non-Rockwell devices.
- **Read-only scope.** `gologix` supports writing tags, but the receiver only uses read paths, consistent with the read-only, non-invasive design goal for OT environments.
- **Host and port are separate config fields.** An earlier iteration combined them into a single `host:port` string, which broke because `gologix` appends its own default port internally. `host` is now host-only and `port` is a dedicated field (default 44818), wired directly into the client.
- **Polling model.** The first implementation uses a fixed, configured list of tags per device polled on an interval via `scraperhelper`, rather than a full subscription/discovery model. Discovery-based polling is a future milestone.
- **Batched reads.** CIP supports multi-service (batched) read requests, which can significantly reduce round-trips when polling many tags. Deferred until basic single-tag polling is working end-to-end (now proven end-to-end against the simulator).
- **Tag type handling.** CIP tags carry their type on the wire, but `gologix.Read` only succeeds when the Go destination type matches the tag's CIP type exactly. `ReadTag` currently probes `float64` (LREAL), then `int32` (DINT), then `bool` (BOOL), so other atomic types — notably REAL (32-bit float), SINT, INT, LINT and the unsigned types — are not read today, and each unsupported read costs up to three round-trips. The fix is to read the tag once and convert based on the type the controller reports (or learn types up front via tag discovery); see Roadmap.
- **OT network safety.** Polling behavior (frequency, request pacing, connection handling) should be designed conservatively to avoid impacting devices that may be sensitive to unexpected load. Connection health is verified actively each cycle rather than trusted from a cached flag, so a dropped connection is detected promptly rather than silently reporting stale "up" status.

## Roadmap

The current implementation is a deliberately narrow v1: one device, a fixed tag list, Rockwell-only tag-based access. The items below are follow-up milestones, not commitments with dates — feedback and contributions on any of these are welcome.

- **Generic CIP object access, beyond Rockwell.** EtherNet/IP itself is an open, multi-vendor protocol (ODVA-standardized CIP), but the current tag-name addressing (`tags: [testtag, ...]`) only works because `gologix` implements Rockwell's proprietary Symbol Object. Reaching non-Rockwell devices means adding a second addressing mode based on generic CIP objects — Class/Instance/Attribute access (e.g. via `gologix`'s `GetAttrSingle`/`GetAttrList`/`GenericCIPMessage` primitives) instead of symbolic tag names. This is a larger design effort: it needs its own config shape (something closer to how the sibling `modbusreceiver` declares register address/type per point) since there's no vendor-provided name resolution to lean on.
- **Read all atomic CIP types.** Decode the value according to the type the controller returns (REAL, SINT, INT, LINT, USINT, UINT, UDINT, ULINT, LREAL, DINT, BOOL) in a single read, instead of probing a fixed list of Go types.
- **Tag/UDT discovery instead of a static tag list.** Use `gologix`'s `ListAllTags`/`ListAllPrograms` to discover available tags and their real CIP types, rather than requiring users to hand-list tag names in config and having `ReadTag` guess the type by probing.
- **Batched multi-service reads.** CIP supports reading multiple tags in a single request. Worthwhile once tag lists grow beyond a handful of tags, to cut down round-trips per scrape cycle.
- **Multiple device support in a single receiver instance.** Currently one receiver instance talks to one device; polling a fleet of PLCs means running one receiver instance per device today.
- **Performance and configuration improvements**, informed by real-world use once this is running against actual hardware rather than only the simulator.
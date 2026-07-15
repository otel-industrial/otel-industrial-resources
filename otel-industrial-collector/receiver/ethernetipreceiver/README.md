# EtherNet/IP Receiver

> **Status:** Initial Implementation In Progress

The EtherNet/IP Receiver is an experimental community effort to explore collecting telemetry from EtherNet/IP (CIP) enabled industrial devices using the OpenTelemetry Collector.

The long-term goal is to provide a vendor-neutral receiver capable of collecting telemetry from PLCs, remote I/O, drives, and other industrial devices that implement the EtherNet/IP protocol.

## Goals

- Read telemetry from EtherNet/IP devices using standard CIP services.
- Map industrial telemetry to OpenTelemetry Metrics.
- Support multiple vendors implementing the EtherNet/IP specification.
- Follow OpenTelemetry Collector receiver design patterns.

## Quick start

### Prerequisites

- Go 1.25+
- OpenTelemetry Collector Builder (ocb) v0.152.0
  ```bash
  go install go.opentelemetry.io/collector/cmd/builder@v0.152.0
  ```

> Version alignment: `ocb`, `otelcol_version` in `builder-config.yaml`, and the OTel collector dependencies in `go.mod` must all match. This project uses ocb v0.152.0 with collector components at v1.58.0 / v0.152.0.

### 1. Clone and build

```bash
git clone https://github.com/otel-industrial/otel-industrial-resources.git
cd otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver
make build
```

This produces `./bin/otelcol-ethernetip` — a custom collector binary with the EtherNet/IP receiver built in.

### 2. Configure

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` — at minimum set `endpoint` to match your device (or leave as-is to point at the local simulator), and update `tags` to match the tag names available on your PLC.

### 3. Run

```bash
make run
# or
./bin/otelcol-ethernetip --config config.yaml
```

### Local testing with the simulator

If you don't have a real EtherNet/IP device available, use the included simulator:

```bash
# Terminal 1 — start the simulator
go run ./cmd/simulator

# Terminal 2 — build and run the collector
make build && make run
```

The simulator serves a few fake tags (`testtag`, `temperature`, `pumpstatus`) with slowly-drifting values on the standard EtherNet/IP ports (`0.0.0.0:44818` TCP, `0.0.0.0:2222` UDP).

> Note: tag names are case-sensitive over the wire — the simulator uses lowercase names, so `config.yaml`/`config.example.yaml` should match.

## Configuration

```yaml
receivers:
  ethernetip:
    endpoint: "127.0.0.1"       # device address, no port (gologix appends the default EtherNet/IP port)
    collection_interval: 5s     # how often to poll
    timeout: 5s                 # per-request timeout
    tags:
      - testtag
      - temperature
      - pumpstatus
```

| Field | Description |
|---|---|
| `endpoint` | Device IP address or hostname, without a port. |
| `collection_interval` | How often to poll the device. |
| `timeout` | Per-request CIP timeout. |
| `tags` | Fixed list of PLC tag names to poll each cycle. |

## Metrics

### `ethernetip.device.up`

Whether the device responded successfully to the last poll (`1`) or not (`0`). Emitted once per scrape cycle regardless of tag configuration.

### `ethernetip.tag.value`

The current value of a polled tag, decoded as a `float64` regardless of the tag's underlying CIP type (the receiver tries `float64`, then `int32`, then `bool` when reading, converting the result to a float). One data point per configured tag, per cycle.

Both metrics share this resource attribute:

| Attribute | Description |
|---|---|
| `ethernetip.device.address` | The configured device endpoint. |

`ethernetip.tag.value` additionally carries:

| Attribute | Description |
|---|---|
| `ethernetip.tag.name` | The name of the polled tag. |

Full metric documentation is auto-generated in `documentation.md` by `mdatagen` from `metadata.yaml`.

## Project structure

```
ethernetipreceiver/
├── config.go                    # Config struct and validation
├── doc.go                       # go:generate mdatagen directive
├── factory.go                   # OTel component factory registration
├── scraper.go                   # Polling loop, metric emission (no separate receiver.go — a single scraper's lifecycle is fully handled by scraperhelper)
├── client.go                    # gologix client wrapper
├── metadata.yaml                # Metric definitions (source of truth)
├── documentation.md             # Auto-generated metric docs (from mdatagen)
├── internal/
│   └── metadata/                # Auto-generated from metadata.yaml
├── testdata/
│   └── config.yaml              # Test configuration
├── cmd/
│   └── simulator/                # EtherNet/IP simulator for local testing
│       └── main.go
└── builder-config.yaml          # OCB manifest for building the collector
```

To regenerate `internal/metadata/` after editing `metadata.yaml`:

```bash
git clone --depth=1 --branch v0.152.0 https://github.com/open-telemetry/opentelemetry-collector.git /tmp/otel-collector-mdatagen-src
cd /tmp/otel-collector-mdatagen-src/cmd/mdatagen && go build -o ~/go/bin/mdatagen .
cd ~/your/ethernetipreceiver
go generate ./...
```

## Development

```bash
make test    # run tests
make build   # build the custom collector binary
make run     # run the collector (requires build first)
make tidy    # tidy go modules
make clean   # remove build artifacts
```

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
- **Generic CIP vs. vendor-specific tag access.** CIP's object model is broad (vendor-specific objects, assemblies, tag-based addressing). `gologix` leans toward Rockwell/Logix-style tag access rather than generic CIP objects; the initial implementation follows that path, with broader vendor-neutral support as a future milestone.
- **Read-only scope.** `gologix` supports writing tags, but the receiver only uses read paths, consistent with the read-only, non-invasive design goal for OT environments.
- **Polling model.** The first implementation uses a fixed, configured list of tags per device polled on an interval via `scraperhelper`, rather than a full subscription/discovery model. Discovery-based polling is a future milestone.
- **Batched reads.** CIP supports multi-service (batched) read requests, which can significantly reduce round-trips when polling many tags. Deferred until basic single-tag polling is working end-to-end (now proven — see Status below).
- **Tag type handling.** CIP tags can be float, integer, or boolean at the wire level. `ReadTag` currently tries these types in sequence rather than discovering the tag's real type up front — simple and working, but not efficient for large tag lists. A future milestone should read type information via tag discovery instead of probing.
- **OT network safety.** Polling behavior (frequency, request pacing, connection handling) should be designed conservatively to avoid impacting devices that may be sensitive to unexpected load.

## Status

Core pipeline working end-to-end against a simulated device:

- [x] Receiver scaffolding (config, factory, metadata.yaml)
- [x] `gologix` client wrapper
- [x] First end-to-end metric (device connectivity)
- [x] Static tag list polling
- [x] Unit tests for scraper logic
- [x] `builder-config.yaml` and a runnable `otelcol-ethernetip` binary
- [x] Local simulator for testing without real hardware
- [ ] `testdata/config.yaml` for automated tests
- [ ] CI workflow
- [ ] Regenerated `documentation.md`

Contributions, feedback, use cases, and implementation ideas are welcome.
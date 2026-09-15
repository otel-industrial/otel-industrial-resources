# OCB Manifests

Small, use-case-specific [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) (OCB) manifests for industrial and OT environments. Each builds a collector binary that contains only the components a given deployment needs, so the result stays small enough for the limited compute, memory, and storage typical of industrial edge hardware. See [issue #1](https://github.com/otel-industrial/otel-industrial-resources/issues/1) for the motivation.

## Building

Install OCB and run it against a manifest:

```sh
# install the builder (matches the pinned collector version below)
go install go.opentelemetry.io/collector/cmd/builder@v0.160.0

# build a distribution
builder --config modbus-edge-gateway.yaml
```

The binary is written to the `output_path` set in the manifest (e.g. `./otelcol-modbus-edge`).

Or use the `Makefile`, which wraps the builder for every manifest:

```sh
make                       # build all distributions
make modbus-edge-gateway   # build one
make clean                 # remove build output
make LDFLAGS=              # keep symbols (unstripped, debuggable)
```

The builder strips symbols (`-s -w`) by default, so the binaries are already stripped; `make LDFLAGS=` opts back into a debuggable build.

## Manifests

Four minimal, single-protocol manifests plus one full multi-protocol gateway:

| Manifest                        | Target environment                              | Industrial receiver(s)        | Components |
| ------------------------------- | ----------------------------------------------- | ----------------------------- | ---------- |
| `modbus-edge-gateway.yaml`      | Modbus TCP/RTU edge (water, HVAC, energy)       | Modbus                        | 8          |
| `rockwell-ethernetip.yaml`      | Rockwell / Allen-Bradley EtherNet/IP cells      | EtherNet/IP                   | 8          |
| `opcua-plant-floor.yaml`        | OPC UA plant floor (SCADA, discrete mfg)        | OPC UA (logs-only, alpha)     | 8          |
| `iiot-sparkplug.yaml`           | MQTT Sparkplug B broker / IIoT                  | Sparkplug                     | 8          |
| `industrial-gateway-full.yaml`  | Single mixed-protocol gateway box               | all four + host self-monitor  | 16         |

"Components" counts receivers + processors + exporters + extensions (config providers excluded). Fewer components means a smaller, leaner binary — the whole point of building your own distribution rather than shipping a full one.

### Shared baseline (minimal manifests)

Every minimal manifest carries the same lean baseline plus its one industrial receiver:

- **Processors:** `memory_limiter`, `batch`
- **Exporters:** `otlp`, `otlphttp`, `file`, `debug`
- **Extensions:** `file_storage` — a persistent send-queue that survives restarts and buffers telemetry across the intermittent uplink outages common in industrial sites
- **Providers:** `file`, `env`, `yaml`

`file` exporter is included both as a store-and-forward path for air-gapped sites with no outbound connection and as a debugging aid (it captures full payloads to disk, where `debug` only tails to stderr).

### Full gateway

`industrial-gateway-full.yaml` is for a single box that talks to more than one kind of equipment at once. It bundles all four industrial receivers plus the components left out of the minimal manifests: `hostmetrics` + `filelog` (gateway self-monitoring), `resource` + `filter` (source tagging and noise control), and the `health_check` extension. Use a minimal manifest instead whenever you only need one protocol.

`resourcedetection` is deliberately *not* included, even here. A gateway polls remote devices, so resourcedetection can only describe the collector host, never the equipment the telemetry came from — it adds no source identity while pulling in large cloud-vendor SDKs. Identify the source with the `resource` processor and operator-set attributes (site, line, asset) instead.

## Optional components

The minimal manifests are deliberately spare. To add a common component, drop its `gomod` line into the relevant section and rebuild. Add only what the deployment needs — every addition grows the binary.

```yaml
# receivers: host self-monitoring
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.160.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.160.0

# processors: attribute enrichment and filtering
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.160.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.160.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.160.0

# extensions: health/liveness endpoint
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.160.0
```

`resourcedetection` describes the **collector host**, not any remote device the collector polls — so it only adds meaningful identity when the collector runs on the machine that *is* the telemetry source (e.g. host self-monitoring). It is also platform-dependent (a no-op where its detectors don't apply) and pulls in large cloud-vendor SDKs. For tagging remote sources, use the `resource` processor with static attributes instead.

OCB cannot merge or include one manifest from another — it takes a single `--config`. To layer a shared base with optional add-ons you would merge the YAML yourself before invoking OCB (e.g. with `yq`), which is why these manifests are self-contained instead.

## Versions

All manifests pin collector **0.160.0** (stable modules **1.66.0**). Keep the builder version, `otelcol_version`, and the core component versions in step when you bump.

Three receivers are referenced by commit pseudo-version rather than a release tag, because they are either untagged or tagged in a way Go cannot resolve for their module path:

- EtherNet/IP — untagged in-repo module
- OPC UA — module in a subdirectory; the repo's root `v0.x` tags don't apply to that path
- Sparkplug — untagged

These pins point at a specific commit and will drift over time. Re-resolve them (and the tagged Modbus version) when refreshing against a newer collector release.

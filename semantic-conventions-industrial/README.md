# Industrial semantic conventions

**Status: development.** Nothing here is stable yet. Attribute names and
definitions can change between releases.

OpenTelemetry semantic conventions for industrial, OT and IIoT environments:
plant locations, industrial assets, field devices such as PLCs, RTUs, drives
and sensors, and the quality of the data read from them. The work is tracked
in [#11](https://github.com/otel-industrial/otel-industrial-resources/issues/11).

Following advice from the OpenTelemetry Semantic Conventions SIG, this registry
starts here, with the same structure as the domain registries such as
[semantic-conventions-genai](https://github.com/open-telemetry/semantic-conventions-genai)
and
[semantic-conventions-mainframe](https://github.com/open-telemetry/semantic-conventions-mainframe),
so it can move to the OpenTelemetry organization later.

## Principles

From the discussion in
[#11](https://github.com/otel-industrial/otel-industrial-resources/issues/11):

- Reuse the core semantic conventions first: `device.*` for field devices
  (PLCs, RTUs, drives, sensors), `host.*` for the machine that runs the
  collector.
- New attributes use the `industrial.*` namespace. ISA-95 / IEC 62264 is the
  reference model for the location hierarchy, without the standard's name in
  attribute names.
- Location, asset and device are separate entities. The asset stays on the
  resource.
- Data quality has a small common core (good, uncertain, bad) that every
  receiver can map to. The reason and the raw source code are optional.
- The receivers in this repository are the first implementations. Renaming
  their existing attributes happens in planned releases, marked as breaking
  in the release notes.

## Layout

    model/                    Weaver registry: YAML definitions (source of truth)
      manifest.yaml           Registry name, version and its dependency on
                              the core semantic conventions
    check.sh                  Validates the registry with OpenTelemetry Weaver

What goes where, as content arrives:

- **`model/`**: attribute, metric and entity definitions in YAML, for example
  `model/industrial/registry.yaml`. Attributes from the core semantic
  conventions, such as `device.id` or `host.name`, are referenced, not
  redefined.
- **`docs/`**: Markdown for people. Explanations and the mapping to source
  standards (OPC UA, Sparkplug B, Modbus, CIP) are hand-written; attribute
  tables will be generated from `model/` by Weaver.

Other folders used by the domain registries, such as `templates/` for doc
generation, `policies/` for extra checks and `reference/` for reference
implementations, will be added when they're needed.

The first definitions, common attributes (location, asset, device) and data
quality, are planned as separate pull requests. See
[#11](https://github.com/otel-industrial/otel-industrial-resources/issues/11).

## Validate

You need Docker or Podman. From this directory:

    bash check.sh                     # Docker
    CONTAINER=podman bash check.sh    # Podman

This runs `weaver registry check` from the
[OpenTelemetry Weaver](https://github.com/open-telemetry/weaver) container
image against `model/`, with the core semantic conventions as a dependency.
It needs network access to download them.

## Contributing

Discuss proposals in
[#11](https://github.com/otel-industrial/otel-industrial-resources/issues/11)
or in `#otel-industrial` on CNCF Slack. Please keep one area per pull request,
and run `check.sh` before opening it.

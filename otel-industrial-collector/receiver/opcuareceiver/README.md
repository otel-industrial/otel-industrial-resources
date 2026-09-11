# OPC UA Receiver

**Status:** External project, alpha.

An OpenTelemetry Collector receiver for OPC UA industrial automation servers implementing the LogObject specification (OPC UA Part 26). Currently scoped to **logs** (via the GetRecords method), not metrics — it collects log records rather than tag/telemetry values. Supports multiple authentication methods, configurable security policies, and severity filtering.

Repository: [bruegth/opentelemetry-collector-opcua-receiver](https://github.com/bruegth/opentelemetry-collector-opcua-receiver)

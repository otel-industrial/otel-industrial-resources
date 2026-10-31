---
title: Device Availability Alerts
linkTitle: Device Availability
description: Detecting industrial devices that stop responding, using the device.up metrics of the Modbus and EtherNet/IP receivers.
status: experimental
weight: 30
---

A PLC, drive or meter that suddenly stops answering is one of the earliest
signals that something changed on the plant floor. It can be a hardware
failure, a network or configuration change, or someone tampering with the
device or the network path to it. Whatever the cause, operations and security
teams want to know quickly, and they want to know which device.

The Modbus and EtherNet/IP receivers report this directly as a metric.

## The metrics

| Receiver | Metric | Value |
|---|---|---|
| Modbus | `modbus.device.up` | `1` if the device answered the last poll, otherwise `0` |
| EtherNet/IP | `ethernetip.device.up` | `1` if the device answered the last poll, otherwise `0` |

Both are integer gauges with one data point per scrape. They are reported even
when the device is down, so a dead device shows up as `0` instead of
disappearing from your dashboards.

`device.up` answers one question: *did the device respond?* It is `0` when the
connection fails, while the receiver waits out its reconnect backoff, or when
the connection drops in the middle of a poll. It stays `1` when the device
answers but rejects some requests, for example a Modbus exception for one
register or an EtherNet/IP tag that cannot be read, and when a Modbus device
reports that it is busy (exception 06).

## device.up and scrape errors

Failed reads are also reported as partial scrape errors and counted in the
collector's own `otelcol_scraper_errored_metric_points` metric. The two signals
complement each other:

| Situation | `device.up` | Errored points |
|---|---|---|
| Device healthy | `1` | `0` |
| Device answers, some registers or tags fail | `1` | number of failed items |
| Device unreachable | `0` | all configured items |

Use `device.up` to alert on devices that are gone, and the errored points to
find configuration problems such as a wrong register address or an unsupported
tag type.

## Example alerts

The examples use PromQL, assuming the metrics are exported to Prometheus.
Metric names depend on your exporter and backend: with the Prometheus
exporter, `modbus.device.up` usually becomes `modbus_device_up`, and some
setups add a unit suffix (`modbus_device_up_ratio`).

**Device down for 3 minutes:**

```promql
max_over_time(modbus_device_up[3m]) == 0
or
max_over_time(ethernetip_device_up[3m]) == 0
```

Using `max_over_time` instead of the latest value avoids alerting on a single
missed poll. Pick a window of at least two or three collection intervals.

**Flapping device:** up and down repeatedly within 15 minutes, which often
points to a cabling, power or network problem:

```promql
changes(modbus_device_up[15m]) > 4
or
changes(ethernetip_device_up[15m]) > 4
```

**No data at all:** `device.up` cannot report a stopped collector or a broken
export path. Add a separate alert for missing data, for example:

```promql
absent_over_time(modbus_device_up[5m])
```

## Telling devices apart

The EtherNet/IP receiver adds the device address (and `device_name`, if
configured) as resource attributes. The Modbus receiver only adds
`modbus.unit_id`, so two Modbus receivers with the same unit ID produce
series that look alike. Until shared device attributes are defined (see the
[semantic conventions](../semantic-conventions/common.md) work), use a
separate pipeline per device and tag it with the
[resource processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor),
for example with the device name and its location in the plant.

## Operational notes

- **Reconnect backoff:** after a connection failure, the receivers wait out
  their `reconnect_on_failure` backoff (10s, growing to 60s by default) before
  they reconnect. A device that comes back can take up to a minute to report
  `1` again.
- **Planned downtime:** silence these alerts during maintenance windows, so a
  planned PLC restart does not page anyone.
- **Change correlation:** for security monitoring, correlate unexpected device
  loss with change records. A device that drops off outside a maintenance
  window deserves a closer look.

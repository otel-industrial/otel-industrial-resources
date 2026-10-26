# industrialscraper

Shared plumbing for OpenTelemetry receivers that poll industrial control
systems (PLCs, RTUs, and similar field devices).

The package complements the collector's `scraper/scraperhelper` and builds on
`config/configretry`, so each receiver stops reimplementing the same connection
and decoding logic. It depends on no PLC protocol library: each receiver keeps
its own protocol client and pdata mapping.

## Status

Early, unstable. The public API is not frozen. In particular the resource
attribute names are provisional and track the semantic-conventions work
(issue #11); they may change before v1.

## What it provides

### Connections with reconnect backoff

A receiver implements the small `Conn` interface (`Connect`, `Close`) over its
own protocol client. `Session` owns the connection lifecycle that
scraperhelper leaves to each receiver:

- `EnsureLive` connects lazily, on the first poll, so an unreachable device
  never blocks collector startup.
- After a failed connect it waits out a `configretry.BackOffConfig` before
  dialing the device again. Polls inside the backoff window get
  `ErrBackingOff` (wrapping the last dial error) without touching the device.
  The wait grows from `InitialInterval` by `Multiplier`, with
  `RandomizationFactor` jitter, up to `MaxInterval`. Once a device has been
  down for `MaxElapsedTime`, every wait is `MaxInterval`. Reconnects never
  stop, and a successful connect resets the backoff.
- `Close` drops the connection. Call it after a transport error, and the next
  `EnsureLive` reconnects. It also matches `scraper.WithShutdown`.

A dead connection is detected by the transport error on the next read, the
same way for every protocol. A `Session` is safe for concurrent use.

Receivers expose the backoff as `reconnect_on_failure`, defaulted from
`DefaultBackOffConfig`.

### PLC-saturation guard

- `DefaultBackOffConfig` returns `configretry` defaults tuned for field
  devices: longer initial and maximum intervals than upstream (10s/60s vs
  5s/30s), so a struggling PLC gets room to recover.
- `DeviceLimiter(endpoint)` returns a process-wide limiter that allows one poll
  at a time per device, so several receivers pointed at the same PLC or
  gateway take turns instead of polling it at once. Hold it for the whole
  scrape.
- `Limiter` is the underlying primitive. `Acquire` honors context
  cancellation even when a slot is free, and `Release` without a matching
  `Acquire` panics as a misuse guard rather than deadlocking shutdown.

### Partial scrape errors

`Failures` counts the items a scrape could not read and returns the
`scrapererror.PartialScrapeError` the scraperhelper controller expects, so the
values that were read are still emitted and the failures show up in
`otelcol_scraper_errored_metric_points`. `Add` records one failed item.
`AddN` records the failing item plus the rest of the scan, skipped after a
transport error or the scrape deadline.

```go
func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.lim.Acquire(ctx); err != nil {
		return pmetric.NewMetrics(), err
	}
	defer s.lim.Release()

	if err := s.sess.EnsureLive(ctx); err != nil {
		return pmetric.NewMetrics(), err // down, or backing off
	}

	var failures industrialscraper.Failures
	for i, item := range s.items {
		if err := ctx.Err(); err != nil { // the controller's timeout
			failures.AddN(len(s.items)-i, err)
			break
		}
		if err := s.read(item); err != nil {
			if !isTransport(err) {
				failures.Add(err)
				continue
			}
			failures.AddN(len(s.items)-i, err)
			_ = s.sess.Close(ctx) // reconnect on the next poll
			break
		}
	}
	return s.mb.Emit(), failures.Err()
}
```

### Byte-order helpers

`Reorder` normalizes raw register bytes from any of the four word/byte orders
(`ABCD`, `DCBA`, `BADC`, `CDAB`) into canonical big-endian, matching the Modbus
orders. The width decoders (`Uint16`/`Int16`, `Uint32`/`Int32`, `Uint64`/`Int64`,
`Float32`, `Float64`) then read the normalized bytes. For register-based
protocols generally, not one. Host-byte-order independent (explicit
`encoding/binary` decode, no memory reinterpret).

### Block-read batching

`Batch` groups a receiver's tags into fixed-size chunks so it can issue one
device read per chunk instead of one per tag. Contiguous-address coalescing
is protocol-specific and stays in the receiver.

### Shared resource attributes

`ResourceAttributes.SetInto` writes the industrial resource attributes
(`device.id`, `plc.ip_address`, `industrial.facility.line`,
`industrial.equipment.type`) under canonical keys, so receivers do not
invent divergent names. Provisional pending issue #11 (see Status).

## Versioning

Consumed as a versioned Go submodule in this repo, pinned to a
path-prefixed tag (for example
`otel-industrial-collector/industrialscraper/v0.1.0`), the same scheme the
EtherNet/IP receiver uses.

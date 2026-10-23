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

### Resilient connections

A receiver implements the small `Conn` interface (`Connect`, `Ping`,
`Close`) over its own protocol client. `Session.EnsureLive` then drives
liveness on each poll: it pings the device and, if the ping fails, closes the
dead connection, reconnects, and re-pings. Closing first avoids leaking the old
socket and the device connection slot it holds. `Ping` must be a real
round-trip rather than a cached flag, because a socket can drop silently between
polls. A `Session` is safe for concurrent use.

```go
type scraper struct {
	sess *industrialscraper.Session
	// ...
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	s.sess = industrialscraper.NewSession(myConn)
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.sess.EnsureLive(ctx); err != nil {
		s.recordUp(0) // unreachable this cycle: record down, skip reads
		return s.emit(), nil
	}
	s.recordUp(1)
	// ... read tags ...
}
```

### PLC-saturation guard

- `DefaultBackOffConfig` returns `configretry` defaults tuned for field
  devices: enabled, with longer initial and maximum intervals than upstream
  (10s/60s vs 5s/30s) so a struggling PLC gets room to recover, and a bounded
  elapsed time so it is not retried forever.
- `Limiter` caps concurrent device operations so aggressive polling cannot
  overload a legacy PLC's CPU. `Acquire`/`Release`, with a zero max meaning
  unlimited. `Acquire` honors context cancellation even when a slot is free;
  `Release` without a matching `Acquire` panics as a misuse guard rather than
  deadlocking shutdown.

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

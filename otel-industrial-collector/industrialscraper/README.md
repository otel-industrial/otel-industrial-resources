# industrialscraper

Shared plumbing for OpenTelemetry receivers that poll industrial control
systems (PLCs, RTUs, and similar field devices).

The package extends the collector's `scraper/scraperhelper` and
`config/configretry` rather than replacing them, so each receiver stops
reimplementing the same connection and decoding logic. It depends on no PLC
protocol library: each receiver keeps its own protocol client and pdata
mapping.

## Status

Early, unstable. The public API is not frozen. In particular the resource
attribute names are provisional and track the semantic-conventions work
(issue #11); they may change before v1.

## What it provides

### Resilient connections

A receiver implements the small `Conn` interface (`Connect`, `Ping`,
`Close`) over its own protocol client. `Session.EnsureLive` then drives
liveness on each poll: it pings the device and, if the ping fails, attempts
a single reconnect. `Ping` must be a real round-trip rather than a cached
flag, because a socket can drop silently between polls.

```go
sess := industrialscraper.NewSession(myConn)

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := sess.EnsureLive(ctx); err != nil {
		s.recordUp(0) // unreachable this cycle: record down, skip reads
		return s.emit(), nil
	}
	s.recordUp(1)
	// ... read tags ...
}
```

### PLC-saturation guard

- `DefaultBackOffConfig` returns `configretry` defaults tuned for field
  devices: enabled, with a bounded elapsed time so a stuck PLC is not
  retried forever.
- `Limiter` caps concurrent device operations so aggressive polling cannot
  overload a legacy PLC's CPU. `Acquire`/`Release`, with a zero max meaning
  unlimited.

### Byte-order helpers

`Uint16`, `Int16`, `Uint32`, `Uint32WordSwapped`, `Float32`, and
`Float32WordSwapped` decode big-endian 16-bit words. 32-bit values come high
word first (ABCD) or low word first (CDAB, word swap). For register-based
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

// Package industrialscraper provides shared plumbing for OpenTelemetry
// receivers that poll industrial control systems (PLCs, RTUs, field
// devices). It extends scraper/scraperhelper and config/configretry, filling
// OT-specific gaps: resilient connections, a poll-concurrency limiter,
// byte-order helpers, block batching, and shared resource attributes. It
// depends on no PLC protocol library.
package industrialscraper

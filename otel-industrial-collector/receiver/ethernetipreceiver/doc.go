// Package ethernetipreceiver implements a receiver that can be used to collect
// telemetry from EtherNet/IP (CIP) enabled industrial devices.
//
//go:generate go tool -modfile=../../internal/tools/go.mod go.opentelemetry.io/collector/cmd/mdatagen metadata.yaml
package ethernetipreceiver

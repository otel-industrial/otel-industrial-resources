// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate go tool -modfile=../../internal/tools/go.mod go.opentelemetry.io/collector/cmd/mdatagen metadata.yaml

// Package modbusreceiver implements an OpenTelemetry Collector receiver
// that polls Modbus TCP devices and converts register and coil values
// into OpenTelemetry metrics.
package modbusreceiver

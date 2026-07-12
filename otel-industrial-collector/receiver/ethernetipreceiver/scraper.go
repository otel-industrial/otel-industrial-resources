package ethernetipreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

type ethernetipScraper struct {
	cfg      *Config
	settings receiver.Settings
	mb       *metadata.MetricsBuilder
}

func newScraper(cfg *Config, settings receiver.Settings) *ethernetipScraper {
	return &ethernetipScraper{
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

func (s *ethernetipScraper) start(_ context.Context, _ component.Host) error {
	// Client connection setup will be added on Day 2.
	return nil
}

func (s *ethernetipScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	// Real CIP polling logic will be added on Day 2.
	// For now, emit nothing so the pipeline compiles and runs end-to-end.
	return s.mb.Emit(), nil
}

package ethernetipreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

type ethernetipScraper struct {
	cfg      *Config
	settings receiver.Settings
	mb       *metadata.MetricsBuilder
	client   cipClient
}

func newScraper(cfg *Config, settings receiver.Settings) *ethernetipScraper {
	return &ethernetipScraper{
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		client:   newGologixClient(cfg.Endpoint),
	}
}

func (s *ethernetipScraper) start(_ context.Context, _ component.Host) error {
	// Attempt an initial connection so failures surface early, but don't
	// fail startup if the device is temporarily unreachable — the scrape
	// loop will retry and report device.up=0 until it succeeds.
	if err := s.client.Connect(); err != nil {
		s.settings.Logger.Warn("initial connection to EtherNet/IP device failed, will retry on next scrape")
	}
	return nil
}

func (s *ethernetipScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(timeNow())

	up := int64(1)
	if !s.client.IsConnected() {
		if err := s.client.Connect(); err != nil {
			up = 0
		} else {
			up = 1
		}
	}

	s.mb.RecordEthernetipDeviceUpDataPoint(now, up)

	rb := s.mb.NewResourceBuilder()
	rb.SetEthernetipDeviceAddress(s.cfg.Endpoint)

	return s.mb.Emit(metadata.WithResource(rb.Emit())), nil
}

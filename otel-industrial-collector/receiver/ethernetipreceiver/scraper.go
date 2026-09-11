package ethernetipreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

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
		client:   newGologixClient(cfg.Host, cfg.Port, cfg.Timeout),
	}
}

func (s *ethernetipScraper) start(_ context.Context, _ component.Host) error {
	if err := s.client.Connect(); err != nil {
		s.settings.Logger.Warn("initial connection to EtherNet/IP device failed, will retry on next scrape")
	}
	return nil
}

func (s *ethernetipScraper) shutdown(_ context.Context) error {
	if err := s.client.Disconnect(); err != nil {
		s.settings.Logger.Warn("error disconnecting from EtherNet/IP device during shutdown",
			zap.Error(err))
	}
	return nil
}

func (s *ethernetipScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(timeNow())

	if err := s.client.Ping(); err != nil {
		// The device may have dropped between scrapes even though we
		// previously connected successfully. Try a fresh connect before
		// giving up on this cycle.
		if connectErr := s.client.Connect(); connectErr != nil {
			s.mb.RecordEthernetipDeviceUpDataPoint(now, 0)
			s.settings.Logger.Warn("device unreachable, skipping tag reads this cycle",
				zap.Error(connectErr))
			return s.emit(), nil
		}
	}

	s.mb.RecordEthernetipDeviceUpDataPoint(now, 1)

	for _, tagName := range s.cfg.Tags {
		value, err := s.client.ReadTag(tagName)
		if err != nil {
			s.settings.Logger.Warn("failed to read tag, skipping",
				zap.String("tag", tagName), zap.Error(err))
			continue
		}
		s.mb.RecordEthernetipTagValueDataPoint(now, value, tagName)
	}

	return s.emit(), nil
}

func (s *ethernetipScraper) emit() pmetric.Metrics {
	rb := s.mb.NewResourceBuilder()
	rb.SetEthernetipDeviceAddress(s.cfg.Host)
	if s.cfg.DeviceName != "" {
		rb.SetEthernetipDeviceName(s.cfg.DeviceName)
	}
	return s.mb.Emit(metadata.WithResource(rb.Emit()))
}

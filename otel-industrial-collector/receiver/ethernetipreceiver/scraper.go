package ethernetipreceiver

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
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
		client:   newGologixClient(cfg.Host, cfg.Port, cfg.RequestTimeout),
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

// scrape reads every configured tag. Tags that were not read (failed reads,
// tags skipped at the scrape deadline, or all tags when the device is
// unreachable) are returned as a scrapererror.PartialScrapeError, so the
// controller still emits the data points that were recorded, including
// ethernetip.device.up, and counts the failures in
// otelcol_scraper_errored_metric_points. device.up means "the device answered
// this poll", so it stays 1 when individual tags fail.
func (s *ethernetipScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(timeNow())

	if err := s.client.Ping(); err != nil {
		// The device may have dropped between scrapes even though we
		// previously connected successfully. Try a fresh connect before
		// giving up on this cycle.
		if connectErr := s.client.Connect(); connectErr != nil {
			s.mb.RecordEthernetipDeviceUpDataPoint(now, 0)
			return s.emit(), scrapererror.NewPartialScrapeError(
				fmt.Errorf("device unreachable: %w", connectErr), len(s.cfg.Tags))
		}
	}

	s.mb.RecordEthernetipDeviceUpDataPoint(now, 1)

	var errs []error
	failed := 0
	for i, tagName := range s.cfg.Tags {
		// The controller cancels ctx when the scrape `timeout` expires.
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("scrape stopped before tag %q: %w", tagName, err))
			failed += len(s.cfg.Tags) - i
			break
		}
		value, err := s.client.ReadTag(tagName)
		if err != nil {
			errs = append(errs, fmt.Errorf("tag %q: %w", tagName, err))
			failed++
			continue
		}
		s.mb.RecordEthernetipTagValueDataPoint(now, value, tagName)
	}

	if len(errs) > 0 {
		return s.emit(), scrapererror.NewPartialScrapeError(errors.Join(errs...), failed)
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

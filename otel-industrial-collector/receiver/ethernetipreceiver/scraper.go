package ethernetipreceiver

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/industrialscraper"
	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

// cipConn adapts a cipClient to industrialscraper.Conn and Pinger, so the
// shared Session pings the open connection (ListIdentity) before each scrape.
type cipConn struct{ c cipClient }

func (c cipConn) Connect(context.Context) error { return c.c.Connect() }
func (c cipConn) Close(context.Context) error   { return c.c.Disconnect() }
func (c cipConn) Ping(context.Context) error    { return c.c.Ping() }

type ethernetipScraper struct {
	cfg      *Config
	settings receiver.Settings
	mb       *metadata.MetricsBuilder
	client   cipClient
	sess     *industrialscraper.Session
	lim      *industrialscraper.Limiter
}

func newScraper(cfg *Config, settings receiver.Settings) *ethernetipScraper {
	return newScraperWithClient(cfg, settings, newGologixClient(cfg.Host, cfg.Port, cfg.RequestTimeout))
}

func newScraperWithClient(cfg *Config, settings receiver.Settings, client cipClient) *ethernetipScraper {
	s := &ethernetipScraper{
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		client:   client,
		sess:     industrialscraper.NewSession(cipConn{client}, cfg.ReconnectOnFailure),
	}
	s.lim = industrialscraper.DeviceLimiter(s.endpoint())
	return s
}

// endpoint is the device address used to share one poll limiter across
// receivers pointed at the same PLC.
func (s *ethernetipScraper) endpoint() string {
	return net.JoinHostPort(s.cfg.Host, strconv.FormatUint(uint64(s.cfg.Port), 10))
}

func (s *ethernetipScraper) shutdown(ctx context.Context) error {
	if err := s.sess.Close(ctx); err != nil {
		s.settings.Logger.Warn("error disconnecting from EtherNet/IP device during shutdown",
			zap.Error(err))
	}
	return nil
}

// scrape reads every configured tag. The connection is opened on the first
// scrape and checked with a ListIdentity ping before each later one. A device
// that is down is redialed only after the reconnect_on_failure backoff, and
// still reports ethernetip.device.up = 0. Failed and skipped tags are returned
// as a scrapererror.PartialScrapeError; the values read are still emitted.
func (s *ethernetipScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.lim.Acquire(ctx); err != nil {
		return pmetric.NewMetrics(), err
	}
	defer s.lim.Release()

	now := pcommon.NewTimestampFromTime(timeNow())
	var failures industrialscraper.Failures

	if err := s.sess.EnsureLive(ctx); err != nil {
		s.mb.RecordEthernetipDeviceUpDataPoint(now, 0)
		failures.AddN(len(s.cfg.Tags), fmt.Errorf("device unreachable: %w", err))
		return s.emit(), failures.Err()
	}
	s.mb.RecordEthernetipDeviceUpDataPoint(now, 1)

	for i, tagName := range s.cfg.Tags {
		// The controller cancels ctx when the scrape `timeout` expires.
		if err := ctx.Err(); err != nil {
			failures.AddN(len(s.cfg.Tags)-i, fmt.Errorf("scrape stopped before tag %q: %w", tagName, err))
			break
		}
		value, err := s.client.ReadTag(tagName)
		if err != nil {
			failures.Add(fmt.Errorf("tag %q: %w", tagName, err))
			continue
		}
		s.mb.RecordEthernetipTagValueDataPoint(now, value, tagName)
	}

	return s.emit(), failures.Err()
}

func (s *ethernetipScraper) emit() pmetric.Metrics {
	rb := s.mb.NewResourceBuilder()
	rb.SetEthernetipDeviceAddress(s.cfg.Host)
	if s.cfg.DeviceName != "" {
		rb.SetEthernetipDeviceName(s.cfg.DeviceName)
	}
	return s.mb.Emit(metadata.WithResource(rb.Emit()))
}

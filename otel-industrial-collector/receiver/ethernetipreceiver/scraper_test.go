package ethernetipreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

// fakeCIPClient lets tests control connection and read behavior without a real PLC.
type fakeCIPClient struct {
	connectErr       error
	connected        bool
	pingErr          error
	readValues       map[string]float64
	readErrs         map[string]error
	disconnectCalled bool
	// onRead, if set, runs after each ReadTag call.
	onRead func(tagName string)
}

func (f *fakeCIPClient) Connect() error {
	if f.connectErr != nil {
		f.connected = false
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeCIPClient) Disconnect() error {
	f.connected = false
	f.disconnectCalled = true
	return nil
}

// Ping mirrors the real client: succeeds only while connected, or if a
// pingErr override is set (to simulate a drop between scrapes).
func (f *fakeCIPClient) Ping() error {
	if f.pingErr != nil {
		f.connected = false
		return f.pingErr
	}
	if !f.connected {
		return errors.New("not connected")
	}
	return nil
}

func (f *fakeCIPClient) ReadTag(tagName string) (float64, error) {
	if f.onRead != nil {
		defer f.onRead(tagName)
	}
	if err, ok := f.readErrs[tagName]; ok {
		return 0, err
	}
	return f.readValues[tagName], nil
}

func newTestScraper(t *testing.T, client cipClient, tags []string) *ethernetipScraper {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.Host = "127.0.0.1"
	cfg.Port = 44818
	cfg.Tags = tags

	settings := receivertest.NewNopSettings(metadata.Type)

	s := newScraper(cfg, settings)
	s.client = client
	return s
}

func TestScrapeAllTagsSucceed(t *testing.T) {
	client := &fakeCIPClient{
		readValues: map[string]float64{"TagA": 1.5, "TagB": 2.5},
	}
	s := newTestScraper(t, client, []string{"TagA", "TagB"})

	timeNow = func() time.Time { return time.Unix(0, 0) }
	defer func() { timeNow = time.Now }()

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.True(t, client.connected)
}

func TestScrapeOneTagFailsOthersSucceed(t *testing.T) {
	client := &fakeCIPClient{
		readValues: map[string]float64{"TagA": 1.5},
		readErrs:   map[string]error{"TagB": errors.New("tag not found")},
	}
	s := newTestScraper(t, client, []string{"TagA", "TagB"})

	metrics, err := s.scrape(context.Background())
	requirePartial(t, err, 1)
	assert.ErrorContains(t, err, `tag "TagB": tag not found`)
	assert.Equal(t, []float64{1.5}, gaugePoints(metrics, "ethernetip.tag.value"))
	assert.Equal(t, []float64{1}, gaugePoints(metrics, "ethernetip.device.up"),
		"device.up means the device answered, so a failed tag leaves it at 1")
}

func TestScrapeDeviceDownSkipsTags(t *testing.T) {
	client := &fakeCIPClient{connectErr: errors.New("connection refused")}
	s := newTestScraper(t, client, []string{"TagA", "TagB"})

	metrics, err := s.scrape(context.Background())
	requirePartial(t, err, 2)
	assert.ErrorContains(t, err, "device unreachable: connection refused")
	// A partial (not plain) error makes the controller still emit device.up = 0.
	assert.Equal(t, []float64{0}, gaugePoints(metrics, "ethernetip.device.up"))
	assert.Empty(t, gaugePoints(metrics, "ethernetip.tag.value"))
	assert.False(t, client.connected)
}

func TestScrapeDetectsMidRunDrop(t *testing.T) {
	client := &fakeCIPClient{connected: true, pingErr: errors.New("connection reset")}
	s := newTestScraper(t, client, []string{"TagA"})

	// Reconnect also fails, simulating a fully dead device.
	client.connectErr = errors.New("connection refused")

	metrics, err := s.scrape(context.Background())
	requirePartial(t, err, 1)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.False(t, client.connected)
}

func TestScrapeStopsAtDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var read []string
	client := &fakeCIPClient{
		connected:  true,
		readValues: map[string]float64{"TagA": 1, "TagB": 2, "TagC": 3},
		onRead: func(tagName string) {
			read = append(read, tagName)
			cancel() // the scrape deadline expires after the first tag
		},
	}
	s := newTestScraper(t, client, []string{"TagA", "TagB", "TagC"})

	metrics, err := s.scrape(ctx)
	requirePartial(t, err, 2)
	// PartialScrapeError does not unwrap, so check the message.
	assert.ErrorContains(t, err, `scrape stopped before tag "TagB": `+context.Canceled.Error())
	assert.Equal(t, []string{"TagA"}, read)
	assert.Equal(t, []float64{1}, gaugePoints(metrics, "ethernetip.tag.value"),
		"values read before the deadline are still emitted")
	assert.Equal(t, []float64{1}, gaugePoints(metrics, "ethernetip.device.up"))
}

func TestScrapeCountsEveryFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &fakeCIPClient{
		connected:  true,
		readValues: map[string]float64{"TagA": 1},
		readErrs:   map[string]error{"TagB": errors.New("unsupported type")},
		onRead: func(tagName string) {
			if tagName == "TagB" {
				cancel()
			}
		},
	}
	s := newTestScraper(t, client, []string{"TagA", "TagB", "TagC", "TagD"})

	// TagB fails, then the deadline skips TagC and TagD.
	_, err := s.scrape(ctx)
	requirePartial(t, err, 3)
	assert.ErrorContains(t, err, `tag "TagB": unsupported type`)
	assert.ErrorContains(t, err, `scrape stopped before tag "TagC"`)
}

func TestNewScraperUsesRequestTimeout(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Host = "127.0.0.1"
	cfg.RequestTimeout = 750 * time.Millisecond
	cfg.Timeout = 30 * time.Second

	s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	assert.Equal(t, 750*time.Millisecond, s.client.(*gologixClient).client.SocketTimeout)
}

func TestShutdownDisconnects(t *testing.T) {
	client := &fakeCIPClient{connected: true}
	s := newTestScraper(t, client, []string{"TagA"})

	require.NoError(t, s.shutdown(context.Background()))
	assert.True(t, client.disconnectCalled)
}

func TestEmitSetsDeviceNameWhenConfigured(t *testing.T) {
	client := &fakeCIPClient{connected: true, readValues: map[string]float64{"TagA": 1}}
	s := newTestScraper(t, client, []string{"TagA"})
	s.cfg.DeviceName = "line1-plc"

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	rm := metrics.ResourceMetrics().At(0)
	name, ok := rm.Resource().Attributes().Get("ethernetip.device.name")
	require.True(t, ok)
	assert.Equal(t, "line1-plc", name.Str())
}

func TestEmitOmitsDeviceNameWhenNotConfigured(t *testing.T) {
	client := &fakeCIPClient{connected: true, readValues: map[string]float64{"TagA": 1}}
	s := newTestScraper(t, client, []string{"TagA"})

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	rm := metrics.ResourceMetrics().At(0)
	_, ok := rm.Resource().Attributes().Get("ethernetip.device.name")
	assert.False(t, ok)
}

func requirePartial(t *testing.T, err error, failed int) {
	t.Helper()
	require.Error(t, err)
	var pe scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, failed, pe.Failed)
}

// gaugePoints returns the double values of the named gauge, or nil if the
// metric is absent.
func gaugePoints(md pmetric.Metrics, name string) []float64 {
	var out []float64
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				if ms.At(k).Name() != name {
					continue
				}
				dps := ms.At(k).Gauge().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dp := dps.At(l)
					if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
						out = append(out, float64(dp.IntValue()))
					} else {
						out = append(out, dp.DoubleValue())
					}
				}
			}
		}
	}
	return out
}

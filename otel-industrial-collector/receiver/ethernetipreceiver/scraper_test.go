package ethernetipreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/receiver/receivertest"

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
	if err, ok := f.readErrs[tagName]; ok {
		return 0, err
	}
	return f.readValues[tagName], nil
}

func newTestScraper(t *testing.T, client cipClient, tags []string) *ethernetipScraper {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "127.0.0.1:44818"
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
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
}

func TestScrapeDeviceDownSkipsTags(t *testing.T) {
	client := &fakeCIPClient{connectErr: errors.New("connection refused")}
	s := newTestScraper(t, client, []string{"TagA"})

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.False(t, client.connected)
}

func TestScrapeDetectsMidRunDrop(t *testing.T) {
	client := &fakeCIPClient{connected: true, pingErr: errors.New("connection reset")}
	s := newTestScraper(t, client, []string{"TagA"})

	// Reconnect also fails, simulating a fully dead device.
	client.connectErr = errors.New("connection refused")

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.False(t, client.connected)
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

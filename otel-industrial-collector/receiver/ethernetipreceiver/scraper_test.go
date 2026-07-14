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

// fakeCIPClient lets tests control connection behavior without a real PLC.
type fakeCIPClient struct {
	connectErr error
	connected  bool
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
	return nil
}

func (f *fakeCIPClient) IsConnected() bool {
	return f.connected
}

func newTestScraper(t *testing.T, client cipClient) *ethernetipScraper {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "127.0.0.1:44818"
	cfg.Tags = []string{"TestTag"}

	settings := receivertest.NewNopSettings(metadata.Type)

	s := newScraper(cfg, settings)
	s.client = client
	return s
}

func TestScrapeDeviceUp(t *testing.T) {
	client := &fakeCIPClient{}
	s := newTestScraper(t, client)

	timeNow = func() time.Time { return time.Unix(0, 0) }
	defer func() { timeNow = time.Now }()

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.True(t, client.connected)
}

func TestScrapeDeviceDown(t *testing.T) {
	client := &fakeCIPClient{connectErr: errors.New("connection refused")}
	s := newTestScraper(t, client)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	assert.False(t, client.connected)
}

package modbusreceiver

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type modbusReceiver struct {
	cfg      *Config
	consumer consumer.Metrics
	logger   *zap.Logger
	scraper  *modbusScraper

	// mu guards cancel and done, which Start writes and Shutdown reads.
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newModbusReceiver(cfg *Config, consumer consumer.Metrics, logger *zap.Logger) *modbusReceiver {
	return &modbusReceiver{
		cfg:      cfg,
		consumer: consumer,
		logger:   logger,
		scraper:  newModbusScraper(cfg, logger),
	}
}

// Start begins the polling loop. Connection to the Modbus device is
// established lazily on the first scrape so Start() always returns quickly.
func (r *modbusReceiver) Start(_ context.Context, _ component.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// The Start context must not be used for long-running work, so the poll
	// loop gets its own context that Shutdown cancels.
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go r.pollLoop(ctx, r.done)
	return nil
}

// Shutdown cancels the polling loop and disconnects from the device.
func (r *modbusReceiver) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()

	if cancel == nil {
		// Never started.
		return nil
	}
	cancel()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return r.scraper.shutdown(ctx)
}

// pollLoop ticks at PollingInterval and calls scrape → consumer pipeline.
func (r *modbusReceiver) pollLoop(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(r.cfg.PollingInterval)
	defer ticker.Stop()

	r.scrapeAndConsume(ctx)

	for {
		select {
		case <-ticker.C:
			r.scrapeAndConsume(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (r *modbusReceiver) scrapeAndConsume(ctx context.Context) {
	metrics, err := r.scraper.scrape(ctx)
	if err != nil {
		r.logger.Error("Modbus scrape failed", zap.Error(err))
		return
	}
	if metrics.DataPointCount() == 0 {
		return
	}
	if err := r.consumer.ConsumeMetrics(ctx, metrics); err != nil {
		r.logger.Error("Failed to consume Modbus metrics", zap.Error(err))
	}
}

var _ interface {
	Start(context.Context, component.Host) error
	Shutdown(context.Context) error
} = (*modbusReceiver)(nil)

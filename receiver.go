package modbusreceiver

import (
	"context"
	"fmt"
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
	cancel   context.CancelFunc
	done     chan struct{}
}

func newModbusReceiver(cfg *Config, consumer consumer.Metrics, logger *zap.Logger) *modbusReceiver {
	return &modbusReceiver{
		cfg:      cfg,
		consumer: consumer,
		logger:   logger,
		scraper:  newModbusScraper(cfg, logger),
		done:     make(chan struct{}),
	}
}

// Start validates config, connects to the Modbus device, and begins polling.
func (r *modbusReceiver) Start(ctx context.Context, _ component.Host) error {
	if err := r.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid modbus receiver config: %w", err)
	}
	if err := r.scraper.start(ctx); err != nil {
		return err
	}
	ctx, r.cancel = context.WithCancel(context.Background())
	go r.pollLoop(ctx)
	return nil
}

// Shutdown cancels the polling loop and disconnects from the device.
func (r *modbusReceiver) Shutdown(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	select {
	case <-r.done:
	case <-ctx.Done():
	}
	return r.scraper.shutdown(ctx)
}

// pollLoop ticks at PollingInterval and calls scrape → consumer pipeline.
func (r *modbusReceiver) pollLoop(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.cfg.PollingInterval)
	defer ticker.Stop()

	// Do an initial scrape immediately on startup.
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

// Compile-time check that modbusReceiver satisfies the receiver.Metrics interface.
var _ interface {
	Start(context.Context, component.Host) error
	Shutdown(context.Context) error
} = (*modbusReceiver)(nil)

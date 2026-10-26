package modbusreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/industrialscraper"
	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/modbusreceiver/internal/metadata"
)

const (
	typeStr   = "modbus"
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a new Modbus receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	cc := scraperhelper.NewDefaultControllerConfig()
	cc.CollectionInterval = 10 * time.Second

	return &Config{
		ControllerConfig:     cc,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		Endpoint:             "localhost:502",
		UnitID:               1,
		RequestTimeout:       5 * time.Second,
		Registers:            []RegisterDefinition{},
		ReconnectOnFailure:   industrialscraper.DefaultBackOffConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	return newMetricsReceiver(cfg.(*Config), settings, consumer, dialTCP)
}

// newMetricsReceiver wires the scraper into a scraperhelper controller.
// The connection is opened lazily on the first scrape, so Start returns
// quickly. dial is a parameter so tests can substitute a fake device.
func newMetricsReceiver(cfg *Config, settings receiver.Settings, consumer consumer.Metrics, dial dialFunc) (receiver.Metrics, error) {
	s := newModbusScraper(cfg, settings, dial)

	sc, err := scraper.NewMetrics(s.scrape, scraper.WithShutdown(s.shutdown))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddMetricsScraper(metadata.Type, sc))
}

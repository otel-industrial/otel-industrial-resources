package modbusreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
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
	return &Config{
		Endpoint:        "localhost:502",
		UnitID:          1,
		PollingInterval: 10 * time.Second,
		Timeout:         5 * time.Second,
		Registers:       []RegisterDefinition{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)
	return newModbusReceiver(rCfg, consumer, settings.Logger), nil
}
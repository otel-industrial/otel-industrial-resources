package modbusreceiver

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	if !ok {
		t.Fatal("expected *Config")
	}
	if rCfg.Endpoint != "localhost:502" {
		t.Errorf("unexpected default endpoint: %s", rCfg.Endpoint)
	}
	if rCfg.UnitID != 1 {
		t.Errorf("unexpected default unit_id: %d", rCfg.UnitID)
	}
	if rCfg.PollingInterval != 10*time.Second {
		t.Errorf("unexpected default polling_interval: %v", rCfg.PollingInterval)
	}
	if rCfg.Timeout != 5*time.Second {
		t.Errorf("unexpected default timeout: %v", rCfg.Timeout)
	}
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Registers = []RegisterDefinition{
		{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint16, ByteOrder: ByteOrderABCD},
	}

	recv, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	if err != nil {
		t.Fatalf("CreateMetrics() error = %v", err)
	}
	if recv == nil {
		t.Fatal("expected non-nil receiver")
	}
}
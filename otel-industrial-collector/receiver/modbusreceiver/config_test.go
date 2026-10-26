package modbusreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/industrialscraper"
	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/modbusreceiver/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	validBase := func() *Config {
		return &Config{
			Endpoint:       "192.168.1.10:502",
			UnitID:         1,
			RequestTimeout: 5 * time.Second,
			Registers: []RegisterDefinition{
				{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint16, ByteOrder: ByteOrderABCD},
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid config",
			mutate:  func(_ *Config) {},
			wantErr: false,
		},
		{
			name:    "missing endpoint",
			mutate:  func(c *Config) { c.Endpoint = "" },
			wantErr: true,
		},
		{
			name:    "zero unit_id",
			mutate:  func(c *Config) { c.UnitID = 0 },
			wantErr: true,
		},
		{
			name:    "zero request_timeout",
			mutate:  func(c *Config) { c.RequestTimeout = 0 },
			wantErr: true,
		},
		{
			name:    "no registers",
			mutate:  func(c *Config) { c.Registers = nil },
			wantErr: true,
		},
		{
			name: "invalid register type",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: "bad_type", DataType: DataTypeUint16, ByteOrder: ByteOrderABCD},
				}
			},
			wantErr: true,
		},
		{
			name: "invalid data type",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeHoldingRegister, DataType: "bad_type", ByteOrder: ByteOrderABCD},
				}
			},
			wantErr: true,
		},
		{
			name: "invalid byte order",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint16, ByteOrder: "WXYZ"},
				}
			},
			wantErr: true,
		},
		{
			name: "bool on holding register is invalid",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
				}
			},
			wantErr: true,
		},
		{
			name: "non-bool on coil is invalid",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeCoil, DataType: DataTypeUint16, ByteOrder: ByteOrderABCD},
				}
			},
			wantErr: true,
		},
		{
			name: "all register types valid",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeCoil, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
					{Address: 1, Type: RegisterTypeDiscreteInput, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
					{Address: 2, Type: RegisterTypeHoldingRegister, DataType: DataTypeInt16, ByteOrder: ByteOrderABCD},
					{Address: 3, Type: RegisterTypeInputRegister, DataType: DataTypeFloat32, ByteOrder: ByteOrderCDAB},
				}
			},
			wantErr: false,
		},
		{
			name: "all data types valid on holding register",
			mutate: func(c *Config) {
				c.Registers = []RegisterDefinition{
					{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeInt16, ByteOrder: ByteOrderABCD},
					{Address: 1, Type: RegisterTypeHoldingRegister, DataType: DataTypeInt32, ByteOrder: ByteOrderDCBA},
					{Address: 3, Type: RegisterTypeHoldingRegister, DataType: DataTypeInt64, ByteOrder: ByteOrderBADC},
					{Address: 7, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint16, ByteOrder: ByteOrderCDAB},
					{Address: 8, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint32, ByteOrder: ByteOrderABCD},
					{Address: 10, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint64, ByteOrder: ByteOrderABCD},
					{Address: 14, Type: RegisterTypeHoldingRegister, DataType: DataTypeFloat32, ByteOrder: ByteOrderABCD},
					{Address: 16, Type: RegisterTypeHoldingRegister, DataType: DataTypeFloat64, ByteOrder: ByteOrderABCD},
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			tt.mutate(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	t.Run("defaults", func(t *testing.T) {
		cfg := loadConfig(t, cm, component.NewID(metadata.Type))

		assert.Equal(t, "localhost:502", cfg.Endpoint)
		assert.Equal(t, 1, cfg.UnitID)
		assert.Equal(t, 10*time.Second, cfg.CollectionInterval)
		assert.Equal(t, time.Second, cfg.InitialDelay)
		assert.Equal(t, time.Duration(0), cfg.Timeout)
		assert.Equal(t, 5*time.Second, cfg.RequestTimeout)
		// Validate fills in per-register defaults.
		assert.Equal(t, []RegisterDefinition{
			{Address: 0, Type: RegisterTypeHoldingRegister, DataType: DataTypeUint16, ByteOrder: ByteOrderABCD},
		}, cfg.Registers)
		assert.Equal(t, metadata.DefaultMetricsConfig(), cfg.Metrics)
		assert.Equal(t, industrialscraper.DefaultBackOffConfig(), cfg.ReconnectOnFailure)
	})

	t.Run("all_settings", func(t *testing.T) {
		cfg := loadConfig(t, cm, component.NewIDWithName(metadata.Type, "all_settings"))

		assert.Equal(t, "plc-line1.example.internal:502", cfg.Endpoint)
		assert.Equal(t, 17, cfg.UnitID)
		assert.Equal(t, 30*time.Second, cfg.CollectionInterval)
		assert.Equal(t, 5*time.Second, cfg.InitialDelay)
		assert.Equal(t, 20*time.Second, cfg.Timeout)
		assert.Equal(t, 2*time.Second, cfg.RequestTimeout)
		assert.Equal(t, []RegisterDefinition{
			{Address: 100, Type: RegisterTypeHoldingRegister, DataType: DataTypeFloat32, ByteOrder: ByteOrderCDAB, Name: "temperature"},
			{Address: 4, Type: RegisterTypeInputRegister, DataType: DataTypeInt16, ByteOrder: ByteOrderABCD, Name: "pressure"},
			{Address: 0, Type: RegisterTypeCoil, DataType: DataTypeBool, ByteOrder: ByteOrderABCD, Name: "pump_status"},
			{Address: 1, Type: RegisterTypeDiscreteInput, DataType: DataTypeBool, ByteOrder: ByteOrderABCD, Name: "valve_open"},
		}, cfg.Registers)
		assert.False(t, cfg.Metrics.ModbusCoilValue.Enabled)
		assert.True(t, cfg.Metrics.ModbusRegisterValue.Enabled, "unset metrics keep their default")
		assert.True(t, cfg.ReconnectOnFailure.Enabled)
		assert.Equal(t, 15*time.Second, cfg.ReconnectOnFailure.InitialInterval)
		assert.Equal(t, 2*time.Minute, cfg.ReconnectOnFailure.MaxInterval)
		assert.Equal(t, 10*time.Minute, cfg.ReconnectOnFailure.MaxElapsedTime)
	})
}

// loadConfig unmarshals the named receiver section of cm over the factory
// defaults and checks that the result passes Validate.
func loadConfig(t *testing.T, cm *confmap.Conf, id component.ID) *Config {
	t.Helper()

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())
	return cfg
}

func TestConfigUnmarshalControllerSettings(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"collection_interval": "30s",
		"initial_delay":       "2s",
		"timeout":             "20s",
		"request_timeout":     "3s",
	})
	require.NoError(t, conf.Unmarshal(cfg))

	assert.Equal(t, 30*time.Second, cfg.CollectionInterval)
	assert.Equal(t, 2*time.Second, cfg.InitialDelay)
	assert.Equal(t, 20*time.Second, cfg.Timeout)
	assert.Equal(t, 3*time.Second, cfg.RequestTimeout)

	cfg.CollectionInterval = 0
	assert.Error(t, cfg.ControllerConfig.Validate(), "collection_interval must be positive")
}

func TestConfigRejectsPollingInterval(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{"polling_interval": "5s"})
	assert.Error(t, conf.Unmarshal(cfg), "the removed polling_interval key should be rejected")
}

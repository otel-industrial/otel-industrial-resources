package ethernetipreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing host",
			cfg:     Config{Host: "", Tags: []string{"TagA"}},
			wantErr: "host must be specified",
		},
		{
			name:    "missing tags",
			cfg:     Config{Host: "127.0.0.1", Tags: nil},
			wantErr: "at least one tag must be specified",
		},
		{
			name: "valid config",
			cfg:  Config{Host: "127.0.0.1", Tags: []string{"TagA"}},
		},
		{
			name:    "port out of range",
			cfg:     Config{Host: "127.0.0.1", Port: 70000, Tags: []string{"TagA"}},
			wantErr: "is not a valid TCP port",
		},
		{
			name: "valid config with unset port",
			cfg:  Config{Host: "127.0.0.1", Port: 0, Tags: []string{"TagA"}},
		},
		{
			name: "valid config with non-default port",
			cfg:  Config{Host: "127.0.0.1", Port: 44819, Tags: []string{"TagA"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	t.Run("defaults", func(t *testing.T) {
		cfg := loadConfig(t, cm, component.NewID(metadata.Type))

		assert.Equal(t, "192.168.1.10", cfg.Host)
		assert.Equal(t, []string{"temperature"}, cfg.Tags)
		assert.Equal(t, uint(44818), cfg.Port)
		assert.Empty(t, cfg.DeviceName)
		assert.Equal(t, 5*time.Second, cfg.Timeout)
		assert.Equal(t, time.Minute, cfg.CollectionInterval)
		assert.Equal(t, time.Second, cfg.InitialDelay)
		assert.True(t, cfg.Metrics.EthernetipDeviceUp.Enabled)
		assert.True(t, cfg.Metrics.EthernetipTagValue.Enabled)
		assert.True(t, cfg.ResourceAttributes.EthernetipDeviceName.Enabled)
	})

	t.Run("all_settings", func(t *testing.T) {
		cfg := loadConfig(t, cm, component.NewIDWithName(metadata.Type, "all_settings"))

		assert.Equal(t, "plc-line1.example.internal", cfg.Host)
		assert.Equal(t, uint(44818), cfg.Port)
		assert.Equal(t, "line1-plc", cfg.DeviceName)
		assert.Equal(t, []string{"temperature", "pumpstatus", "flow_rate"}, cfg.Tags)
		assert.Equal(t, 2*time.Second, cfg.Timeout)
		assert.Equal(t, 30*time.Second, cfg.CollectionInterval)
		assert.Equal(t, 5*time.Second, cfg.InitialDelay)
		assert.True(t, cfg.Metrics.EthernetipTagValue.Enabled)
		assert.False(t, cfg.ResourceAttributes.EthernetipDeviceName.Enabled)
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
package ethernetipreceiver

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/ethernetipreceiver/internal/metadata"
)

// Config defines configuration for the EtherNet/IP receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// Endpoint is the address of the EtherNet/IP device, e.g. "192.168.1.10".
	Endpoint string `mapstructure:"endpoint"`

	// Tags is the fixed list of PLC tag names to poll.
	Tags []string `mapstructure:"tags"`

	// Timeout is the per-request timeout for CIP operations.
	Timeout time.Duration `mapstructure:"timeout"`
}

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	if len(cfg.Tags) == 0 {
		return errors.New("at least one tag must be specified")
	}
	return nil
}

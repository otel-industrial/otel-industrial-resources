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

	// Host is the address of the EtherNet/IP device, e.g. "192.168.1.10"
	// or a hostname. Do not include a port here; use Port instead.
	Host string `mapstructure:"host"`

	// Port is the CIP port on the device. Defaults to 44818, the
	// standard EtherNet/IP port.
	Port uint `mapstructure:"port"`

	// DeviceName is an optional user-friendly name for the device,
	// populated into the ethernetip.device.name resource attribute.
	// If empty, that attribute is omitted from emitted metrics.
	DeviceName string `mapstructure:"device_name"`

	// Tags is the fixed list of PLC tag names to poll.
	Tags []string `mapstructure:"tags"`

	// Timeout is the per-request timeout for CIP operations.
	Timeout time.Duration `mapstructure:"timeout"`
}

func (cfg *Config) Validate() error {
	if cfg.Host == "" {
		return errors.New("host must be specified")
	}
	if len(cfg.Tags) == 0 {
		return errors.New("at least one tag must be specified")
	}
	return nil
}

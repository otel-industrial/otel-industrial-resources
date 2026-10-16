package modbusreceiver

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/goburrow/modbus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/modbusreceiver/internal/metadata"
)

// errDecode marks errors caused by an unexpected response payload rather than
// by the transport. They do not require dropping the connection.
var errDecode = errors.New("decode error")

// dialFunc opens a connection to a Modbus device and returns a client plus the
// closer for the underlying transport. It is a field on the scraper so tests
// can substitute a fake device.
type dialFunc func(cfg *Config) (modbus.Client, io.Closer, error)

func dialTCP(cfg *Config) (modbus.Client, io.Closer, error) {
	handler := modbus.NewTCPClientHandler(cfg.Endpoint)
	handler.Timeout = cfg.RequestTimeout
	handler.SlaveId = byte(cfg.UnitID)
	if err := handler.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Modbus endpoint %s: %w", cfg.Endpoint, err)
	}
	return modbus.NewClient(handler), handler, nil
}

type modbusScraper struct {
	cfg    *Config
	logger *zap.Logger
	dial   dialFunc

	// mu guards the connection and mb, which accumulates data points
	// between Emit calls.
	mu     sync.Mutex
	mb     *metadata.MetricsBuilder
	client modbus.Client
	closer io.Closer
}

func newModbusScraper(cfg *Config, settings receiver.Settings) *modbusScraper {
	return &modbusScraper{
		cfg:    cfg,
		logger: settings.Logger,
		dial:   dialTCP,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// connectLocked establishes the connection to the Modbus device.
// Called lazily on first scrape and on reconnect after a transport failure.
// s.mu must be held.
func (s *modbusScraper) connectLocked() error {
	client, closer, err := s.dial(s.cfg)
	if err != nil {
		return err
	}
	s.client = client
	s.closer = closer
	s.logger.Info("Connected to Modbus device",
		zap.String("endpoint", s.cfg.Endpoint),
		zap.Int("unit_id", s.cfg.UnitID),
	)
	return nil
}

// disconnectLocked closes the transport and clears the client so the next
// scrape reconnects. s.mu must be held.
func (s *modbusScraper) disconnectLocked() error {
	var err error
	if s.closer != nil {
		err = s.closer.Close()
	}
	s.client = nil
	s.closer = nil
	return err
}

// shutdown closes the connection to the device.
func (s *modbusScraper) shutdown(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disconnectLocked()
}

// scrape polls all configured registers and returns a pmetric.Metrics.
//
// Error handling per register:
//   - Modbus exception responses (e.g. illegal data address) and decode errors
//     only affect that register; the scan continues on the same connection.
//   - Any other error is treated as a transport failure: the connection is
//     dropped, the rest of this scan is skipped, and the next scrape reconnects.
//   - If ctx is done (the controller's timeout), the rest of the scan is skipped.
//
// Metrics collected before a failure are still returned, together with a
// scrapererror.PartialScrapeError counting the registers that were not read.
func (s *modbusScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		if err := s.connectLocked(); err != nil {
			return pmetric.NewMetrics(), err
		}
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	var errs []error
	failed := 0
	for i, reg := range s.cfg.Registers {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("scan stopped at register %d: %w", reg.Address, err))
			failed += len(s.cfg.Registers) - i
			break
		}
		err := s.scrapeRegister(now, reg)
		if err == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("register %d (%s, %s): %w", reg.Address, reg.Type, reg.DataType, err))
		if !isTransportError(err) {
			failed++
			continue
		}
		// This register and every one after it in the scan were not read.
		failed += len(s.cfg.Registers) - i
		if cerr := s.disconnectLocked(); cerr != nil {
			s.logger.Debug("Error closing Modbus connection", zap.Error(cerr))
		}
		break
	}

	md := s.mb.Emit()
	if len(errs) > 0 {
		return md, scrapererror.NewPartialScrapeError(errors.Join(errs...), failed)
	}
	return md, nil
}

// isTransportError reports whether err means the connection is no longer usable.
func isTransportError(err error) bool {
	var mbErr *modbus.ModbusError
	if errors.As(err, &mbErr) {
		return false
	}
	return !errors.Is(err, errDecode)
}

func (s *modbusScraper) scrapeRegister(now pcommon.Timestamp, reg RegisterDefinition) error {
	switch reg.Type {
	case RegisterTypeCoil:
		raw, err := s.client.ReadCoils(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadCoils(%d): %w", reg.Address, err)
		}
		s.recordCoil(now, reg, raw)

	case RegisterTypeDiscreteInput:
		raw, err := s.client.ReadDiscreteInputs(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadDiscreteInputs(%d): %w", reg.Address, err)
		}
		s.recordCoil(now, reg, raw)

	case RegisterTypeHoldingRegister, RegisterTypeInputRegister:
		count := reg.registerCount()
		var (
			raw []byte
			err error
		)
		if reg.Type == RegisterTypeHoldingRegister {
			raw, err = s.client.ReadHoldingRegisters(reg.Address, count)
		} else {
			raw, err = s.client.ReadInputRegisters(reg.Address, count)
		}
		if err != nil {
			return fmt.Errorf("ReadRegisters(%d, count=%d): %w", reg.Address, count, err)
		}
		raw = reg.ByteOrder.reorder(raw)
		return s.decodeAndRecord(now, reg, raw)
	}

	return nil
}

func bitValue(raw []byte) int64 {
	if len(raw) > 0 && raw[0]&0x01 == 1 {
		return 1
	}
	return 0
}

func (s *modbusScraper) recordCoil(now pcommon.Timestamp, reg RegisterDefinition, raw []byte) {
	s.mb.RecordModbusCoilValueDataPoint(now, bitValue(raw),
		reg.Name,
		int64(reg.Address),
		metadata.MapAttributeModbusRegisterType[string(reg.Type)],
		int64(s.cfg.UnitID),
	)
}

// decodeAndRecord decodes raw register bytes and records them as a
// modbus.register.value data point. metadata.yaml declares the metric as a
// double gauge, so every data type is converted to float64; int64 and uint64
// values beyond 2^53 lose precision.
func (s *modbusScraper) decodeAndRecord(now pcommon.Timestamp, reg RegisterDefinition, raw []byte) error {
	need := int(reg.registerCount()) * 2
	if len(raw) < need {
		return fmt.Errorf("%w: not enough bytes for %s: got %d, want %d", errDecode, reg.DataType, len(raw), need)
	}

	var val float64
	switch reg.DataType {
	case DataTypeInt16:
		val = float64(int16(binary.BigEndian.Uint16(raw)))
	case DataTypeInt32:
		val = float64(int32(binary.BigEndian.Uint32(raw)))
	case DataTypeInt64:
		val = float64(int64(binary.BigEndian.Uint64(raw)))
	case DataTypeUint16:
		val = float64(binary.BigEndian.Uint16(raw))
	case DataTypeUint32:
		val = float64(binary.BigEndian.Uint32(raw))
	case DataTypeUint64:
		val = float64(binary.BigEndian.Uint64(raw))
	case DataTypeFloat32:
		val = float64(math.Float32frombits(binary.BigEndian.Uint32(raw)))
	case DataTypeFloat64:
		val = math.Float64frombits(binary.BigEndian.Uint64(raw))
	default:
		return fmt.Errorf("%w: unsupported data_type %q", errDecode, reg.DataType)
	}

	s.mb.RecordModbusRegisterValueDataPoint(now, val,
		metadata.MapAttributeModbusByteOrder[string(reg.ByteOrder)],
		metadata.MapAttributeModbusDataType[string(reg.DataType)],
		reg.Name,
		int64(reg.Address),
		metadata.MapAttributeModbusRegisterType[string(reg.Type)],
		int64(s.cfg.UnitID),
	)
	return nil
}

package modbusreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/goburrow/modbus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/industrialscraper"
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

// modbusConn implements industrialscraper.Conn over a goburrow client.
type modbusConn struct {
	cfg    *Config
	logger *zap.Logger
	dial   dialFunc
	client modbus.Client
	closer io.Closer
}

func (c *modbusConn) Connect(context.Context) error {
	client, closer, err := c.dial(c.cfg)
	if err != nil {
		return err
	}
	c.client, c.closer = client, closer
	c.logger.Info("Connected to Modbus device",
		zap.String("endpoint", c.cfg.Endpoint),
		zap.Int("unit_id", c.cfg.UnitID),
	)
	return nil
}

func (c *modbusConn) Close(context.Context) error {
	var err error
	if c.closer != nil {
		err = c.closer.Close()
	}
	c.client, c.closer = nil, nil
	return err
}

type modbusScraper struct {
	cfg  *Config
	conn *modbusConn
	sess *industrialscraper.Session
	lim  *industrialscraper.Limiter

	// mu guards the connection and mb, which accumulates data points
	// between Emit calls.
	mu sync.Mutex
	mb *metadata.MetricsBuilder
}

func newModbusScraper(cfg *Config, settings receiver.Settings, dial dialFunc) *modbusScraper {
	conn := &modbusConn{cfg: cfg, logger: settings.Logger, dial: dial}
	return &modbusScraper{
		cfg:  cfg,
		conn: conn,
		sess: industrialscraper.NewSession(conn, cfg.ReconnectOnFailure),
		lim:  industrialscraper.DeviceLimiter(cfg.Endpoint),
		mb:   metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// shutdown closes the connection to the device.
func (s *modbusScraper) shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sess.Close(ctx)
}

// scrape polls all configured registers and returns a pmetric.Metrics.
//
// Polls to one endpoint are serialized process-wide, and a device that is
// down is redialed only after the reconnect_on_failure backoff.
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
	if err := s.lim.Acquire(ctx); err != nil {
		return pmetric.NewMetrics(), err
	}
	defer s.lim.Release()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sess.EnsureLive(ctx); err != nil {
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	var failures industrialscraper.Failures
	for i, reg := range s.cfg.Registers {
		if err := ctx.Err(); err != nil {
			failures.AddN(len(s.cfg.Registers)-i, fmt.Errorf("scan stopped at register %d: %w", reg.Address, err))
			break
		}
		err := s.scrapeRegister(now, reg)
		if err == nil {
			continue
		}
		err = fmt.Errorf("register %d (%s, %s): %w", reg.Address, reg.Type, reg.DataType, err)
		if !isTransportError(err) {
			failures.Add(err)
			continue
		}
		// This register and every one after it in the scan were not read.
		failures.AddN(len(s.cfg.Registers)-i, err)
		if cerr := s.sess.Close(ctx); cerr != nil {
			s.conn.logger.Debug("Error closing Modbus connection", zap.Error(cerr))
		}
		break
	}

	return s.mb.Emit(), failures.Err()
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
		raw, err := s.conn.client.ReadCoils(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadCoils(%d): %w", reg.Address, err)
		}
		s.recordCoil(now, reg, raw)

	case RegisterTypeDiscreteInput:
		raw, err := s.conn.client.ReadDiscreteInputs(reg.Address, 1)
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
			raw, err = s.conn.client.ReadHoldingRegisters(reg.Address, count)
		} else {
			raw, err = s.conn.client.ReadInputRegisters(reg.Address, count)
		}
		if err != nil {
			return fmt.Errorf("ReadRegisters(%d, count=%d): %w", reg.Address, count, err)
		}
		raw = industrialscraper.Reorder(industrialscraper.ByteOrder(reg.ByteOrder), raw)
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
		val = float64(industrialscraper.Int16(raw))
	case DataTypeInt32:
		val = float64(industrialscraper.Int32(raw))
	case DataTypeInt64:
		val = float64(industrialscraper.Int64(raw))
	case DataTypeUint16:
		val = float64(industrialscraper.Uint16(raw))
	case DataTypeUint32:
		val = float64(industrialscraper.Uint32(raw))
	case DataTypeUint64:
		val = float64(industrialscraper.Uint64(raw))
	case DataTypeFloat32:
		val = float64(industrialscraper.Float32(raw))
	case DataTypeFloat64:
		val = industrialscraper.Float64(raw)
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

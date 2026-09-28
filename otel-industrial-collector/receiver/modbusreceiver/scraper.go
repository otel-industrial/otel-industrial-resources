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
	"go.uber.org/zap"
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
	handler.Timeout = cfg.Timeout
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

	mu     sync.Mutex
	client modbus.Client
	closer io.Closer
}

func newModbusScraper(cfg *Config, logger *zap.Logger) *modbusScraper {
	return &modbusScraper{cfg: cfg, logger: logger, dial: dialTCP}
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
//     Metrics collected before the failure are still returned.
func (s *modbusScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		if err := s.connectLocked(); err != nil {
			return pmetric.NewMetrics(), err
		}
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/modbusreceiver")

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, reg := range s.cfg.Registers {
		err := s.scrapeRegister(sm, now, reg)
		if err == nil {
			continue
		}
		s.logger.Warn("Failed to scrape register",
			zap.Uint16("address", reg.Address),
			zap.String("type", string(reg.Type)),
			zap.String("data_type", string(reg.DataType)),
			zap.Error(err),
		)
		if !isTransportError(err) {
			continue
		}
		if cerr := s.disconnectLocked(); cerr != nil {
			s.logger.Debug("Error closing Modbus connection", zap.Error(cerr))
		}
		break
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

func (s *modbusScraper) scrapeRegister(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition) error {
	switch reg.Type {
	case RegisterTypeCoil:
		raw, err := s.client.ReadCoils(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadCoils(%d): %w", reg.Address, err)
		}
		s.appendIntMetric(sm, now, reg, "modbus.coil.value", "The value read from a Modbus coil (0 or 1).", bitValue(raw))

	case RegisterTypeDiscreteInput:
		raw, err := s.client.ReadDiscreteInputs(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadDiscreteInputs(%d): %w", reg.Address, err)
		}
		s.appendIntMetric(sm, now, reg, "modbus.coil.value", "The value read from a Modbus discrete input (0 or 1).", bitValue(raw))

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
		return s.decodeAndAppend(sm, now, reg, raw)
	}

	return nil
}

func bitValue(raw []byte) int64 {
	if len(raw) > 0 && raw[0]&0x01 == 1 {
		return 1
	}
	return 0
}

func (s *modbusScraper) decodeAndAppend(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition, raw []byte) error {
	const metricName = "modbus.register.value"
	const metricDesc = "The decoded value read from a Modbus register."

	need := int(reg.registerCount()) * 2
	if len(raw) < need {
		return fmt.Errorf("%w: not enough bytes for %s: got %d, want %d", errDecode, reg.DataType, len(raw), need)
	}

	switch reg.DataType {
	case DataTypeInt16:
		val := int64(int16(binary.BigEndian.Uint16(raw)))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeInt32:
		val := int64(int32(binary.BigEndian.Uint32(raw)))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeInt64:
		val := int64(binary.BigEndian.Uint64(raw))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint16:
		val := float64(binary.BigEndian.Uint16(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint32:
		val := float64(binary.BigEndian.Uint32(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint64:
		val := float64(binary.BigEndian.Uint64(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeFloat32:
		val := float64(math.Float32frombits(binary.BigEndian.Uint32(raw)))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeFloat64:
		val := math.Float64frombits(binary.BigEndian.Uint64(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	default:
		return fmt.Errorf("%w: unsupported data_type %q", errDecode, reg.DataType)
	}

	return nil
}

func (s *modbusScraper) appendIntMetric(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition, name, desc string, val int64) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit("1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntValue(val)
	setAttributes(dp.Attributes(), reg, s.cfg.UnitID)
}

func (s *modbusScraper) appendDoubleMetric(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition, name, desc string, val float64) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit("1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetDoubleValue(val)
	setAttributes(dp.Attributes(), reg, s.cfg.UnitID)
}

// setAttributes sets the data point attributes declared in metadata.yaml.
// modbus.data_type and modbus.byte_order only apply to register banks;
// coils and discrete inputs are single bits, so they are omitted there.
func setAttributes(attrs pcommon.Map, reg RegisterDefinition, unitID int) {
	attrs.PutInt("modbus.register_address", int64(reg.Address))
	attrs.PutInt("modbus.unit_id", int64(unitID))
	attrs.PutStr("modbus.register_type", string(reg.Type))
	if reg.Type == RegisterTypeHoldingRegister || reg.Type == RegisterTypeInputRegister {
		attrs.PutStr("modbus.data_type", string(reg.DataType))
		attrs.PutStr("modbus.byte_order", string(reg.ByteOrder))
	}
	if reg.Name != "" {
		attrs.PutStr("modbus.name", reg.Name)
	}
}

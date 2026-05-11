package modbusreceiver

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/goburrow/modbus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type modbusScraper struct {
	cfg     *Config
	logger  *zap.Logger
	handler modbus.ClientHandler
	client  modbus.Client
}

func newModbusScraper(cfg *Config, logger *zap.Logger) *modbusScraper {
	return &modbusScraper{cfg: cfg, logger: logger}
}

// start opens the TCP connection to the Modbus device.
func (s *modbusScraper) start(_ context.Context) error {
	handler := modbus.NewTCPClientHandler(s.cfg.Endpoint)
	handler.Timeout = s.cfg.Timeout
	handler.SlaveId = s.cfg.UnitID

	if err := handler.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Modbus endpoint %s: %w", s.cfg.Endpoint, err)
	}

	s.handler = handler
	s.client = modbus.NewClient(handler)
	s.logger.Info("Connected to Modbus device",
		zap.String("endpoint", s.cfg.Endpoint),
		zap.Uint8("unit_id", s.cfg.UnitID),
	)
	return nil
}

// shutdown closes the TCP connection.
func (s *modbusScraper) shutdown(_ context.Context) error {
	if h, ok := s.handler.(*modbus.TCPClientHandler); ok {
		return h.Close()
	}
	return nil
}

// scrape polls all configured registers and returns a pmetric.Metrics.
func (s *modbusScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/modbusreceiver")

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, reg := range s.cfg.Registers {
		if err := s.scrapeRegister(sm, now, reg); err != nil {
			s.logger.Warn("Failed to scrape register",
				zap.Uint16("address", reg.Address),
				zap.String("type", string(reg.Type)),
				zap.String("data_type", string(reg.DataType)),
				zap.Error(err),
			)
		}
	}

	return md, nil
}

func (s *modbusScraper) scrapeRegister(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition) error {
	switch reg.Type {
	case RegisterTypeCoil:
		raw, err := s.client.ReadCoils(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadCoils(%d): %w", reg.Address, err)
		}
		val := int64(0)
		if len(raw) > 0 && raw[0]&0x01 == 1 {
			val = 1
		}
		s.appendIntMetric(sm, now, reg, "modbus.coil.value", "The value read from a Modbus coil (0 or 1).", val)

	case RegisterTypeDiscreteInput:
		raw, err := s.client.ReadDiscreteInputs(reg.Address, 1)
		if err != nil {
			return fmt.Errorf("ReadDiscreteInputs(%d): %w", reg.Address, err)
		}
		val := int64(0)
		if len(raw) > 0 && raw[0]&0x01 == 1 {
			val = 1
		}
		s.appendIntMetric(sm, now, reg, "modbus.coil.value", "The value read from a Modbus discrete input (0 or 1).", val)

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

		// Apply byte order reordering before decoding.
		raw = reg.ByteOrder.reorder(raw)

		return s.decodeAndAppend(sm, now, reg, raw)
	}

	return nil
}

// decodeAndAppend decodes raw (already reordered) bytes into the correct Go type
// and appends it as a metric data point.
func (s *modbusScraper) decodeAndAppend(sm pmetric.ScopeMetrics, now pcommon.Timestamp, reg RegisterDefinition, raw []byte) error {
	const metricName = "modbus.register.value"
	const metricDesc = "The decoded value read from a Modbus register."

	switch reg.DataType {
	case DataTypeInt16:
		if len(raw) < 2 {
			return fmt.Errorf("not enough bytes for int16: got %d", len(raw))
		}
		val := int64(int16(binary.BigEndian.Uint16(raw)))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeInt32:
		if len(raw) < 4 {
			return fmt.Errorf("not enough bytes for int32: got %d", len(raw))
		}
		val := int64(int32(binary.BigEndian.Uint32(raw)))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeInt64:
		if len(raw) < 8 {
			return fmt.Errorf("not enough bytes for int64: got %d", len(raw))
		}
		val := int64(binary.BigEndian.Uint64(raw))
		s.appendIntMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint16:
		if len(raw) < 2 {
			return fmt.Errorf("not enough bytes for uint16: got %d", len(raw))
		}
		val := float64(binary.BigEndian.Uint16(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint32:
		if len(raw) < 4 {
			return fmt.Errorf("not enough bytes for uint32: got %d", len(raw))
		}
		val := float64(binary.BigEndian.Uint32(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeUint64:
		if len(raw) < 8 {
			return fmt.Errorf("not enough bytes for uint64: got %d", len(raw))
		}
		val := float64(binary.BigEndian.Uint64(raw))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeFloat32:
		if len(raw) < 4 {
			return fmt.Errorf("not enough bytes for float32: got %d", len(raw))
		}
		bits := binary.BigEndian.Uint32(raw)
		val := float64(math.Float32frombits(bits))
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	case DataTypeFloat64:
		if len(raw) < 8 {
			return fmt.Errorf("not enough bytes for float64: got %d", len(raw))
		}
		bits := binary.BigEndian.Uint64(raw)
		val := math.Float64frombits(bits)
		s.appendDoubleMetric(sm, now, reg, metricName, metricDesc, val)

	default:
		return fmt.Errorf("unsupported data_type %q", reg.DataType)
	}

	return nil
}

func (s *modbusScraper) appendIntMetric(
	sm pmetric.ScopeMetrics,
	now pcommon.Timestamp,
	reg RegisterDefinition,
	name, desc string,
	val int64,
) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit("1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntValue(val)
	setAttributes(dp.Attributes(), reg, s.cfg.UnitID)
}

func (s *modbusScraper) appendDoubleMetric(
	sm pmetric.ScopeMetrics,
	now pcommon.Timestamp,
	reg RegisterDefinition,
	name, desc string,
	val float64,
) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit("1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetDoubleValue(val)
	setAttributes(dp.Attributes(), reg, s.cfg.UnitID)
}

func setAttributes(attrs pcommon.Map, reg RegisterDefinition, unitID byte) {
	attrs.PutInt("modbus.register_address", int64(reg.Address))
	attrs.PutInt("modbus.unit_id", int64(unitID))
	attrs.PutStr("modbus.register_type", string(reg.Type))
	attrs.PutStr("modbus.data_type", string(reg.DataType))
	attrs.PutStr("modbus.byte_order", string(reg.ByteOrder))
	if reg.Name != "" {
		attrs.PutStr("modbus.name", reg.Name)
	}
}
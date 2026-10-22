package modbusreceiver

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/goburrow/modbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/receiver/modbusreceiver/internal/metadata"
)

// fakeDevice is an in-memory Modbus device. It embeds modbus.Client so it
// satisfies the interface; only the read methods used by the scraper are
// implemented, and calling anything else panics.
type fakeDevice struct {
	modbus.Client

	mu     sync.Mutex
	read   func(kind RegisterType, address, quantity uint16) ([]byte, error)
	closed int
}

func (f *fakeDevice) do(kind RegisterType, address, quantity uint16) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.read(kind, address, quantity)
}

func (f *fakeDevice) setRead(fn func(kind RegisterType, address, quantity uint16) ([]byte, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.read = fn
}

func (f *fakeDevice) ReadCoils(a, q uint16) ([]byte, error) {
	return f.do(RegisterTypeCoil, a, q)
}

func (f *fakeDevice) ReadDiscreteInputs(a, q uint16) ([]byte, error) {
	return f.do(RegisterTypeDiscreteInput, a, q)
}

func (f *fakeDevice) ReadHoldingRegisters(a, q uint16) ([]byte, error) {
	return f.do(RegisterTypeHoldingRegister, a, q)
}

func (f *fakeDevice) ReadInputRegisters(a, q uint16) ([]byte, error) {
	return f.do(RegisterTypeInputRegister, a, q)
}

func (f *fakeDevice) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

func (f *fakeDevice) closedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// okRead returns a zero-filled response of the right size for every request.
func okRead(kind RegisterType, _, quantity uint16) ([]byte, error) {
	if kind == RegisterTypeCoil || kind == RegisterTypeDiscreteInput {
		return []byte{0x01}, nil
	}
	return make([]byte, int(quantity)*2), nil
}

// newTestScraper returns a scraper wired to dev, plus a counter of dial calls.
func newTestScraper(cfg *Config, dev *fakeDevice) (*modbusScraper, *int) {
	dials := 0
	s := newModbusScraper(cfg, testSettings())
	s.dial = func(*Config) (modbus.Client, io.Closer, error) {
		dials++
		return dev, dev, nil
	}
	return s, &dials
}

func testSettings() receiver.Settings {
	return receivertest.NewNopSettings(metadata.Type)
}

func testConfig(regs ...RegisterDefinition) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Registers = regs
	return cfg
}

func holding(addr uint16, dt DataType) RegisterDefinition {
	return RegisterDefinition{Address: addr, Type: RegisterTypeHoldingRegister, DataType: dt, ByteOrder: ByteOrderABCD}
}

// dataPoints returns all data points in md keyed by modbus.register_address.
func dataPoints(t *testing.T, md pmetric.Metrics) map[int64]pmetric.NumberDataPoint {
	t.Helper()
	out := map[int64]pmetric.NumberDataPoint{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				dps := ms.At(k).Gauge().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dp := dps.At(l)
					addr, ok := dp.Attributes().Get("modbus.register_address")
					require.True(t, ok, "data point without modbus.register_address")
					out[addr.Int()] = dp
				}
			}
		}
	}
	return out
}

func TestScrapeDecodesDataTypes(t *testing.T) {
	tests := []struct {
		name      string
		dataType  DataType
		byteOrder ByteOrder
		raw       []byte
		want      float64
	}{
		{name: "int16 negative", dataType: DataTypeInt16, raw: []byte{0xFF, 0xFE}, want: -2},
		{name: "uint16", dataType: DataTypeUint16, raw: []byte{0x03, 0xF5}, want: 1013},
		{name: "int32", dataType: DataTypeInt32, raw: []byte{0xFF, 0xFF, 0xFF, 0x9C}, want: -100},
		{name: "uint32", dataType: DataTypeUint32, raw: []byte{0x00, 0x01, 0x00, 0x00}, want: 65536},
		{name: "int64", dataType: DataTypeInt64, raw: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, want: -1},
		{name: "uint64", dataType: DataTypeUint64, raw: []byte{0, 0, 0, 0, 0, 0, 0x01, 0x00}, want: 256},
		{name: "float32 ABCD", dataType: DataTypeFloat32, raw: []byte{0x41, 0xB4, 0x00, 0x00}, want: 22.5},
		{name: "float32 CDAB", dataType: DataTypeFloat32, byteOrder: ByteOrderCDAB, raw: []byte{0x00, 0x00, 0x41, 0xB4}, want: 22.5},
		{name: "float64", dataType: DataTypeFloat64, raw: float64Bytes(-3.25), want: -3.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := holding(7, tt.dataType)
			if tt.byteOrder != "" {
				reg.ByteOrder = tt.byteOrder
			}
			dev := &fakeDevice{read: func(kind RegisterType, address, quantity uint16) ([]byte, error) {
				assert.Equal(t, RegisterTypeHoldingRegister, kind)
				assert.Equal(t, uint16(7), address)
				assert.Equal(t, tt.dataType.registerWidth(), quantity)
				return tt.raw, nil
			}}
			s, _ := newTestScraper(testConfig(reg), dev)

			md, err := s.scrape(context.Background())
			require.NoError(t, err)
			dp, ok := dataPoints(t, md)[7]
			require.True(t, ok)

			// metadata.yaml declares modbus.register.value as a double gauge
			// for every data type.
			require.Equal(t, pmetric.NumberDataPointValueTypeDouble, dp.ValueType())
			assert.InDelta(t, tt.want, dp.DoubleValue(), 1e-9)
		})
	}
}

// All registers of a kind share one metric, with one data point per register,
// and the metric metadata comes from metadata.yaml.
func TestScrapeGroupsDataPointsByMetric(t *testing.T) {
	cfg := testConfig(
		RegisterDefinition{Address: 1, Type: RegisterTypeCoil, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
		RegisterDefinition{Address: 2, Type: RegisterTypeDiscreteInput, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
		holding(3, DataTypeInt16),
		holding(4, DataTypeUint32),
		RegisterDefinition{Address: 6, Type: RegisterTypeInputRegister, DataType: DataTypeFloat32, ByteOrder: ByteOrderABCD},
	)
	s, _ := newTestScraper(cfg, &fakeDevice{read: okRead})

	md, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	sm := md.ResourceMetrics().At(0).ScopeMetrics()
	require.Equal(t, 1, sm.Len())
	assert.Equal(t, metadata.ScopeName, sm.At(0).Scope().Name())

	got := map[string]pmetric.Metric{}
	ms := sm.At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		got[ms.At(i).Name()] = ms.At(i)
	}
	require.Len(t, got, 2)

	coil := got["modbus.coil.value"]
	assert.Equal(t, "The value read from a Modbus coil or discrete input (0 or 1).", coil.Description())
	assert.Equal(t, 2, coil.Gauge().DataPoints().Len())

	reg := got["modbus.register.value"]
	assert.Equal(t, "The decoded value read from a Modbus holding or input register.", reg.Description())
	assert.Equal(t, 3, reg.Gauge().DataPoints().Len())
}

func TestScrapeRespectsMetricEnabled(t *testing.T) {
	cfg := testConfig(
		RegisterDefinition{Address: 1, Type: RegisterTypeCoil, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
		holding(2, DataTypeUint16),
	)
	cfg.Metrics.ModbusCoilValue.Enabled = false
	s, _ := newTestScraper(cfg, &fakeDevice{read: okRead})

	md, err := s.scrape(context.Background())
	require.NoError(t, err)
	dps := dataPoints(t, md)
	assert.NotContains(t, dps, int64(1), "disabled modbus.coil.value should not be emitted")
	assert.Contains(t, dps, int64(2))
}

func TestScrapeAttributes(t *testing.T) {
	cfg := testConfig(
		RegisterDefinition{Address: 1, Type: RegisterTypeCoil, DataType: DataTypeBool, ByteOrder: ByteOrderABCD, Name: "pump"},
		RegisterDefinition{Address: 2, Type: RegisterTypeDiscreteInput, DataType: DataTypeBool, ByteOrder: ByteOrderABCD},
		RegisterDefinition{Address: 3, Type: RegisterTypeInputRegister, DataType: DataTypeUint16, ByteOrder: ByteOrderDCBA, Name: "temp"},
	)
	s, _ := newTestScraper(cfg, &fakeDevice{read: okRead})

	md, err := s.scrape(context.Background())
	require.NoError(t, err)
	dps := dataPoints(t, md)
	require.Len(t, dps, 3)

	// Bit banks: no data_type / byte_order (not declared for modbus.coil.value).
	for _, addr := range []int64{1, 2} {
		attrs := dps[addr].Attributes()
		_, ok := attrs.Get("modbus.data_type")
		assert.False(t, ok, "address %d: unexpected modbus.data_type", addr)
		_, ok = attrs.Get("modbus.byte_order")
		assert.False(t, ok, "address %d: unexpected modbus.byte_order", addr)
		assert.Equal(t, int64(1), dps[addr].IntValue())
	}
	name, ok := dps[1].Attributes().Get("modbus.name")
	require.True(t, ok)
	assert.Equal(t, "pump", name.Str())
	// The generated builder always sets declared attributes.
	name, ok = dps[2].Attributes().Get("modbus.name")
	require.True(t, ok)
	assert.Empty(t, name.Str())

	// Register bank: all attributes present.
	attrs := dps[3].Attributes()
	for key, want := range map[string]string{
		"modbus.register_type": "input_register",
		"modbus.data_type":     "uint16",
		"modbus.byte_order":    "DCBA",
		"modbus.name":          "temp",
	} {
		v, ok := attrs.Get(key)
		require.True(t, ok, "missing %s", key)
		assert.Equal(t, want, v.Str(), key)
	}
	unit, ok := attrs.Get("modbus.unit_id")
	require.True(t, ok)
	assert.Equal(t, int64(1), unit.Int())
}

// Regression test for a mid-scan transport error: the scraper used to clear
// its client and keep looping, so the next register dereferenced a nil client
// and panicked, killing the poll loop.
func TestScrapeTransportErrorMidScan(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeUint16), holding(1, DataTypeUint16), holding(2, DataTypeUint16))
	dev := &fakeDevice{}
	dev.setRead(func(kind RegisterType, address, quantity uint16) ([]byte, error) {
		if address == 1 {
			return nil, io.EOF
		}
		return okRead(kind, address, quantity)
	})
	s, dials := newTestScraper(cfg, dev)

	var md pmetric.Metrics
	require.NotPanics(t, func() {
		var err error
		md, err = s.scrape(context.Background())
		// Registers 1 and 2 were not read.
		requirePartial(t, err, 2)
	})

	// Register 0 was read before the failure; the rest of the scan was skipped.
	dps := dataPoints(t, md)
	assert.Contains(t, dps, int64(0))
	assert.NotContains(t, dps, int64(1))
	assert.NotContains(t, dps, int64(2))
	assert.Equal(t, 1, dev.closedCount(), "connection should be closed after a transport error")
	assert.Nil(t, s.client)

	// The device recovers: the next scrape reconnects and reads everything.
	dev.setRead(okRead)
	md, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Len(t, dataPoints(t, md), 3)
	assert.Equal(t, 2, *dials, "scraper should redial after a transport error")
}

// A Modbus exception response means the device is reachable but rejected one
// request (e.g. illegal data address). Only that register should be skipped.
func TestScrapeModbusExceptionKeepsConnection(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeUint16), holding(1, DataTypeUint16), holding(2, DataTypeUint16))
	dev := &fakeDevice{read: func(kind RegisterType, address, quantity uint16) ([]byte, error) {
		if address == 1 {
			return nil, &modbus.ModbusError{FunctionCode: 0x03, ExceptionCode: modbus.ExceptionCodeIllegalDataAddress}
		}
		return okRead(kind, address, quantity)
	}}
	s, dials := newTestScraper(cfg, dev)

	md, err := s.scrape(context.Background())
	requirePartial(t, err, 1)
	dps := dataPoints(t, md)
	assert.Contains(t, dps, int64(0))
	assert.NotContains(t, dps, int64(1))
	assert.Contains(t, dps, int64(2))
	assert.Equal(t, 0, dev.closedCount())
	assert.NotNil(t, s.client)

	_, err = s.scrape(context.Background())
	requirePartial(t, err, 1)
	assert.Equal(t, 1, *dials, "an exception response must not force a reconnect")
}

func TestScrapeShortResponseKeepsConnection(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeFloat32), holding(2, DataTypeUint16))
	dev := &fakeDevice{read: func(kind RegisterType, address, quantity uint16) ([]byte, error) {
		if address == 0 {
			return []byte{0x41, 0xB4}, nil // float32 needs 4 bytes
		}
		return okRead(kind, address, quantity)
	}}
	s, _ := newTestScraper(cfg, dev)

	md, err := s.scrape(context.Background())
	requirePartial(t, err, 1)
	dps := dataPoints(t, md)
	assert.NotContains(t, dps, int64(0))
	assert.Contains(t, dps, int64(2))
	assert.Equal(t, 0, dev.closedCount())
}

func TestScrapeDialError(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeUint16))
	dev := &fakeDevice{read: okRead}
	s := newModbusScraper(cfg, testSettings())
	dialErr := errors.New("connection refused")
	fail := true
	s.dial = func(*Config) (modbus.Client, io.Closer, error) {
		if fail {
			return nil, nil, dialErr
		}
		return dev, dev, nil
	}

	md, err := s.scrape(context.Background())
	require.ErrorIs(t, err, dialErr)
	assert.Equal(t, 0, md.DataPointCount())

	fail = false
	md, err = s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, md.DataPointCount())
}

// dialTCP against a device that accepts the connection but never answers:
// the read must fail after request_timeout instead of hanging. The kernel
// completes the handshake from the listen backlog, so no Accept is needed.
func TestDialTCPAppliesRequestTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	cfg := testConfig(holding(0, DataTypeUint16))
	cfg.Endpoint = ln.Addr().String()
	cfg.RequestTimeout = 200 * time.Millisecond

	client, closer, err := dialTCP(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })

	start := time.Now()
	_, err = client.ReadHoldingRegisters(0, 1)
	elapsed := time.Since(start)

	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "want a timeout error, got %v", err)
	assert.GreaterOrEqual(t, elapsed, cfg.RequestTimeout)
	assert.Less(t, elapsed, 2*time.Second, "read should give up after request_timeout")
}

func TestDialTCPConnectError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg := testConfig(holding(0, DataTypeUint16))
	cfg.Endpoint = ln.Addr().String()
	require.NoError(t, ln.Close()) // nothing listens on this port now

	_, _, err = dialTCP(cfg)
	require.ErrorContains(t, err, "failed to connect to Modbus endpoint "+cfg.Endpoint)
}

func TestScraperShutdownClosesConnection(t *testing.T) {
	dev := &fakeDevice{read: okRead}
	s, _ := newTestScraper(testConfig(holding(0, DataTypeUint16)), dev)

	// Shutdown before any scrape is a no-op.
	require.NoError(t, s.shutdown(context.Background()))
	assert.Equal(t, 0, dev.closedCount())

	_, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.shutdown(context.Background()))
	assert.Equal(t, 1, dev.closedCount())
	assert.Nil(t, s.client)
}

func TestScrapeStopsWhenContextDone(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeUint16), holding(1, DataTypeUint16))
	dev := &fakeDevice{read: okRead}
	s, _ := newTestScraper(cfg, dev)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	md, err := s.scrape(ctx)
	requirePartial(t, err, 2)
	// PartialScrapeError does not unwrap, so check the message.
	assert.ErrorContains(t, err, context.Canceled.Error())
	assert.Equal(t, 0, md.DataPointCount())
	assert.Equal(t, 0, dev.closedCount(), "a timed-out scan should keep the connection")
}

// End to end through the scraperhelper controller: a transport error must
// not stop polling.
func TestReceiverKeepsPollingAfterTransportError(t *testing.T) {
	cfg := testConfig(holding(0, DataTypeUint16), holding(1, DataTypeUint16))
	cfg.CollectionInterval = 10 * time.Millisecond
	cfg.InitialDelay = 0

	var mu sync.Mutex
	calls := 0
	dev := &fakeDevice{read: func(kind RegisterType, address, quantity uint16) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return okRead(kind, address, quantity)
	}}

	sink := new(consumertest.MetricsSink)
	r, err := newMetricsReceiver(cfg, testSettings(), sink, func(*Config) (modbus.Client, io.Closer, error) {
		return dev, dev, nil
	})
	require.NoError(t, err)

	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool { return sink.DataPointCount() >= 2 }, 5*time.Second, 10*time.Millisecond,
		"receiver should keep polling and recover after a transport error")
	require.NoError(t, r.Shutdown(context.Background()))
	assert.GreaterOrEqual(t, dev.closedCount(), 1, "shutdown should close the connection")
}

func requirePartial(t *testing.T, err error, failed int) {
	t.Helper()
	require.Error(t, err)
	var pe scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, failed, pe.Failed)
}

func float64Bytes(v float64) []byte {
	bits := math.Float64bits(v)
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = byte(bits)
		bits >>= 8
	}
	return out
}

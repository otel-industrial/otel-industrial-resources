package modbusreceiver

import (
	"errors"
	"fmt"
	"time"
)

// RegisterType represents the Modbus register bank to read from.
type RegisterType string

const (
	RegisterTypeCoil            RegisterType = "coil"
	RegisterTypeDiscreteInput   RegisterType = "discrete_input"
	RegisterTypeHoldingRegister RegisterType = "holding_register"
	RegisterTypeInputRegister   RegisterType = "input_register"
)

// DataType describes how the raw register bytes are interpreted.
type DataType string

const (
	DataTypeBool    DataType = "bool"
	DataTypeInt16   DataType = "int16"
	DataTypeInt32   DataType = "int32"
	DataTypeInt64   DataType = "int64"
	DataTypeUint16  DataType = "uint16"
	DataTypeUint32  DataType = "uint32"
	DataTypeUint64  DataType = "uint64"
	DataTypeFloat32 DataType = "float32"
	DataTypeFloat64 DataType = "float64"
)

// registerWidth returns how many 16-bit Modbus registers the data type occupies.
func (d DataType) registerWidth() uint16 {
	switch d {
	case DataTypeBool, DataTypeInt16, DataTypeUint16:
		return 1
	case DataTypeInt32, DataTypeUint32, DataTypeFloat32:
		return 2
	case DataTypeInt64, DataTypeUint64, DataTypeFloat64:
		return 4
	default:
		return 1
	}
}

// ByteOrder describes the byte/word arrangement of multi-register values.
//
//	Given four bytes A B C D (A = most-significant):
//	ABCD – big-endian, most significant byte first          (e.g. many PLCs)
//	DCBA – little-endian, least significant byte first      (e.g. x86-style)
//	BADC – big-endian words, bytes swapped within each word (e.g. some Schneider devices)
//	CDAB – little-endian words, bytes swapped within each word (e.g. some ABB devices)
type ByteOrder string

const (
	ByteOrderABCD ByteOrder = "ABCD" // big-endian
	ByteOrderDCBA ByteOrder = "DCBA" // little-endian
	ByteOrderBADC ByteOrder = "BADC" // mid-big / byte-swapped
	ByteOrderCDAB ByteOrder = "CDAB" // mid-little / word-swapped
)

// reorder rearranges raw Modbus bytes according to the byte order.
// raw must be exactly 2*n bytes (one byte pair per register).
// The returned slice is always the same length as raw.
func (bo ByteOrder) reorder(raw []byte) []byte {
	out := make([]byte, len(raw))
	switch bo {
	case ByteOrderABCD:
		copy(out, raw)

	case ByteOrderDCBA:
		for i, b := range raw {
			out[len(raw)-1-i] = b
		}

	case ByteOrderBADC:
		for i := 0; i+1 < len(raw); i += 2 {
			out[i] = raw[i+1]
			out[i+1] = raw[i]
		}

	case ByteOrderCDAB:
		for i := 0; i+1 < len(raw); i += 2 {
			j := len(raw) - 2 - i
			out[i] = raw[j]
			out[i+1] = raw[j+1]
		}
	}
	return out
}

// RegisterDefinition describes a single logical value to read from a Modbus device.
type RegisterDefinition struct {
	// Address is the zero-based Modbus register (or coil) address.
	Address uint16 `mapstructure:"address"`

	// Type selects the Modbus register bank (coil, discrete_input,
	// holding_register, input_register).
	Type RegisterType `mapstructure:"type"`

	// DataType controls how the raw bytes are decoded.
	// For coil / discrete_input registers only "bool" is valid.
	// Default: "uint16" for register banks, "bool" for bit banks.
	DataType DataType `mapstructure:"data_type"`

	// ByteOrder controls multi-register byte arrangement.
	// Ignored for single-register (16-bit) and bit types.
	// Default: ABCD (big-endian).
	ByteOrder ByteOrder `mapstructure:"byte_order"`

	// Name is an optional human-readable label emitted as a metric attribute.
	Name string `mapstructure:"name"`
}

// registerCount returns how many consecutive Modbus registers to request.
func (r *RegisterDefinition) registerCount() uint16 {
	return r.DataType.registerWidth()
}

// Config holds all configuration for the Modbus receiver.
type Config struct {
	// Endpoint is the Modbus TCP host:port (e.g. "192.168.1.10:502").
	Endpoint string `mapstructure:"endpoint"`

	// UnitID is the Modbus slave/unit identifier (1–247).
	UnitID byte `mapstructure:"unit_id"`

	// PollingInterval controls how often registers are read.
	PollingInterval time.Duration `mapstructure:"polling_interval"`

	// Timeout is the per-request TCP timeout.
	Timeout time.Duration `mapstructure:"timeout"`

	// Registers is the list of registers/coils to poll.
	Registers []RegisterDefinition `mapstructure:"registers"`
}

// Validate checks the configuration for required fields and valid values.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint must be set (e.g. \"192.168.1.10:502\")")
	}
	if c.UnitID == 0 {
		return errors.New("unit_id must be between 1 and 247")
	}
	if c.PollingInterval <= 0 {
		return errors.New("polling_interval must be a positive duration")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be a positive duration")
	}
	if len(c.Registers) == 0 {
		return errors.New("at least one register definition must be provided")
	}
	for i, r := range c.Registers {
		if err := r.validate(); err != nil {
			return fmt.Errorf("registers[%d]: %w", i, err)
		}
	}
	return nil
}

func (r *RegisterDefinition) validate() error {
	switch r.Type {
	case RegisterTypeCoil, RegisterTypeDiscreteInput,
		RegisterTypeHoldingRegister, RegisterTypeInputRegister:
	case "":
		return errors.New("type must be set")
	default:
		return fmt.Errorf("unknown register type %q", r.Type)
	}

	if r.DataType == "" {
		if r.Type == RegisterTypeCoil || r.Type == RegisterTypeDiscreteInput {
			r.DataType = DataTypeBool
		} else {
			r.DataType = DataTypeUint16
		}
	}
	if r.ByteOrder == "" {
		r.ByteOrder = ByteOrderABCD
	}

	switch r.DataType {
	case DataTypeBool, DataTypeInt16, DataTypeInt32, DataTypeInt64,
		DataTypeUint16, DataTypeUint32, DataTypeUint64,
		DataTypeFloat32, DataTypeFloat64:
	default:
		return fmt.Errorf("unknown data_type %q", r.DataType)
	}

	isBitBank := r.Type == RegisterTypeCoil || r.Type == RegisterTypeDiscreteInput
	if isBitBank && r.DataType != DataTypeBool {
		return fmt.Errorf("data_type %q is not valid for register type %q: only bool is supported", r.DataType, r.Type)
	}
	isRegBank := r.Type == RegisterTypeHoldingRegister || r.Type == RegisterTypeInputRegister
	if isRegBank && r.DataType == DataTypeBool {
		return fmt.Errorf("data_type bool is not valid for register type %q: use coil or discrete_input instead", r.Type)
	}

	switch r.ByteOrder {
	case ByteOrderABCD, ByteOrderDCBA, ByteOrderBADC, ByteOrderCDAB:
	default:
		return fmt.Errorf("unknown byte_order %q: must be one of ABCD, DCBA, BADC, CDAB", r.ByteOrder)
	}

	return nil
}

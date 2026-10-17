package modbusreceiver

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	validBase := func() *Config {
		return &Config{
			Endpoint:        "192.168.1.10:502",
			UnitID:          1,
			PollingInterval: 10 * time.Second,
			Timeout:         5 * time.Second,
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
			name:    "zero polling interval",
			mutate:  func(c *Config) { c.PollingInterval = 0 },
			wantErr: true,
		},
		{
			name:    "zero timeout",
			mutate:  func(c *Config) { c.Timeout = 0 },
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

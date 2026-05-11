# modbusreceiver

An OpenTelemetry Collector receiver that polls a **Modbus TCP** device and converts register/coil values into OpenTelemetry metrics.

## Status

| Property                 | Value          |
|--------------------------|----------------|
| Stability                | development    |
| Supported pipeline types | metrics        |
| Module                   | `github.com/lukaszciukaj/modbusreceiver` |

## Configuration

```yaml
receivers:
  modbus:
    # Modbus TCP endpoint (host:port)
    endpoint: "192.168.1.10:502"

    # Modbus slave/unit ID (1–247)
    unit_id: 1

    # How often to poll the device
    polling_interval: 10s

    # Per-request TCP timeout
    timeout: 5s

    # List of registers/coils to read
    registers:
      - address: 0
        type: holding_register
        data_type: float32
        byte_order: ABCD
        name: "temperature"

      - address: 10
        type: holding_register
        data_type: uint16
        byte_order: ABCD
        name: "pressure_raw"

      - address: 100
        type: coil
        data_type: bool
        name: "pump_status"
```

## Register Types

| `type`              | Modbus FC | Description                    |
|---------------------|-----------|--------------------------------|
| `coil`              | FC01      | Read/write single bits         |
| `discrete_input`    | FC02      | Read-only single bits          |
| `holding_register`  | FC03      | Read/write 16-bit registers    |
| `input_register`    | FC04      | Read-only 16-bit registers     |

## Data Types

| `data_type` | Registers | Notes                              |
|-------------|-----------|------------------------------------|
| `bool`      | —         | Coil / discrete input only         |
| `int16`     | 1         |                                    |
| `uint16`    | 1         |                                    |
| `int32`     | 2         |                                    |
| `uint32`    | 2         |                                    |
| `float32`   | 2         | IEEE 754                           |
| `int64`     | 4         |                                    |
| `uint64`    | 4         |                                    |
| `float64`   | 4         | IEEE 754                           |

## Byte Orders

| `byte_order` | Description                                        | Example devices          |
|--------------|----------------------------------------------------|--------------------------|
| `ABCD`       | Big-endian (default)                               | Most PLCs                |
| `DCBA`       | Little-endian                                      | x86-style devices        |
| `BADC`       | Big-endian words, bytes swapped within word        | Some Schneider devices   |
| `CDAB`       | Little-endian words, bytes swapped within word     | Some ABB devices         |

## Metrics

### `modbus.register.value`
Decoded value from a holding or input register. Type depends on `data_type` (int gauge for signed integers, double gauge for unsigned and floats).

### `modbus.coil.value`
Binary value (0 or 1) from a coil or discrete input.

Both metrics share these attributes:

| Attribute                 | Description                        |
|---------------------------|------------------------------------|
| `modbus.register_address` | Register address                   |
| `modbus.unit_id`          | Modbus unit/slave ID               |
| `modbus.register_type`    | Register bank type                 |
| `modbus.data_type`        | Data type used for decoding        |
| `modbus.byte_order`       | Byte order used for decoding       |
| `modbus.name`             | Optional name from config          |

## Development

```bash
go mod tidy
go build ./...
go test ./...
```

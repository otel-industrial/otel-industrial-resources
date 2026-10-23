package industrialscraper

import (
	"encoding/binary"
	"math"
)

// ByteOrder is the word/byte arrangement of a multi-register value, matching the
// four orders modbusreceiver uses. See README.
type ByteOrder string

const (
	ByteOrderABCD ByteOrder = "ABCD" // big-endian, high word first
	ByteOrderDCBA ByteOrder = "DCBA" // little-endian, byte- and word-reversed
	ByteOrderBADC ByteOrder = "BADC" // byte-swapped within each word
	ByteOrderCDAB ByteOrder = "CDAB" // word-swapped, bytes kept
)

// Reorder rearranges raw register bytes into canonical big-endian (ABCD) for the
// decoders below, so decoding is host-byte-order independent. An unknown order
// is treated as ABCD. It handles any even length (2/4/8 bytes). This mirrors
// modbusreceiver's ByteOrder.reorder.
func Reorder(bo ByteOrder, raw []byte) []byte {
	out := make([]byte, len(raw))
	switch bo {
	case ByteOrderDCBA:
		for i, b := range raw {
			out[len(raw)-1-i] = b
		}
	case ByteOrderBADC:
		for i := 0; i+1 < len(raw); i += 2 {
			out[i], out[i+1] = raw[i+1], raw[i]
		}
	case ByteOrderCDAB:
		for i := 0; i+1 < len(raw); i += 2 {
			j := len(raw) - 2 - i
			out[i], out[i+1] = raw[j], raw[j+1]
		}
	default: // ABCD and unknown
		copy(out, raw)
	}
	return out
}

// The decoders read canonical big-endian (ABCD) bytes; call Reorder first for any
// other order. Each wants exactly its width (2, 4, or 8 bytes) and panics on a
// short slice.

// Uint16 decodes one word.
func Uint16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }

// Int16 decodes one word, signed.
func Int16(b []byte) int16 { return int16(binary.BigEndian.Uint16(b)) }

// Uint32 decodes two words.
func Uint32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }

// Int32 decodes two words, signed.
func Int32(b []byte) int32 { return int32(binary.BigEndian.Uint32(b)) }

// Uint64 decodes four words.
func Uint64(b []byte) uint64 { return binary.BigEndian.Uint64(b) }

// Int64 decodes four words, signed.
func Int64(b []byte) int64 { return int64(binary.BigEndian.Uint64(b)) }

// Float32 decodes an IEEE-754 single.
func Float32(b []byte) float32 { return math.Float32frombits(binary.BigEndian.Uint32(b)) }

// Float64 decodes an IEEE-754 double.
func Float64(b []byte) float64 { return math.Float64frombits(binary.BigEndian.Uint64(b)) }

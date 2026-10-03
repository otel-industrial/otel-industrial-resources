package industrialscraper

import (
	"encoding/binary"
	"math"
)

// Big-endian 16-bit word decoders for register-based field data; see README
// for word order (ABCD/CDAB) and host-independence. Each wants exactly its
// width (2 or 4 bytes) and panics on a short slice.

// Uint16 decodes one word.
func Uint16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }

// Int16 decodes one word, signed.
func Int16(b []byte) int16 { return int16(binary.BigEndian.Uint16(b)) }

// Uint32 decodes two words, high word first (ABCD).
func Uint32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }

// Uint32WordSwapped decodes two words, low word first (CDAB).
func Uint32WordSwapped(b []byte) uint32 {
	return uint32(binary.BigEndian.Uint16(b[0:2])) |
		uint32(binary.BigEndian.Uint16(b[2:4]))<<16
}

// Float32 decodes an IEEE-754 float, high word first (ABCD).
func Float32(b []byte) float32 {
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}

// Float32WordSwapped decodes an IEEE-754 float, low word first (CDAB).
func Float32WordSwapped(b []byte) float32 {
	return math.Float32frombits(Uint32WordSwapped(b))
}

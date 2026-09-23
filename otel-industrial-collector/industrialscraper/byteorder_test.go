package industrialscraper

import (
	"math"
	"testing"
)

func TestUint16(t *testing.T) {
	if got := Uint16([]byte{0x01, 0x02}); got != 0x0102 {
		t.Errorf("Uint16 = %#04x, want 0x0102", got)
	}
}

func TestUint32(t *testing.T) {
	if got := Uint32([]byte{0x01, 0x02, 0x03, 0x04}); got != 0x01020304 {
		t.Errorf("Uint32 = %#08x, want 0x01020304", got)
	}
}

func TestUint32WordSwapped(t *testing.T) {
	// Value 0x01020304 has high word 0x0102 and low word 0x0304. A
	// word-swapped device sends the low word first: [03 04][01 02].
	if got := Uint32WordSwapped([]byte{0x03, 0x04, 0x01, 0x02}); got != 0x01020304 {
		t.Errorf("Uint32WordSwapped = %#08x, want 0x01020304", got)
	}
}

func TestFloat32(t *testing.T) {
	// 1.0 == 0x3f800000, high word first.
	if got := Float32([]byte{0x3f, 0x80, 0x00, 0x00}); got != 1.0 {
		t.Errorf("Float32 = %v, want 1.0", got)
	}
}

func TestFloat32WordSwapped(t *testing.T) {
	// 1.0 (0x3f800000) with words swapped on the wire: [00 00][3f 80].
	if got := Float32WordSwapped([]byte{0x00, 0x00, 0x3f, 0x80}); got != 1.0 {
		t.Errorf("Float32WordSwapped = %v, want 1.0", got)
	}
}

func TestFloat32NaNRoundTrip(t *testing.T) {
	// Guard the bit-level decode against a non-trivial pattern.
	nan := math.Float32frombits(0x7fc00000)
	if got := Float32([]byte{0x7f, 0xc0, 0x00, 0x00}); !math.IsNaN(float64(got)) {
		t.Errorf("Float32 = %v, want NaN (%v)", got, nan)
	}
}

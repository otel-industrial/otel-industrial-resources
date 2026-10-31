package industrialscraper

import (
	"math"
	"testing"
)

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestReorder normalizes the four Modbus word/byte orders to big-endian (ABCD),
// matching modbusreceiver's ByteOrder.reorder.
func TestReorder(t *testing.T) {
	abcd := []byte{0x01, 0x02, 0x03, 0x04}
	tests := []struct {
		order ByteOrder
		raw   []byte
	}{
		{ByteOrderABCD, []byte{0x01, 0x02, 0x03, 0x04}},
		{ByteOrderDCBA, []byte{0x04, 0x03, 0x02, 0x01}},
		{ByteOrderBADC, []byte{0x02, 0x01, 0x04, 0x03}},
		{ByteOrderCDAB, []byte{0x03, 0x04, 0x01, 0x02}},
	}
	for _, tt := range tests {
		if got := Reorder(tt.order, tt.raw); !bytesEqual(got, abcd) {
			t.Errorf("Reorder(%s, % x) = % x, want % x", tt.order, tt.raw, got, abcd)
		}
	}
}

func TestReorderWidths(t *testing.T) {
	if got := Reorder(ByteOrderDCBA, []byte{0x02, 0x01}); !bytesEqual(got, []byte{0x01, 0x02}) {
		t.Errorf("2-byte DCBA = % x", got)
	}
	in := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	if got := Reorder(ByteOrderDCBA, in); !bytesEqual(got, want) {
		t.Errorf("8-byte DCBA = % x, want % x", got, want)
	}
}

func TestDecoders16(t *testing.T) {
	if got := Uint16([]byte{0x01, 0x02}); got != 0x0102 {
		t.Errorf("Uint16 = %#x", got)
	}
	if got := Int16([]byte{0xff, 0xfe}); got != -2 {
		t.Errorf("Int16 = %d, want -2", got)
	}
}

func TestDecoders32(t *testing.T) {
	if got := Uint32([]byte{0x01, 0x02, 0x03, 0x04}); got != 0x01020304 {
		t.Errorf("Uint32 = %#x", got)
	}
	if got := Int32([]byte{0xff, 0xff, 0xff, 0xfe}); got != -2 {
		t.Errorf("Int32 = %d, want -2", got)
	}
	if got := Float32([]byte{0x3f, 0x80, 0x00, 0x00}); got != 1.0 {
		t.Errorf("Float32 = %v, want 1.0", got)
	}
}

func TestDecoders64(t *testing.T) {
	if got := Uint64([]byte{0, 0, 0, 0, 0, 0, 0, 1}); got != 1 {
		t.Errorf("Uint64 = %d, want 1", got)
	}
	if got := Int64([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}); got != -2 {
		t.Errorf("Int64 = %d, want -2", got)
	}
	if got := Float64([]byte{0x3f, 0xf0, 0, 0, 0, 0, 0, 0}); got != 1.0 {
		t.Errorf("Float64 = %v, want 1.0", got)
	}
}

// TestReorderThenDecode is the adoption path: normalize a non-ABCD order, then
// decode big-endian.
func TestReorderThenDecode(t *testing.T) {
	// 1.0 float32 (0x3f800000) sent CDAB (words swapped) is [00 00][3f 80].
	raw := []byte{0x00, 0x00, 0x3f, 0x80}
	if got := Float32(Reorder(ByteOrderCDAB, raw)); got != 1.0 {
		t.Errorf("CDAB float32 = %v, want 1.0", got)
	}
}

func TestFloatNaN(t *testing.T) {
	if got := Float32([]byte{0x7f, 0xc0, 0x00, 0x00}); !math.IsNaN(float64(got)) {
		t.Errorf("Float32 = %v, want NaN", got)
	}
	if got := Float64([]byte{0x7f, 0xf8, 0, 0, 0, 0, 0, 0}); !math.IsNaN(got) {
		t.Errorf("Float64 = %v, want NaN", got)
	}
}

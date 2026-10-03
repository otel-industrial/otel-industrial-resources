package industrialscraper

import (
	"reflect"
	"testing"
)

func TestBatch(t *testing.T) {
	tags := []string{"a", "b", "c", "d", "e"}
	tests := []struct {
		name string
		in   []string
		size int
		want [][]string
	}{
		{"even split", tags[:4], 2, [][]string{{"a", "b"}, {"c", "d"}}},
		{"remainder", tags, 2, [][]string{{"a", "b"}, {"c", "d"}, {"e"}}},
		{"size exceeds len", tags[:2], 10, [][]string{{"a", "b"}}},
		{"zero size is one group", tags, 0, [][]string{tags}},
		{"empty is nil", nil, 3, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Batch(tt.in, tt.size); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Batch(%v, %d) = %v, want %v", tt.in, tt.size, got, tt.want)
			}
		})
	}
}

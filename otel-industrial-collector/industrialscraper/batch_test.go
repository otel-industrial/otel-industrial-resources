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

// TestBatchDoesNotAlias: the groups must not share the input's backing array, or
// appending to one group overwrites the next group and the caller's slice.
func TestBatchDoesNotAlias(t *testing.T) {
	t.Run("sized groups", func(t *testing.T) {
		in := []int{1, 2, 3, 4}
		groups := Batch(in, 2)
		groups[0] = append(groups[0], 99)
		if groups[1][0] != 3 {
			t.Errorf("append to groups[0] overwrote groups[1][0] = %d, want 3", groups[1][0])
		}
		if in[2] != 3 {
			t.Errorf("append to groups[0] overwrote caller slice in[2] = %d, want 3", in[2])
		}
	})

	t.Run("single group (size<=0)", func(t *testing.T) {
		in := []int{1, 2, 3}
		groups := Batch(in, 0)
		groups[0] = append(groups[0], 99)
		if len(in) != 3 || in[len(in)-1] != 3 {
			t.Errorf("append to the single group mutated the caller slice: %v", in)
		}
	})
}

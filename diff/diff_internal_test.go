package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevenshtein(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"both empty", "", "", 0},
		{"first empty", "", "abc", 3},
		{"second empty", "abc", "", 3},
		{"equal", "kitten", "kitten", 0},
		{"kitten and sitting", "kitten", "sitting", 3},
		{"one substitution", "flaw", "flap", 1},
		{"one insertion", "Milbrook", "Millbrook", 1},
		{"swapped letters", "ab", "ba", 2},
		{"a byte of 2 differs from a byte of 1", "é", "e", 2},
		{"2 bytes of the same letter", "café", "cafe", 2},
		{"2 letters with the same first byte", "é", "è", 1},
		{"a 3-byte letter and a 1-byte letter", "\u20ac", "a", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, levenshtein(tt.a, tt.b))
			assert.Equal(t, tt.want, levenshtein(tt.b, tt.a))
		})
	}
}

func TestTextScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want float64
	}{
		{"equal", "Hello.", "Hello.", 1},
		{"both empty", "", "", 1},
		{"one empty", "", "Hello.", 0},
		{"one edit in 12 bytes", "Hello there.", "Hello there!", 1 - 1.0/12},
		{"likeness of 0.7", "abcdefghij", "abcdefgXYZ", 0.7},
		{"likeness of 0.6", "abcdefghij", "abcdefXYZW", 0},
		{"the shorter text is under 70 percent", "abcdefghij", "abcdef", 0},
		{"the shorter text is 70 percent", "abcdefghij", "abcdefg", 0.7},
		{"bytes, not runes", "ééééé", "eeeee", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, textScore(tt.a, tt.b), 1e-12)
		})
	}
}

func TestNearest(t *testing.T) {
	t.Parallel()

	ascending := func(from, to int) []int {
		out := make([]int, 0, to-from)
		for n := from; n < to; n++ {
			out = append(out, n)
		}
		return out
	}
	tests := []struct {
		name   string
		sorted []int
		index  int
		first  int
		last   int
	}{
		{"fewer than the window", ascending(0, 10), 5, 0, 9},
		{"exactly the window", ascending(0, 50), 49, 0, 49},
		{"in the middle, the lower index wins the tie", ascending(0, 101), 50, 25, 74},
		{"at the start", ascending(0, 100), 0, 0, 49},
		{"at the end", ascending(0, 100), 99, 50, 99},
		{"after the last", ascending(0, 100), 500, 50, 99},
		{"before the first", ascending(10, 110), 0, 10, 59},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := nearest(tt.sorted, tt.index)
			assert.Len(t, got, min(len(tt.sorted), sameTextWindow))
			assert.ElementsMatch(t, ascending(tt.first, tt.last+1), got)
		})
	}
}

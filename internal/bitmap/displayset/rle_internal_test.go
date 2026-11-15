package displayset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeRLE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rle   []byte
		size  int
		want  []byte
		count int
	}{
		{name: "single pixels", rle: []byte{1, 2, 3}, size: 3, want: []byte{1, 2, 3}, count: 3},
		{name: "zero run of up to 63", rle: []byte{0, 3, 5}, size: 4, want: []byte{0, 0, 0, 5}, count: 4},
		{name: "zero run of 14 bits", rle: []byte{0, 0x40 | 1, 2}, size: 258, want: make([]byte, 258), count: 258},
		{name: "colour run of up to 63", rle: []byte{0, 0x80 | 3, 7}, size: 3, want: []byte{7, 7, 7}, count: 3},
		{name: "colour run of 14 bits", rle: []byte{0, 0xC0 | 1, 2, 9}, size: 258, want: fill(258, 9), count: 258},
		{name: "end of line adds nothing, rows do not align", rle: []byte{1, 0, 0, 2, 0, 0}, size: 2, want: []byte{1, 2}, count: 2},
		{name: "stops at the size and cuts the extra pixels", rle: []byte{0, 0xC0 | 0x3F, 0xFF, 4, 1}, size: 5, want: fill(5, 4), count: 5},
		{name: "a missing byte reads as 0", rle: []byte{1, 0, 0x80 | 2}, size: 3, want: []byte{1, 0, 0}, count: 3},
		{name: "a lone zero is a run of 0", rle: []byte{1, 0}, size: 2, want: []byte{1}, count: 1},
		{name: "too few pixels", rle: []byte{1, 1}, size: 8, want: []byte{1, 1}, count: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, count := decodeRLE(tt.rle, tt.size)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, count)
		})
	}
}

func fill(n int, v byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = v
	}
	return out
}

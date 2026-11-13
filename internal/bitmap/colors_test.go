package bitmap_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
)

func TestToRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		y, cr, cb byte
		bt601     [3]byte
		bt709     [3]byte
	}{
		{y: 235, cr: 128, cb: 128, bt601: [3]byte{255, 255, 255}, bt709: [3]byte{255, 255, 255}},
		{y: 16, cr: 128, cb: 128, bt601: [3]byte{0, 0, 0}, bt709: [3]byte{0, 0, 0}},
		{y: 126, cr: 128, cb: 128, bt601: [3]byte{128, 128, 128}, bt709: [3]byte{128, 128, 128}},
		{y: 219, cr: 138, cb: 16, bt601: [3]byte{252, 255, 10}, bt709: [3]byte{254, 255, 0}},
		{y: 218, cr: 142, cb: 43, bt601: [3]byte{255, 255, 64}, bt709: [3]byte{255, 246, 56}},
		{y: 81, cr: 240, cb: 90, bt601: [3]byte{254, 0, 0}, bt709: [3]byte{255, 24, 0}},
		{y: 145, cr: 34, cb: 54, bt601: [3]byte{0, 255, 1}, bt709: [3]byte{0, 216, 0}},
		{y: 170, cr: 166, cb: 16, bt601: [3]byte{240, 192, 0}, bt709: [3]byte{247, 183, 0}},
		{y: 0, cr: 0, cb: 0, bt601: [3]byte{0, 136, 0}, bt709: [3]byte{0, 77, 0}},
		{y: 255, cr: 255, cb: 255, bt601: [3]byte{255, 125, 255}, bt709: [3]byte{255, 184, 255}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.y, tt.cr, tt.cb), func(t *testing.T) {
			t.Parallel()

			r, g, b := bitmap.ToRGB(tt.y, tt.cr, tt.cb, bitmap.BT601)
			assert.Equal(t, [3]byte{r, g, b}, tt.bt601, "BT.601")
			r, g, b = bitmap.ToRGB(tt.y, tt.cr, tt.cb, bitmap.BT709)
			assert.Equal(t, [3]byte{r, g, b}, tt.bt709, "BT.709")
		})
	}
}

func TestMatrixFor(t *testing.T) {
	t.Parallel()

	assert.Equal(t, bitmap.BT601, bitmap.MatrixFor(576))
	assert.Equal(t, bitmap.BT709, bitmap.MatrixFor(577))
}

func TestToYCrCb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rgb    uint32
		matrix bitmap.Matrix
		code   [3]byte
	}{
		{name: "white 576", rgb: 0xFFFFFF, matrix: bitmap.MatrixFor(576), code: [3]byte{235, 128, 128}},
		{name: "black 1080", rgb: 0x000000, matrix: bitmap.MatrixFor(1080), code: [3]byte{16, 128, 128}},
		{name: "gray 1080", rgb: 0x808080, matrix: bitmap.MatrixFor(1080), code: [3]byte{126, 128, 128}},
		{name: "yellow 576", rgb: 0xFCFF0A, matrix: bitmap.MatrixFor(576), code: [3]byte{212, 143, 20}},
		{name: "yellow 1080", rgb: 0xFEFF00, matrix: bitmap.MatrixFor(1080), code: [3]byte{219, 138, 16}},
		{name: "step of 2 in BT.601", rgb: 0xFFFF40, matrix: bitmap.MatrixFor(576), code: [3]byte{218, 142, 43}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			y, cr, cb := bitmap.ToYCrCb(tt.rgb, tt.matrix)
			assert.Equal(t, [3]byte{y, cr, cb}, tt.code)
			r, g, b := bitmap.ToRGB(y, cr, cb, tt.matrix)
			assert.Equal(t, tt.rgb, uint32(r)<<16|uint32(g)<<8|uint32(b))
		})
	}
}

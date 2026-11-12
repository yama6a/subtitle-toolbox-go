package bitmap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	grayType    = 0
	rgbType     = 2
	paletteType = 3
	grayAlpha   = 4
	rgbaType    = 6
)

func TestDecodePNG(t *testing.T) {
	t.Parallel()

	two, three := []byte{255, 0, 0, 0, 0, 255}, []byte{255, 0, 0, 0, 0, 255, 0, 255, 0}
	tests := []struct {
		name string
		spec pngSpec
		want []uint32
	}{
		{"gray 1 bit", pngSpec{width: 2, height: 1, depth: 1, colorType: grayType, rows: [][]byte{{0x80}}}, []uint32{0xFFFFFFFF, 0x000000FF}},
		{"gray 2 bits", pngSpec{width: 4, height: 1, depth: 2, colorType: grayType, rows: [][]byte{{0x1B}}}, []uint32{0x000000FF, 0x555555FF, 0xAAAAAAFF, 0xFFFFFFFF}},
		{"gray 4 bits", pngSpec{width: 2, height: 1, depth: 4, colorType: grayType, rows: [][]byte{{0x5F}}}, []uint32{0x555555FF, 0xFFFFFFFF}},
		{"gray 8 bits", pngSpec{width: 2, height: 1, depth: 8, colorType: grayType, rows: [][]byte{{0x10, 0xF0}}}, []uint32{0x101010FF, 0xF0F0F0FF}},
		{"gray 16 bits", pngSpec{width: 2, height: 1, depth: 16, colorType: grayType, rows: [][]byte{{0x12, 0x34, 0xFE, 0xDC}}}, []uint32{0x121212FF, 0xFEFEFEFF}},
		{"gray 16 bits with a colour key", pngSpec{width: 2, height: 1, depth: 16, colorType: grayType, trns: []byte{0x12, 0x34}, rows: [][]byte{{0x12, 0x34, 0x12, 0x35}}}, []uint32{0x12121200, 0x121212FF}},
		{"RGB 8 bits", pngSpec{width: 2, height: 1, depth: 8, colorType: rgbType, rows: [][]byte{{10, 20, 30, 255, 0, 128}}}, []uint32{0x0A141EFF, 0xFF0080FF}},
		{"RGB 8 bits with a colour key", pngSpec{width: 2, height: 1, depth: 8, colorType: rgbType, trns: []byte{0, 10, 0, 20, 0, 30}, rows: [][]byte{{10, 20, 30, 10, 20, 31}}}, []uint32{0x0A141E00, 0x0A141FFF}},
		{"RGB 16 bits", pngSpec{width: 1, height: 1, depth: 16, colorType: rgbType, rows: [][]byte{{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}}}, []uint32{0x12569AFF}},
		{"RGB 16 bits with a colour key", pngSpec{width: 2, height: 1, depth: 16, colorType: rgbType, trns: []byte{1, 2, 3, 4, 5, 6}, rows: [][]byte{{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 7}}}, []uint32{0x01030500, 0x010305FF}},
		{"palette 1 bit", pngSpec{width: 2, height: 1, depth: 1, colorType: paletteType, palette: two, trns: []byte{128}, rows: [][]byte{{0x40}}}, []uint32{0xFF000080, 0x0000FFFF}},
		{"palette 2 bits", pngSpec{width: 3, height: 1, depth: 2, colorType: paletteType, palette: three, trns: []byte{0, 255, 7}, rows: [][]byte{{0x24}}}, []uint32{0xFF000000, 0x00FF0007, 0x0000FFFF}},
		{"palette 4 bits", pngSpec{width: 2, height: 1, depth: 4, colorType: paletteType, palette: three, rows: [][]byte{{0x20}}}, []uint32{0x00FF00FF, 0xFF0000FF}},
		{"palette 8 bits", pngSpec{width: 3, height: 1, depth: 8, colorType: paletteType, palette: three, trns: []byte{0, 255, 7}, rows: [][]byte{{2, 1, 0}}}, []uint32{0x00FF0007, 0x0000FFFF, 0xFF000000}},
		{"gray and alpha 8 bits", pngSpec{width: 1, height: 1, depth: 8, colorType: grayAlpha, rows: [][]byte{{0x40, 0x80}}}, []uint32{0x40404080}},
		{"gray and alpha 16 bits", pngSpec{width: 1, height: 1, depth: 16, colorType: grayAlpha, rows: [][]byte{{0x40, 0x00, 0x80, 0x01}}}, []uint32{0x40404080}},
		{"RGBA 8 bits", pngSpec{width: 1, height: 1, depth: 8, colorType: rgbaType, rows: [][]byte{{10, 20, 30, 40}}}, []uint32{0x0A141E28}},
		{"RGBA 16 bits keeps the high bytes", pngSpec{width: 1, height: 1, depth: 16, colorType: rgbaType, rows: [][]byte{{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0x80, 0xFF}}}, []uint32{0x12569A80}},
		{"3 IDAT chunks", pngSpec{width: 2, height: 2, depth: 8, colorType: rgbaType, idatChunks: 3, rows: [][]byte{{1, 2, 3, 4, 5, 6, 7, 8}, {9, 10, 11, 12, 13, 14, 15, 16}}}, []uint32{0x01020304, 0x05060708, 0x090A0B0C, 0x0D0E0F10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := bitmap.DecodePNG(tt.spec.bytes())
			require.NoError(t, err)
			assert.Equal(t, tt.spec.width, got.Width)
			assert.Equal(t, tt.spec.height, got.Height)
			assert.Equal(t, tt.want, got.RGBA)
		})
	}
}

func TestDecodePNGRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := pngSpec{width: 2, height: 1, depth: 8, colorType: rgbaType, rows: [][]byte{{1, 2, 3, 4, 5, 6, 7, 8}}}.bytes()
	huge := pngSpec{width: 65535, height: 65535, depth: 8, colorType: rgbaType}.bytes()
	zero := pngSpec{width: 0, height: 1, depth: 8, colorType: rgbaType}.bytes()
	badZlib := append(append([]byte(nil), valid[:33]...), chunk("IDAT", []byte("not zlib data"))...)
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"no signature", []byte("GIF89a, not a PNG file at all"), "Cannot decode the PNG: the data does not start with the PNG signature."},
		{"cut chunk", valid[:len(valid)-20], "Cannot decode the PNG: png: invalid format: unexpected EOF."},
		{"no IHDR", append([]byte(pngSignatureForTest), chunk("IEND", nil)...), "Cannot decode the PNG: png: invalid format: chunk out of order."},
		{"size 0", zero, "Cannot decode the PNG: the width and the height must be at least 1."},
		{"size over the limit", huge[:33], "Cannot decode the PNG: an image of 65535x65535 pixels is larger than the limit of 7680 pixels per side and 8294400 pixels in total."},
		{"bad zlib data", append(badZlib, chunk("IEND", nil)...), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := bitmap.DecodePNG(tt.input)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			if tt.want == "" {
				assert.Contains(t, invalid.Msg, "Cannot decode the PNG: ")
				return
			}
			assert.Equal(t, tt.want, invalid.Msg)
		})
	}
}

const pngSignatureForTest = "\x89PNG\r\n\x1a\n"

func TestDecodeIndexedPNG(t *testing.T) {
	t.Parallel()

	palette := pngSpec{
		width: 3, height: 2, depth: 8, colorType: paletteType, palette: []byte{255, 0, 0, 0, 255, 0}, trns: []byte{0},
		rows: [][]byte{{0, 1, 5}, {1, 1, 0}},
	}.bytes()
	got, ok, err := bitmap.DecodeIndexedPNG(palette)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []byte{0, 1, 5, 1, 1, 0}, got.Indexes)
	assert.Equal(t, []uint32{0xFF000000, 0x00FF00FF}, got.Palette[:2])

	_, ok, err = bitmap.DecodeIndexedPNG(pngSpec{width: 1, height: 1, depth: 8, colorType: rgbaType, rows: [][]byte{{1, 2, 3, 4}}}.bytes())
	require.NoError(t, err)
	assert.False(t, ok)
}

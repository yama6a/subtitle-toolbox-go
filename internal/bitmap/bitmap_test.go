package bitmap_test

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestEncodeThenDecodeGivesTheExactPixels(t *testing.T) {
	t.Parallel()

	rgba := bitmap.Pixels{Width: 3, Height: 2, RGBA: []uint32{0x11223344, 0xFFFFFFFF, 0x00000001, 0x80808080, 0x0A0B0C0D, 0x00000000}}
	data := bitmap.EncodeRGBA(rgba)
	got, err := bitmap.DecodePNG(data)
	require.NoError(t, err)
	assert.Equal(t, rgba, got)
	assert.Equal(t, data, bitmap.EncodeRGBA(rgba), "equal pixels give equal bytes")

	tests := []struct {
		name    string
		palette []uint32
		tRNS    bool
	}{
		{"opaque palette has no tRNS chunk", []uint32{0xFF0000FF, 0x00FF00FF, 0x0000FFFF}, false},
		{"palette with alpha has a tRNS chunk", []uint32{0x00000000, 0x00FF00FF, 0x0000FF80}, true},
		{"palette of 256 entries", func() []uint32 {
			p := make([]uint32, 256)
			for i := range p {
				p[i] = uint32(i)<<8 | 0xFF
			}
			return p
		}(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ix := bitmap.Indexed{Width: 3, Height: 2, Indexes: []byte{0, 1, 2, 2, 1, byte(len(tt.palette) - 1)}, Palette: tt.palette}
			data := bitmap.EncodeIndexed(ix)
			assert.Equal(t, tt.tRNS, bytes.Contains(data, []byte("tRNS")))
			got, ok, err := bitmap.DecodeIndexedPNG(data)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, ix.Indexes, got.Indexes)
			assert.Equal(t, tt.palette, got.Palette)
		})
	}
}

func TestReduce(t *testing.T) {
	t.Parallel()

	many := make([]uint32, 300)
	for i := range many {
		many[i] = uint32(i+1)<<8 | 0xFF
	}
	tests := []struct {
		name        string
		pixels      []uint32
		maxColors   int
		wantPalette []uint32 // nil checks only wantLen
		wantIndexes []byte
		wantLen     int
	}{
		{
			name: "first appearance with the transparent colour first", pixels: []uint32{0xFF0000FF, 0x00000000, 0x00FF00FF, 0xFF0000FF}, maxColors: 256,
			wantPalette: []uint32{0x00000000, 0xFF0000FF, 0x00FF00FF}, wantIndexes: []byte{1, 0, 2, 1},
		},
		{
			name: "colours of alpha 0 keep their order and stay apart", pixels: []uint32{0x0000FFFF, 0x11111100, 0x22222200}, maxColors: 4,
			wantPalette: []uint32{0x11111100, 0x22222200, 0x0000FFFF}, wantIndexes: []byte{2, 0, 1},
		},
		{name: "300 colours give 255 opaque entries and entry 0", pixels: many, maxColors: 256, wantLen: 256},
		{name: "max colours 16", pixels: many, maxColors: 16, wantLen: 16},
		{
			name: "median cut splits where half of the pixels lie", maxColors: 3,
			pixels:      []uint32{0x000000FF, 0x000000FF, 0x000000FF, 0x0A0000FF, 0x140000FF, 0xFF0000FF, 0x12345600},
			wantPalette: []uint32{0x00000000, 0x000000FF, 0x5F0000FF}, wantIndexes: []byte{1, 1, 1, 2, 2, 2, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			palette, indexes, err := bitmap.Reduce(tt.pixels, tt.maxColors)
			require.NoError(t, err)
			require.Len(t, indexes, len(tt.pixels))
			if tt.wantPalette != nil {
				assert.Equal(t, tt.wantPalette, palette)
				assert.Equal(t, tt.wantIndexes, indexes)
				return
			}
			require.Len(t, palette, tt.wantLen)
			assert.Zero(t, palette[0])
			for _, c := range palette[1:] {
				assert.EqualValues(t, 0xFF, c&0xFF)
			}
			for _, v := range indexes {
				assert.NotZero(t, v)
			}
		})
	}

	for _, n := range []int{1, 257} {
		_, _, err := bitmap.Reduce(many, n)
		var invalid *model.InvalidArgumentError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, "The color count must be from 2 to 256, got "+strconv.Itoa(n)+".", invalid.Msg)
	}
}

func TestReduceIndexedEqualsReduce(t *testing.T) {
	t.Parallel()

	var palette [256]uint32
	for i := range palette {
		palette[i] = uint32(i)<<24 | uint32(255-i)<<8 | 0xFF
	}
	palette[7] = 0x00000000
	palette[9] = 0xAABBCC00
	short := []uint32{0x11223344, 0x00000000}
	tests := []struct {
		name   string
		images []bitmap.Indexed
		ok     bool
	}{
		{"2 images", []bitmap.Indexed{
			{Width: 2, Height: 2, Indexes: []byte{3, 7, 3, 4}, Palette: palette[:]},
			{Width: 2, Height: 1, Indexes: []byte{9, 4}, Palette: palette[:]},
		}, true},
		{"a short palette, an index past it reads as transparent", []bitmap.Indexed{
			{Width: 4, Height: 1, Indexes: []byte{0, 1, 5, 0}, Palette: short},
			{Width: 1, Height: 1, Indexes: []byte{200}, Palette: palette[:]},
		}, true},
		{"256 colours", []bitmap.Indexed{{Width: 256, Height: 1, Indexes: allIndexes(), Palette: palette[:]}}, true},
		{"257 colours", []bitmap.Indexed{
			{Width: 256, Height: 1, Indexes: allIndexes(), Palette: palette[:]},
			{Width: 1, Height: 1, Indexes: []byte{0}, Palette: []uint32{0x01020304}},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPalette, gotIndexes, ok := bitmap.ReduceIndexed(tt.images)
			assert.Equal(t, tt.ok, ok)
			if !ok {
				return
			}
			var pixels []uint32
			for _, im := range tt.images {
				for _, v := range im.Indexes {
					var c uint32
					if int(v) < len(im.Palette) {
						c = im.Palette[v]
					}
					pixels = append(pixels, c)
				}
			}
			wantPalette, wantIndexes, err := bitmap.Reduce(pixels, 256)
			require.NoError(t, err)
			assert.Equal(t, wantPalette, gotPalette)
			assert.Equal(t, wantIndexes, gotIndexes)
		})
	}
}

func allIndexes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestImageOf(t *testing.T) {
	t.Parallel()

	part := func(x, y, w, h int, index byte, color uint32) bitmap.Part {
		p := bitmap.Part{X: x, Y: y, Width: w, Height: h, Indexes: bytes.Repeat([]byte{index}, w*h)}
		p.Palette[index] = color
		return p
	}
	tests := []struct {
		name  string
		parts []bitmap.Part
		want  model.Image
		pixel []uint32
	}{
		{
			"one part keeps its place and gives a palette PNG",
			[]bitmap.Part{part(10, 20, 2, 1, 5, 0xFF0000FF)},
			model.Image{X: 10, Y: 20, Width: 2, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080},
			[]uint32{0xFF0000FF, 0xFF0000FF},
		},
		{
			"2 parts side by side",
			[]bitmap.Part{part(0, 0, 1, 1, 1, 0xFF0000FF), part(2, 1, 1, 1, 2, 0x0000FFFF)},
			model.Image{X: 0, Y: 0, Width: 3, Height: 2, ScreenWidth: 1920, ScreenHeight: 1080},
			[]uint32{0xFF0000FF, 0, 0, 0, 0, 0x0000FFFF},
		},
		{
			"alpha 128 over alpha 200",
			[]bitmap.Part{part(5, 5, 1, 1, 1, 0x000000C8), part(5, 5, 1, 1, 1, 0xFF000080)},
			model.Image{X: 5, Y: 5, Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080},
			[]uint32{0x8F0000E4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := bitmap.ImageOf(tt.parts, 1920, 1080)
			require.NoError(t, err)
			pixels, err := bitmap.DecodePNG(got.PNG)
			require.NoError(t, err)
			assert.Equal(t, tt.pixel, pixels.RGBA)
			got.PNG = nil
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := bitmap.ImageOf([]bitmap.Part{part(0, 0, 4000, 1, 1, 1), part(4000, 0, 4000, 1, 1, 1)}, 1920, 1080)
	var parse *model.ParseError
	require.ErrorAs(t, err, &parse)
	assert.Equal(t, "Cannot join the parts of one display set: an image of 8000x1 pixels is larger than the limit of 7680 pixels per side and 8294400 pixels in total.", parse.Msg)
}

func TestImageOfRepeatsItsBytesWhenOnlyUnusedPaletteEntriesChange(t *testing.T) {
	t.Parallel()

	a := bitmap.Part{Width: 2, Height: 1, Indexes: []byte{3, 4}}
	a.Palette[3], a.Palette[4], a.Palette[9] = 0xFF0000FF, 0x00FF00FF, 0x11111111
	b := a
	b.Palette[9] = 0x22222222
	one, err := bitmap.ImageOf([]bitmap.Part{a}, 100, 100)
	require.NoError(t, err)
	two, err := bitmap.ImageOf([]bitmap.Part{b}, 100, 100)
	require.NoError(t, err)
	assert.Equal(t, one.PNG, two.PNG)
}

func TestCueAlignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		y, h int
		want model.Alignment
	}{
		{"centre in the top third", 50, 100, model.TopCenter},
		{"bottom of the screen", 940, 100, model.BottomCenter},
		{"centre on the edge of the top third", 280, 160, model.BottomCenter},
		{"centre just above the edge", 279, 160, model.TopCenter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			im := model.Image{PNG: []byte{1}, Y: tt.y, Height: tt.h, Width: 10, ScreenWidth: 1920, ScreenHeight: 1080}
			c := bitmap.Cue(time.Second, 2*time.Second, im, true)
			assert.Equal(t, tt.want, c.Position.Alignment)
			assert.Equal(t, im, *c.Image)
			assert.True(t, c.Forced)
			assert.Empty(t, c.Lines)
			im.PNG[0] = 9
			assert.Equal(t, byte(1), c.Image.PNG[0])
		})
	}
}

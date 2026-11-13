package tesseract

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
)

func TestPGM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pixels    bitmap.Pixels
		scale     float64
		noInvert  bool
		threshold int
		want      []byte
	}{
		{"light text becomes dark", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0xFFFFFFFF}}, 1, false, 0, []byte{0}},
		{"no invert keeps light", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0xFFFFFFFF}}, 1, true, 0, []byte{255}},
		{"transparent is drawn on black", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0xFFFFFF00}}, 1, false, 0, []byte{255}},
		{"half alpha white", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0xFFFFFF80}}, 1, false, 0, []byte{127}},
		{"luma weights", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0xFF0000FF}}, 1, true, 0, []byte{76}},
		{"threshold keeps grey 128", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0x808080FF}}, 1, true, 127, []byte{255}},
		{"threshold turns grey 128 black", bitmap.Pixels{Width: 1, Height: 1, RGBA: []uint32{0x808080FF}}, 1, true, 129, []byte{0}},
		{"bilinear scale 2", bitmap.Pixels{Width: 2, Height: 1, RGBA: []uint32{0x000000FF, 0xFFFFFFFF}}, 2, true, 0, []byte{0, 64, 191, 255, 0, 64, 191, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := pgm(tt.pixels, tt.scale, tt.noInvert, tt.threshold)
			w := tt.pixels.Width*int(tt.scale) + 2*border
			h := tt.pixels.Height*int(tt.scale) + 2*border
			header := "P5\n" + strconv.Itoa(w) + " " + strconv.Itoa(h) + "\n255\n"
			require.True(t, bytes.HasPrefix(got, []byte(header)), "header %q", got[:len(header)])
			body := got[len(header):]
			require.Len(t, body, w*h)

			var inner []byte
			for y := border; y < h-border; y++ {
				inner = append(inner, body[y*w+border:(y+1)*w-border]...)
			}
			assert.Equal(t, tt.want, inner)
			assert.Equal(t, byte(255), body[0])
			assert.Equal(t, byte(255), body[len(body)-1])
		})
	}
}

func TestParseTSV(t *testing.T) {
	t.Parallel()

	const header = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"
	tests := []struct {
		name      string
		in        string
		wantLines []string
		wantConf  float64
	}{
		{
			"words grouped by line", header + "5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t90\tHello\n5\t1\t1\t1\t1\t2\t0\t0\t1\t1\t70\tworld\n5\t1\t1\t1\t2\t1\t0\t0\t1\t1\t80\tBye\n",
			[]string{"Hello world", "Bye"},
			0.8,
		},
		{
			"lines keep the order of the first word", header + "5\t1\t2\t1\t1\t1\t0\t0\t1\t1\t100\tb\n5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t100\ta\n5\t1\t2\t1\t1\t2\t0\t0\t1\t1\t100\tc\n",
			[]string{"b c", "a"},
			1,
		},
		{
			"skips short rows, other levels and blank text", header + "4\t1\t1\t1\t1\t0\t0\t0\t1\t1\t50\tline\n5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t60\t \n5\t1\t1\t1\t1\t1\t0\t0\t1\t1\n5\t1\t1\t1\t1\t2\t0\t0\t1\t1\t60\tok\n",
			[]string{"ok"},
			0.6,
		},
		{
			"escapes markup and trims CR", header + "5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t90\ta<b>&c\r\n",
			[]string{"a&lt;b&gt;&amp;c"},
			0.9,
		},
		{
			"clamps and counts bad confidence as 0", header + "5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t150\ta\n5\t1\t1\t1\t1\t2\t0\t0\t1\t1\t-1\tb\n5\t1\t1\t1\t1\t3\t0\t0\t1\t1\tx\tc\n5\t1\t1\t1\t1\t4\t0\t0\t1\t1\tnan\td\n",
			[]string{"a b c d"},
			0.25,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lines, conf := parseTSV(tt.in)
			assert.Equal(t, tt.wantLines, lines)
			require.NotNil(t, conf)
			assert.InDelta(t, tt.wantConf, *conf, 1e-9)
		})
	}

	for _, in := range []string{"", header, header + "1\t1\t0\t0\t0\t0\t0\t0\t1\t1\t-1\t\n"} {
		lines, conf := parseTSV(in)
		assert.Nil(t, lines)
		assert.Nil(t, conf)
	}
}

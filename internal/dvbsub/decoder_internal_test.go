package dvbsub

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func segment(typ byte, page int, payload ...byte) []byte {
	return append([]byte{syncByte, typ, byte(page >> 8), byte(page), byte(len(payload) >> 8), byte(len(payload))}, payload...)
}

func join(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// pageSeg is a page composition segment. Each region is {id, x, y}.
func pageSeg(page int, timeOut, version, state byte, regions ...[3]int) []byte {
	p := make([]byte, 0, 2+6*len(regions))
	p = append(p, timeOut, version<<4|state<<2)
	for _, r := range regions {
		p = append(p, byte(r[0]), 0, byte(r[1]>>8), byte(r[1]), byte(r[2]>>8), byte(r[2]))
	}
	return segment(segPage, page, p...)
}

// regionSeg is a region composition segment with one object at 0, 0. code4 holds the 4-bit and 2-bit background codes.
func regionSeg(id, width, height, depthBits int, code8, code4 byte, object int) []byte {
	p := []byte{byte(id), 0, byte(width >> 8), byte(width), byte(height >> 8), byte(height), byte(depthBits << 2), 1, code8, code4}
	if object >= 0 {
		p = append(p, byte(object>>8), byte(object), 0, 0, 0, 0)
	}
	return segment(segRegion, 1, p...)
}

func objectSeg(page, id int, flags byte, top []byte, bottomLength int) []byte {
	p := make([]byte, 0, 7+len(top))
	p = append(p, byte(id>>8), byte(id), flags, byte(len(top)>>8), byte(len(top)), byte(bottomLength>>8), byte(bottomLength))
	return segment(segObject, page, append(p, top...)...)
}

func bits(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	for len(s)%8 != 0 {
		s += "0"
	}
	out := make([]byte, len(s)/8)
	for i := range out {
		v, _ := strconv.ParseUint(s[i*8:i*8+8], 2, 8)
		out[i] = byte(v)
	}
	return out
}

func TestDefaultCLUTEqualsThePHPTable(t *testing.T) {
	t.Parallel()

	c := defaultCLUT()
	got := make([]string, 0, len(c.bits2)+len(c.bits4)+len(c.bits8))
	for i, v := range c.bits2 {
		got = append(got, fmt.Sprintf("2 %d %08X", i, v))
	}
	for i, v := range c.bits4 {
		got = append(got, fmt.Sprintf("4 %d %08X", i, v))
	}
	for i, v := range c.bits8 {
		got = append(got, fmt.Sprintf("8 %d %08X", i, v))
	}
	assert.Equal(t, strings.Fields(strings.ReplaceAll(string(testfiles.Read(t, "dvbsub/default_clut.txt")), "\n", " ")), strings.Fields(strings.Join(got, " ")))
}

func TestPixelCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bits   int
		data   string
		pixels int
		want   []run
		next   int // 0 skips the check of the offset after the string
	}{
		{"2 bit one pixel", 2, "01", 9, []run{{1, 1}}, 0},
		{"2 bit short run", 2, "00 1 010 11", 9, []run{{3, 5}}, 0},
		{"2 bit one pixel of code 0", 2, "00 0 1", 9, []run{{0, 1}}, 0},
		{"2 bit end of string", 2, "00 0 0 00", 9, nil, 1},
		{"2 bit two pixels of code 0", 2, "00 0 0 01", 9, []run{{0, 2}}, 0},
		{"2 bit run of 12 or more", 2, "00 0 0 10 0011 10", 99, []run{{2, 15}}, 0},
		{"2 bit run of 29 or more", 2, "00 0 0 11 00000101 01", 99, []run{{1, 34}}, 0},
		{"2 bit string ends at the row end", 2, "01 01 01 01", 2, []run{{1, 1}, {1, 1}}, 0},
		{"4 bit one pixel", 4, "0101", 9, []run{{5, 1}}, 0},
		{"4 bit end of string", 4, "0000 0 000", 9, nil, 1},
		{"4 bit run of code 0", 4, "0000 0 011", 9, []run{{0, 5}}, 0},
		{"4 bit short run", 4, "0000 1 0 11 0110", 99, []run{{6, 7}}, 0},
		{"4 bit one pixel of code 0", 4, "0000 1 1 00", 9, []run{{0, 1}}, 0},
		{"4 bit two pixels of code 0", 4, "0000 1 1 01", 9, []run{{0, 2}}, 0},
		{"4 bit run of 9 or more", 4, "0000 1 1 10 0001 0111", 99, []run{{7, 10}}, 0},
		{"4 bit run of 25 or more", 4, "0000 1 1 11 00000010 1001", 99, []run{{9, 27}}, 0},
		{"8 bit one pixel", 8, "00000101", 9, []run{{5, 1}}, 0},
		{"8 bit end of string", 8, "00000000 0 0000000", 9, nil, 2},
		{"8 bit run of code 0", 8, "00000000 0 0000100", 9, []run{{0, 4}}, 0},
		{"8 bit run", 8, "00000000 1 0000011 00001111", 9, []run{{15, 3}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runs, next := pixelCodes(bits(tt.data), 0, tt.bits, tt.pixels)
			assert.Equal(t, tt.want, runs)
			if tt.next != 0 {
				assert.Equal(t, tt.next, next)
			}
		})
	}
}

func TestObjectPixels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		depthBits    int // 1 = 2 bit, 2 = 4 bit, 3 = 8 bit
		width        int
		height       int
		code4, code8 byte
		flags        byte
		top          []byte
		bottomLength int
		want         []byte
	}{
		{
			name: "4 bit codes, and a bottom field of length 0 copies the top field", depthBits: 2, width: 4, height: 2,
			top:  []byte{0x11, 0x12, 0x34, endOfLine},
			want: []byte{1, 2, 3, 4, 1, 2, 3, 4},
		},
		{
			name: "2 bit codes go through a map table into a 4 bit region, and code 1 keeps the pixels below", depthBits: 2, width: 4, height: 1,
			code4: 0x90, flags: 0x02,
			top:  []byte{map2to4, 0x05, 0x67, 0x10, 0x6E, endOfLine},
			want: []byte{9, 6, 7, 6},
		},
		{
			name: "2 bit codes in an 8 bit region use the default 2 to 8 table", depthBits: 3, width: 3, height: 1,
			top:  []byte{0x10, 0x6C, endOfLine},
			want: []byte{0x77, 0x88, 0xFF},
		},
		{
			name: "4 bit codes in an 8 bit region use a 4 to 8 table", depthBits: 3, width: 2, height: 1,
			top:  []byte{map4to8, 0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 0x11, 0x12, endOfLine},
			want: []byte{10, 20},
		},
		{
			name: "an 8 bit string that ends with 1 zero byte", depthBits: 3, width: 3, height: 1,
			top:  []byte{0x12, 5, 6, 7, 0x00, endOfLine, 0x12, 9, 9, 9},
			want: []byte{5, 6, 7},
		},
		{
			name: "an 8 bit string that ends with 2 zero bytes, and a field with a deeper string", depthBits: 3, width: 3, height: 1,
			top:  []byte{0x12, 5, 6, 0x00, 0x00, endOfLine},
			want: []byte{5, 6, 0},
		},
		{
			name: "a string deeper than the region ends the field", depthBits: 2, width: 2, height: 1,
			top:  []byte{0x12, 5, 6},
			want: []byte{0, 0},
		},
		{
			name: "pixels past the region are dropped", depthBits: 2, width: 2, height: 1,
			top:  []byte{0x11, 0x12, 0x34, endOfLine, 0x11, 0x56},
			want: []byte{1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := NewDecoder(1, 1, false, model.CommonReadOptions{})
			data := join(
				pageSeg(1, 5, 0, 2, [3]int{1, 0, 0}),
				regionSeg(1, tt.width, tt.height, tt.depthBits, tt.code8, tt.code4, 1),
				objectSeg(1, 1, tt.flags, tt.top, tt.bottomLength),
			)
			require.NoError(t, d.Add(data, 0, 0, false))
			assert.Equal(t, tt.want, d.regions[1].pixels)
		})
	}
}

// pixelsOf decodes the image of a cue.
func pixelsOf(t *testing.T, c model.Cue) bitmap.Pixels {
	t.Helper()

	p, err := bitmap.DecodePNG(c.Image.PNG)
	require.NoError(t, err)
	return p
}

func TestCues(t *testing.T) {
	t.Parallel()

	page := func(timeOut, version, state byte) []byte { return pageSeg(1, timeOut, version, state, [3]int{1, 3, 4}) }
	shape := join(regionSeg(1, 2, 1, 2, 0, 0, 1), objectSeg(1, 1, 0, []byte{0x11, 0x12, endOfLine}, 0))
	end := segment(segEnd, 1)

	t.Run("a repeated display set continues the cue until its time-out, and an empty page ends it", func(t *testing.T) {
		t.Parallel()

		d := NewDecoder(1, 1, false, model.CommonReadOptions{})
		require.NoError(t, d.Add(join(page(3, 0, 2), shape, end), time.Second, 0, false))
		require.NoError(t, d.Add(join(page(3, 0, 0), end), 2*time.Second, 0, false))
		require.NoError(t, d.Add(join(pageSeg(1, 3, 1, 0), end), 10*time.Second, 0, false))
		cues := d.Finish(time.Second)
		require.Len(t, cues, 1)
		assert.Equal(t, [2]time.Duration{time.Second, 5 * time.Second}, [2]time.Duration{cues[0].Start, cues[0].End})
		assert.Equal(t, [4]int{3, 4, 2, 1}, [4]int{cues[0].Image.X, cues[0].Image.Y, cues[0].Image.Width, cues[0].Image.Height})
		assert.Equal(t, [2]int{720, 576}, [2]int{cues[0].Image.ScreenWidth, cues[0].Image.ScreenHeight})
	})

	t.Run("a display set without an end segment shows at the end of its packet, and a transparent region is skipped", func(t *testing.T) {
		t.Parallel()

		d := NewDecoder(1, 1, false, model.CommonReadOptions{})
		empty := regionSeg(2, 8, 8, 2, 0, 0, -1)
		data := join(pageSeg(1, 0, 0, 2, [3]int{1, 3, 4}, [3]int{2, 50, 50}), shape, empty)
		require.NoError(t, d.Add(data, 2500*time.Microsecond, 0, false))
		cues := d.Finish(1500 * time.Millisecond)
		require.Len(t, cues, 1)
		assert.Equal(t, [2]time.Duration{3 * time.Millisecond, 1503 * time.Millisecond}, [2]time.Duration{cues[0].Start, cues[0].End})
		assert.Equal(t, 2, cues[0].Image.Width)
	})

	t.Run("a block end limits the expiry", func(t *testing.T) {
		t.Parallel()

		d := NewDecoder(1, 1, false, model.CommonReadOptions{})
		require.NoError(t, d.Add(join(page(30, 0, 2), shape, end), time.Second, 2*time.Second, true))
		cues := d.Finish(time.Hour)
		require.Len(t, cues, 1)
		assert.Equal(t, 2*time.Second, cues[0].End)
	})

	t.Run("a CLUT, a display window and the pages of the service", func(t *testing.T) {
		t.Parallel()

		display := segment(segDisplay, 1, 0x08, 0, 99, 0, 49, 0, 10, 0, 90, 0, 20, 0, 40)
		white := segment(segCLUT, 1, 1, 0, 1, 0x40|0x01, 235, 128, 128, 0)
		other := segment(segCLUT, 9, 1, 0, 1, 0x40|0x01, 16, 128, 128, 0)
		d := NewDecoder(1, 2, false, model.CommonReadOptions{})
		data := join(display, other, pageSeg(9, 1, 0, 2), page(5, 0, 2), white, shape, end)
		require.NoError(t, d.Add(data, 0, 0, false))
		cues := d.Finish(time.Second)
		require.Len(t, cues, 1)
		im := cues[0].Image
		assert.Equal(t, [6]int{13, 24, 2, 1, 100, 50}, [6]int{im.X, im.Y, im.Width, im.Height, im.ScreenWidth, im.ScreenHeight})
		assert.Equal(t, []uint32{0xFFFFFFFF, 0x00FF00FF}, pixelsOf(t, cues[0]).RGBA)
	})

	t.Run("a region over the size limit fails", func(t *testing.T) {
		t.Parallel()

		err := NewDecoder(1, 1, false, model.CommonReadOptions{}).Add(regionSeg(1, 8000, 10, 2, 0, 0, -1), 0, 0, false)
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, "Cannot read region 1: an image of 8000x10 pixels is larger than the limit of 7680 pixels per side and 8294400 pixels in total.", perr.Msg)
	})
}

func TestPixelLimits(t *testing.T) {
	t.Parallel()

	const totalLimit = "the regions hold more than 8294400 pixels in total."

	t.Run("regions that hold more than the pixel limit in total fail in strict mode", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 0, 256*20)
		for id := range 256 {
			data = append(data, regionSeg(id, 3840, 2160, 2, 0, 0x10, -1)...)
		}
		err := NewDecoder(1, 1, false, model.CommonReadOptions{}).Add(data, 0, 0, false)
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, "Cannot read region 1: "+totalLimit, perr.Msg)
	})

	t.Run("lenient mode skips the region and warns", func(t *testing.T) {
		t.Parallel()

		data := join(regionSeg(0, 3840, 2160, 2, 0, 0x10, -1), regionSeg(1, 10, 10, 2, 0, 0x10, -1))
		d := NewDecoder(1, 1, false, model.CommonReadOptions{Lenient: true})
		require.NoError(t, d.Add(data, 0, 0, false))
		assert.Len(t, d.regions, 1)
		require.Len(t, d.Warnings(), 1)
		assert.Equal(t, "Cannot read region 1: "+totalLimit, d.Warnings()[0].Message)
		assert.Equal(t, model.Skipped, d.Warnings()[0].Action)
	})

	t.Run("a region that a page lists 32 times renders once, and a repeated page does not render again", func(t *testing.T) {
		t.Parallel()

		entries := make([][3]int, 32)
		data := join(pageSeg(1, 5, 0, 2, entries...), regionSeg(0, 3840, 2160, 2, 0, 0x10, -1), segment(segEnd, 1))
		for range 200 {
			data = append(data, join(pageSeg(1, 5, 0, 0), segment(segEnd, 1))...)
		}
		d := NewDecoder(1, 1, false, model.CommonReadOptions{})
		require.NoError(t, d.Add(data, 0, 0, false))
		cues := d.Finish(time.Second)
		require.Len(t, cues, 1)
		assert.Equal(t, [2]int{3840, 2160}, [2]int{cues[0].Image.Width, cues[0].Image.Height})
	})
}

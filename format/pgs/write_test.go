package pgs_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/imaging"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// solid returns an image cue of one colour that is w x h pixels at x, y of a 1920x1080 screen.
func solid(t *testing.T, start, end float64, x, y, w int, c color.NRGBA) model.Cue {
	t.Helper()

	const h = 2
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	im, err := imaging.NewImage(img, x, y, 1920, 1080)
	require.NoError(t, err)
	return model.Cue{Start: seconds(start), End: seconds(end), Image: &im}
}

func writeCues(t *testing.T, cues ...model.Cue) []byte {
	t.Helper()

	s := &model.Subtitle{}
	s.AddCues(cues...)
	var b bytes.Buffer
	_, err := pgs.Write(&b, s, model.CommonWriteOptions{}, pgs.WriteOptions{})
	require.NoError(t, err)
	return b.Bytes()
}

// segment is one segment of a written stream.
type segment struct {
	pts  uint32
	typ  byte
	data []byte
}

func segments(t *testing.T, content []byte) []segment {
	t.Helper()

	var out []segment
	for len(content) > 0 {
		require.GreaterOrEqual(t, len(content), 13)
		size := int(binary.BigEndian.Uint16(content[11:]))
		out = append(out, segment{binary.BigEndian.Uint32(content[2:]), content[10], content[13 : 13+size]})
		content = content[13+size:]
	}
	return out
}

// displaySets lists "pts:state:objects" for each PCS of content.
func displaySets(t *testing.T, content []byte) []string {
	t.Helper()

	var out []string
	for _, s := range segments(t, content) {
		if s.typ == 0x16 {
			out = append(out, string(rune('0'+s.pts/90000))+":"+map[byte]string{0x80: "show", 0: "clear"}[s.data[7]]+string(rune('0'+s.data[10])))
		}
	}
	return out
}

func TestWritePlansDisplaySets(t *testing.T) {
	t.Parallel()

	red, blue, green := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}, color.NRGBA{G: 255, A: 255}
	tests := []struct {
		name string
		cues []model.Cue
		want []string
	}{
		{"show and clear", []model.Cue{solid(t, 1, 2, 0, 0, 4, red)}, []string{"1:show1", "2:clear0"}},
		{"a cue that starts at the end of the previous one replaces it", []model.Cue{solid(t, 1, 3, 0, 0, 4, red), solid(t, 3, 4, 0, 0, 4, blue)}, []string{"1:show1", "3:show1", "4:clear0"}},
		{"overlapping cues share display sets", []model.Cue{solid(t, 1, 4, 0, 0, 4, red), solid(t, 2, 6, 10, 0, 4, blue)}, []string{"1:show1", "2:show2", "4:show1", "6:clear0"}},
		{"a third cue drops the first", []model.Cue{solid(t, 1, 9, 0, 0, 4, red), solid(t, 2, 9, 10, 0, 4, blue), solid(t, 3, 9, 20, 0, 4, green)}, []string{"1:show1", "2:show2", "3:show2", "9:clear0"}},
		{"cues with the same start share one display set", []model.Cue{solid(t, 1, 2, 0, 0, 4, red), solid(t, 1, 3, 10, 0, 4, blue)}, []string{"1:show2", "2:show1", "3:clear0"}},
		{"a cue that ends at its start shows at once", []model.Cue{solid(t, 1, 1, 0, 0, 4, red), solid(t, 1, 3, 10, 0, 4, blue)}, []string{"1:show1", "1:show1", "3:clear0"}},
		{"a cue that ends before it starts ends at its start", []model.Cue{solid(t, 3, 1, 0, 0, 4, red)}, []string{"3:show1", "3:clear0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, displaySets(t, writeCues(t, tt.cues...)))
		})
	}
}

func TestWriteSegments(t *testing.T) {
	t.Parallel()

	red, blue := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	overlapping := segments(t, writeCues(t, solid(t, 1, 4, 0, 0, 4, red), solid(t, 2, 6, 2, 1, 4, blue)))
	// The second display set shows 2 overlapping images in one window from 0,0 to 6,3.
	var windows, palettes [][]byte
	for _, s := range overlapping {
		switch s.typ {
		case 0x17:
			windows = append(windows, s.data)
		case 0x14:
			palettes = append(palettes, s.data)
		}
	}
	assert.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 4, 0, 2}, windows[0])
	assert.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 6, 0, 3}, windows[1])
	assert.Len(t, palettes[1], 2+2*5)

	forced := solid(t, 1, 2, 0, 0, 4, red)
	forced.Forced = true
	pcs := segments(t, writeCues(t, forced))[0].data
	assert.Equal(t, byte(0x40), pcs[14])

	// A 400x400 image with 251 colours needs more than one fragment.
	noise := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	for i := 0; i < len(noise.Pix); i += 4 {
		v := byte(i / 4 * 7 % 251)
		noise.Pix[i], noise.Pix[i+1], noise.Pix[i+2], noise.Pix[i+3] = v, v, 0, 255
	}
	im, err := imaging.NewImage(noise, 0, 0, 1920, 1080)
	require.NoError(t, err)
	var objects []segment
	for _, s := range segments(t, writeCues(t, model.Cue{Start: time.Second, End: 2 * time.Second, Image: &im})) {
		if s.typ == 0x15 {
			objects = append(objects, s)
		}
	}
	require.Greater(t, len(objects), 1)
	assert.Len(t, objects[0].data, 65535)
	assert.Equal(t, byte(0x80), objects[0].data[3])
	assert.Equal(t, byte(0x40), objects[len(objects)-1].data[3])
	total := 0
	for i, o := range objects {
		total += len(o.data) - 4
		if i == 0 {
			total -= 3
		}
	}
	assert.Equal(t, int(objects[0].data[4])<<16|int(binary.BigEndian.Uint16(objects[0].data[5:])), total)
}

func TestWriteIndexedMatchesRGBA(t *testing.T) {
	t.Parallel()

	// colors returns 12 pixels for each of n opaque colours. Two families share no colour.
	colors := func(family uint32, n int) []uint32 {
		pixels := make([]uint32, 12*n)
		for i := range pixels {
			pixels[i] = (family<<16|uint32(i/12)<<8)<<8 | 0xFF
		}
		return pixels
	}
	tests := []struct {
		name   string
		images [][]uint32
	}{
		{"1 colour", [][]uint32{colors(0, 1)}},
		{"2 overlapping images of 200 colours each", [][]uint32{colors(1, 200), colors(2, 200)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var rgbaCues, indexedCues []model.Cue
			for i, pixels := range tt.images {
				width, height := 12, len(pixels)/12
				place := func(png []byte) model.Cue {
					return model.Cue{
						Start: time.Duration(i+1) * time.Second, End: 6 * time.Second,
						Image: &model.Image{PNG: png, X: 2 * i, Width: width, Height: height, ScreenWidth: 720, ScreenHeight: 576},
					}
				}
				palette, indexes, err := bitmap.Reduce(pixels, 256)
				require.NoError(t, err)
				rgbaCues = append(rgbaCues, place(bitmap.EncodeRGBA(bitmap.Pixels{Width: width, Height: height, RGBA: pixels})))
				indexedCues = append(indexedCues, place(bitmap.EncodeIndexed(bitmap.Indexed{Width: width, Height: height, Indexes: indexes, Palette: palette})))
			}
			assert.Equal(t, writeCues(t, rgbaCues...), writeCues(t, indexedCues...))
		})
	}
}

func TestWriteUnwritable(t *testing.T) {
	t.Parallel()

	good := solid(t, 1.5, 3, 0, 0, 2, color.NRGBA{R: 255, A: 255})
	with := func(edit func(c *model.Cue)) model.Cue {
		c := good.Clone()
		edit(&c)
		return c
	}
	const head = "Cannot write cue "
	tests := []struct {
		name   string
		cue    model.Cue
		reason string
	}{
		{
			"text cue",
			model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"a"}},
			head + "1 to 2 as PGS: the cue holds no image, and PGS output cannot hold text.",
		},
		{
			"size mismatch", with(func(c *model.Cue) { c.Image.Width = 3 }),
			head + "1.5 to 3 as PGS: the PNG has 2x2 pixels, but the image data says 3x2.",
		},
		{
			"negative x", with(func(c *model.Cue) { c.Image.X = -1 }),
			head + "1.5 to 3 as PGS: the image position and size must be from 0 to 65535.",
		},
		{
			"negative start", with(func(c *model.Cue) { c.Start = -500 * time.Millisecond }),
			head + "-0.5 to 3 as PGS: a time stamp must be from 0 to 4294967295 ticks of 90 kHz.",
		},
		{
			"not a PNG", with(func(c *model.Cue) { c.Image.PNG = []byte("text") }),
			head + "1.5 to 3 as PGS: the PNG does not decode.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCues(good, tt.cue)
			var b bytes.Buffer
			_, err := pgs.Write(&b, s, model.CommonWriteOptions{}, pgs.WriteOptions{})
			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, model.PGS, uerr.Format)
			assert.Equal(t, tt.reason, uerr.Reason)
			assert.Empty(t, b.Bytes())
		})
	}
}

// cueKey is what a PGS round trip keeps of a cue.
func cueKey(t *testing.T, c model.Cue) []any {
	t.Helper()

	img, err := imaging.Decode(*c.Image)
	require.NoError(t, err)
	return []any{
		c.Start, c.End, c.Forced, c.Position.Alignment, c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height,
		c.Image.ScreenWidth, c.Image.ScreenHeight, img.Pix,
	}
}

func TestWriteRoundTripsFixtures(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "pgs/*.sup"))
	require.NoError(t, err)
	fixing, err := filepath.Glob(testfiles.Path(t, "fixing/ocr-*.sup"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	require.Len(t, fixing, 4)
	byteForByte := map[string]bool{"text_1080p.sup": true, "text_cyrillic_1080p.sup": true}
	for _, name := range fixing {
		byteForByte[filepath.Base(name)] = true
	}
	for _, name := range append(files, fixing...) {
		base := filepath.Base(name)
		dir := filepath.Base(filepath.Dir(name))
		t.Run(dir+"/"+base, func(t *testing.T) {
			t.Parallel()

			input := testfiles.Read(t, dir+"/"+base)
			first, _ := read(t, input, model.CommonReadOptions{})
			var out bytes.Buffer
			_, err := pgs.Write(&out, first, model.CommonWriteOptions{}, pgs.WriteOptions{})
			require.NoError(t, err)
			if byteForByte[base] {
				assert.True(t, bytes.Equal(input, out.Bytes()), "the written file differs from the input")
			}
			second, _ := read(t, out.Bytes(), model.CommonReadOptions{})
			require.Equal(t, first.Len(), second.Len())
			for i, c := range first.Cues() {
				d, _ := second.Cue(i)
				assert.Equal(t, cueKey(t, c), cueKey(t, d), "cue %d", i)
			}
		})
	}
}

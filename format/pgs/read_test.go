package pgs_test

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// wantCue is the expected part of an image cue. The pixel goldens of the fixtures cover the rest.
type wantCue struct {
	start, end time.Duration
	rect       [4]int // x, y, width, height
	forced     bool
	top        bool // the alignment is TopCenter
	indexed    bool // the PNG is a palette PNG, else RGBA
	pixels     map[[2]int]uint32
}

func TestReadDisplaySets(t *testing.T) {
	t.Parallel()

	const w, h = 1920, 1080
	line := func(id uint16, x, y int, pixels string) obj {
		return obj{id: id, x: x, y: y, w: len(pixels), h: 1, pixels: pixels}
	}
	// update is a display set of the normal state that changes the palette of the epoch.
	update := func(b *builder, pts uint32, entries map[byte]yccEntry, o obj) *builder {
		return b.presentation(pts, w, h, stateNormal, 0x80, o).palette(pts, 1, entries).end(pts)
	}
	cropped := obj{id: 0, x: 100, y: 500, w: 4, h: 2, pixels: "\x01\x02\x03\x04\x01\x01\x01\x01", crop: &[4]int{1, 0, 2, 1}}
	windowed := obj{id: 0, window: 3, x: 100, y: 500, w: 4, h: 2, pixels: "\x01\x02\x03\x04\x01\x01\x01\x01"}

	tests := []struct {
		name  string
		build func() *builder
		want  []wantCue
	}{
		{
			name: "one object gives an indexed PNG of BT.709 colours",
			build: func() *builder {
				return new(builder).shows(ticks(1), w, h, obj{id: 0, x: 10, y: 900, w: 4, h: 2, pixels: "\x01\x01\x02\x02\x03\x03\x00\x00"}).
					clears(ticks(2), w, h)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{10, 900, 4, 2}, indexed: true,
				pixels: map[[2]int]uint32{{0, 0}: rgbaWhite, {2, 0}: rgbaBlack, {0, 1}: rgbaYellow709, {3, 1}: 0},
			}},
		},
		{
			name: "a screen of 576 lines uses BT.601",
			build: func() *builder {
				return new(builder).shows(ticks(1), 720, 576, line(0, 0, 500, "\x03\x01")).clears(ticks(2), 720, 576)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{0, 500, 2, 1}, indexed: true,
				pixels: map[[2]int]uint32{{0, 0}: rgbaYellow601, {1, 0}: rgbaWhite},
			}},
		},
		{
			name: "times round half up to milliseconds",
			build: func() *builder {
				return new(builder).shows(45, w, h, line(0, 0, 500, "\x01")).clears(90044, w, h)
			},
			want: []wantCue{{start: time.Millisecond, end: time.Second, rect: [4]int{0, 500, 1, 1}, indexed: true}},
		},
		{
			name: "2 objects share one transparent RGBA image",
			build: func() *builder {
				return new(builder).shows(ticks(1), w, h, line(0, 10, 900, "\x01\x02"), line(1, 14, 904, "\x03\x03")).
					clears(ticks(2), w, h)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{10, 900, 6, 5},
				pixels: map[[2]int]uint32{{0, 0}: rgbaWhite, {1, 0}: rgbaBlack, {2, 0}: 0, {4, 4}: rgbaYellow709, {5, 4}: rgbaYellow709},
			}},
		},
		{
			name: "a later object draws over an object it overlaps",
			build: func() *builder {
				return new(builder).shows(ticks(1), w, h, line(0, 10, 900, "\x02\x02\x02"), line(1, 11, 900, "\x00\x05\x01")).
					clears(ticks(2), w, h)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{10, 900, 4, 1},
				pixels: map[[2]int]uint32{{0, 0}: rgbaBlack, {1, 0}: rgbaBlack, {2, 0}: 0x808080FF, {3, 0}: rgbaWhite},
			}},
		},
		{
			name: "a palette update changes the colours of the epoch",
			build: func() *builder {
				b := new(builder).shows(ticks(1), w, h, line(0, 0, 900, "\x01\x03"))
				return update(b, ticks(2), map[byte]yccEntry{white: testPalette[gray]}, line(0, 0, 900, "")).clears(ticks(3), w, h)
			},
			want: []wantCue{
				{start: seconds(1), end: seconds(2), rect: [4]int{0, 900, 2, 1}, indexed: true, pixels: map[[2]int]uint32{{0, 0}: rgbaWhite, {1, 0}: rgbaYellow709}},
				{start: seconds(2), end: seconds(3), rect: [4]int{0, 900, 2, 1}, indexed: true, pixels: map[[2]int]uint32{{0, 0}: rgbaGray, {1, 0}: rgbaYellow709}},
			},
		},
		{
			name: "a palette update of an entry that no pixel uses keeps the cue",
			build: func() *builder {
				b := new(builder).shows(ticks(1), w, h, line(0, 0, 900, "\x01\x01\x03\x03"))
				return update(b, ticks(2), map[byte]yccEntry{black: {126, 128, 128, 255}}, line(0, 0, 900, "")).clears(ticks(3), w, h)
			},
			want: []wantCue{{start: seconds(1), end: seconds(3), rect: [4]int{0, 900, 4, 1}, indexed: true}},
		},
		{
			name: "an acquisition point without objects shows the object of the epoch again",
			build: func() *builder {
				b := new(builder).shows(ticks(1), w, h, line(0, 0, 900, "\x01\x03"))
				return b.presentation(ticks(2), w, h, 0x40, 0, line(0, 0, 900, "")).end(ticks(2)).clears(ticks(3), w, h)
			},
			want: []wantCue{{start: seconds(1), end: seconds(3), rect: [4]int{0, 900, 2, 1}, indexed: true}},
		},
		{
			name: "a cropped object keeps the position of the composition object",
			build: func() *builder {
				return new(builder).shows(ticks(1), 720, 576, cropped).clears(ticks(2), 720, 576)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{100, 500, 2, 1}, indexed: true,
				pixels: map[[2]int]uint32{{0, 0}: rgbaBlack, {1, 0}: rgbaYellow601},
			}},
		},
		{
			name: "an object is clipped to its window",
			build: func() *builder {
				b := new(builder).presentation(ticks(1), 720, 576, stateEpochStart, 0, windowed).windows(ticks(1), map[byte][4]int{3: {101, 500, 2, 1}})
				return b.palette(ticks(1), 0, testPalette).object(ticks(1), windowed, 65000).end(ticks(1)).clears(ticks(2), 720, 576)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{101, 500, 2, 1}, indexed: true,
				pixels: map[[2]int]uint32{{0, 0}: rgbaBlack, {1, 0}: rgbaYellow601},
			}},
		},
		{
			name: "a forced object at the top of the screen",
			build: func() *builder {
				o := line(0, 0, 0, "\x01")
				o.forced = true
				return new(builder).shows(ticks(1), w, h, o).clears(ticks(2), w, h)
			},
			want: []wantCue{{start: seconds(1), end: seconds(2), rect: [4]int{0, 0, 1, 1}, forced: true, top: true, indexed: true}},
		},
		{
			name: "a new epoch drops the objects of the old epoch",
			build: func() *builder {
				ref := line(0, 0, 0, "")
				return new(builder).shows(ticks(1), w, h, line(0, 0, 0, "\x01\x03")).
					shows(ticks(2), w, h, ref)
			},
			want: []wantCue{{start: seconds(1), end: seconds(2), rect: [4]int{0, 0, 2, 1}, top: true, indexed: true}},
		},
		{
			name: "a reference to an unknown object is skipped",
			build: func() *builder {
				return new(builder).presentation(ticks(1), w, h, stateEpochStart, 0, line(0, 0, 900, ""), line(9, 0, 0, "")).
					palette(ticks(1), 0, testPalette).object(ticks(1), line(0, 0, 900, "\x01\x03"), 65000).end(ticks(1)).clears(ticks(2), w, h)
			},
			want: []wantCue{{start: seconds(1), end: seconds(2), rect: [4]int{0, 900, 2, 1}, indexed: true}},
		},
		{
			name: "references to unknown objects show nothing",
			build: func() *builder {
				return new(builder).presentation(ticks(1), w, h, stateEpochStart, 0, line(9, 0, 0, "")).palette(ticks(1), 0, testPalette).end(ticks(1))
			},
		},
		{
			name: "an object in 2-byte fragments reads as one object",
			build: func() *builder {
				o := line(0, 0, 900, "\x01\x03\x01\x03\x01\x03")
				return new(builder).presentation(ticks(1), w, h, stateEpochStart, 0, o).palette(ticks(1), 0, testPalette).
					object(ticks(1), o, 2).segment(ticks(1), 0x15, append(u16(7), 0, 0x40, 1)).end(ticks(1)).clears(ticks(2), w, h)
			},
			want: []wantCue{{
				start: seconds(1), end: seconds(2), rect: [4]int{0, 900, 6, 1}, indexed: true,
				pixels: map[[2]int]uint32{{0, 0}: rgbaWhite, {5, 0}: rgbaYellow709},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := read(t, tt.build().Bytes(), model.CommonReadOptions{})
			assert.Empty(t, warnings)
			require.Equal(t, len(tt.want), s.Len())
			for i, want := range tt.want {
				c, _ := s.Cue(i)
				require.NotNil(t, c.Image)
				assert.Equal(t, want.start, c.Start, "start of cue %d", i)
				assert.Equal(t, want.end, c.End, "end of cue %d", i)
				assert.Equal(t, want.rect, [4]int{c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height}, "rectangle of cue %d", i)
				assert.Equal(t, want.forced, c.Forced, "forced flag of cue %d", i)
				assert.Equal(t, want.top, c.Position.Alignment == model.TopCenter, "alignment of cue %d", i)
				assert.Empty(t, c.Lines)
				cfg, err := png.DecodeConfig(bytes.NewReader(c.Image.PNG))
				require.NoError(t, err)
				_, isPalette := cfg.ColorModel.(color.Palette)
				assert.Equal(t, want.indexed, isPalette, "PNG of cue %d is a palette PNG", i)
				for at, rgba := range want.pixels {
					assert.Equal(t, rgba, pixel(t, c, at[0], at[1]), "pixel %v of cue %d", at, i)
				}
			}
		})
	}
}

func TestReadFixtureLastCueDuration(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "pgs/shapes_1080p.sup")
	for _, tt := range []struct {
		name string
		last *time.Duration
		end  time.Duration
	}{
		{name: "default 5 s", end: 25 * time.Second},
		{name: "1.5 s", last: ptr(1500 * time.Millisecond), end: 21500 * time.Millisecond},
		{name: "0 s ends the cue at its start", last: ptr(time.Duration(0)), end: 20 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := read(t, content, model.CommonReadOptions{LastCueDuration: tt.last})
			last, ok := s.Cue(s.Len() - 1)
			require.True(t, ok)
			assert.Equal(t, 20*time.Second, last.Start)
			assert.Equal(t, tt.end, last.End)
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestReadDamage(t *testing.T) {
	t.Parallel()

	const w, h = 1920, 1080
	valid := new(builder).end(0).Bytes()
	good := func(b *builder, pts uint32, pixels string) *builder {
		return b.shows(pts, w, h, obj{id: 0, x: 0, y: 900, w: 4, h: 2, pixels: pixels})
	}
	// damaged builds a stream with a good display set at 1 s and at 3 s, and a clear at 4 s. The second display set at
	// 2 s starts with the segments that second writes.
	damaged := func(second func(b *builder)) []byte {
		b := good(new(builder), ticks(1), "\x01\x01\x02\x02\x03\x03\x03\x03")
		second(b)
		return good(b, ticks(3), "\x03\x03\x03\x03\x01\x01\x02\x02").clears(ticks(4), w, h).Bytes()
	}
	palette := func(b *builder) *builder { return b.palette(ticks(2), 0, testPalette) }
	ref := obj{id: 0, y: 900}
	// secondObject holds an object segment of the display set at 2 s: id 0, a first fragment, a size and run-length data.
	secondObject := func(width, height int, rle ...byte) func(b *builder) {
		return func(b *builder) {
			b.presentation(ticks(2), w, h, stateEpochStart, 0, ref)
			palette(b).segment(ticks(2), 0x15, append(append(u16(0), 0, 0xC0, 0, 0, byte(4+len(rle))), append(u16(width, height), rle...)...)).end(ticks(2))
		}
	}
	// compositionData is the data of the presentation segment at 2 s.
	compositionData := func(data ...byte) func(b *builder) {
		return func(b *builder) { b.segment(ticks(2), 0x16, data); palette(b).end(ticks(2)) }
	}
	header := append(u16(w, h), 0x10, 0, 1, stateEpochStart, 0, 0, 1)
	twoTop := func(b *builder) {
		big := func(id uint16, x int) obj {
			return obj{id: id, x: x, y: 0, w: 3000, h: 1500, pixels: string(bytes.Repeat([]byte{1}, 3000*1500))}
		}
		b.presentation(ticks(2), w, h, stateEpochStart, 0, big(0, 0), big(1, 3000)).palette(ticks(2), 0, testPalette)
		b.object(ticks(2), big(0, 0), 65000).object(ticks(2), big(1, 3000), 65000).end(ticks(2))
	}
	skipAt2 := []wantCue{{start: seconds(1), end: seconds(2)}, {start: seconds(3), end: seconds(4)}}
	both := good(good(new(builder), ticks(1), "\x01\x01\x02\x02\x03\x03\x03\x03"), ticks(2), "\x03\x03\x03\x03\x01\x01\x02\x02").Bytes()
	cutSet := both[:len(both)-20]
	limitText := " pixels is larger than the limit of 7680 pixels per side and 8294400 pixels in total."

	tests := []struct {
		name    string
		content []byte
		strict  string    // the strict error, "" for none
		cues    []wantCue // the cues of a lenient read
		warns   []model.ParseWarning
	}{
		{name: "empty input"},
		{
			name: "no magic bytes", content: append([]byte("XG"), valid[2:]...),
			strict: "The segment at byte 0 does not start with the PG magic bytes.",
			warns:  []model.ParseWarning{{Message: "The segment at byte 0 does not start with the PG magic bytes.", Action: model.Skipped}},
		},
		{
			name: "one byte of the magic", content: []byte("P"),
			strict: "The segment at byte 0 does not start with the PG magic bytes.",
			warns:  []model.ParseWarning{{Message: "The segment at byte 0 does not start with the PG magic bytes.", Action: model.Skipped}},
		},
		{
			name: "garbage after a display set", content: append(bytes.Clone(valid), "junk"...),
			strict: "The segment at byte 13 does not start with the PG magic bytes.",
			warns:  []model.ParseWarning{{Message: "The segment at byte 13 does not start with the PG magic bytes.", Block: 1, Action: model.Skipped}},
		},
		{
			name: "cut header", content: []byte("PG\x00\x00"),
			strict: "The segment header at byte 0 is cut off.",
			warns:  []model.ParseWarning{{Message: "The segment header at byte 0 is cut off.", Action: model.Skipped}},
		},
		{
			name: "cut segment data", content: new(builder).palette(0, 0, testPalette).Bytes()[:44],
			strict: "The segment at byte 0 has 32 bytes of data, but the file ends before.",
			warns:  []model.ParseWarning{{Message: "The segment at byte 0 has 32 bytes of data, but the file ends before.", Action: model.Skipped}},
		},
		{
			name: "cut fixture keeps the cues before the damage", content: testfiles.Read(t, "pgs/shapes_576p.sup")[:1400],
			strict: "The segment header at byte 1398 is cut off.",
			cues:   []wantCue{{start: 500 * time.Millisecond, end: 3 * time.Second}},
			warns:  []model.ParseWarning{{Message: "The segment header at byte 1398 is cut off.", Block: 8, Action: model.Skipped}},
		},
		{
			name: "cut display set ends the shown cue and shows nothing", content: cutSet,
			strict: "The segment at byte 202 has 22 bytes of data, but the file ends before.", cues: []wantCue{{start: seconds(1), end: seconds(2)}},
			warns: []model.ParseWarning{{Message: "The segment at byte 202 has 22 bytes of data, but the file ends before.", Block: 6, Action: model.Skipped}},
		},
		{
			name: "short presentation segment", content: damaged(compositionData(header[:10]...)),
			strict: "The presentation composition segment at 2 s is cut off.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "The presentation composition segment at 2 s is cut off.", Block: 5, Action: model.Skipped}},
		},
		{
			name: "cut composition object", content: damaged(compositionData(header...)),
			strict: "The composition object 0 at 2 s is cut off.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "The composition object 0 at 2 s is cut off.", Block: 5, Action: model.Skipped}},
		},
		{
			name: "cut cropping", content: damaged(compositionData(append(bytes.Clone(header), 0, 0, 0, 0x80, 0, 0, 0, 0, 0, 1)...)),
			strict: "The cropping of composition object 0 at 2 s is cut off.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "The cropping of composition object 0 at 2 s is cut off.", Block: 5, Action: model.Skipped}},
		},
		{
			name: "cut first object fragment", content: damaged(func(b *builder) {
				b.presentation(ticks(2), w, h, stateEpochStart, 0, ref)
				palette(b).segment(ticks(2), 0x15, []byte{0, 0, 0, 0xC0, 0, 0}).end(ticks(2))
			}),
			strict: "The first definition segment of object 0 is cut off.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "The first definition segment of object 0 is cut off.", Block: 7, Action: model.Skipped}},
		},
		{
			name: "object over the size limit", content: damaged(secondObject(7681, 1)),
			strict: "Cannot read object 0: an image of 7681x1" + limitText, cues: skipAt2,
			warns: []model.ParseWarning{{Message: "Cannot read object 0: an image of 7681x1" + limitText, Block: 7, Action: model.Skipped}},
		},
		{
			name: "too few run-length pixels", content: damaged(secondObject(4, 2, 1, 1)),
			strict: "Object 0 at 2 s has 2 pixels of run-length data, but its size of 4x2 needs 8.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "Object 0 at 2 s has 2 pixels of run-length data, but its size of 4x2 needs 8.", Block: 8, Action: model.Skipped}},
		},
		{
			name: "parts over the size limit", content: damaged(twoTop),
			strict: "Cannot join the parts of one display set: an image of 6000x1500" + limitText, cues: skipAt2,
			warns: []model.ParseWarning{{Message: "Cannot join the parts of one display set: an image of 6000x1500" + limitText, Block: 9, Action: model.Skipped}},
		},
		{
			name: "screen of 0x0", content: damaged(func(b *builder) {
				o := obj{id: 0, w: 2, h: 1, pixels: "\x01\x01"}
				b.presentation(ticks(2), 0, 0, stateEpochStart, 0, o).palette(ticks(2), 0, testPalette).object(ticks(2), o, 65000).end(ticks(2))
			}),
			strict: "Cannot create a cue image of 2x1 pixels on a screen of 0x0 pixels: every size must be at least 1.", cues: skipAt2,
			warns: []model.ParseWarning{{Message: "Cannot create a cue image of 2x1 pixels on a screen of 0x0 pixels: every size must be at least 1.", Block: 8, Action: model.Skipped}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := pgs.Read(bytes.NewReader(tt.content), model.CommonReadOptions{}, pgs.ReadOptions{})
			if tt.strict == "" {
				require.NoError(t, err)
			} else {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.strict, perr.Msg)
				assert.Equal(t, model.PGS, perr.Format)
				assert.Zero(t, perr.Line)
			}

			s, warnings := read(t, tt.content, model.CommonReadOptions{Lenient: true})
			require.Equal(t, len(tt.cues), s.Len())
			for i, want := range tt.cues {
				c, _ := s.Cue(i)
				assert.Equal(t, [2]time.Duration{want.start, want.end}, [2]time.Duration{c.Start, c.End}, "times of cue %d", i)
			}
			assert.Equal(t, tt.warns, warnings)
		})
	}
}

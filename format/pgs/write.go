package pgs

import (
	"bytes"
	"encoding/binary"
	"io"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	ticksPerMillisecond = 90
	maxPTS              = 0xFFFFFFFF
	maxField            = 0xFFFF
	maxSegmentData      = 0xFFFF
	maxRun              = 0x3FFF
	maxShown            = 2
	frameRateCode       = 0x10

	segmentPalette      = 0x14
	segmentObject       = 0x15
	segmentPresentation = 0x16
	segmentWindow       = 0x17
	segmentEnd          = 0x80

	stateNormal     = 0x00
	stateEpochStart = 0x80
	flagForced      = 0x40
	sequenceFirst   = 0x80
	sequenceLast    = 0x40

	objectHeader      = 4
	firstObjectHeader = objectHeader + 3
)

// Write writes the image cues of s as a PGS (.sup) stream. Each cue becomes a display set that shows it and one that
// clears it. At most 2 cues show at once. A cue without an image, a time or size that PGS cannot hold, or a PNG that
// does not decode gives an *model.UnwritableError, and Write writes nothing.
// The lines of an image cue, LineEnding, BOM and StripTags have no effect.
func Write(w io.Writer, s *model.Subtitle, _ model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	e := encoder{ycc: map[yccKey][3]byte{}}
	if err := e.encode(s); err != nil {
		return WriteReport{}, err
	}
	return WriteReport{}, writing.WriteBytes(w, model.PGS, e.out.Bytes())
}

type yccKey struct {
	matrix bitmap.Matrix
	rgb    uint32
}

// bitmapOf holds the decoded PNG of a cue as palette pixels, or as RGBA pixels when the PNG has no palette.
type bitmapOf struct {
	indexed *bitmap.Indexed
	pixels  bitmap.Pixels
}

// shownCue is a cue on the screen. End is in ticks of 90 kHz.
type shownCue struct {
	image  model.Image
	bitmap bitmapOf
	forced bool
	end    int64
}

type window struct{ x, y, w, h int }

type encoder struct {
	out    bytes.Buffer
	number uint16
	ycc    map[yccKey][3]byte
}

func (e *encoder) encode(s *model.Subtitle) error {
	cues := s.Cues()
	var shown []shownCue
	for i, c := range cues {
		start, end, err := checkCue(i, c)
		if err != nil {
			return err
		}
		bm, err := decode(i, c)
		if err != nil {
			return err
		}
		shown = e.endShown(shown, start, false)
		shown = append(shown, shownCue{image: *c.Image, bitmap: bm, forced: c.Forced, end: end})
		if len(shown) > maxShown {
			shown = shown[1:]
		}
		// A later cue with the same start joins this display set, so the screen does not change twice at one time.
		sameStart := i+1 < len(cues) && cues[i+1].Start.Milliseconds()*ticksPerMillisecond == start
		if !sameStart || end <= start {
			e.show(shown, start)
		}
	}
	e.endShown(shown, 0, true)
	return nil
}

func unwritable(i int, c model.Cue, reason string) *model.UnwritableError {
	return &model.UnwritableError{
		Format: model.PGS, CueIndex: i,
		Reason: "Cannot write cue " + timecode.Seconds(c.Start) + " to " + timecode.Seconds(c.End) + " as PGS: " + reason,
	}
}

// checkCue returns the start and end of c in ticks of 90 kHz.
func checkCue(i int, c model.Cue) (start, end int64, err error) {
	if c.Image == nil {
		return 0, 0, unwritable(i, c, "the cue holds no image, and PGS output cannot hold text.")
	}
	start, end = c.Start.Milliseconds()*ticksPerMillisecond, c.End.Milliseconds()*ticksPerMillisecond
	if start < 0 || start > maxPTS || end < 0 || end > maxPTS {
		return 0, 0, unwritable(i, c, "a time stamp must be from 0 to "+strconv.Itoa(maxPTS)+" ticks of 90 kHz.")
	}
	im := c.Image
	for _, v := range [...]int{im.X, im.Y, im.Width, im.Height, im.ScreenWidth, im.ScreenHeight} {
		if v < 0 || v > maxField {
			return 0, 0, unwritable(i, c, "the image position and size must be from 0 to "+strconv.Itoa(maxField)+".")
		}
	}
	return start, end, nil
}

func decode(i int, c model.Cue) (bitmapOf, error) {
	var bm bitmapOf
	ix, ok, err := bitmap.DecodeIndexedPNG(c.Image.PNG)
	if err == nil && !ok {
		bm.pixels, err = bitmap.DecodePNG(c.Image.PNG)
	}
	if err != nil {
		u := unwritable(i, c, "the PNG does not decode.")
		u.Err = err
		return bitmapOf{}, u
	}
	w, h := bm.pixels.Width, bm.pixels.Height
	if ok {
		bm.indexed, w, h = &ix, ix.Width, ix.Height
	}
	if w != c.Image.Width || h != c.Image.Height {
		return bitmapOf{}, unwritable(i, c, "the PNG has "+strconv.Itoa(w)+"x"+strconv.Itoa(h)+" pixels, but the image data says "+
			strconv.Itoa(c.Image.Width)+"x"+strconv.Itoa(c.Image.Height)+".")
	}
	return bm, nil
}

// endShown writes a display set at each end of a shown cue before before, or at each end when all is true.
// It returns the cues that stay. Cues that end exactly at before leave without a display set, because the display
// set of the next cue replaces them.
func (e *encoder) endShown(shown []shownCue, before int64, all bool) []shownCue {
	for len(shown) > 0 {
		t := slices.MinFunc(shown, func(a, b shownCue) int { return int(a.end - b.end) }).end
		if !all && t >= before {
			break
		}
		previous := shown
		shown = slices.DeleteFunc(slices.Clone(shown), func(c shownCue) bool { return c.end <= t })
		if len(shown) == 0 {
			e.clear(previous, t)
		} else {
			e.show(shown, t)
		}
	}
	return slices.DeleteFunc(shown, func(c shownCue) bool { return !all && c.end <= before })
}

func (e *encoder) show(shown []shownCue, pts int64) {
	palette, indexes := reducePalette(shown)
	windows := windowsOf(shown)
	first := shown[0].image

	refs := make([]byte, 0, 8*len(shown))
	objects := make([][]byte, 0, len(shown))
	offset := 0
	for id, c := range shown {
		windowID := id
		if len(windows) == 1 {
			windowID = 0
		}
		flags := byte(0)
		if c.forced {
			flags = flagForced
		}
		refs = binary.BigEndian.AppendUint16(refs, uint16(id))
		refs = append(refs, byte(windowID), flags)
		refs = binary.BigEndian.AppendUint16(refs, uint16(c.image.X))
		refs = binary.BigEndian.AppendUint16(refs, uint16(c.image.Y))
		size := c.image.Width * c.image.Height
		objects = append(objects, objectSegments(pts, id, c.image.Width, c.image.Height, encodeRLE(indexes[offset:offset+size], c.image.Width)))
		offset += size
	}

	e.segment(pts, segmentPresentation, append(e.presentation(first, stateEpochStart, len(shown)), refs...))
	e.segment(pts, segmentWindow, windowData(windows))
	e.segment(pts, segmentPalette, e.paletteData(palette, bitmap.MatrixFor(first.ScreenHeight)))
	for _, o := range objects {
		e.out.Write(o)
	}
	e.segment(pts, segmentEnd, nil)
}

func (e *encoder) clear(shown []shownCue, pts int64) {
	e.segment(pts, segmentPresentation, e.presentation(shown[0].image, stateNormal, 0))
	e.segment(pts, segmentWindow, windowData(windowsOf(shown)))
	e.segment(pts, segmentEnd, nil)
}

func (e *encoder) presentation(im model.Image, state byte, objects int) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(im.ScreenWidth))
	b = binary.BigEndian.AppendUint16(b, uint16(im.ScreenHeight))
	b = append(b, frameRateCode)
	b = binary.BigEndian.AppendUint16(b, e.number)
	e.number++
	return append(b, state, 0, 0, byte(objects))
}

// windowsOf returns one window per image. Two images that overlap share one window around both.
func windowsOf(shown []shownCue) []window {
	ws := make([]window, len(shown))
	for i, c := range shown {
		ws[i] = window{c.image.X, c.image.Y, c.image.Width, c.image.Height}
	}
	if len(ws) < 2 {
		return ws
	}
	a, b := ws[0], ws[1]
	if a.x >= b.x+b.w || b.x >= a.x+a.w || a.y >= b.y+b.h || b.y >= a.y+a.h {
		return ws
	}
	x, y := min(a.x, b.x), min(a.y, b.y)
	return []window{{x, y, max(a.x+a.w, b.x+b.w) - x, max(a.y+a.h, b.y+b.h) - y}}
}

func windowData(ws []window) []byte {
	b := []byte{byte(len(ws))}
	for id, w := range ws {
		b = append(b, byte(id))
		for _, v := range [...]int{w.x, w.y, w.w, w.h} {
			b = binary.BigEndian.AppendUint16(b, uint16(v))
		}
	}
	return b
}

func (e *encoder) paletteData(palette []uint32, m bitmap.Matrix) []byte {
	b := make([]byte, 2, 2+5*len(palette))
	for id, c := range palette {
		key := yccKey{m, c >> 8}
		code, ok := e.ycc[key]
		if !ok {
			y, cr, cb := bitmap.ToYCrCb(key.rgb, m)
			code = [3]byte{y, cr, cb}
			e.ycc[key] = code
		}
		b = append(b, byte(id), code[0], code[1], code[2], byte(c))
	}
	return b
}

// reducePalette returns one palette of at most 256 colours for all shown images, and one entry per pixel of each image.
func reducePalette(shown []shownCue) ([]uint32, []byte) {
	indexed := make([]bitmap.Indexed, 0, len(shown))
	for _, c := range shown {
		if c.bitmap.indexed == nil {
			break
		}
		indexed = append(indexed, *c.bitmap.indexed)
	}
	if len(indexed) == len(shown) {
		if palette, indexes, ok := bitmap.ReduceIndexed(indexed); ok {
			return palette, indexes
		}
	}
	var pixels []uint32
	for _, c := range shown {
		if ix := c.bitmap.indexed; ix != nil {
			for _, v := range ix.Indexes {
				var rgba uint32
				if int(v) < len(ix.Palette) {
					rgba = ix.Palette[v]
				}
				pixels = append(pixels, rgba)
			}
			continue
		}
		pixels = append(pixels, c.bitmap.pixels.RGBA...)
	}
	palette, indexes, _ := bitmap.Reduce(pixels, 256)
	return palette, indexes
}

// objectSegments splits the object data into fragments. The first one holds the 3-byte length of all data.
func objectSegments(pts int64, id, width, height int, rle []byte) []byte {
	data := binary.BigEndian.AppendUint16(nil, uint16(width))
	data = binary.BigEndian.AppendUint16(data, uint16(height))
	data = append(data, rle...)

	var out bytes.Buffer
	for first, rest := true, data; first || len(rest) > 0; first = false {
		limit := maxSegmentData - objectHeader
		if first {
			limit = maxSegmentData - firstObjectHeader
		}
		n := min(limit, len(rest))
		seq := byte(0)
		if first {
			seq |= sequenceFirst
		}
		if n == len(rest) {
			seq |= sequenceLast
		}
		b := binary.BigEndian.AppendUint16(nil, uint16(id))
		b = append(b, 0, seq)
		if first {
			b = append(b, byte(len(data)>>16))
			b = binary.BigEndian.AppendUint16(b, uint16(len(data)&0xFFFF))
		}
		writeSegment(&out, pts, segmentObject, append(b, rest[:n]...))
		rest = rest[n:]
	}
	return out.Bytes()
}

func encodeRLE(indexes []byte, width int) []byte {
	var rle []byte
	for row := 0; row+width <= len(indexes); row += width {
		line := indexes[row : row+width]
		for pos := 0; pos < width; {
			c, run := line[pos], 1
			for pos+run < width && line[pos+run] == c && run < maxRun {
				run++
			}
			rle = appendRun(rle, c, run)
			pos += run
		}
		rle = append(rle, 0, 0)
	}
	return rle
}

func appendRun(rle []byte, c byte, run int) []byte {
	switch {
	case c != 0 && run <= 2:
		for range run {
			rle = append(rle, c)
		}
		return rle
	case c == 0 && run < 64:
		return append(rle, 0, byte(run))
	case c == 0:
		return append(rle, 0, 0x40|byte(run>>8), byte(run))
	case run < 64:
		return append(rle, 0, 0x80|byte(run), c)
	}
	return append(rle, 0, 0xC0|byte(run>>8), byte(run), c)
}

func (e *encoder) segment(pts int64, typ byte, data []byte) { writeSegment(&e.out, pts, typ, data) }

func writeSegment(b *bytes.Buffer, pts int64, typ byte, data []byte) {
	var h [13]byte
	h[0], h[1] = 'P', 'G'
	binary.BigEndian.PutUint32(h[2:], uint32(pts))
	h[10] = typ
	binary.BigEndian.PutUint16(h[11:], uint16(len(data)))
	b.Write(h[:])
	b.Write(data)
}

// Package displayset decodes PGS segments into image cues. format/pgs reads .sup files with it,
// container/mkv reads S_HDMV/PGS blocks with it.
package displayset

import (
	"encoding/binary"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// The segment types.
const (
	segPalette      = 0x14
	segObject       = 0x15
	segComposition  = 0x16
	segWindow       = 0x17
	segEnd          = 0x80
	ticksPerSecond  = 90000
	ticksPerMilli   = ticksPerSecond / 1000
	stateEpochStart = 0x80
	flagCropped     = 0x80
	flagForced      = 0x40
	firstFragment   = 0x80
)

// The sizes of the parts of a segment, in bytes.
const (
	compositionHeader = 11
	compositionObject = 8
	cropping          = 8
	windowEntry       = 9
	paletteHeader     = 2
	paletteEntry      = 5
	objectHeader      = 4
	firstObjectHeader = 11
)

// entry is one palette entry in limited range Y, Cr, Cb with alpha. defined is false for an entry the palette lacks.
type entry struct {
	y, cr, cb, alpha byte
	defined          bool
}

type object struct {
	width, height int
	rle           []byte
}

type window struct{ x, y, width, height int }

// reference is one object of a presentation composition segment.
type reference struct {
	id       uint16
	windowID byte
	forced   bool
	x, y     int
	cropped  bool
	crop     window
}

// composition is the presentation composition segment that waits for the end of its display set.
type composition struct {
	time                      time.Duration
	screenWidth, screenHeight int
	paletteID                 byte
	refs                      []reference
}

// shown is the image on the screen and the time it appeared.
type shown struct {
	start  time.Duration
	im     model.Image
	forced bool
}

func (s *shown) same(o *shown) bool {
	return s.forced == o.forced && s.im.X == o.im.X && s.im.Y == o.im.Y && s.im.Width == o.im.Width &&
		s.im.Height == o.im.Height && s.im.ScreenWidth == o.im.ScreenWidth && s.im.ScreenHeight == o.im.ScreenHeight &&
		slices.Equal(s.im.PNG, o.im.PNG)
}

// Decoder turns the segments of one PGS stream into image cues.
type Decoder struct {
	st       *parsing.State
	palettes map[byte]*[256]entry
	objects  map[uint16]*object
	windows  map[byte]window
	pending  *composition
	shown    *shown
	cues     []model.Cue
	segments int
}

// NewDecoder returns a decoder that reads Lenient and LastCueDuration of common.
func NewDecoder(common model.CommonReadOptions) *Decoder {
	return &Decoder{
		st:       parsing.NewState(model.PGS, common),
		palettes: map[byte]*[256]entry{},
		objects:  map[uint16]*object{},
		windows:  map[byte]window{},
	}
}

// Add reads one segment. pts counts 90 kHz ticks and may exceed 32 bits. data is not kept.
//
// Strict mode returns a *model.ParseError for a damaged segment. Lenient mode records a Skipped warning instead,
// ends the shown cue at the time of the display set and shows nothing for that display set. The Block of such a warning
// is the count of segments that Add has read, with the damaged one.
func (d *Decoder) Add(pts int64, segmentType byte, data []byte) error {
	d.segments++
	switch segmentType {
	case segComposition:
		return d.readComposition(pts, data)
	case segWindow:
		d.readWindows(data)
	case segPalette:
		d.readPalette(data)
	case segObject:
		return d.readObject(data)
	case segEnd:
		return d.endDisplaySet(false)
	}
	return nil
}

// Finish ends the pending display set and the shown cue. cutOff is true when damage ended the stream, so the pending
// display set shows nothing. A last cue ends after the last cue duration. Finish also returns the strict error of the
// pending display set.
func (d *Decoder) Finish(cutOff bool) ([]model.Cue, error) {
	if err := d.endDisplaySet(cutOff); err != nil {
		return nil, err
	}
	if d.shown != nil {
		d.endShown(d.st.LastCueEnd(d.shown.start))
	}
	return d.cues, nil
}

// Warnings returns the warnings of the lenient reads, in read order.
func (d *Decoder) Warnings() []model.ParseWarning {
	return d.st.Warnings()
}

// Skip records the Skipped warning of damage that the caller found before Add, such as a cut-off segment.
// Lenient mode returns nil, strict mode returns err. The Block is the count of segments that Add has read.
func (d *Decoder) Skip(err *model.ParseError) error {
	return d.st.Fail(err, model.NoLine, d.segments, nil)
}

func (d *Decoder) fail(msg string) error {
	return d.Skip(&model.ParseError{Format: model.PGS, Msg: msg})
}

// timeOf returns the whole milliseconds of pts ticks, half up, below the time limit.
func timeOf(pts int64) time.Duration {
	const limit = int64(timecode.MaxValidTime/time.Millisecond) * ticksPerMilli
	switch {
	case pts <= 0:
		return 0
	case pts >= limit:
		return timecode.MaxValidTime
	}
	return time.Duration((pts+ticksPerMilli/2)/ticksPerMilli) * time.Millisecond
}

func seconds(t time.Duration) string { return timecode.Seconds(t) }

// errorMessage returns the message of an error of package bitmap or model.
func errorMessage(err error) string {
	var perr *model.ParseError
	var aerr *model.InvalidArgumentError
	switch {
	case errors.As(err, &perr):
		return perr.Msg
	case errors.As(err, &aerr):
		return aerr.Msg
	}
	return err.Error()
}

func (d *Decoder) readComposition(pts int64, data []byte) error {
	if err := d.endDisplaySet(false); err != nil {
		return err
	}
	t := timeOf(pts)
	c, msg := d.parseComposition(t, data)
	if msg != "" {
		if err := d.fail(msg); err != nil {
			return err
		}
		d.show(t, nil)
		return nil
	}
	d.pending = c
	return nil
}

// parseComposition returns the composition, or the message of the damage. A composition with the epoch start
// state clears the palettes, objects and windows first.
func (d *Decoder) parseComposition(t time.Duration, data []byte) (*composition, string) {
	if len(data) < compositionHeader {
		return nil, "The presentation composition segment at " + seconds(t) + " s is cut off."
	}
	if data[7]&stateEpochStart != 0 {
		clear(d.palettes)
		clear(d.objects)
		clear(d.windows)
	}
	c := &composition{
		time:         t,
		screenWidth:  int(binary.BigEndian.Uint16(data)),
		screenHeight: int(binary.BigEndian.Uint16(data[2:])),
		paletteID:    data[9],
		refs:         make([]reference, 0, data[10]),
	}
	pos := compositionHeader
	for i := range int(data[10]) {
		if len(data) < pos+compositionObject {
			return nil, "The composition object " + strconv.Itoa(i) + " at " + seconds(t) + " s is cut off."
		}
		ref := reference{
			id:       binary.BigEndian.Uint16(data[pos:]),
			windowID: data[pos+2],
			forced:   data[pos+3]&flagForced != 0,
			cropped:  data[pos+3]&flagCropped != 0,
			x:        int(binary.BigEndian.Uint16(data[pos+4:])),
			y:        int(binary.BigEndian.Uint16(data[pos+6:])),
		}
		pos += compositionObject
		if ref.cropped {
			if len(data) < pos+cropping {
				return nil, "The cropping of composition object " + strconv.Itoa(i) + " at " + seconds(t) + " s is cut off."
			}
			ref.crop = window{
				int(binary.BigEndian.Uint16(data[pos:])), int(binary.BigEndian.Uint16(data[pos+2:])),
				int(binary.BigEndian.Uint16(data[pos+4:])), int(binary.BigEndian.Uint16(data[pos+6:])),
			}
			pos += cropping
		}
		c.refs = append(c.refs, ref)
	}
	return c, ""
}

func (d *Decoder) readWindows(data []byte) {
	if len(data) == 0 {
		return
	}
	for i := 0; i < int(data[0]) && len(data) >= 1+(i+1)*windowEntry; i++ {
		w := data[1+i*windowEntry:]
		d.windows[w[0]] = window{
			int(binary.BigEndian.Uint16(w[1:])), int(binary.BigEndian.Uint16(w[3:])),
			int(binary.BigEndian.Uint16(w[5:])), int(binary.BigEndian.Uint16(w[7:])),
		}
	}
}

func (d *Decoder) readPalette(data []byte) {
	if len(data) < paletteHeader {
		return
	}
	for e := data[paletteHeader:]; len(e) >= paletteEntry; e = e[paletteEntry:] {
		pal := d.palettes[data[0]]
		if pal == nil {
			pal = new([256]entry)
			d.palettes[data[0]] = pal
		}
		pal[e[0]] = entry{y: e[1], cr: e[2], cb: e[3], alpha: e[4], defined: true}
	}
}

func (d *Decoder) readObject(data []byte) error {
	if len(data) < objectHeader {
		return nil
	}
	id := binary.BigEndian.Uint16(data)
	if data[3]&firstFragment == 0 {
		if o := d.objects[id]; o != nil {
			o.rle = append(o.rle, data[objectHeader:]...)
		}
		return nil
	}
	var damage string
	switch {
	case len(data) < firstObjectHeader:
		damage = "The first definition segment of object " + strconv.Itoa(int(id)) + " is cut off."
	default:
		w, h := int(binary.BigEndian.Uint16(data[7:])), int(binary.BigEndian.Uint16(data[9:]))
		if reason := bitmap.SizeLimit(w, h); reason != "" {
			damage = "Cannot read object " + strconv.Itoa(int(id)) + ": " + reason
			break
		}
		d.objects[id] = &object{width: w, height: h, rle: slices.Clone(data[firstObjectHeader:])}
		return nil
	}
	if err := d.fail(damage); err != nil {
		return err
	}
	if c := d.pending; c != nil {
		d.pending = nil
		d.show(c.time, nil)
	}
	return nil
}

// endDisplaySet renders the pending composition and updates the shown cue.
func (d *Decoder) endDisplaySet(cutOff bool) error {
	c := d.pending
	if c == nil {
		return nil
	}
	d.pending = nil
	var next *shown
	if !cutOff {
		var msg string
		if next, msg = d.render(c); msg != "" {
			if err := d.fail(msg); err != nil {
				return err
			}
			next = nil
		}
	}
	d.show(c.time, next)
	return nil
}

// show puts next on the screen at t. An image equal to the shown one keeps its cue. Else the shown cue ends at t.
func (d *Decoder) show(t time.Duration, next *shown) {
	if d.shown != nil {
		if next != nil && next.same(d.shown) {
			return
		}
		d.endShown(t)
	}
	d.shown = next
}

func (d *Decoder) endShown(end time.Duration) {
	s := d.shown
	d.shown = nil
	d.cues = append(d.cues, bitmap.Cue(s.start, max(end, s.start), s.im, s.forced))
}

// render returns the image of c, nil when c shows nothing, or the message of the damage.
func (d *Decoder) render(c *composition) (*shown, string) {
	pal := d.palettes[c.paletteID]
	if pal == nil || len(c.refs) == 0 {
		return nil, ""
	}
	colors := colorMap(pal, bitmap.MatrixFor(c.screenHeight))
	var parts []bitmap.Part
	forced := false
	for _, ref := range c.refs {
		part, ok, msg := d.part(c, ref, &colors)
		if msg != "" {
			return nil, msg
		}
		if ok {
			parts = append(parts, part)
			forced = forced || ref.forced
		}
	}
	if len(parts) == 0 {
		return nil, ""
	}
	im, err := bitmap.ImageOf(parts, c.screenWidth, c.screenHeight)
	if err == nil {
		err = im.Validate()
	}
	if err != nil {
		return nil, errorMessage(err)
	}
	return &shown{start: c.time, im: im, forced: forced}, ""
}

// part returns the visible part of the object of ref with its pixels and colours. ok is false when the object is
// unknown or nothing of it shows. A message reports an object with too little run-length data.
func (d *Decoder) part(c *composition, ref reference, colors *[256]uint32) (part bitmap.Part, ok bool, msg string) {
	obj := d.objects[ref.id]
	if obj == nil {
		return bitmap.Part{}, false, ""
	}
	a, ok := d.visibleArea(ref, obj)
	if !ok {
		return bitmap.Part{}, false, ""
	}
	size := obj.width * obj.height
	indexes, got := decodeRLE(obj.rle, size)
	if got < size {
		return bitmap.Part{}, false, "Object " + strconv.Itoa(int(ref.id)) + " at " + seconds(c.time) + " s has " + strconv.Itoa(got) +
			" pixels of run-length data, but its size of " + strconv.Itoa(obj.width) + "x" + strconv.Itoa(obj.height) +
			" needs " + strconv.Itoa(size) + "."
	}
	part = a.p
	part.Indexes = make([]byte, 0, part.Width*part.Height)
	for row := range part.Height {
		from := (a.top+row)*obj.width + a.left
		part.Indexes = append(part.Indexes, indexes[from:from+part.Width]...)
	}
	part.Palette = *colors
	return part, true, ""
}

// area is the part of an object that shows: the part with its screen position, and where it starts in the object.
type area struct {
	p         bitmap.Part
	left, top int
}

// visibleArea returns the part of obj inside the cropping rectangle and the window of ref, or false when none shows.
// A cropped object keeps the screen position x, y of the reference, as libbluray draws it.
func (d *Decoder) visibleArea(ref reference, obj *object) (area, bool) {
	left, top, width, height := 0, 0, obj.width, obj.height
	if ref.cropped {
		left, top, width, height = ref.crop.x, ref.crop.y, ref.crop.width, ref.crop.height
	}
	right, bottom := min(left+width, obj.width), min(top+height, obj.height)
	x, y := ref.x, ref.y
	if w, ok := d.windows[ref.windowID]; ok {
		clipLeft, clipTop := max(0, w.x-x), max(0, w.y-y)
		right, bottom = min(right, left+w.x+w.width-x), min(bottom, top+w.y+w.height-y)
		left, top, x, y = left+clipLeft, top+clipTop, x+clipLeft, y+clipTop
	}
	if right <= left || bottom <= top {
		return area{}, false
	}
	return area{p: bitmap.Part{X: x, Y: y, Width: right - left, Height: bottom - top}, left: left, top: top}, true
}

// decodeRLE returns the run-length decoded pixels of rle, one palette entry byte each, at most size. The count is the
// number of pixels that rle holds, and is less than size when rle ends early.
func decodeRLE(rle []byte, size int) ([]byte, int) {
	at := func(i int) byte {
		if i < len(rle) {
			return rle[i]
		}
		return 0
	}
	out := make([]byte, 0, min(size, 4*len(rle)))
	for i := 0; i < len(rle) && len(out) < size; {
		if rle[i] != 0 {
			out = append(out, rle[i])
			i++
			continue
		}
		flags := at(i + 1)
		i += 2
		run := int(flags & 0x3F)
		if flags&0x40 != 0 {
			run = run<<8 | int(at(i))
			i++
		}
		var color byte
		if flags&0x80 != 0 {
			color = at(i)
			i++
		}
		run = min(run, size-len(out))
		out = slices.Grow(out, run)[:len(out)+run]
		for j := len(out) - run; j < len(out); j++ {
			out[j] = color
		}
	}
	return out, len(out)
}

// colorMap returns the 0xRRGGBBAA colour of each of the 256 palette entries. An entry the palette lacks is transparent.
func colorMap(pal *[256]entry, m bitmap.Matrix) [256]uint32 {
	var colors [256]uint32
	for i, e := range pal {
		if e.defined {
			r, g, b := bitmap.ToRGB(e.y, e.cr, e.cb, m)
			colors[i] = uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(e.alpha)
		}
	}
	return colors
}

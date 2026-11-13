// Package dvbsub decodes DVB subtitle segments (ETSI EN 300 743) into image cues.
//
// The default CLUTs, the reduced range CLUT entries and the display sets without an end segment follow the behaviour of
// FFmpeg libavcodec/dvbsubdec.c. The code is written from the specification.
package dvbsub

import (
	"encoding/binary"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	segmentHeader = 6
	syncByte      = 0x0F

	segPage    = 0x10
	segRegion  = 0x11
	segCLUT    = 0x12
	segObject  = 0x13
	segDisplay = 0x14
	segEnd     = 0x80

	defaultWidth  = 720
	defaultHeight = 576
)

type pageRegion struct{ id, x, y int }

type objectRef struct{ id, x, y int }

// region is one rectangle of a page. pixels holds one pixel code per pixel, row by row.
type region struct {
	width, height, depth, clut int
	pixels                     []byte
	objects                    []objectRef
}

// pending is the display set that the last page composition segment started.
type pending struct {
	time, expires time.Duration
	hasExpiry     bool
}

// shownCue is the image on the screen and the time it appeared.
type shownCue struct {
	start, expires time.Duration
	hasExpiry      bool
	im             model.Image
}

type cueRecord struct {
	start, end time.Duration
	im         model.Image
}

// Decoder turns the segments of one DVB subtitle service into image cues.
type Decoder struct {
	pages   [2]int
	all     bool
	width   int
	height  int
	left    int
	top     int
	version int // -1 before the first page composition segment

	pageRegions []pageRegion
	regions     map[int]*region
	cluts       map[int]*clut
	pending     *pending
	shown       *shownCue
	cues        []cueRecord
}

// NewDecoder reads the segments of the composition page and the ancillary page.
// all reads every page, for a Matroska track without page numbers.
func NewDecoder(compositionPage, ancillaryPage int, all bool) *Decoder {
	return &Decoder{
		pages: [2]int{compositionPage, ancillaryPage}, all: all,
		width: defaultWidth, height: defaultHeight, version: -1,
		regions: map[int]*region{}, cluts: map[int]*clut{},
	}
}

// Add reads the segments of one PES payload or one Matroska block that starts at start.
// end is the end of a Matroska block with a duration, hasEnd false otherwise.
func (d *Decoder) Add(data []byte, start, end time.Duration, hasEnd bool) error {
	offset := 0
	if len(data) >= 2 && data[0] == 0x20 && data[1] == 0x00 {
		offset = 2
	}
	for offset+segmentHeader <= len(data) && data[offset] == syncByte {
		typ := data[offset+1]
		page := int(binary.BigEndian.Uint16(data[offset+2:]))
		length := int(binary.BigEndian.Uint16(data[offset+4:]))
		offset += segmentHeader
		segment := data[offset:min(len(data), offset+length)]
		offset += length
		if !d.all && page != d.pages[0] && page != d.pages[1] {
			continue
		}
		if err := d.read(typ, segment, start, end, hasEnd); err != nil {
			return err
		}
	}
	return d.show()
}

func (d *Decoder) read(typ byte, segment []byte, start, end time.Duration, hasEnd bool) error {
	switch typ {
	case segPage:
		d.readPage(segment, start, end, hasEnd)
	case segRegion:
		return d.readRegion(segment)
	case segCLUT:
		d.readCLUT(segment)
	case segObject:
		d.readObject(segment)
	case segDisplay:
		d.readDisplay(segment)
	case segEnd:
		return d.show()
	}
	return nil
}

// Finish ends the shown cue and returns the cues. Times below 0 become 0.
func (d *Decoder) Finish(lastCueDuration time.Duration) []model.Cue {
	if s := d.shown; s != nil {
		end := parsing.EndAfter(s.start, lastCueDuration)
		if s.hasExpiry {
			end = s.expires
		}
		d.endCue(end)
	}
	cues := make([]model.Cue, len(d.cues))
	for i, c := range d.cues {
		start := wholeMillis(c.start)
		cues[i] = bitmap.Cue(start, max(wholeMillis(c.end), start), c.im, false)
	}
	return cues
}

// wholeMillis returns t in whole milliseconds, half up, from 0 to the time limit.
func wholeMillis(t time.Duration) time.Duration {
	if t <= 0 {
		return 0
	}
	if t >= timecode.MaxValidTime {
		return timecode.MaxValidTime
	}
	return (t + 500*time.Microsecond).Truncate(time.Millisecond)
}

func (d *Decoder) readPage(data []byte, start, end time.Duration, hasEnd bool) {
	if len(data) < 2 {
		return
	}
	p := &pending{time: start}
	if timeOut := time.Duration(data[0]) * time.Second; timeOut > 0 {
		p.expires, p.hasExpiry = start+timeOut, true
	}
	if hasEnd && (!p.hasExpiry || end < p.expires) {
		p.expires, p.hasExpiry = end, true
	}
	d.pending = p
	version := int(data[1] >> 4)
	if version == d.version {
		return
	}
	d.version = version
	if data[1]>>2&0x03 != 0 {
		clear(d.regions)
		clear(d.cluts)
	}
	d.pageRegions = d.pageRegions[:0]
	for e := data[2:]; len(e) >= 6; e = e[6:] {
		d.pageRegions = append(d.pageRegions, pageRegion{int(e[0]), int(binary.BigEndian.Uint16(e[2:])), int(binary.BigEndian.Uint16(e[4:]))})
	}
}

func (d *Decoder) readRegion(data []byte) error {
	if len(data) < 10 {
		return nil
	}
	id, flags := int(data[0]), data[1]
	width, height := int(binary.BigEndian.Uint16(data[2:])), int(binary.BigEndian.Uint16(data[4:]))
	depth := regionDepth(data[6] >> 2 & 0x07)
	if depth == 0 || width == 0 || height == 0 {
		return nil
	}
	if reason := bitmap.SizeLimit(width, height); reason != "" {
		return &model.ParseError{Msg: "Cannot read region " + strconv.Itoa(id) + ": " + reason}
	}
	r := &region{width: width, height: height, depth: depth, clut: int(data[7]), objects: regionObjects(data)}
	if old := d.regions[id]; old != nil && old.width == width && old.height == height && flags&0x08 == 0 {
		r.pixels = old.pixels
	} else {
		r.pixels = filled(width*height, backgroundCode(data, depth))
	}
	d.regions[id] = r
	return nil
}

func filled(n int, code byte) []byte {
	b := make([]byte, n)
	if code != 0 {
		for i := range b {
			b[i] = code
		}
	}
	return b
}

// regionDepth returns the bits per pixel code of a depth field, 0 for an unknown depth.
func regionDepth(field byte) int {
	if field < 1 || field > 3 {
		return 0
	}
	return 1 << field
}

// backgroundCode returns the pixel code that fills a region of depth bits per code.
func backgroundCode(data []byte, depth int) byte {
	switch depth {
	case 4:
		return data[9] >> 4
	case 2:
		return data[9] >> 2 & 0x03
	}
	return data[8]
}

// regionObjects returns the objects of type 0 (basic bitmap) in the entries from byte 10. Types 1 and 2 have 2 more bytes.
func regionObjects(data []byte) []objectRef {
	var objects []objectRef
	for offset := 10; offset+6 <= len(data); {
		x := binary.BigEndian.Uint16(data[offset+2:])
		switch x >> 14 {
		case 0:
			objects = append(objects, objectRef{int(binary.BigEndian.Uint16(data[offset:])), int(x & 0x0FFF), int(binary.BigEndian.Uint16(data[offset+4:]) & 0x0FFF)})
			offset += 6
		case 1, 2:
			offset += 8
		default:
			offset += 6
		}
	}
	return objects
}

func (d *Decoder) readCLUT(data []byte) {
	id := 0
	if len(data) > 0 {
		id = int(data[0])
	}
	c, ok := d.cluts[id]
	if !ok {
		def := defaultCLUT()
		c = &def
		d.cluts[id] = c
	}
	c.set(data)
}

// readObject draws the object into each region that lists it. Only objects coded as pixels are read, not character strings.
func (d *Decoder) readObject(data []byte) {
	if len(data) < 7 || data[2]>>2&0x03 != 0 {
		return
	}
	id := int(binary.BigEndian.Uint16(data))
	nonModifying := data[2]&0x02 != 0
	topLength, bottomLength := int(binary.BigEndian.Uint16(data[3:])), int(binary.BigEndian.Uint16(data[5:]))
	top := data[7:min(len(data), 7+topLength)]
	bottom := top
	if bottomLength != 0 {
		bottom = data[min(len(data), 7+topLength):min(len(data), 7+topLength+bottomLength)]
	}
	for _, r := range d.regions {
		for _, o := range r.objects {
			if o.id == id {
				r.drawField(top, o.x, o.y, nonModifying)
				r.drawField(bottom, o.x, o.y+1, nonModifying)
			}
		}
	}
}

func (d *Decoder) readDisplay(data []byte) {
	if len(data) < 5 {
		return
	}
	d.width, d.height = int(binary.BigEndian.Uint16(data[1:]))+1, int(binary.BigEndian.Uint16(data[3:]))+1
	d.left, d.top = 0, 0
	if data[0]&0x08 != 0 && len(data) >= 13 {
		d.left, d.top = int(binary.BigEndian.Uint16(data[5:])), int(binary.BigEndian.Uint16(data[9:]))
	}
}

// show puts the pending display set on the screen. An image equal to the shown one that has not expired continues its cue.
func (d *Decoder) show() error {
	p := d.pending
	if p == nil {
		return nil
	}
	d.pending = nil
	im, ok, err := d.render()
	if err != nil {
		return err
	}
	if s := d.shown; s != nil {
		if ok && sameImage(im, s.im) && (!s.hasExpiry || s.expires >= p.time) {
			s.expires, s.hasExpiry = p.expires, p.hasExpiry
			return nil
		}
		end := p.time
		if s.hasExpiry {
			end = min(s.expires, end)
		}
		d.endCue(end)
	}
	d.shown = nil
	if ok {
		d.shown = &shownCue{start: p.time, expires: p.expires, hasExpiry: p.hasExpiry, im: im}
	}
	return nil
}

func sameImage(a, b model.Image) bool {
	return a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height &&
		a.ScreenWidth == b.ScreenWidth && a.ScreenHeight == b.ScreenHeight && string(a.PNG) == string(b.PNG)
}

func (d *Decoder) endCue(end time.Duration) {
	d.cues = append(d.cues, cueRecord{start: d.shown.start, end: end, im: d.shown.im})
	d.shown = nil
}

// render returns the image of the page. ok is false when no region shows a pixel.
func (d *Decoder) render() (im model.Image, ok bool, err error) {
	var parts []bitmap.Part
	for _, pr := range d.pageRegions {
		r := d.regions[pr.id]
		if r == nil {
			continue
		}
		var colors [256]uint32
		if c := d.cluts[r.clut]; c != nil {
			colors = c.colors(r.depth)
		} else {
			def := defaultCLUT()
			colors = def.colors(r.depth)
		}
		var used [256]bool
		for _, v := range r.pixels {
			used[v] = true
		}
		for code, u := range used {
			if u && colors[code]&0xFF != 0 {
				parts = append(parts, bitmap.Part{X: d.left + pr.x, Y: d.top + pr.y, Width: r.width, Height: r.height, Indexes: r.pixels, Palette: colors})
				break
			}
		}
	}
	if len(parts) == 0 {
		return model.Image{}, false, nil
	}
	im, err = bitmap.ImageOf(parts, d.width, d.height)
	return im, err == nil, err
}

package cea708

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
)

const (
	windowCount = 8
	// A service number 7 in a service block header means the next byte holds the number.
	extendedService = 7
	// The vertical anchor of an absolute position counts 75 rows, the size of a 16:9 screen.
	absoluteRows = 75
	noAnchor     = -1
)

// Triplet is one valid cc_data triplet: its cc_type and its 2 data bytes.
type Triplet struct {
	Type byte
	Data [2]byte
}

// Frame is the valid triplets of the caption data packets at one time.
type Frame struct {
	Time     time.Duration
	Triplets []Triplet
}

// State is the text of the visible windows after a change.
type State struct {
	Time        time.Duration
	Lines       []string // core markup, empty when no visible window holds text
	AnchorPoint int      // 0 top left to 8 bottom right, of the first window with text. -1 without text
}

type pen struct {
	italic, underline bool
	color             byte
}

type cell struct {
	r rune
	pen
}

type window struct {
	visible  bool
	rows     int
	anchor   int
	priority int
	seq      int // order of the first definition
	cells    map[int]map[int]cell
	row, col int
	pen      pen
}

// Decoder reads the DTVCC packets of one service.
type Decoder struct {
	service int
	windows [windowCount]*window
	current *window
	seq     int
	packet  []byte
	size    int
}

// NewDecoder returns a decoder of the service 1 to 63.
func NewDecoder(service int) *Decoder {
	return &Decoder{service: service}
}

// Decode returns the displayed text at each change, in time order. The decoder keeps its state between calls.
func (d *Decoder) Decode(frames []Frame) []State {
	var states []State
	for _, f := range frames {
		for _, t := range f.Triplets {
			switch {
			case t.Type == DTVCCStart:
				d.packet = d.packet[:0]
				d.size = int(t.Data[0]&0x3F) * 2
				if d.size == 0 {
					d.size = 128
				}
			case t.Type != DTVCCData || d.size == 0:
				continue
			}
			d.packet = append(d.packet, t.Data[:]...)
			if len(d.packet) >= d.size {
				states = d.readPacket(d.packet[1:d.size], f.Time, states)
				d.size = 0
			}
		}
	}
	return states
}

func (d *Decoder) readPacket(data []byte, at time.Duration, states []State) []State {
	for off := 0; off < len(data); {
		header := data[off]
		off++
		service, size := int(header>>5), int(header&0x1F)
		if service == 0 || size == 0 {
			break
		}
		if service == extendedService {
			service = 0
			if off < len(data) {
				service = int(data[off] & 0x3F)
			}
			off++
		}
		if service == d.service {
			d.interpret(data[min(off, len(data)):min(off+size, len(data))])
			states = d.record(at, states)
		}
		off += size
	}
	return states
}

func (d *Decoder) interpret(block []byte) {
	for off := 0; off < len(block); {
		code := block[off]
		off++
		switch {
		case code == codeEXT1:
			off = d.extended(block, off)
		case code < 0x20:
			d.c0(code)
			switch {
			case code >= codeP16:
				off += 2
			case code > codeEXT1:
				off++
			}
		case code < 0x80:
			if code == musicNote {
				d.write('\u266a')
			} else {
				d.write(rune(code))
			}
		case code < 0xA0:
			count := c1Parameters[code-0x80]
			var params [6]byte
			copy(params[:count], block[min(off, len(block)):])
			d.c1(code, params)
			off += count
		default:
			d.write(rune(code))
		}
	}
}

// extended reads the code after EXT1 and returns the offset after it and its parameter bytes.
func (d *Decoder) extended(block []byte, off int) int {
	at := func(i int) byte {
		if i < len(block) {
			return block[i]
		}
		return 0
	}
	code := at(off)
	off++
	switch {
	case code < 0x20:
		return off + int(code)/8
	case code < 0x80:
		if r, ok := g2[code]; ok {
			d.write(r)
		}
		return off
	case code < 0x88:
		return off + 4
	case code < 0x90:
		return off + 5
	case code < 0xA0:
		return off + 1 + int(at(off)&0x3F)
	}
	return off
}

func (d *Decoder) c0(code byte) {
	w := d.current
	if w == nil {
		return
	}
	switch code {
	case codeBS:
		if w.col > 0 {
			w.col--
			delete(w.cells[w.row], w.col)
		}
	case codeFF:
		w.cells = map[int]map[int]cell{}
		w.row, w.col = 0, 0
	case codeCR:
		w.col = 0
		if w.row+1 < w.rows {
			w.row++
			return
		}
		scrolled := map[int]map[int]cell{}
		for row, cells := range w.cells {
			if row > 0 {
				scrolled[row-1] = cells
			}
		}
		w.cells = scrolled
	case codeHCR:
		delete(w.cells, w.row)
		w.col = 0
	}
}

func (d *Decoder) c1(code byte, p [6]byte) {
	switch {
	case code < cmdCLW:
		if w := d.windows[code-cmdCW0]; w != nil {
			d.current = w
		}
		return
	case code >= cmdDF0:
		d.define(int(code-cmdDF0), p)
		return
	case code >= cmdCLW && code <= cmdDLW:
		d.windowCommand(code, p[0])
		return
	case code == cmdRST:
		d.windows = [windowCount]*window{}
		d.current = nil
		return
	}
	if w := d.current; w != nil {
		w.penCommand(code, p)
	}
}

func (w *window) penCommand(code byte, p [6]byte) {
	switch code {
	case cmdSPA:
		w.pen.italic = p[1]&0x80 != 0
		w.pen.underline = p[1]&0x40 != 0
	case cmdSPC:
		w.pen.color = p[0] & 0x3F
	case cmdSPL:
		w.row = int(p[0] & 0x0F)
		w.col = int(p[1] & 0x3F)
	}
}

func (d *Decoder) windowCommand(code, bitmap byte) {
	for id, w := range d.windows {
		if bitmap>>id&1 == 0 || w == nil {
			continue
		}
		switch code {
		case cmdCLW:
			w.cells = map[int]map[int]cell{}
		case cmdDSW:
			w.visible = true
		case cmdHDW:
			w.visible = false
		case cmdTGW:
			w.visible = !w.visible
		case cmdDLW:
			if d.current == w {
				d.current = nil
			}
			d.windows[id] = nil
		}
	}
}

// define defines a window or changes its attributes. A new window starts empty with the pen at row 0, column 0.
func (d *Decoder) define(id int, p [6]byte) {
	w := d.windows[id]
	if w == nil {
		d.seq++
		w = &window{seq: d.seq, cells: map[int]map[int]cell{}, pen: pen{color: white}}
		d.windows[id] = w
	}
	denominator := 100
	if p[1]&0x80 == 0 {
		denominator = absoluteRows
	}
	vertical := int(p[1] & 0x7F)
	row := 1
	switch {
	case vertical*3 < denominator:
		row = 0
	case vertical*3 > 2*denominator:
		row = 2
	}
	w.visible = p[0]&0x20 != 0
	w.rows = int(p[3]&0x0F) + 1
	w.anchor = 3*row + int(p[3]>>4)%3
	w.priority = int(p[0] & 0x07)
	d.current = w
}

func (d *Decoder) write(r rune) {
	w := d.current
	if w == nil {
		return
	}
	if w.cells[w.row] == nil {
		w.cells[w.row] = map[int]cell{}
	}
	w.cells[w.row][w.col] = cell{r: r, pen: w.pen}
	w.col++
}

// record adds the text of the visible windows as a state when it differs from the last state.
func (d *Decoder) record(at time.Duration, states []State) []State {
	lines, anchor := d.visibleLines()
	if len(states) == 0 && len(lines) == 0 || len(states) > 0 && slices.Equal(states[len(states)-1].Lines, lines) {
		return states
	}
	return append(states, State{Time: at, Lines: lines, AnchorPoint: anchor})
}

// visibleLines returns the rows of the visible windows and the anchor point of the first window with text.
// A window with a lower priority number comes first.
func (d *Decoder) visibleLines() (lines []string, anchor int) {
	var visible []*window
	for _, w := range d.windows {
		if w != nil && w.visible {
			visible = append(visible, w)
		}
	}
	slices.SortFunc(visible, func(a, b *window) int {
		return cmp.Or(cmp.Compare(a.priority, b.priority), cmp.Compare(a.seq, b.seq))
	})
	anchor = noAnchor
	for _, w := range visible {
		rows := slices.Sorted(maps.Keys(w.cells))
		for _, row := range rows {
			if line := renderRow(w.cells[row]); line != "" {
				lines = append(lines, line)
				if anchor == noAnchor {
					anchor = w.anchor
				}
			}
		}
	}
	return lines, anchor
}

// renderRow writes a row of cells as core markup without the spaces at both ends.
func renderRow(cells map[int]cell) string {
	last := -1
	for col := range cells {
		last = max(last, col)
	}
	runs := make([]markupcore.Run, 0, last+1)
	for col := 0; col <= last; col++ {
		c, ok := cells[col]
		if !ok {
			c = cell{r: ' ', pen: pen{color: white}}
		}
		run := markupcore.Run{Text: string(c.r), Italic: c.italic, Underline: c.underline}
		if c.color != white {
			run.Color = "#" + channelHex[c.color>>4] + channelHex[c.color>>2&3] + channelHex[c.color&3]
		}
		runs = append(runs, run)
	}
	return strings.Trim(markupcore.StyleRuns(runs, true), " ")
}

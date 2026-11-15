// Package arib decodes ARIB STD-B24 caption data groups into screen states.
//
// Data groups and data units: ARIB STD-B24 volume 1 part 3 chapter 9. Codes and control functions: volume 1 part 2 chapter 7.
// Adapted from xqq/libaribcaption, src/decoder/decoder_impl.cpp
// (https://github.com/xqq/libaribcaption/blob/37dde0f32652c32aa98bf92278944d1cfb2e7354/src/decoder/decoder_impl.cpp), MIT.
package arib

import (
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

type encoding int

const (
	encJapanese encoding = iota
	encLatin
	encUTF8
)

const (
	// tenthSecond is a wait of the TIME control of one tenth of a second, in 90 kHz clock ticks.
	tenthSecond = 9000
	white       = "#ffffff"
	// maxRepeat limits the cells that one RPC control writes. A row of the widest format holds 96 cells.
	maxRepeat = 1024
	// maxCells limits the characters on the screen. A screen of the widest format with 1 by 1 dot cells holds
	// more than 500,000. The largest screen of ARIB TR-B14 profile A, 1920 by 1080, holds 864 at normal size and
	// 3,456 at small size.
	maxCells = 4096
	// The states that one PES packet adds may hold text of up to stateTextBase plus stateTextPerByte times its size.
	// One state is as long as the screen, so a group that repeats TIME would else grow without a limit.
	stateTextBase    = 32 << 10
	stateTextPerByte = 32
	// maxParameter limits a number of a CSI control, so that the screen coordinates stay small.
	maxParameter = 1 << 20
)

// State is what the caption screen shows from Start on. No lines clear the screen.
type State struct {
	Start     int64 // 90 kHz ticks
	Lines     []string
	Alignment model.Alignment
}

// Warning is a problem of the stream at a 90 kHz time. The decoder repaired it, or it skipped data when Skipped is true.
type Warning struct {
	Time    int64
	Message string
	Skipped bool
}

// cell is one character of the screen at its cell.
type cell struct {
	x, y, width, height int
	ruby                bool
	color               string
	text                string
}

type position struct{ x, y int }

// Decoder decodes the first language of an ARIB caption stream.
type Decoder struct {
	encoding        encoding
	managementGroup int
	time            int64
	sets            [4]int
	left, right     int
	format          int

	planeWidth, planeHeight int
	areaX, areaY            int
	areaWidth, areaHeight   int
	charWidth, charHeight   int
	spacingX, spacingY      int
	scaleX, scaleY          float64
	positioned              bool
	x, y                    int
	palette                 int
	color                   string
	repeat                  int

	glyphs   map[int]glyph
	cells    []cell
	cellAt   map[position]int
	states   []State
	warnings []Warning
	warned   map[string]bool

	groupText   int
	groupBudget int
}

// NewDecoder returns a decoder. The ISO 639-2 language of the stream picks the coding until the caption management data does.
func NewDecoder(language string) *Decoder {
	d := &Decoder{
		managementGroup: -1,
		format:          7,
		glyphs:          map[int]glyph{},
		cellAt:          map[position]int{},
		warned:          map[string]bool{},
	}
	d.encoding = pickEncoding([]languageEntry{{code: language}})
	d.reset()
	return d
}

// Add reads the data of a PES packet with its time in 90 kHz clock ticks.
func (d *Decoder) Add(data []byte, pts int64) {
	id, body, ok := dataGroup(data)
	if !ok {
		return
	}
	d.time = max(pts, d.time)
	d.groupText, d.groupBudget = 0, stateTextBase+stateTextPerByte*len(data)
	switch {
	case id&0x0F == 0 && id>>5 != d.managementGroup:
		d.managementGroup = id >> 5
		d.managementData(body)
	case id&0x0F == 1:
		d.statement(body)
	default:
		return
	}
	d.snapshot()
}

// dataGroup returns the id and the body of the data group in the data of a PES packet.
func dataGroup(data []byte) (id int, body []byte, ok bool) {
	if len(data) < 3 || data[0] != 0x80 || data[1] != 0xFF {
		return 0, nil, false
	}
	start := 3 + int(data[2]&0x0F)
	if start+5 > len(data) {
		return 0, nil, false
	}
	size := int(data[start+3])<<8 | int(data[start+4])
	body = sub(data, start+5, size)
	return int(data[start] >> 2), body, len(body) > 0 && len(body) >= size
}

func (d *Decoder) statement(body []byte) {
	offset := 1
	if tmd := body[0] >> 6; tmd == 1 || tmd == 2 {
		offset += 5
	}
	d.dataUnits(sub(body, offset+3, uint24(body, offset)))
}

// Finish returns the states of the screen.
func (d *Decoder) Finish() []State { return d.states }

// Warnings returns the DRCS glyphs that have no known character.
func (d *Decoder) Warnings() []Warning { return d.warnings }

// Language returns the first language of the caption management data in one PES payload, or "".
func Language(data []byte) string {
	if len(data) < 3 || data[0] != 0x80 {
		return ""
	}
	start := 3 + int(data[2]&0x0F)
	if start+5 > len(data) || data[start]>>2&0x0F != 0 {
		return ""
	}
	body := data[start+5:]
	offset := 0
	if len(body) > 0 {
		offset = managementOffset(body)
	}
	languages, _ := parseLanguages(body, offset)
	if len(languages) == 0 {
		return ""
	}
	return languages[0].code
}

func managementOffset(body []byte) int {
	if body[0]>>6 == 2 {
		return 6
	}
	return 1
}

func (d *Decoder) managementData(body []byte) {
	offset := managementOffset(body)
	languages, offset := parseLanguages(body, offset)
	if len(languages) > 0 {
		d.format = languages[0].format - 1
		if e := pickEncoding(languages); e != d.encoding {
			d.encoding = e
			d.reset()
		} else {
			d.resetSets()
			d.resetFormat()
		}
	}
	d.dataUnits(sub(body, offset+3, uint24(body, offset)))
}

type languageEntry struct {
	code        string
	tcs, format int
}

// parseLanguages reads the language loop of caption management data from offset and returns the offset after it.
func parseLanguages(body []byte, offset int) ([]languageEntry, int) {
	count := at(body, offset)
	offset++
	var languages []languageEntry
	for range count {
		if offset+5 > len(body) {
			break
		}
		offset++
		if display := body[offset-1] & 0x0F; display >= 0x0C && display <= 0x0E {
			offset++
		}
		flags := at(body, offset+3)
		languages = append(languages, languageEntry{lower(sub(body, offset, 3)), flags >> 2 & 0x03, flags >> 4})
		offset += 4
	}
	return languages, offset
}

func lower(b []byte) string {
	out := slices.Clone(b)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + 'a' - 'A'
		}
	}
	return string(out)
}

func pickEncoding(languages []languageEntry) encoding {
	has := func(codes ...string) bool {
		return slices.ContainsFunc(languages, func(l languageEntry) bool { return slices.Contains(codes, l.code) })
	}
	switch {
	case slices.ContainsFunc(languages, func(l languageEntry) bool { return l.tcs == 1 }):
		return encUTF8
	case has("jpn"):
		return encJapanese
	case has("por", "spa"):
		return encLatin
	case has("eng", "tgl"):
		return encUTF8
	default:
		return encJapanese
	}
}

func (d *Decoder) dataUnits(units []byte) {
	for offset := 0; offset+5 <= len(units) && units[offset] == 0x1F; {
		parameter := units[offset+1]
		size := uint24(units, offset+2)
		unit := sub(units, offset+5, size)
		if len(unit) < size {
			return
		}
		switch parameter {
		case 0x20:
			d.text(unit)
		case 0x30, 0x31:
			for _, g := range parseDRCS(unit, parameter == 0x31) {
				d.glyphs[g.key] = g.glyph
			}
		}
		offset += 5 + size
	}
}

// skip adds the warning with the id once. The reader dropped data.
func (d *Decoder) skip(id, message string) {
	if !d.warned[id] {
		d.warned[id] = true
		d.warnings = append(d.warnings, Warning{Time: d.time, Message: message, Skipped: true})
	}
}

// snapshot records the screen at the decoder time. A state at the time of the last state replaces it.
func (d *Decoder) snapshot() {
	if d.groupText > d.groupBudget {
		return
	}
	lines, alignment := d.render()
	for _, l := range lines {
		d.groupText += len(l)
	}
	if d.groupText > d.groupBudget {
		d.skip("states", "The caption states of one PES packet are longer than 32 KiB plus 32 times the packet. The reader dropped the states past that.")
		return
	}
	d.addState(State{d.time, lines, alignment})
}

// addState adds the state. A state at the time of the last state replaces it and drops it when it equals the one before.
func (d *Decoder) addState(state State) {
	last := len(d.states) - 1
	same := func(s State) bool { return slices.Equal(s.Lines, state.Lines) && s.Alignment == state.Alignment }
	switch {
	case last >= 0 && d.states[last].Start == state.Start:
		d.states[last] = state
		if last > 0 && same(d.states[last-1]) {
			d.states = d.states[:last]
		}
	case last < 0 || !same(d.states[last]):
		d.states = append(d.states, state)
	}
}

// sub returns up to n bytes of b from start, empty when start is past the end.
func sub(b []byte, start, n int) []byte {
	start = min(start, len(b))
	return b[start:min(len(b), start+n)]
}

func at(b []byte, i int) int {
	if i < 0 || i >= len(b) {
		return 0
	}
	return int(b[i])
}

func uint24(b []byte, i int) int { return at(b, i)<<16 | at(b, i+1)<<8 | at(b, i+2) }

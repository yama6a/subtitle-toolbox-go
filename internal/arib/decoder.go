// Package arib decodes ARIB STD-B24 caption data groups into screen states.
//
// Data groups and data units: ARIB STD-B24 volume 1 part 3 chapter 9. Codes and control functions: volume 1 part 2 chapter 7.
// Adapted from xqq/libaribcaption, src/decoder/decoder_impl.cpp
// (https://github.com/xqq/libaribcaption/blob/37dde0f32652c32aa98bf92278944d1cfb2e7354/src/decoder/decoder_impl.cpp), MIT.
package arib

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
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

func (d *Decoder) text(b []byte) {
	for offset := 0; offset < len(b); {
		if d.encoding == encUTF8 {
			offset += d.utf8Code(b, offset)
		} else {
			offset += d.eightUnitCode(b, offset)
		}
	}
}

func (d *Decoder) utf8Code(b []byte, i int) int {
	switch c := b[i]; {
	case c <= 0x1F:
		return d.c0(b, i)
	case c == 0x7F:
		return 1
	case c == 0xC2 && at(b, i+1) >= 0x80 && at(b, i+1) <= 0x9F:
		return 1 + d.c1(b, i+1)
	default:
		return d.utf8(b, i)
	}
}

func (d *Decoder) eightUnitCode(b []byte, i int) int {
	switch c := b[i]; {
	case c <= 0x20:
		return d.c0(b, i)
	case c < 0x7F:
		return d.graphic(b, i, d.sets[d.left])
	case c <= 0xA0:
		return d.c1(b, i)
	case c < 0xFF:
		return d.graphic(b, i, d.sets[d.right])
	default:
		return 1
	}
}

// c0 runs a C0 control function and returns the number of bytes it takes, at least 1.
func (d *Decoder) c0(b []byte, i int) int {
	switch b[i] {
	case 0x16:
		d.move(at(b, i+1)&0x3F, 0)
		return 2
	case 0x19:
		return 1 + d.graphic(b, i+1, d.sets[2])
	case 0x1B:
		return 1 + d.escape(b, i+1)
	case 0x1C:
		d.setPosition(at(b, i+2)&0x3F, at(b, i+1)&0x3F)
		return 3
	case 0x1D:
		return 1 + d.graphic(b, i+1, d.sets[3])
	case 0x20:
		d.space()
		return 1
	}
	d.c0Single(b[i])
	return 1
}

// space writes SP, a full width space in Japanese captions at normal size.
func (d *Decoder) space() {
	if d.encoding == encJapanese && !d.halfWidth() {
		d.put("\u3000")
	} else {
		d.put(" ")
	}
}

// c0Single runs a C0 control function of one byte.
func (d *Decoder) c0Single(c byte) {
	switch c {
	case 0x08:
		d.move(-1, 0)
	case 0x09:
		d.move(1, 0)
	case 0x0A:
		d.move(0, 1)
	case 0x0B:
		d.move(0, -1)
	case 0x0C:
		d.clearScreen()
	case 0x0D:
		d.newline()
	case 0x0E:
		d.left = 1
	case 0x0F:
		d.left = 0
	}
}

// escape runs the ESC sequence that starts at b[i] and returns the number of bytes after ESC.
func (d *Decoder) escape(b []byte, i int) int {
	first := at(b, i)
	switch {
	case first == 0x6E || first == 0x6F:
		d.left = first - 0x6C
	case first >= 0x7C && first <= 0x7E:
		d.right = 0x7F - first
	case first == 0x24:
		return d.designateTwoByte(b, i)
	case first >= 0x28 && first <= 0x2B:
		return d.designate(b, i, first-0x28)
	}
	return 1
}

// designateTwoByte reads the final bytes after ESC 0x24 and sets G0 to G3 to a 2-byte set.
func (d *Decoder) designateTwoByte(b []byte, i int) int {
	second := at(b, i+1)
	if second < 0x28 || second > 0x2B {
		d.sets[0] = second
		return 2
	}
	if at(b, i+2) == 0x20 {
		d.sets[second-0x28] = setDRCS | at(b, i+3)
		return 4
	}
	d.sets[second-0x28] = at(b, i+2)
	return 3
}

// designate reads the final byte after ESC 0x28 to 0x2B and sets G0 to G3 to a 1-byte set.
func (d *Decoder) designate(b []byte, i, index int) int {
	if at(b, i+1) == 0x20 {
		d.sets[index] = setDRCS | at(b, i+2)
		return 3
	}
	d.sets[index] = at(b, i+1)
	return 2
}

// c1 runs a C1 control function and returns the number of bytes it takes, at least 1.
func (d *Decoder) c1(b []byte, i int) int {
	code := b[i]
	switch {
	case code >= 0x80 && code <= 0x87:
		d.color = color(d.palette, int(code)-0x80)
	case code >= 0x88 && code <= 0x8B:
		return d.size(code, at(b, i+1))
	case code >= 0x90 && code <= 0x98:
		return d.c1Parameters(b, i)
	case code == 0x9B:
		return 1 + d.csi(b, i+1)
	case code == 0x9D:
		return d.timeControl(b, i)
	}
	return 1
}

// size runs SSZ, MSZ, NSZ or SZX.
func (d *Decoder) size(code byte, parameter int) int {
	switch code {
	case 0x88:
		d.scaleX, d.scaleY = 0.5, 0.5
	case 0x89:
		d.scaleX, d.scaleY = 0.5, 1
	case 0x8A:
		d.scaleX, d.scaleY = 1, 1
	default:
		if parameter == 0x41 || parameter == 0x45 {
			d.scaleY = 2
		}
		if parameter == 0x44 || parameter == 0x45 {
			d.scaleX = 2
		}
		return 2
	}
	return 1
}

// c1Parameters runs the C1 functions 0x90 to 0x98 that take parameters.
func (d *Decoder) c1Parameters(b []byte, i int) int {
	parameter := at(b, i+1)
	switch b[i] {
	case 0x90:
		return d.setColor(b, i)
	case 0x91, 0x93, 0x94, 0x97:
		return 2
	case 0x92:
		if parameter == 0x20 {
			return 3
		}
		return 2
	case 0x95:
		// A macro definition. TR-B14 does not use them, so the decoder skips it to its end code.
		if end := indexOf(b[i+1:], 0x95, 0x4F); end >= 0 {
			return end + 3
		}
		return len(b) - i
	case 0x98:
		d.repeat = parameter & 0x3F
		if d.repeat == 0 {
			d.repeat = -1
		}
		return 2
	}
	return 1
}

// setColor runs COL: the palette with 0x20, else a color of the palette.
func (d *Decoder) setColor(b []byte, i int) int {
	parameter := at(b, i+1)
	if parameter == 0x20 {
		d.palette = at(b, i+2) & 0x07
		return 3
	}
	if parameter&0xF0 == 0x40 {
		d.color = color(d.palette, parameter&0x0F)
	}
	return 2
}

// timeControl runs TIME. The wait with 0x20 takes a snapshot first, and the other forms are skipped.
func (d *Decoder) timeControl(b []byte, i int) int {
	switch at(b, i+1) {
	case 0x20:
		d.snapshot()
		d.time += int64(at(b, i+2)&0x3F) * tenthSecond
		return 3
	case 0x28:
		return 3
	}
	end := i + 2
	for end < len(b) && (b[end] < 0x40 || b[end] > 0x7E) {
		end++
	}
	return min(end+1, len(b)) - i
}

func indexOf(b []byte, x, y byte) int {
	for i := range len(b) - 1 {
		if b[i] == x && b[i+1] == y {
			return i
		}
	}
	return -1
}

// csi reads a control sequence: decimal parameters split by 0x3B, then 0x20 and the final byte.
func (d *Decoder) csi(b []byte, i int) int {
	parameters, end := csiParameters(b, i)
	if end+1 >= len(b) {
		return len(b) - i
	}
	d.applyCSI(b[end+1], parameters)
	return end + 2 - i
}

// csiParameters returns the numbers of a control sequence and the index of the 0x20 that ends them.
func csiParameters(b []byte, i int) ([]int, int) {
	var parameters []int
	value := 0
	end := i
	for ; end < len(b) && b[end] != ' '; end++ {
		switch c := b[end]; {
		case c == 0x3B:
			parameters = append(parameters, value)
			value = 0
		case c >= 0x30 && c <= 0x39:
			value = min(value*10+int(c)-0x30, maxParameter)
		}
	}
	return append(parameters, value), end
}

func (d *Decoder) applyCSI(final byte, parameters []int) {
	first, second := parameters[0], 0
	if len(parameters) > 1 {
		second = parameters[1]
	}
	switch final {
	case 0x53:
		if len(parameters) == 1 {
			d.format = first
		}
		d.resetFormat()
	case 0x56:
		d.areaWidth, d.areaHeight = first, second
	case 0x57:
		d.charWidth, d.charHeight = first, second
	case 0x58:
		d.spacingX = first
	case 0x59:
		d.spacingY = first
	case 0x5F:
		d.displayPosition(parameters)
	case 0x61:
		d.x, d.y, d.positioned = first, second, true
	}
}

// displayPosition runs SDP. It also positions the cursor at 0, 0 when nothing did before.
func (d *Decoder) displayPosition(parameters []int) {
	d.areaX = parameters[0]
	if len(parameters) > 1 {
		d.areaY = parameters[1]
	}
	if !d.positioned {
		d.setPosition(0, 0)
	}
}

func (d *Decoder) graphic(b []byte, i, set int) int {
	count := 1
	if twoByteSet(set) {
		count = 2
	}
	if i+count > len(b) {
		return len(b) - i
	}
	first, second := int(b[i]&0x7F), 0
	if count == 2 {
		second = int(b[i+1] & 0x7F)
	}
	d.writeGraphic(set, first, second, count)
	return count
}

func (d *Decoder) writeGraphic(set, first, second, count int) {
	switch {
	case set == setMacro:
		d.macro(first)
	case set >= setDRCS:
		code := first
		if count == 2 {
			code = first<<8 | second
		}
		d.put(d.drcs(set<<16 | code))
	case first >= 0x21 && first <= 0x7E && (count == 1 || second >= 0x21 && second <= 0x7E):
		d.put(character(set, first, second, d.halfWidth(), d.encoding == encLatin))
	}
}

func (d *Decoder) macro(code int) {
	if code >= 0x60 && code <= 0x6F {
		d.text([]byte(defaultMacros[code-0x60]))
	}
}

func (d *Decoder) utf8(b []byte, i int) int {
	r, n := utf8.DecodeRune(b[i:])
	switch {
	case r == utf8.RuneError && n == 1:
		d.put(replacement)
		return 1
	case r >= 0xEC00 && r <= 0xF8FF:
		d.put(d.drcs(setDRCS0<<16 | int(r)))
	default:
		d.put(string(r))
	}
	return n
}

func (d *Decoder) drcs(key int) string {
	g, found := d.glyphs[key]
	if found && g.character != "" {
		return g.character
	}
	set := key>>16 - setDRCS0
	code := fmt.Sprintf("0x%02X", key&0xFFFF)
	if set == 0 {
		code = fmt.Sprintf("0x%04X", key&0xFFFF)
	}
	id := fmt.Sprintf("DRCS-%d %s", set, code)
	message := fmt.Sprintf("The character %s of the set DRCS-%d has no glyph in the stream. The reader wrote U+3013 for it.", code, set)
	if found {
		id = g.hash
		message = fmt.Sprintf("The glyph of the character %s of the set DRCS-%d (%s dots, MD5 %s) has no known character. The reader wrote U+3013 for it.",
			code, set, g.size, g.hash)
	}
	if !d.warned[id] {
		d.warned[id] = true
		d.warnings = append(d.warnings, Warning{Time: d.time, Message: message})
	}
	return "\u3013"
}

func (d *Decoder) put(text string) {
	if text == "" {
		return
	}
	if !d.positioned {
		d.setPosition(0, 0)
	}
	ruby := d.encoding == encJapanese &&
		(d.scaleX == 0.5 && d.scaleY == 0.5 || d.charWidth == 18 && d.charHeight == 18)
	for range d.takeRepeat() {
		d.store(cell{d.x, d.y - d.sectionHeight(), d.sectionWidth(), d.sectionHeight(), ruby, d.color, text})
		d.move(1, 0)
	}
}

// takeRepeat returns how many cells the next character fills, and ends the repeat.
func (d *Decoder) takeRepeat() int {
	times := 1
	switch {
	case d.repeat > 0:
		times = d.repeat
	case d.repeat < 0:
		times = min(maxRepeat, max(1, (d.areaX+d.areaWidth-d.x)/max(1, d.sectionWidth())))
	}
	d.repeat = 0
	return times
}

// store puts c at its cell. A character past maxCells is dropped with one warning.
func (d *Decoder) store(c cell) {
	key := position{c.x, c.y}
	if i, ok := d.cellAt[key]; ok {
		d.cells[i] = c
		return
	}
	if len(d.cells) >= maxCells {
		d.skip("cells", fmt.Sprintf("The caption screen holds more than %d characters. The reader dropped the characters past that.", maxCells))
		return
	}
	d.cellAt[key] = len(d.cells)
	d.cells = append(d.cells, c)
}

func (d *Decoder) halfWidth() bool { return d.scaleX*2 == d.scaleY }

func (d *Decoder) sectionWidth() int {
	return int(math.Floor(float64(d.charWidth+d.spacingX) * d.scaleX))
}

func (d *Decoder) sectionHeight() int {
	return int(math.Floor(float64(d.charHeight+d.spacingY) * d.scaleY))
}

func (d *Decoder) setPosition(column, row int) {
	d.x = d.areaX + column*d.sectionWidth()
	d.y = d.areaY + (row+1)*d.sectionHeight()
	d.positioned = true
}

func (d *Decoder) newline() {
	if !d.positioned {
		d.setPosition(0, 0)
	}
	d.x = d.areaX
	d.y += d.sectionHeight()
}

// move moves the active position by whole cells. It wraps at the edges of the display area, as STD-B24 section 7.2.5 says.
func (d *Decoder) move(columns, rows int) {
	if !d.positioned {
		d.setPosition(0, 0)
	}
	width := max(1, d.sectionWidth())
	height := max(1, d.sectionHeight())
	for ; columns < 0; columns++ {
		d.x -= width
		if d.x < d.areaX {
			d.x = d.areaX + d.areaWidth - width
			rows--
		}
	}
	for ; columns > 0; columns-- {
		d.x += width
		if d.x >= d.areaX+d.areaWidth {
			d.x = d.areaX
			rows++
		}
	}
	for ; rows < 0; rows++ {
		d.y -= height
		if d.y < d.areaY {
			d.y = d.areaY + d.areaHeight
		}
	}
	for ; rows > 0; rows-- {
		d.y += height
		if d.y > d.areaY+d.areaHeight {
			d.y = d.areaY + height
		}
	}
}

// clearScreen clears the screen and resets the decoder, as the CS control does.
func (d *Decoder) clearScreen() {
	d.cells = d.cells[:0]
	clear(d.cellAt)
	d.reset()
}

// reset returns to the state after a clear screen: the default sets, writing format, size and color.
func (d *Decoder) reset() {
	d.resetSets()
	d.resetFormat()
	d.areaX, d.areaY, d.x, d.y, d.positioned = 0, 0, 0, 0, false
	// Latin captions are middle size by default, Japanese captions normal size.
	d.scaleX, d.scaleY = 1, 1
	if d.encoding == encLatin {
		d.scaleX = 0.5
	}
	d.palette, d.color, d.repeat = 0, white, 0
}

func (d *Decoder) resetSets() {
	d.sets = [4]int{setKanji, setAlphanumeric, setHiragana, setMacro}
	if d.encoding == encLatin {
		d.sets = [4]int{setAlphanumeric, setAlphanumeric, setLatinExtension, setLatinSpecial}
	}
	d.left, d.right = 0, 2
}

// resetFormat sets the plane, the display area and the character size of the writing format of SWF, as ARIB TR-B14
// profile A gives them.
func (d *Decoder) resetFormat() {
	width, height, spacingX, spacingY := 960, 540, 4, 24
	switch d.format {
	case 5:
		width, height = 1920, 1080
	case 8:
		spacingX = 12
	case 9:
		width, height, spacingY = 720, 480, 16
	case 10:
		width, height, spacingX = 720, 480, 8
	}
	d.planeWidth, d.planeHeight, d.areaWidth, d.areaHeight = width, height, width, height
	d.charWidth, d.charHeight = 36, 36
	d.spacingX, d.spacingY = spacingX, spacingY
	if d.encoding == encLatin {
		d.spacingX, d.spacingY = 2, 16
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

package cea608

import (
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

// CodeLine is the start frame at 29.97 fps and the words of one SCC line, or of one run of MCC pairs.
type CodeLine struct {
	Start int64
	Words []uint16
}

// Mode is the caption mode that shows a state.
type Mode string

// The caption modes. They are the values of "mode" in the scc format data of a cue.
const (
	PopOn   Mode = "pop-on"
	RollUp  Mode = "roll-up"
	PaintOn Mode = "paint-on"
	text    Mode = "text"
)

// Line is one row of the displayed captions as core markup.
type Line struct {
	Row    int // 1 to 15
	Column int // 0 to 31, the first non-blank cell
	Text   string
}

// State is the displayed captions after a display change.
type State struct {
	Frame int64
	Lines []Line // empty after an erase
	Mode  Mode
	// line is the index of the code line that made the state.
	line int
	// direct is true for a paint-on or roll-up change.
	direct bool
}

// RollUpLine is one row of roll-up captions from its first character until it rolls up or leaves the screen.
type RollUpLine struct {
	Start  int64
	End    int64 // valid when HasEnd
	HasEnd bool  // false for a row still on the screen at the end of the data
	Line
}

type change int

const (
	noChange change = iota
	directChange
	replaceChange
)

type attributes struct {
	color     int
	italic    bool
	underline bool
}

type cell struct {
	r rune
	attributes
}

// memory maps a row to a column to a cell.
type memory map[int]map[int]cell

// Decoder decodes the words of one data channel into the displayed captions.
type Decoder struct {
	channel int

	displayed    memory
	nonDisplayed memory
	mode         Mode
	row          int
	column       int // 32 means that the last character went to the last column
	attrs        attributes
	rollUpRows   int
	active       int
	inXDS        bool
	lastControl  int // -1 for none
	change       change
	frame        int64

	hasRollUpStart bool
	rollUpStart    int64
	rollUpRow      int
	rollUpLines    []RollUpLine
}

// NewDecoder returns a decoder of one data channel. Channel 1 reads CC1 and CC3, channel 2 reads CC2 and CC4.
func NewDecoder(channel int) *Decoder {
	return &Decoder{
		channel:      channel,
		displayed:    memory{},
		nonDisplayed: memory{},
		mode:         PopOn,
		row:          Rows,
		active:       1,
		lastControl:  -1,
		rollUpRow:    Rows,
	}
}

// Decode returns the displayed captions at each change, in time order.
func (d *Decoder) Decode(lines []CodeLine) []State {
	var states []State
	d.frame = 0
	d.rollUpLines = nil
	for i, line := range lines {
		if line.Start > d.frame {
			d.lastControl = -1
		}
		for k, w := range line.Words {
			d.frame = max(d.frame, line.Start+int64(k))
			d.decodeWord(byte(w>>8), byte(w))
			if d.change != noChange {
				states = d.recordState(states, i)
				d.change = noChange
			}
		}
		d.frame = max(d.frame, line.Start+int64(len(line.Words)))
	}
	d.endRollUpLine(0, false)
	return states
}

// RollUpLines returns each row of roll-up captions that Decode saw, in time order.
func (d *Decoder) RollUpLines() []RollUpLine {
	return d.rollUpLines
}

// decodeWord applies the parity and redundancy rules of 47 CFR 15.119 (i) and (j).
func (d *Decoder) decodeWord(first, second byte) {
	code := first & 0x7F
	if code < 0x10 || code > 0x1F {
		d.lastControl = -1
		d.decodeCharacterPair(first, second)
		return
	}
	word := int(first)<<8 | int(second)
	if !HasOddParity(first) {
		redundant := d.lastControl >= 0 && byte(d.lastControl) == second
		d.lastControl = -1
		if !redundant {
			d.decodeCharacterPair(0x80, second)
		}
		return
	}
	if !HasOddParity(second) || d.lastControl == word {
		d.lastControl = -1
		return
	}
	d.lastControl = word
	d.inXDS = false
	d.active = 1
	if code&0x08 != 0 {
		d.active = 2
	}
	if d.active == d.channel {
		d.decodeControl(code&0x17, second&0x7F)
	}
}

func (d *Decoder) decodeCharacterPair(first, second byte) {
	code := first & 0x7F
	if code > 0x00 && code < 0x10 && HasOddParity(first) {
		d.inXDS = code != 0x0F
		return
	}
	if d.inXDS || d.active != d.channel || d.mode == text {
		return
	}
	for _, b := range [...]byte{first, second} {
		if !HasOddParity(b) {
			continue
		}
		if r, ok := standardCharacter(b & 0x7F); ok {
			d.writeCharacter(r, true)
		}
	}
}

func (d *Decoder) decodeControl(first, second byte) {
	if (first == 0x14 || first == 0x15) && second <= 0x2F {
		d.decodeCommand(second)
		return
	}
	if d.mode == text {
		return
	}
	if p, ok := decodePac(first, second); ok {
		d.applyPac(p)
		return
	}
	d.decodeCharacterCode(first, second)
}

func (d *Decoder) decodeCharacterCode(first, second byte) {
	switch first {
	case firstMidRow:
		d.decodeMidRowOrSpecial(second)
	case 0x12, 0x13:
		// An extended character replaces the standard character that goes before it for older decoders.
		if r, ok := extendedCharacter(first, second); ok {
			d.column = max(0, d.column-1)
			d.writeCharacter(r, true)
		}
	case firstTabOffset:
		if second >= 0x21 && second <= 0x23 {
			d.column = min(Columns-1, d.column+int(second)-tabOffsetBase)
		}
	}
}

func (d *Decoder) decodeMidRowOrSpecial(second byte) {
	if second&0xF0 == 0x20 {
		style, underline := decodeMidRow(second)
		if style != styleItalic {
			d.attrs.color = style
		}
		d.attrs.italic = style == styleItalic
		d.attrs.underline = underline
		d.writeCharacter(' ', true)
		return
	}
	if r, ok := specialCharacter(second); ok {
		d.writeCharacter(r, second != transparentSpace)
	}
}

func (d *Decoder) decodeCommand(command byte) {
	switch command {
	case cmdRU2, cmdRU3, cmdRU4:
		d.startRollUp(int(command-cmdRU2) + 2)
	case cmdBS:
		if d.mode != text && d.column > 0 {
			d.column--
			d.setCell(0, false)
		}
	case cmdDER:
		d.deleteToEndOfRow()
	case cmdCR:
		if d.mode == RollUp {
			d.endRollUpLine(d.frame, true)
			d.rollUp()
		}
	default:
		d.decodeModeOrMemoryCommand(command)
	}
}

func (d *Decoder) decodeModeOrMemoryCommand(command byte) {
	switch command {
	case cmdRCL:
		d.mode = PopOn
	case cmdRDC:
		d.mode = PaintOn
	case cmdTR, cmdRTD:
		d.mode = text
	case cmdEDM:
		d.endRollUpLine(d.frame, true)
		d.displayed = memory{}
		d.change = replaceChange
	case cmdENM:
		d.nonDisplayed = memory{}
	case cmdEOC:
		d.endRollUpLine(d.frame, true)
		d.displayed, d.nonDisplayed = d.nonDisplayed, d.displayed
		d.mode = PopOn
		d.change = replaceChange
	}
}

func (d *Decoder) deleteToEndOfRow() {
	if d.mode == text {
		return
	}
	cells := d.target()[d.row]
	for column := range cells {
		if column >= d.column {
			delete(cells, column)
		}
	}
	d.markDirectChange()
}

func (d *Decoder) startRollUp(rows int) {
	if d.mode != RollUp {
		d.endRollUpLine(d.frame, true)
		d.displayed = memory{}
		d.nonDisplayed = memory{}
		d.row = Rows
		d.change = replaceChange
	}
	d.mode = RollUp
	d.rollUpRows = rows
	d.column = 0
	d.attrs = attributes{}
	for row := range d.displayed {
		if row > d.row || row <= d.row-rows {
			delete(d.displayed, row)
			d.markDirectChange()
		}
	}
}

func (d *Decoder) rollUp() {
	rolled := memory{}
	for row := max(1, d.row-d.rollUpRows+1); row < d.row; row++ {
		if cells, ok := d.displayed[row+1]; ok {
			rolled[row] = cells
		}
	}
	d.displayed = rolled
	d.column = 0
	d.attrs = attributes{}
	d.markDirectChange()
}

func (d *Decoder) applyPac(p pac) {
	if d.mode == RollUp && p.row != d.row && len(d.displayed) > 0 {
		moved := memory{}
		for row, cells := range d.displayed {
			if newRow := row + p.row - d.row; newRow >= 1 && newRow <= Rows {
				moved[newRow] = cells
			}
		}
		d.displayed = moved
		d.rollUpRow += p.row - d.row
		d.markDirectChange()
	}
	d.row = p.row
	d.column = p.column
	d.attrs = attributes{color: p.color, italic: p.italic, underline: p.underline}
}

// writeCharacter writes r, or clears the cell when visible is false.
func (d *Decoder) writeCharacter(r rune, visible bool) {
	if d.mode == text {
		return
	}
	d.column = min(d.column, Columns-1)
	d.setCell(r, visible)
	d.column++
}

func (d *Decoder) setCell(r rune, visible bool) {
	if visible && d.mode == RollUp {
		if !d.hasRollUpStart {
			d.hasRollUpStart = true
			d.rollUpStart = d.frame
		}
		d.rollUpRow = d.row
	}
	mem := d.target()
	if !visible {
		delete(mem[d.row], d.column)
	} else {
		if mem[d.row] == nil {
			mem[d.row] = map[int]cell{}
		}
		mem[d.row][d.column] = cell{r: r, attributes: d.attrs}
	}
	d.markDirectChange()
}

func (d *Decoder) target() memory {
	if d.mode == PopOn {
		return d.nonDisplayed
	}
	return d.displayed
}

func (d *Decoder) markDirectChange() {
	if d.mode != PopOn && d.change == noChange {
		d.change = directChange
	}
}

// recordState adds the displayed captions as a new state.
// Paint-on and roll-up data on one code line give one state.
func (d *Decoder) recordState(states []State, lineIndex int) []State {
	lines := d.renderDisplayed()
	mode := d.mode
	if mode == text {
		mode = PopOn
	}
	if len(states) == 0 {
		return append(states, State{Frame: d.frame, Lines: lines, Mode: mode, line: lineIndex, direct: d.change == directChange})
	}
	last := &states[len(states)-1]
	switch {
	case slices.Equal(last.Lines, lines):
	case d.change == directChange && last.direct && last.line == lineIndex && len(last.Lines) > 0 && len(lines) > 0:
		last.Lines = lines
		last.Mode = mode
	default:
		states = append(states, State{Frame: d.frame, Lines: lines, Mode: mode, line: lineIndex, direct: d.change == directChange})
	}
	return states
}

func (d *Decoder) renderDisplayed() []Line {
	rows := make([]int, 0, len(d.displayed))
	for row := range d.displayed {
		rows = append(rows, row)
	}
	slices.Sort(rows)
	var lines []Line
	for _, row := range rows {
		if column, markup, ok := renderRow(d.displayed[row]); ok {
			lines = append(lines, Line{Row: row, Column: column, Text: markup})
		}
	}
	return lines
}

// renderRow returns the column of the first cell that is not a space and the markup of the row from there to
// the last such cell. Missing cells inside are white spaces. ok is false for a row without such a cell.
func renderRow(cells map[int]cell) (column int, markup string, ok bool) {
	first, last := -1, -1
	for column, c := range cells {
		if c.r == ' ' {
			continue
		}
		if first < 0 || column < first {
			first = column
		}
		last = max(last, column)
	}
	if first < 0 {
		return 0, "", false
	}
	runs := make([]markupcore.Run, 0, last-first+1)
	for column := first; column <= last; column++ {
		c, found := cells[column]
		if !found {
			c = cell{r: ' '}
		}
		run := markupcore.Run{Text: string(c.r), Italic: c.italic, Underline: c.underline}
		if c.color != white {
			run.Color = colors[c.color]
		}
		runs = append(runs, run)
	}
	return first, markupcore.StyleRuns(runs, true), true
}

// endRollUpLine adds the base row of roll-up captions as a roll-up line that ends at end, if a character went to it.
func (d *Decoder) endRollUpLine(end int64, hasEnd bool) {
	if !d.hasRollUpStart {
		return
	}
	start := d.rollUpStart
	d.hasRollUpStart = false
	for _, line := range d.renderDisplayed() {
		if line.Row == d.rollUpRow {
			d.rollUpLines = append(d.rollUpLines, RollUpLine{Start: start, End: end, HasEnd: hasEnd, Line: line})
		}
	}
}

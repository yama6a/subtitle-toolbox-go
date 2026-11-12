package cea608

import (
	"regexp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
)

// Char is one character of a line with its style.
type Char struct {
	R         rune
	Color     int // 0 to 6, see the color indexes of preamble address codes
	Italic    bool
	Underline bool
}

// Cell is one screen cell of an encoded row: a character, or a mid-row code that shows as a space.
type Cell struct {
	R      rune
	MidRow byte // second byte of the mid-row code, 0 for a character
}

// code holds the 7-bit codes of a character. An extended character has a pair and the byte of the standard
// character that goes before the pair.
type code struct {
	hasByte, hasPair bool
	b                byte
	pair             [2]byte
}

var characterCodes = buildCharacterCodes() //nolint:gochecknoglobals // lookup built once from the tables

// buildCharacterCodes adds the extended sets, then the special set, then the standard set. A later entry wins,
// so a space is 0x20 and not the transparent space.
func buildCharacterCodes() map[rune]code {
	m := map[rune]code{}
	for set, runes := range extendedSets {
		first := byte(0x12 + set)
		for i, r := range runes {
			m[r] = code{hasByte: true, b: extendedFallbacks[set][i], hasPair: true, pair: [2]byte{first, byte(0x20 + i)}}
		}
	}
	for i, r := range specialSet {
		m[r] = code{hasPair: true, pair: [2]byte{firstMidRow, byte(0x30 + i)}}
	}
	for c := byte(0x20); c <= 0x7E; c++ {
		r, _ := standardCharacter(c)
		m[r] = code{hasByte: true, b: c}
	}
	return m
}

// Encodable reports whether CEA-608 has a code for r.
func Encodable(r rune) bool {
	_, ok := characterCodes[r]
	return ok
}

var spaces = strings.NewReplacer("\u00a0", " ", "\t", " ") //nolint:gochecknoglobals // constant table

var tagRegexp = regexp.MustCompile(`(?i)^<\s*(/?)\s*([a-z]+)\b([^>]*)>$`)

type lineStyle struct {
	italic, underline int
	colors            []int
}

func (s *lineStyle) top() int {
	if len(s.colors) == 0 {
		return white
	}
	return s.colors[len(s.colors)-1]
}

func (s *lineStyle) apply(name string, closing bool, attrs string) {
	delta := 1
	if closing {
		delta = -1
	}
	switch name {
	case "i":
		s.italic = max(0, s.italic+delta)
	case "u":
		s.underline = max(0, s.underline+delta)
	case "font":
		s.applyFont(closing, attrs)
	}
}

func (s *lineStyle) applyFont(closing bool, attrs string) {
	if closing {
		if len(s.colors) > 0 {
			s.colors = s.colors[:len(s.colors)-1]
		}
		return
	}
	color := s.top()
	if c, ok := markupcore.FontColor(attrs); ok {
		if hex, ok := markupcore.ColorHex(c); ok {
			if i := slices.Index(colors[:], hex); i >= 0 {
				color = i
			}
		}
	}
	s.colors = append(s.colors, color)
}

// StyledCharacters splits a line of core markup into characters with their color, italics and underline.
// The spaces at both ends of the line drop.
func StyledCharacters(line string) []Char {
	var style lineStyle
	var chars []Char
	for i, part := range markupcore.SplitTags(markupcore.WebVTTColorsToFont(line)) {
		if i%2 == 1 {
			if m := tagRegexp.FindStringSubmatch(part); m != nil {
				style.apply(markupcore.ASCIILower(m[2]), m[1] == "/", m[3])
			}
			continue
		}
		decoded := spaces.Replace(markupcore.DecodeEntities(part))
		for _, r := range decoded {
			chars = append(chars, Char{R: r, Color: style.top(), Italic: style.italic > 0, Underline: style.underline > 0})
		}
	}
	start, end := 0, len(chars)
	for start < end && chars[start].R == ' ' {
		start++
	}
	for end > start && chars[end-1].R == ' ' {
		end--
	}
	return chars[start:end]
}

func (c Char) attrs() attributes {
	return attributes{c.Color, c.Italic, c.Underline}
}

// Cells turns the characters into cells. A style change takes mid-row codes, which show as spaces.
// The codes replace the spaces right before the change, one space per code.
// A line that needs more than 32 cells drops every style.
func Cells(chars []Char) []Cell {
	var cells []Cell
	var current attributes
	for _, c := range chars {
		if want := c.attrs(); c.R != ' ' && want != current {
			cells = appendCodes(cells, midRowCodes(current, want))
			current = want
		}
		cells = append(cells, Cell{R: c.R})
	}
	if len(cells) > Columns {
		cells = cells[:0]
		for _, c := range chars {
			cells = append(cells, Cell{R: c.R})
		}
	}
	return cells
}

// appendCodes appends the mid-row codes, which replace the spaces right before them, one space per code.
func appendCodes(cells []Cell, codes []byte) []Cell {
	for range codes {
		if n := len(cells); n == 0 || cells[n-1].MidRow != 0 || cells[n-1].R != ' ' {
			break
		}
		cells = cells[:len(cells)-1]
	}
	for _, b := range codes {
		cells = append(cells, Cell{MidRow: b})
	}
	return cells
}

// midRowCodes returns the second bytes of the mid-row codes that change the style. A color code turns italics off,
// and the italics code keeps the color, as 47 CFR 15.119 (h)(1)(ii) says.
func midRowCodes(from, to attributes) []byte {
	if to.color != from.color || (from.italic && !to.italic) || (!to.italic && to.underline != from.underline) {
		codes := []byte{midRow(to.color, !to.italic && to.underline)}
		if to.italic {
			codes = append(codes, midRow(styleItalic, to.underline))
		}
		return codes
	}
	return []byte{midRow(styleItalic, to.underline)}
}

// RowWords returns the words of one row: the PAC (preamble address code) twice, a tab offset twice when the
// start column is not a multiple of 4, then the cells. A single standard character before a code gets the
// filler byte 0x80. A character without a code writes nothing, so callers check Encodable first.
// It returns no words for a row outside 1 to 15 or a column outside 0 to 31.
func RowWords(row, column int, cells []Cell) []uint16 {
	if row < 1 || row > Rows || column < 0 || column >= Columns {
		return nil
	}
	p, start, cells := rowPac(row, column, cells)
	first, second := encodePac(p)
	w := Word(first, second)
	words := []uint16{w, w}
	if start%indentStep > 0 {
		tab := Word(firstTabOffset, byte(tabOffsetBase+start%indentStep))
		words = append(words, tab, tab)
	}
	return append(words, cellWords(cells)...)
}

// rowPac returns the PAC, the column that the PAC and the tab offset reach, and the cells after the PAC.
// When the leading mid-row codes do not fit before column, the PAC takes over their style at column 0.
func rowPac(row, column int, cells []Cell) (pac, int, []Cell) {
	leading := 0
	for leading < len(cells) && cells[leading].MidRow != 0 {
		leading++
	}
	if start := column - leading; start >= 0 {
		return pac{row: row, column: start / indentStep * indentStep}, start, cells
	}
	var attrs attributes
	for _, c := range cells[:leading] {
		style, underline := decodeMidRow(c.MidRow)
		if style != styleItalic {
			attrs.color = style
		}
		attrs.italic = style == styleItalic
		attrs.underline = underline
	}
	cells = cells[leading:]
	if attrs.italic && attrs.color != white {
		cells = append([]Cell{{MidRow: midRow(styleItalic, attrs.underline)}}, cells...)
		return pac{row: row, color: attrs.color}, 0, cells
	}
	return pac{row: row, color: attrs.color, italic: attrs.italic, underline: attrs.underline}, 0, cells
}

func cellWords(cells []Cell) []uint16 {
	var words []uint16
	pending, hasPending := byte(0), false
	for _, c := range cells {
		var cd code
		if c.MidRow != 0 {
			cd = code{hasPair: true, pair: [2]byte{firstMidRow, c.MidRow}}
		} else {
			cd = characterCodes[c.R]
		}
		if cd.hasByte {
			if hasPending {
				words = append(words, Word(pending, cd.b))
				hasPending = false
			} else {
				pending, hasPending = cd.b, true
			}
		}
		if cd.hasPair {
			if hasPending {
				words = append(words, Word(pending, 0))
				hasPending = false
			}
			w := Word(cd.pair[0], cd.pair[1])
			words = append(words, w, w)
		}
	}
	if hasPending {
		words = append(words, Word(pending, 0))
	}
	return words
}

// The commands that load a pop-on caption on data channel 1.
const (
	CommandRCL = cmdRCL
	CommandENM = cmdENM
)

// ControlWord returns the control word of a command on data channel 1.
func ControlWord(command byte) uint16 {
	return Word(firstControl, command)
}

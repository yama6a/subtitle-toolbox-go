package cea608

import "math/bits"

const (
	// Rows is the number of rows of the caption screen.
	Rows = 15
	// Columns is the number of columns of the caption screen.
	Columns = 32
	// MaxLines is the number of rows that a caption shows at once.
	MaxLines = 4
)

// white is the color index of white. Color indexes 0 to 6 select the entries of colors.
const white = 0

const (
	cmdRCL = 0x20 // resume caption loading
	cmdBS  = 0x21 // backspace
	cmdDER = 0x24 // delete to end of row
	cmdRU2 = 0x25
	cmdRU3 = 0x26
	cmdRU4 = 0x27
	cmdRDC = 0x29 // resume direct captioning
	cmdTR  = 0x2A // text restart
	cmdRTD = 0x2B // resume text display
	cmdEDM = 0x2C // erase displayed memory
	cmdCR  = 0x2D // carriage return
	cmdENM = 0x2E // erase non-displayed memory
	cmdEOC = 0x2F // end of caption
)

const (
	firstMidRow    = 0x11
	firstTabOffset = 0x17
	tabOffsetBase  = 0x20
	indentStep     = 4
	styleItalic    = 7
)

// colors holds the "#rrggbb" value of each color index of preamble address codes and mid-row codes.
var colors = [...]string{"#ffffff", "#00ff00", "#0000ff", "#00ffff", "#ff0000", "#ffff00", "#ff00ff"} //nolint:gochecknoglobals // constant table

// standardOverrides holds the characters of the standard set that differ from ASCII.
var standardOverrides = map[byte]rune{ //nolint:gochecknoglobals // constant table
	0x2A: 'á', 0x5C: 'é', 0x5E: 'í', 0x5F: 'ó', 0x60: 'ú',
	0x7B: 'ç', 0x7C: '\u00f7', 0x7D: 'Ñ', 0x7E: 'ñ',
}

// specialSet holds the second bytes 0x30 to 0x3F after 0x11. Byte 0x39 is the transparent space.
var specialSet = []rune("\u00ae\u00b0\u00bd\u00bf\u2122\u00a2\u00a3\u266aà èâêîôû") //nolint:gochecknoglobals // constant table

const transparentSpace = 0x39

// extendedSets holds the second bytes 0x20 to 0x3F after 0x12 and after 0x13.
var extendedSets = [2][]rune{ //nolint:gochecknoglobals // constant table
	[]rune("ÁÉÓÚÜü\u2018\u00a1*\u2019\u2014\u00a9\u2120\u2022\u201c\u201dÀÂÇÈÊËëÎÏïÔÙùÛ\u00ab\u00bb"),
	[]rune("ÃãÍÌìÒòÕõ{}\\^_\u00a6~ÄäÖöß\u00a5\u00a4|ÅåØø\u250c\u2510\u2514\u2518"),
}

// extendedFallbacks holds the standard character that a decoder without extended characters keeps,
// from the CCD column of the McPoodle table.
var extendedFallbacks = [2]string{ //nolint:gochecknoglobals // constant table
	"AEOUUu'!#'-cs.\"\"AACEEEeIIiOUuU\"\"",
	"AaIIiOoOo[]//---AaOosYC/AaOo++++",
}

// pacRows holds the rows of a preamble address code by the low 3 bits of its first byte and bit 0x20 of its second byte. 0 means none.
var pacRows = [8][2]int{ //nolint:gochecknoglobals // constant table
	{11, 0}, {1, 2}, {3, 4}, {12, 13}, {14, 15}, {5, 6}, {7, 8}, {9, 10},
}

// HasOddParity reports whether b has an odd number of set bits.
func HasOddParity(b byte) bool {
	return bits.OnesCount8(b)%2 == 1
}

// WithParity sets bit 7 of the 7-bit value b so that the byte has odd parity.
func WithParity(b byte) byte {
	b &= 0x7F
	if HasOddParity(b) {
		return b
	}
	return b | 0x80
}

// Word returns both bytes with parity, the first byte in the high half.
func Word(first, second byte) uint16 {
	return uint16(WithParity(first))<<8 | uint16(WithParity(second))
}

func standardCharacter(code byte) (rune, bool) {
	if code < 0x20 || code > 0x7E {
		return 0, false
	}
	if r, ok := standardOverrides[code]; ok {
		return r, true
	}
	return rune(code), true
}

func specialCharacter(second byte) (rune, bool) {
	if second < 0x30 || int(second-0x30) >= len(specialSet) {
		return 0, false
	}
	return specialSet[second-0x30], true
}

func extendedCharacter(first, second byte) (rune, bool) {
	if first < 0x12 || first > 0x13 || second < 0x20 || second > 0x3F {
		return 0, false
	}
	return extendedSets[first-0x12][second-0x20], true
}

type pac struct {
	row, column, color int
	italic, underline  bool
}

func decodePac(first, second byte) (pac, bool) {
	if first < 0x10 || first > 0x17 || second < 0x40 || second > 0x7F {
		return pac{}, false
	}
	half := 0
	if second&0x20 != 0 {
		half = 1
	}
	row := pacRows[first&0x07][half]
	if row == 0 {
		return pac{}, false
	}
	attributes := second & 0x1F
	underline := attributes&0x01 == 1
	if attributes >= 0x10 {
		return pac{row: row, column: int(attributes&0x0E>>1) * indentStep, underline: underline}, true
	}
	style := int(attributes >> 1)
	if style == styleItalic {
		return pac{row: row, italic: true, underline: underline}, true
	}
	return pac{row: row, color: style, underline: underline}, true
}

// encodePac returns the 7-bit bytes of the preamble address code for a row from 1 to 15.
// Italics wins over a color. Only column 0 holds a color or italics.
func encodePac(p pac) (first, second byte) {
	for low, rows := range pacRows {
		for half, row := range rows {
			if row != p.row {
				continue
			}
			var attributes byte
			switch {
			case p.italic:
				attributes = 0x0E
			case p.color != white:
				attributes = byte(p.color) << 1
			default:
				attributes = 0x10 | byte(p.column/indentStep)<<1&0x0E
			}
			second = 0x40 | byte(half)<<5 | attributes
			if p.underline {
				second |= 0x01
			}
			return 0x10 | byte(low), second
		}
	}
	return 0, 0
}

// midRow returns the second byte of the mid-row code for a color index, or for italics with styleItalic.
func midRow(style int, underline bool) byte {
	b := 0x20 | byte(style)<<1
	if underline {
		b |= 0x01
	}
	return b
}

// decodeMidRow returns the style of a mid-row code. The italics code has no color of its own.
func decodeMidRow(second byte) (style int, underline bool) {
	return int(second & 0x0E >> 1), second&0x01 == 1
}

// Color returns the "#rrggbb" value of a color index 0 to 6 of a preamble address code or a mid-row code.
func Color(index int) string {
	return colors[index]
}

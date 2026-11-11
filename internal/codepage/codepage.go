// Package codepage holds the single-byte code pages of EBU Tech 3264 and the ISO 6937 codec.
// Bytes below 0x80 are ASCII in every table.
package codepage

// Replacement is the byte that every encoder writes for a character outside its table.
const Replacement = '?'

// Table is a single-byte code page.
type Table struct {
	upper   [128]rune
	reverse map[rune]byte
}

func newTable(upper [128]rune) *Table {
	t := &Table{upper: upper, reverse: make(map[rune]byte, len(upper))}
	for i, r := range upper {
		if r != 0 {
			t.reverse[r] = byte(0x80 + i)
		}
	}
	return t
}

// The code pages of EBU Tech 3264. The ISO 8859 names follow x/text charmap.
//
//nolint:gochecknoglobals,revive // fixed code page tables
var (
	// ISO8859_5 is Cyrillic, character code table 01.
	ISO8859_5 = newTable(iso8859_5Upper)
	// ISO8859_6 is Arabic, character code table 02.
	ISO8859_6 = newTable(iso8859_6Upper)
	// ISO8859_7_1987 is the 1987 edition of Greek, character code table 03.
	ISO8859_7_1987 = newTable(iso8859_7_1987Upper)
	// ISO8859_8_1988 is the 1988 edition of Hebrew, character code table 04.
	ISO8859_8_1988 = newTable(iso8859_8_1988Upper)
	// CP437 is DOS code page 437.
	CP437 = newTable(cp437Upper)
	// CP850 is DOS code page 850.
	CP850 = newTable(cp850Upper)
	// CP860 is DOS code page 860.
	CP860 = newTable(cp860Upper)
	// CP863 is DOS code page 863.
	CP863 = newTable(cp863Upper)
	// CP865 is DOS code page 865.
	CP865 = newTable(cp865Upper)
)

// Decode returns the text of b. It drops each byte without an entry.
func (t *Table) Decode(b []byte) string {
	out := make([]rune, 0, len(b))
	for _, c := range b {
		switch {
		case c < 0x80:
			out = append(out, rune(c))
		case t.upper[c-0x80] != 0:
			out = append(out, t.upper[c-0x80])
		}
	}
	return string(out)
}

// Encode returns the bytes of s and writes Replacement for each code point outside the table.
func (t *Table) Encode(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		c, ok := t.EncodeRune(r)
		if !ok {
			c = Replacement
		}
		out = append(out, c)
	}
	return out
}

// EncodeRune returns the byte of r, or false when r is outside the table.
func (t *Table) EncodeRune(r rune) (byte, bool) {
	if r >= 0 && r < 0x80 {
		return byte(r), true
	}
	c, ok := t.reverse[r]
	return c, ok
}

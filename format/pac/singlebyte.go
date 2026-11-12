package pac

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

// table is the character table of a single-byte code page, by byte code.
type table struct {
	chars   map[uint32]string
	reverse map[string]uint32
}

// newTable merges the entry lists. The first list wins for a code that two lists hold.
func newTable(lists ...[]entry) *table {
	t := &table{chars: map[uint32]string{}, reverse: map[string]uint32{}}
	var order []entry
	for _, list := range lists {
		for _, e := range list {
			if _, ok := t.chars[e.code]; !ok {
				order = append(order, e)
				t.chars[e.code] = e.text
			}
		}
	}
	// The first code of a character wins, as in the writer of Subtitle Edit.
	for _, e := range order {
		if _, ok := t.reverse[t.chars[e.code]]; !ok {
			t.reverse[t.chars[e.code]] = e.code
		}
	}
	return t
}

//nolint:gochecknoglobals // tables built once from the fixed entries
var (
	latinTable    = sync.OnceValue(func() *table { return newTable(latinEntries) })
	turkishTable  = sync.OnceValue(func() *table { return newTable(turkishEntries, latinEntries) })
	greekTable    = sync.OnceValue(func() *table { return newTable(greekEntries) })
	cyrillicTable = sync.OnceValue(func() *table { return newTable(cyrillicEntries) })
	arabicTable   = sync.OnceValue(func() *table { return newTable(arabicEntries) })
	hebrewTable   = sync.OnceValue(func() *table { return newTable(hebrewEntries) })
	thaiTable     = sync.OnceValue(func() *table { return newTable(thaiEntries) })
)

// arabicMarks are the vowel marks that the file stores before their letter.
//
//nolint:gochecknoglobals // lookup table of Arabic vowel marks
var arabicMarks = map[uint32]bool{0xE7: true, 0xEA: true, 0xE8: true, 0xE5: true, 0xE4: true, 0xE6: true, 0xE3: true, 0xE9: true, 0xE2: true, 0xE0: true, 0xE1: true}

func (c CodePage) table() *table {
	switch c.singleByte() {
	case LatinTurkish:
		return turkishTable()
	case Greek:
		return greekTable()
	case Cyrillic:
		return cyrillicTable()
	case Arabic:
		return arabicTable()
	case Hebrew:
		return hebrewTable()
	case Thai:
		return thaiTable()
	case CodePageAuto, Latin, LatinCzech, ChineseTraditional, ChineseSimplified, Korean, Japanese, Unicode:
		return latinTable()
	}
	return latinTable()
}

// code returns the big-endian number of the n bytes of b from i, or false when b is too short.
func code(b []byte, i, n int) (uint32, bool) {
	if i+n > len(b) {
		return 0, false
	}
	var v uint32
	for _, x := range b[i : i+n] {
		v = v<<8 | uint32(x)
	}
	return v, true
}

// lookup returns the text of the code at b[i] and its size in bytes. A Latin code of 2 bytes loses to a code that
// starts at its second byte, so "L" before an accented letter stays "L".
func (c CodePage) lookup(t *table, b []byte, i int) (string, int, bool) {
	if c.isLatin() {
		return c.lookupLatin(t, b, i)
	}
	for _, n := range [...]int{3, 2, 1} {
		if v, ok := code(b, i, n); ok {
			if text, found := t.chars[v]; found {
				return text, n, true
			}
		}
	}
	return "", 1, false
}

func (c CodePage) lookupLatin(t *table, b []byte, i int) (string, int, bool) {
	pair, okPair := code(b, i, 2)
	next, okNext := code(b, i+1, 2)
	if text, found := t.chars[pair]; okPair && found {
		if _, nextFound := t.chars[next]; !okNext || !nextFound {
			return text, 2, true
		}
	}
	one, _ := code(b, i, 1)
	text, found := t.chars[one]
	return text, 1, found
}

// decodeSingleByte decodes single-byte text. It returns the text and the number of bytes from 80h that the code page
// lacks. Control codes, such as the colors 01h to 07h, give no text. FFh is a space.
func (c CodePage) decodeSingleByte(b []byte) (string, int) {
	t := c.table()
	var text strings.Builder
	undefined := 0
	for i := 0; i < len(b); {
		char, size, missing := c.decodeAt(t, b, i)
		text.WriteString(char)
		if missing {
			undefined++
		}
		i += size
	}
	return text.String(), undefined
}

// decodeAt returns the text and the size of the code at b[i], and whether the code page lacks the byte.
func (c CodePage) decodeAt(t *table, b []byte, i int) (string, int, bool) {
	char, size, found := c.lookup(t, b, i)
	if found {
		char, size = c.joinArabicMark(t, b, i, char, size)
		return char, size, false
	}
	ch := b[i]
	return c.withoutEntry(ch), 1, ch >= 0x80 && ch != 0xFF && !c.isLatin()
}

// joinArabicMark puts the letter that follows a vowel mark before the mark.
func (c CodePage) joinArabicMark(t *table, b []byte, i int, char string, size int) (string, int) {
	if c != Arabic || size != 1 || !arabicMarks[uint32(b[i])] || i+1 >= len(b) || b[i+1] < 0x80 {
		return char, size
	}
	if letter, letterSize, ok := c.lookup(t, b, i+1); ok {
		return letter + char, letterSize + 1
	}
	return char, size
}

// withoutEntry returns the text of a byte that the table lacks.
func (c CodePage) withoutEntry(ch byte) string {
	switch {
	case ch == 0xFF:
		return " "
	case ch >= 0x20 && ch < 0x7F:
		return string(rune(ch))
	case ch >= 0xA0 && c.isLatin():
		if r := c.latinCharmap().DecodeByte(ch); r != utf8.RuneError {
			return string(r)
		}
	}
	return ""
}

// encodeSingleByte encodes single-byte text and writes "?" for each character outside the code page.
// An Arabic vowel mark goes before its letter.
func (c CodePage) encodeSingleByte(text string) []byte {
	t := c.table()
	chars := []rune(text)
	var out []byte
	for i := 0; i < len(chars); i++ {
		encoded, used := c.encodeAt(t, chars, i)
		i += used - 1
		if c == Arabic {
			if mark, ok := arabicMarkAt(t, chars, i+1); ok {
				encoded = append(appendCode(nil, mark), encoded...)
				i++
			}
		}
		out = append(out, encoded...)
	}
	return out
}

// encodeAt returns the bytes of the characters at chars[i] and how many characters they take. It matches the
// longest table entry of 3, 2 or 1 characters.
func (c CodePage) encodeAt(t *table, chars []rune, i int) ([]byte, int) {
	for _, size := range [...]int{3, 2, 1} {
		if i+size > len(chars) {
			continue
		}
		if v, ok := t.reverse[string(chars[i:i+size])]; ok {
			return appendCode(nil, v), size
		}
	}
	ch := chars[i]
	if _, inTable := t.chars[uint32(ch)]; ch >= 0x20 && ch < 0x7F && !inTable {
		return []byte{byte(ch)}, 1
	}
	if c.isLatin() {
		return c.latinFallback(ch), 1
	}
	return []byte(replacement), 1
}

// arabicMarkAt returns the code of the Arabic vowel mark at chars[i].
func arabicMarkAt(t *table, chars []rune, i int) (uint32, bool) {
	if i >= len(chars) {
		return 0, false
	}
	v, ok := t.reverse[string(chars[i])]
	return v, ok && arabicMarks[v]
}

// latinFallback returns the byte of ch in ISO-8859-1, -2 or -9 from A0h, else "?".
func (c CodePage) latinFallback(ch rune) []byte {
	if b, ok := c.latinCharmap().EncodeRune(ch); ok && b >= 0xA0 {
		return []byte{b}
	}
	return []byte(replacement)
}

// appendCode appends the bytes of code from the first byte that is not zero.
func appendCode(b []byte, code uint32) []byte {
	started := false
	for shift := 24; shift >= 0; shift -= 8 {
		if v := byte(code >> shift); started || v != 0 {
			b = append(b, v)
			started = true
		}
	}
	return b
}

package markupcore

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// HSpaceClass is the PCRE class \h as a character class of package regexp.
const HSpaceClass = `[\t \x{A0}\x{1680}\x{180E}\x{2000}-\x{200A}\x{202F}\x{205F}\x{3000}]`

// IsHSpace reports whether r is horizontal white space, as the PCRE class \h.
func IsHSpace(r rune) bool {
	switch r {
	case '\t', ' ', 0xA0, 0x1680, 0x180E, 0x202F, 0x205F, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// TextSpan is a byte range of the text of TextRuns.
type TextSpan struct{ Start, End int }

// rawPlace is the place of the raw bytes of one visible byte: the line index, the offset and the length, such as
// 5 for &amp;. An LF between lines has the line -1.
type rawPlace struct{ line, offset, length int }

// TextRuns is the text runs of lines joined with LF, with the place of each byte in the raw lines.
type TextRuns struct {
	Text string
	raw  []rawPlace
}

// TextRunsOf returns the text runs of lines.
func TextRunsOf(lines []string) TextRuns {
	var b strings.Builder
	var raw []rawPlace
	for li, line := range lines {
		if li > 0 {
			b.WriteByte('\n')
			raw = append(raw, rawPlace{line: -1})
		}
		offset := 0
		for k, part := range SplitTags(line) {
			for i := 0; k%2 == 0 && i < len(part); {
				c, n := decodeAt(part[i:])
				b.WriteByte(c)
				raw = append(raw, rawPlace{li, offset + i, n})
				i += n
			}
			offset += len(part)
		}
	}
	return TextRuns{Text: b.String(), raw: raw}
}

func decodeAt(s string) (byte, int) {
	switch {
	case strings.HasPrefix(s, "&lt;"):
		return '<', 4
	case strings.HasPrefix(s, "&gt;"):
		return '>', 4
	case strings.HasPrefix(s, "&amp;"):
		return '&', 5
	}
	return s[0], 1
}

// Cut removes the raw bytes of the spans from lines. Tags between them stay.
func (v TextRuns) Cut(lines []string, spans []TextSpan) []string {
	gone := make([][]bool, len(lines))
	for _, s := range spans {
		for _, p := range v.raw[s.Start:s.End] {
			if p.line < 0 {
				continue
			}
			if gone[p.line] == nil {
				gone[p.line] = make([]bool, len(lines[p.line]))
			}
			for k := range p.length {
				gone[p.line][p.offset+k] = true
			}
		}
	}
	out := slices.Clone(lines)
	for li, g := range gone {
		if g == nil {
			continue
		}
		b := make([]byte, 0, len(g))
		for k, removed := range g {
			if !removed {
				b = append(b, lines[li][k])
			}
		}
		out[li] = string(b)
	}
	return out
}

// SkipHSpace skips the horizontal space at text[p:].
func SkipHSpace(text string, p int) int {
	for p < len(text) {
		r, n := utf8.DecodeRuneInString(text[p:])
		if !IsHSpace(r) {
			break
		}
		p += n
	}
	return p
}

// Package hilabels holds the speaker label rule of the hearing-impaired removal and the helpers that edit the
// visible text of lines. A speaker label is a name such as JOHN: at the start of a line, or after its dialogue dash.
//
// Adapted from Subtitle Edit, src/libse/Forms/RemoveTextForHI.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/5b9ee8baf08c472c7a74fbc337446b656dedabb4/src/libse/Forms/RemoveTextForHI.cs), MIT.
package hilabels

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

// hspace is the PCRE class \h.
const hspace = `[\t \x{A0}\x{1680}\x{180E}\x{2000}-\x{200A}\x{202F}\x{205F}\x{3000}]`

// The PHP upper case pattern starts with a lookahead for an upper case letter. Remove checks the captured
// label for one in its place.
var (
	upperLabel = regexp.MustCompile(labelPattern(`[\p{Lu}\p{N}][\p{Lu}\p{N}.'&-]*(?:` + hspace + `[\p{Lu}\p{N}.'&-]+){0,3}`))
	anyLabel   = regexp.MustCompile(labelPattern(`\p{Lu}[\p{L}\p{N}.'&-]*(?:` + hspace + `[\p{L}\p{N}.'&-]+){0,3}`))
)

func labelPattern(label string) string {
	return `(?m)^` + hspace + `*(?:-` + hspace + `*)?(` + label + `)` + hspace + `*:(?:` + hspace + `+|$)`
}

// IsHSpace reports whether r is horizontal white space, as the PCRE class \h.
func IsHSpace(r rune) bool {
	switch r {
	case '\t', ' ', 0xA0, 0x1680, 0x180E, 0x202F, 0x205F, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// Span is a byte range of the visible text.
type Span struct{ Start, End int }

// rawPlace is the place of the raw bytes of one visible byte: the line index, the offset and the length, such as
// 5 for &amp;. An LF between lines has the line -1.
type rawPlace struct{ line, offset, length int }

// Visible is the text of lines between tags with &lt;, &gt; and &amp; decoded, the lines joined with LF.
type Visible struct {
	Text string
	raw  []rawPlace
}

// VisibleOf returns the visible text of lines.
func VisibleOf(lines []string) Visible {
	var b strings.Builder
	var raw []rawPlace
	for li, line := range lines {
		if li > 0 {
			b.WriteByte('\n')
			raw = append(raw, rawPlace{line: -1})
		}
		offset := 0
		for k, part := range markupcore.SplitTags(line) {
			for i := 0; k%2 == 0 && i < len(part); {
				c, n := decodeAt(part[i:])
				b.WriteByte(c)
				raw = append(raw, rawPlace{li, offset + i, n})
				i += n
			}
			offset += len(part)
		}
	}
	return Visible{Text: b.String(), raw: raw}
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

// VisibleLine returns the visible text of one line.
func VisibleLine(line string) string {
	return VisibleOf([]string{line}).Text
}

// Cut removes the raw bytes of the visible spans from lines. Tags between them stay.
func (v Visible) Cut(lines []string, spans []Span) []string {
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

// Remove removes each speaker label with its colon and the spaces after it. A dash before it stays.
func Remove(lines []string, anyCase bool) []string {
	re := upperLabel
	if anyCase {
		re = anyLabel
	}
	v := VisibleOf(lines)
	var spans []Span
	for _, m := range re.FindAllStringSubmatchIndex(v.Text, -1) {
		if anyCase || strings.IndexFunc(v.Text[m[2]:m[3]], unicode.IsUpper) >= 0 {
			spans = append(spans, Span{Start: m[2], End: m[1]})
		}
	}
	return v.Cut(lines, spans)
}

// RemoveLabel removes the speaker label of one line, and then what the removal empties, as the hearing-impaired
// removal does. The result is cleaned like a cue line. It is the line itself, cleaned, when the line has no label.
// A line of invalid UTF-8 has no label.
func RemoveLabel(line string, anyCase bool) string {
	if !utf8.ValidString(VisibleLine(line)) {
		return markupcore.CleanLine(line)
	}
	lines := []string{markupcore.CleanLine(line)}
	out := RemoveEmptied(lines, Remove(lines, anyCase))
	if len(out) == 0 {
		return ""
	}
	return markupcore.CleanLine(out[0])
}

// AfterLead skips the horizontal space at text[start:], then a dash and its horizontal space.
func AfterLead(text string, start int) int {
	p := SkipHSpace(text, start)
	if p < len(text) && text[p] == '-' {
		p = SkipHSpace(text, p+1)
	}
	return p
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

// RemoveEmptied removes each changed line that had visible text and now holds none or only a dash, and its
// empty tag pairs. Its other tags carry to the next kept line, or to the end of the last one. Then, when a line
// went and only 1 of 2 or more dash lines is left, that dash goes too, as one speaker needs no dash.
func RemoveEmptied(original, lines []string) []string {
	var out []string
	pending := ""
	dashLines := 0
	anyRemoved := false
	for i, line := range lines {
		was := VisibleLine(original[i]) //nolint:gosec // the caller passes slices of the same length
		if isDashLine(was) {
			dashLines++
		}
		if line == original[i] { //nolint:gosec // the caller passes slices of the same length
			out = append(out, pending+line)
			pending = ""
			continue
		}
		line = RemoveBlankTagPairs(line)
		if emptied(was, line) {
			pending += tagsOf(line)
			anyRemoved = true
			continue
		}
		out = append(out, pending+line)
		pending = ""
	}
	if pending != "" && len(out) > 0 {
		out[len(out)-1] += pending
	}
	if anyRemoved && dashLines >= 2 {
		return removeLastDash(out)
	}
	return out
}

func emptied(was, line string) bool {
	t := strings.Trim(VisibleLine(line), markupcore.PHPTrimSet)
	return strings.Trim(was, markupcore.PHPTrimSet) != "" && (t == "" || t == "-")
}

func tagsOf(line string) string {
	var b strings.Builder
	for k, part := range markupcore.SplitTags(line) {
		if k%2 == 1 {
			b.WriteString(part)
		}
	}
	return b.String()
}

func isDashLine(text string) bool {
	p := SkipHSpace(text, 0)
	return p < len(text) && text[p] == '-'
}

func removeLastDash(lines []string) []string {
	dash := -1
	for i, line := range lines {
		if isDashLine(VisibleLine(line)) {
			if dash >= 0 {
				return lines
			}
			dash = i
		}
	}
	if dash < 0 {
		return lines
	}
	v := VisibleOf(lines[dash : dash+1])
	end := SkipHSpace(v.Text, SkipHSpace(v.Text, 0)+1)
	out := slices.Clone(lines)
	out[dash] = v.Cut(lines[dash:dash+1], []Span{{Start: 0, End: end}})[0]
	return out
}

// RemoveBlankTagPairs removes each pair such as <c.loud> </c> that holds only horizontal space, until none is left.
// The names compare with case. markupcore.RemoveEmptyTagPairs knows the style tags only.
func RemoveBlankTagPairs(line string) string {
	for {
		var b strings.Builder
		copied := 0
		for i := strings.IndexByte(line, '<'); i >= 0 && i < len(line); i++ {
			if line[i] != '<' {
				continue
			}
			if end := blankTagPairAt(line, i); end > 0 {
				b.WriteString(line[copied:i])
				copied = end
				i = end - 1
			}
		}
		if copied == 0 {
			return line
		}
		b.WriteString(line[copied:])
		line = b.String()
	}
}

const asciiSpace = " \t\n\r\f\v"

func blankTagPairAt(s string, i int) int {
	name, j := openTagAt(s, i)
	if name == "" {
		return 0
	}
	j = SkipHSpace(s, j)
	if !strings.HasPrefix(s[j:], "</"+name) {
		return 0
	}
	j += 2 + len(name)
	j = len(s) - len(strings.TrimLeft(s[j:], asciiSpace))
	if j == len(s) || s[j] != '>' {
		return 0
	}
	return j + 1
}

// openTagAt matches <name>, or <name followed by white space or a dot and any text up to >, at s[i]. It returns
// the name and the end of the tag, or "" for no match.
func openTagAt(s string, i int) (name string, end int) {
	j := i + 1 + tagNameLength(s[i+1:])
	if j == i+1 || j == len(s) {
		return "", 0
	}
	name = s[i+1 : j]
	if strings.IndexByte(asciiSpace+".", s[j]) >= 0 {
		k := strings.IndexAny(s[j:], "<>")
		if k < 0 {
			return "", 0
		}
		j += k
	}
	if s[j] != '>' {
		return "", 0
	}
	return name, j + 1
}

// tagNameLength returns the length of the ASCII letter and the ASCII letters and digits at the start of s.
func tagNameLength(s string) int {
	if s == "" || !isASCIILetter(s[0]) {
		return 0
	}
	n := 1
	for n < len(s) && (isASCIILetter(s[n]) || ('0' <= s[n] && s[n] <= '9')) {
		n++
	}
	return n
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

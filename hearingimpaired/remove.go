package hearingimpaired

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// hspace is the PCRE class \h.
const hspace = `[\t \x{A0}\x{1680}\x{180E}\x{2000}-\x{200A}\x{202F}\x{205F}\x{3000}]`

// The PHP upper case pattern starts with a lookahead for an upper case letter. removeLabels checks the captured
// label for one in its place.
var (
	upperLabel = regexp.MustCompile(labelPattern(`[\p{Lu}\p{N}][\p{Lu}\p{N}.'&-]*(?:` + hspace + `[\p{Lu}\p{N}.'&-]+){0,3}`))
	anyLabel   = regexp.MustCompile(labelPattern(`\p{Lu}[\p{L}\p{N}.'&-]*(?:` + hspace + `[\p{L}\p{N}.'&-]+){0,3}`))
)

func labelPattern(label string) string {
	return `(?m)^` + hspace + `*(?:-` + hspace + `*)?(` + label + `)` + hspace + `*:(?:` + hspace + `+|$)`
}

func isHSpace(r rune) bool {
	switch r {
	case '\t', ' ', 0xA0, 0x1680, 0x180E, 0x202F, 0x205F, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// removeFromLines removes the annotations that opts selects from the lines of one cue, and the lines that they
// emptied. Lines whose visible text is not valid UTF-8 stay as they are, as the PHP patterns with the u flag fail.
func removeFromLines(lines []string, opts Options) []string {
	if !utf8.ValidString(visibleOf(lines).text) {
		return lines
	}
	out := lines
	pairs := opts.brackets()
	for {
		v := visibleOf(out)
		var spans []span
		for _, p := range pairs {
			for _, s := range bracketMatches(v.text, p) {
				spans = append(spans, widen(v.text, s))
			}
		}
		if len(spans) == 0 {
			break
		}
		out = v.cut(out, spans)
	}
	if !opts.KeepSpeakerLabels {
		out = removeLabels(out, opts.AnyCaseLabels)
	}
	out = removeMusic(out, opts)
	return removeEmptiedLines(lines, out)
}

func (o Options) brackets() []BracketPair {
	pairs := slices.Clone(o.CustomBrackets)
	if !o.KeepSquareBrackets {
		pairs = append(pairs, BracketPair{Open: "[", Close: "]"})
	}
	if !o.KeepParentheses {
		pairs = append(pairs, BracketPair{Open: "(", Close: ")"})
	}
	return pairs
}

// span is a byte range of the visible text.
type span struct{ start, end int }

// rawPlace is the place of the raw bytes of one visible byte: the line index, the offset and the length, such as
// 5 for &amp;. An LF between lines has the line -1.
type rawPlace struct{ line, offset, length int }

// visible is the text of lines between tags with &lt;, &gt; and &amp; decoded, the lines joined with LF.
type visible struct {
	text string
	raw  []rawPlace
}

func visibleOf(lines []string) visible {
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
	return visible{b.String(), raw}
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

func visibleLine(line string) string {
	return visibleOf([]string{line}).text
}

// cut removes the raw bytes of the visible spans from lines. Tags between them stay.
func (v visible) cut(lines []string, spans []span) []string {
	gone := make([][]bool, len(lines))
	for _, s := range spans {
		for _, p := range v.raw[s.start:s.end] {
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

// widen adds the spaces and tabs after s when a line break, a space or a tab comes before it. Then it adds
// the spaces and tabs before s when the text ends after s or a line break or punctuation follows.
func widen(text string, s span) span {
	if s.start == 0 || strings.IndexByte("\n \t", text[s.start-1]) >= 0 {
		s.end = len(text) - len(strings.TrimLeft(text[s.end:], " \t"))
	}
	if s.end == len(text) || strings.IndexByte("\n.,!?;:", text[s.end]) >= 0 {
		s.start = len(strings.TrimRight(text[:s.start], " \t"))
	}
	return s
}

// bracketMatches returns each p.Open that no backslash follows, up to the next p.Close and an optional colon,
// with no p.Open between them.
func bracketMatches(text string, p BracketPair) []span {
	var out []span
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], p.Open) && !strings.HasPrefix(text[i+len(p.Open):], `\`) {
			if end, ok := closeOf(text, i+len(p.Open), p); ok {
				out = append(out, span{i, end})
				i = end
				continue
			}
		}
		_, n := utf8.DecodeRuneInString(text[i:])
		i += n
	}
	return out
}

func closeOf(text string, j int, p BracketPair) (int, bool) {
	for j < len(text) {
		if strings.HasPrefix(text[j:], p.Close) {
			end := j + len(p.Close)
			if end < len(text) && text[end] == ':' {
				end++
			}
			return end, true
		}
		if strings.HasPrefix(text[j:], p.Open) {
			return 0, false
		}
		_, n := utf8.DecodeRuneInString(text[j:])
		j += n
	}
	return 0, false
}

// removeLabels removes each speaker label with its colon and the spaces after it. A dash before it stays.
func removeLabels(lines []string, anyCase bool) []string {
	re := upperLabel
	if anyCase {
		re = anyLabel
	}
	v := visibleOf(lines)
	var spans []span
	for _, m := range re.FindAllStringSubmatchIndex(v.text, -1) {
		if anyCase || strings.IndexFunc(v.text[m[2]:m[3]], unicode.IsUpper) >= 0 {
			spans = append(spans, span{m[2], m[1]})
		}
	}
	return v.cut(lines, spans)
}

// removeMusic removes lyrics with opts.Lyrics, and lines of only music symbols unless opts.KeepMusicLines.
func removeMusic(lines []string, opts Options) []string {
	if !opts.Lyrics && opts.KeepMusicLines {
		return lines
	}
	v := visibleOf(lines)
	var spans []span
	if opts.Lyrics {
		spans = lyricsSpans(v.text)
	}
	return v.cut(lines, append(spans, musicLineSpans(v.text, opts)...))
}

// lyricsSpans pairs the music symbols in order: the first with the second, the third with the fourth.
func lyricsSpans(text string) []span {
	var spans []span
	open := -1
	for i := 0; i < len(text); {
		n := symbolAt(text, i)
		switch {
		case n == 0:
			_, n = utf8.DecodeRuneInString(text[i:])
		case open < 0:
			open = i
		default:
			spans = append(spans, widen(text, span{open, i + n}))
			open = -1
		}
		i += n
	}
	return spans
}

func musicLineSpans(text string, opts Options) []span {
	var spans []span
	for start := 0; start <= len(text); {
		end := len(text)
		if k := strings.IndexByte(text[start:], '\n'); k >= 0 {
			end = start + k
		}
		line := text[:end]
		if (opts.Lyrics && (startsWithSymbol(line, start) || endsWithSymbol(line, start))) ||
			(!opts.KeepMusicLines && onlySymbols(line, start)) {
			spans = append(spans, widen(text, span{start, end}))
		}
		start = end + 1
	}
	return spans
}

// symbolAt returns the length of the music symbol at text[i], or 0. A music symbol is U+2669 to U+266C, or a #
// with white space or the text edge on both sides, as PHP \s with the u flag reads white space.
func symbolAt(text string, i int) int {
	r, n := utf8.DecodeRuneInString(text[i:])
	if r >= 0x2669 && r <= 0x266C {
		return n
	}
	if r != '#' {
		return 0
	}
	before, m := utf8.DecodeLastRuneInString(text[:i])
	after, k := utf8.DecodeRuneInString(text[i+1:])
	if (m == 0 || markupcore.IsSpace(before)) && (k == 0 || markupcore.IsSpace(after)) {
		return 1
	}
	return 0
}

// afterLead skips the horizontal space at text[start:], then a dash and its horizontal space.
func afterLead(text string, start int) int {
	p := skipHSpace(text, start)
	if p < len(text) && text[p] == '-' {
		p = skipHSpace(text, p+1)
	}
	return p
}

func skipHSpace(text string, p int) int {
	for p < len(text) {
		r, n := utf8.DecodeRuneInString(text[p:])
		if !isHSpace(r) {
			break
		}
		p += n
	}
	return p
}

// The line helpers get the visible text up to the end of the line, so that symbolAt sees the text before the
// line, and start, the offset of the line.

func startsWithSymbol(line string, start int) bool {
	p := afterLead(line, start)
	return p < len(line) && symbolAt(line, p) > 0
}

func endsWithSymbol(line string, start int) bool {
	p := len(line)
	for p > start {
		r, n := utf8.DecodeLastRuneInString(line[start:p])
		if !isHSpace(r) {
			break
		}
		p -= n
	}
	_, n := utf8.DecodeLastRuneInString(line[start:p])
	return n > 0 && symbolAt(line, p-n) == n
}

func onlySymbols(line string, start int) bool {
	p := afterLead(line, start)
	if p == len(line) || symbolAt(line, p) == 0 {
		return false
	}
	for p < len(line) {
		n := symbolAt(line, p)
		if n == 0 {
			var r rune
			if r, n = utf8.DecodeRuneInString(line[p:]); !isHSpace(r) {
				return false
			}
		}
		p += n
	}
	return true
}

// removeEmptiedLines removes each changed line that had visible text and now holds none or only a dash, and its
// empty tag pairs. Its other tags carry to the next kept line, or to the end of the last one. Then, when a line
// went and only 1 of 2 or more dash lines is left, that dash goes too, as one speaker needs no dash.
func removeEmptiedLines(original, lines []string) []string {
	var out []string
	pending := ""
	dashLines := 0
	anyRemoved := false
	for i, line := range lines {
		was := visibleLine(original[i])
		if isDashLine(was) {
			dashLines++
		}
		if line == original[i] {
			out = append(out, pending+line)
			pending = ""
			continue
		}
		line = removeBlankTagPairs(line)
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
	t := strings.Trim(visibleLine(line), markupcore.PHPTrimSet)
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
	p := skipHSpace(text, 0)
	return p < len(text) && text[p] == '-'
}

func removeLastDash(lines []string) []string {
	dash := -1
	for i, line := range lines {
		if isDashLine(visibleLine(line)) {
			if dash >= 0 {
				return lines
			}
			dash = i
		}
	}
	if dash < 0 {
		return lines
	}
	v := visibleOf(lines[dash : dash+1])
	end := skipHSpace(v.text, skipHSpace(v.text, 0)+1)
	out := slices.Clone(lines)
	out[dash] = v.cut(lines[dash:dash+1], []span{{0, end}})[0]
	return out
}

// removeBlankTagPairs removes each pair such as <c.loud> </c> that holds only horizontal space, until none is left.
// The names compare with case. markupcore.RemoveEmptyTagPairs knows the style tags only.
func removeBlankTagPairs(line string) string {
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
	j = skipHSpace(s, j)
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

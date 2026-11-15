// Package speakerlabel holds the speaker label rule: a name such as JOHN: at the start of a line, or after its
// dialogue dash.
//
// Adapted from Subtitle Edit, src/libse/Forms/RemoveTextForHI.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/5b9ee8baf08c472c7a74fbc337446b656dedabb4/src/libse/Forms/RemoveTextForHI.cs), MIT.
package speakerlabel

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
)

// The upper case pattern has no lookahead for an upper case letter. Remove checks the captured
// label for one instead.
var (
	upperLabel = regexp.MustCompile(labelPattern(`[\p{Lu}\p{N}][\p{Lu}\p{N}.'&-]*(?:` + markupcore.HSpaceClass + `[\p{Lu}\p{N}.'&-]+){0,3}`))
	anyLabel   = regexp.MustCompile(labelPattern(`\p{Lu}[\p{L}\p{N}.'&-]*(?:` + markupcore.HSpaceClass + `[\p{L}\p{N}.'&-]+){0,3}`))
)

func labelPattern(label string) string {
	return `(?m)^` + markupcore.HSpaceClass + `*(?:-` + markupcore.HSpaceClass + `*)?(` + label + `)` + markupcore.HSpaceClass + `*:(?:` + markupcore.HSpaceClass + `+|$)`
}

// runText returns the text run of one line.
func runText(line string) string {
	return markupcore.TextRunsOf([]string{line}).Text
}

// Remove removes each speaker label with its colon and the spaces after it. A dash before it stays.
func Remove(lines []string, anyCase bool) []string {
	re := upperLabel
	if anyCase {
		re = anyLabel
	}
	v := markupcore.TextRunsOf(lines)
	var spans []markupcore.TextSpan
	for _, m := range re.FindAllStringSubmatchIndex(v.Text, -1) {
		if anyCase || strings.IndexFunc(v.Text[m[2]:m[3]], unicode.IsUpper) >= 0 {
			spans = append(spans, markupcore.TextSpan{Start: m[2], End: m[1]})
		}
	}
	return v.Cut(lines, spans)
}

// RemoveLabel removes the speaker label of one line, and then what the removal empties, as the hearing-impaired
// removal does. The result is cleaned like a cue line. It is the line itself, cleaned, when the line has no label.
// A line of invalid UTF-8 has no label.
func RemoveLabel(line string, anyCase bool) string {
	if !utf8.ValidString(runText(line)) {
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
	p := markupcore.SkipHSpace(text, start)
	if p < len(text) && text[p] == '-' {
		p = markupcore.SkipHSpace(text, p+1)
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
		was := runText(original[i]) //nolint:gosec // the caller passes slices of the same length
		if isDashLine(was) {
			dashLines++
		}
		if line == original[i] { //nolint:gosec // the caller passes slices of the same length
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
	t := strings.Trim(runText(line), markupcore.TrimSet)
	return strings.Trim(was, markupcore.TrimSet) != "" && (t == "" || t == "-")
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
	p := markupcore.SkipHSpace(text, 0)
	return p < len(text) && text[p] == '-'
}

func removeLastDash(lines []string) []string {
	dash := -1
	for i, line := range lines {
		if isDashLine(runText(line)) {
			if dash >= 0 {
				return lines
			}
			dash = i
		}
	}
	if dash < 0 {
		return lines
	}
	v := markupcore.TextRunsOf(lines[dash : dash+1])
	end := markupcore.SkipHSpace(v.Text, markupcore.SkipHSpace(v.Text, 0)+1)
	out := slices.Clone(lines)
	out[dash] = v.Cut(lines[dash:dash+1], []markupcore.TextSpan{{Start: 0, End: end}})[0]
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
	j = markupcore.SkipHSpace(s, j)
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

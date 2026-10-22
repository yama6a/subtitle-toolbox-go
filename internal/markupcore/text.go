package markupcore

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
)

var (
	rubyRegexp      = regexp.MustCompile(`(?is)<ruby\b[^>]*>(.*?)(?:</ruby[ \t\n\r\f\v]*>|\z)`)
	rpRegexp        = regexp.MustCompile(`(?is)<rp\b[^>]*>.*?</rp[ \t\n\r\f\v]*>`)
	rtStartRegexp   = regexp.MustCompile(`(?i)<rt\b[^>]*>`)
	rtEndRegexp     = regexp.MustCompile(`(?i)</rt[ \t\n\r\f\v]*>`)
	lineBreakRegexp = regexp.MustCompile(`[ \t\n\r\f\v]*\n[ \t\n\r\f\v]*`)
	voiceTagRegexp  = regexp.MustCompile(VoiceTagPattern)
)

// PlainText removes the tags and decodes the entities, for formats without markup. It does not trim.
func PlainText(text string) string {
	return DecodeEntities(StripAllTags(text))
}

// VisibleText returns PlainText trimmed, for example "Hi & bye" for "<i>Hi</i> &amp; bye ".
func VisibleText(text string) string {
	return strings.Trim(PlainText(text), PHPTrimSet)
}

// VisibleLength returns the number of characters of VisibleText, counted as grapheme.Count does.
func VisibleLength(text string) int {
	return grapheme.Count(VisibleText(text))
}

// PlainLines writes ruby as text, then returns the visible text of each line without the empty ones.
func PlainLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		if v := VisibleText(RubyAsText(line)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// RubyAsText writes ruby as "base (annotation)" for formats without ruby. It drops <rp> elements, because the
// written parentheses take their place. A ruby without its end tag runs to the end of the text.
func RubyAsText(text string) string {
	if !strings.Contains(asciiLower(text), "<ruby") {
		return text
	}
	// PHP ends an open ruby with $, which matches before a final LF.
	body, lf := strings.CutSuffix(text, "\n")
	var b strings.Builder
	copied := 0
	for _, m := range rubyRegexp.FindAllStringSubmatchIndex(body, -1) {
		b.WriteString(body[copied:m[0]])
		b.WriteString(rubyText(body[m[2]:m[3]]))
		copied = m[1]
	}
	b.WriteString(body[copied:])
	if lf {
		b.WriteByte('\n')
	}
	return b.String()
}

func rubyText(ruby string) string {
	parts := rtStartRegexp.Split(rpRegexp.ReplaceAllString(ruby, ""), -1)
	text := parts[0]
	for _, part := range parts[1:] {
		annotation, base := part, ""
		if loc := rtEndRegexp.FindStringIndex(part); loc != nil {
			annotation, base = part[:loc[0]], part[loc[1]:]
		}
		if annotation = strings.Trim(annotation, PHPTrimSet); annotation == "" {
			text += base
		} else {
			text = strings.TrimRight(text, PHPTrimSet) + " (" + annotation + ")" + base
		}
	}
	return text
}

// ToSingleLine trims text and joins its lines with a space, for formats that hold one line per cue.
func ToSingleLine(text string) string {
	return lineBreakRegexp.ReplaceAllLiteralString(strings.Trim(text, PHPTrimSet), " ")
}

// Words splits text without tags at Unicode white space. Invalid UTF-8 splits at ASCII white space.
func Words(text string) []string {
	if utf8.ValidString(text) {
		return strings.FieldsFunc(text, unicode.IsSpace)
	}
	return strings.FieldsFunc(text, func(r rune) bool { return r < utf8.RuneSelf && strings.ContainsRune(asciiSpace, r) })
}

// Characters splits text into UTF-8 characters, or into bytes when it is not valid UTF-8.
func Characters(text string) []string {
	if text == "" {
		return nil
	}
	if !utf8.ValidString(text) {
		chars := make([]string, len(text))
		for i := range len(text) {
			chars[i] = text[i : i+1]
		}
		return chars
	}
	return strings.Split(text, "")
}

// Speaker returns the name of the first <v> tag, with entities decoded and trimmed.
func Speaker(text string) (string, bool) {
	m := voiceTagRegexp.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return strings.Trim(DecodeEntities(m[2]), PHPTrimSet), true
}

// AddSpeaker puts the <v> tag of name before the first line that holds more than white space, and trims the left
// side of that line. An empty name adds no tag.
func AddSpeaker(lines []string, name string) []string {
	lines = slices.Clone(lines)
	if name == "" {
		return lines
	}
	for i, line := range lines {
		if strings.Trim(line, PHPTrimSet) != "" {
			lines[i] = VoiceTag(name, "") + strings.TrimLeft(line, PHPTrimSet)
			break
		}
	}
	return lines
}

// VoiceTag returns a <v> tag for the speaker name with &, < and > escaped and quotes kept.
// The class holds voice classes such as ".loud".
func VoiceTag(name, class string) string {
	return "<v" + class + " " + EscapeText(name) + ">"
}

// MapTextRuns calls fn for each text run between tags that is not empty, with &lt;, &gt; and &amp; decoded.
// First and last mark the first and the last such run of the line. A changed run is escaped like its raw form.
func MapTextRuns(lines []string, fn func(text string, first, last bool) string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		parts := SplitTags(line)
		var runs []int
		for j := 0; j < len(parts); j += 2 {
			if parts[j] != "" {
				runs = append(runs, j)
			}
		}
		for k, j := range runs {
			text := UnescapeText(parts[j])
			if mapped := fn(text, k == 0, k == len(runs)-1); mapped != text {
				parts[j] = EscapeTextLike(mapped, parts[j])
			}
		}
		out[i] = strings.Join(parts, "")
	}
	return out
}

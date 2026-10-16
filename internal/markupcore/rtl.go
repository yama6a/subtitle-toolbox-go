package markupcore

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
)

const (
	rightToLeftEmbedding = "\u202B"
	popFormatting        = "\u202C"
	anyTagPattern        = `(?:` + TagPattern + `|\{\\[^{}]*\})`
)

var (
	// An ASS drawing command such as {\p1} makes the text after it a path, not a line to embed.
	assDrawingRegexp = regexp.MustCompile(`\{[^{}]*\\p[1-9]`)
	tagRunsRegexp    = regexp.MustCompile(`(?s)\A(` + anyTagPattern + `*)(.*?)(` + anyTagPattern + `*)(\n?)\z`)
	markRemover      = strings.NewReplacer(rightToLeftEmbedding, "", popFormatting, "") //nolint:gochecknoglobals // a replacer holds no state
)

// HasRightToLeftLetter reports whether the visible text of line holds a letter
// of the Arabic, Hebrew, Syriac, Thaana or N'Ko script. Invalid UTF-8 holds none.
func HasRightToLeftLetter(line string) bool {
	text := VisibleText(line)
	if !utf8.ValidString(text) {
		return false
	}
	return strings.IndexFunc(text, isRightToLeftLetter) >= 0
}

func isRightToLeftLetter(r rune) bool {
	// PCRE2 matches \p{Arabic} against Script_Extensions. U+0640 (tatweel) has the script Common but extends to
	// Arabic and Syriac, and the Go tables hold only the script.
	return r == 0x0640 || unicode.IsLetter(r) && (unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Hebrew, r) ||
		unicode.Is(unicode.Syriac, r) || unicode.Is(unicode.Thaana, r) || unicode.Is(unicode.Nko, r))
}

// EmbedRightToLeft wraps the text of a right-to-left line in U+202B and U+202C, inside its leading and trailing tags.
// Sentence-end punctuation at the start of the text moves to its end first. A final LF stays after the closing mark.
// A line without a right-to-left letter, a line with an ASS drawing command and a line of invalid UTF-8 stay as they are.
func EmbedRightToLeft(line string) string {
	if !utf8.ValidString(line) {
		return line
	}
	text := markRemover.Replace(line)
	if assDrawingRegexp.MatchString(text) || !HasRightToLeftLetter(text) {
		return line
	}

	m := tagRunsRegexp.FindStringSubmatch(text)
	leading, middle, trailing, lf := m[1], m[2], m[3], m[4]
	if dash, run, rest, space, ok := splitLeadingPunctuation(middle); ok {
		return EmbedRightToLeft(leading + dash + rest + run + space + trailing + lf)
	}
	return leading + rightToLeftEmbedding + middle + popFormatting + trailing + lf
}

// HoldsRightToLeftEmbedding reports whether a line already holds the embedding of EmbedRightToLeft.
func HoldsRightToLeftEmbedding(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, rightToLeftEmbedding) && EmbedRightToLeft(line) == line {
			return true
		}
	}
	return false
}

// IsBidiControl reports whether r is one of the bidi controls that RemoveBidiControls removes.
func IsBidiControl(r rune) bool {
	return grapheme.IsBidiControl(r)
}

// splitLeadingPunctuation splits a middle such as "- ?text  " into the dash part "- ", the punctuation run "?", the
// rest "text" and the white space "  ". It reports false when the middle has no such start or when the rest already
// ends with sentence-end punctuation.
func splitLeadingPunctuation(middle string) (dash, run, rest, space string, ok bool) {
	i := 0
	if r, size := utf8.DecodeRuneInString(middle); IsDialogueDash(r) {
		i = size + len(middle[size:]) - len(strings.TrimLeft(middle[size:], " \t"))
	}
	if strings.HasPrefix(middle[i:], "..") {
		return "", "", "", "", false
	}
	j := i
	for _, r := range middle[i:] {
		if !isSentenceEnd(r) {
			break
		}
		j += utf8.RuneLen(r)
	}
	if j == i {
		return "", "", "", "", false
	}
	body := strings.TrimLeft(middle[j:], " \t")
	rest = strings.TrimRightFunc(body, IsSpace)
	if rest == "" {
		return "", "", "", "", false
	}
	if r, _ := utf8.DecodeLastRuneInString(rest); isSentenceEnd(r) {
		return "", "", "", "", false
	}
	return middle[:i], middle[i:j], rest, body[len(rest):], true
}

func isSentenceEnd(r rune) bool {
	return strings.ContainsRune(".,!?:;\u061F\u060C", r)
}

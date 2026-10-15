package markupcore

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	rightToLeftEmbedding = "\u202B"
	popFormatting        = "\u202C"
)

var (
	// An ASS drawing command such as {\p1} makes the text after it a path, not a line to embed.
	assDrawingRegexp = regexp.MustCompile(`\{[^{}]*\\p[1-9]`)
	tagAtStartRegexp = regexp.MustCompile(`\A(?:` + TagPattern + `|\{\\[^{}]*\})`)
)

// HasRightToLeftLetter reports whether the visible text of line holds a letter
// of the Arabic, Hebrew, Syriac, Thaana or N'Ko script. Invalid UTF-8 holds none.
func HasRightToLeftLetter(line string) bool {
	text := VisibleText(line)
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if unicode.IsLetter(r) && (unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Hebrew, r) ||
			unicode.Is(unicode.Syriac, r) || unicode.Is(unicode.Thaana, r) || unicode.Is(unicode.Nko, r)) {
			return true
		}
	}
	return false
}

// EmbedRightToLeft wraps the text of a right-to-left line in U+202B and U+202C, inside its leading and trailing tags.
// Sentence-end punctuation at the start of the text moves to its end first. A line without a right-to-left letter,
// a line with an ASS drawing command and a line of invalid UTF-8 stay as they are.
func EmbedRightToLeft(line string) string {
	text := strings.NewReplacer(rightToLeftEmbedding, "", popFormatting, "").Replace(line)
	if assDrawingRegexp.MatchString(text) || !HasRightToLeftLetter(text) || !utf8.ValidString(text) {
		return line
	}

	leading, middle, trailing := splitTagRuns(text)
	if dash, run, rest, space, ok := splitLeadingPunctuation(middle); ok {
		return EmbedRightToLeft(leading + dash + rest + run + space + trailing)
	}
	return leading + rightToLeftEmbedding + middle + popFormatting + trailing
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
	return 0x200E <= r && r <= 0x200F || 0x202A <= r && r <= 0x202E || 0x2066 <= r && r <= 0x2069
}

// splitTagRuns splits text into the longest run of tags at its start, the middle and the longest run of tags at its end.
func splitTagRuns(text string) (leading, middle, trailing string) {
	start := 0
	for end := tagEnd(text[start:]); end > 0; end = tagEnd(text[start:]) {
		start += end
	}
	// endsInTags[i] is true when text[i:] holds tags only.
	endsInTags := make([]bool, len(text)+1)
	endsInTags[len(text)] = true
	for i := len(text) - 1; i >= start; i-- {
		if end := tagEnd(text[i:]); end > 0 {
			endsInTags[i] = endsInTags[i+end]
		}
	}
	stop := start
	for !endsInTags[stop] {
		stop++
	}
	return text[:start], text[start:stop], text[stop:]
}

// tagEnd returns the length of the tag at the start of s, or 0 when s starts with none.
func tagEnd(s string) int {
	if loc := tagAtStartRegexp.FindStringIndex(s); loc != nil {
		return loc[1]
	}
	return 0
}

// splitLeadingPunctuation splits a middle such as "- ?text  " into the dash part "- ", the punctuation run "?", the
// rest "text" and the white space "  ". It reports false when the middle has no such start or when the rest already
// ends with sentence-end punctuation.
func splitLeadingPunctuation(middle string) (dash, run, rest, space string, ok bool) {
	i := 0
	if r, size := utf8.DecodeRuneInString(middle); isDialogueDash(r) {
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
	rest = strings.TrimRightFunc(body, unicode.IsSpace)
	if rest == "" {
		return "", "", "", "", false
	}
	if r, _ := utf8.DecodeLastRuneInString(rest); isSentenceEnd(r) {
		return "", "", "", "", false
	}
	return middle[:i], middle[i:j], rest, body[len(rest):], true
}

func isDialogueDash(r rune) bool {
	return r == '-' || r == '\u2010' || r == '\u2013' || r == '\u2014'
}

func isSentenceEnd(r rune) bool {
	return strings.ContainsRune(".,!?:;\u061F\u060C", r)
}

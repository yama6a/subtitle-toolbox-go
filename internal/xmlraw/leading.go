package xmlraw

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
)

// InvalidUTF8Reason is the reason of the error that a writer returns for text that is not valid UTF-8.
const InvalidUTF8Reason = "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding."

var (
	lineBreakRegexp = regexp.MustCompile(`\r\n?|\n`)
	eolRegexp       = regexp.MustCompile(`\r+\n|\r`)
)

// StartRegexp returns the pattern of a place where XML can start: an XML declaration, a comment, a DOCTYPE or a root
// element that rootPattern matches.
func StartRegexp(rootPattern string) *regexp.Regexp {
	return regexp.MustCompile(`<(?:\?xml[\t\n\v\f\r ]|!--|!DOCTYPE[\t\n\v\f\r ]|` + rootPattern + `[\t\n\v\f\r />])`)
}

// SkipLeadingText removes the text before the first match of start, unless the content starts with white space
// and "<". Line breaks replace that text after the XML declaration, so a parser still reports the line numbers of
// the file. It returns the new content and the removed text without white space at its ends.
func SkipLeadingText(content string, start *regexp.Regexp) (repaired, skipped string) {
	trimmed := strings.TrimLeft(content, markupcore.PHPTrimSet)
	at := len(content) - len(trimmed)
	if !strings.HasPrefix(trimmed, "<") {
		loc := start.FindStringIndex(content)
		if loc == nil {
			return content, ""
		}
		at = loc[0]
	}
	if at == 0 {
		return content, ""
	}

	prefix := content[:at]
	breaks := strings.Repeat("\n", len(lineBreakRegexp.FindAllStringIndex(prefix, -1)))
	content = content[at:]
	if strings.HasPrefix(content, "<?xml") {
		if end := strings.Index(content, "?>"); end >= 0 {
			return content[:end+2] + breaks + content[end+2:], strings.Trim(prefix, markupcore.PHPTrimSet)
		}
	}
	return breaks + content, strings.Trim(prefix, markupcore.PHPTrimSet)
}

// LeadingText returns the line where the text skipped by SkipLeadingText starts, its lines, and the message
// "The file has text before the XML: ...".
func LeadingText(content, skipped string) (line int, lines []string, msg string) {
	before, _, _ := strings.Cut(content, skipped)
	line = 1 + strings.Count(before, "\n")
	lines = strings.Split(eolRegexp.ReplaceAllLiteralString(skipped, "\n"), "\n")
	return line, lines, "The file has text before the XML: \"" + lines[0] + "\"."
}

// Text returns s without the characters that XML 1.0 forbids: the C0 controls except tab, LF and CR, U+FFFE and
// U+FFFF. It returns false when s is not valid UTF-8.
func Text(s string) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xfffe || r == 0xffff {
			return -1
		}
		return r
	}, s), true
}

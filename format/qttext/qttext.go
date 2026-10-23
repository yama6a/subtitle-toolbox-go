// Package qttext reads and writes QuickTime text, the text track format of QuickTime Player 7 and older Apple tools.
//
// A header of text descriptors such as {QTtext} and {timeScale:30} comes first. Then each sample starts with a
// timestamp such as [00:00:01.15], and the text of the sample follows. A sample ends at the next timestamp, and a
// sample without text is a gap. The extension is .txt, which FormatFromPath maps to plain text.
package qttext

import (
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/phpkey"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.QuickTimeText)

// ReadOptions holds the settings of the QuickTime text reader. It has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the QuickTime text writer. It has none yet.
type WriteOptions struct{}

const (
	// white is the \s of PCRE, which Go regexp lacks in the form of VT.
	white = `[ \t\n\r\f\v]`
	// descriptorPattern matches a descriptor such as {italic} or {font:Tahoma}.
	descriptorPattern = `\{[A-Za-z-]+(?::[^}]*)?\}`
)

//nolint:gochecknoglobals // compiled patterns
var (
	timestampRegexp  = regexp.MustCompile(`^\[(\d+):([0-5]\d):([0-5]\d)(?:\.(\d+))?\]`)
	descriptorRegexp = regexp.MustCompile(descriptorPattern)
	headerRegexp     = regexp.MustCompile(`^` + white + `*\{[qQ][tT][tT][eE][xX][tT]\}`)
	detectRegexp     = regexp.MustCompile(`\A\{[qQ][tT][tT][eE][xX][tT]\}`)
	timeScaleRegexp  = valueRegexp("timeScale")
	timeStampsRegexp = valueRegexp("timeStamps")
)

// valueRegexp matches {name: value} with the name in any ASCII case, and captures the trimmed value. Go (?i) would
// also match U+017F as s.
func valueRegexp(name string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`\{`)
	for _, r := range name {
		b.WriteString("[" + strings.ToLower(string(r)) + strings.ToUpper(string(r)) + "]")
	}
	b.WriteString(white + `*:` + white + `*([^}]*?)` + white + `*\}`)
	return regexp.MustCompile(b.String())
}

// Detect reports whether content starts with {QTtext}, in any case. The content must have no leading BOM or white
// space, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return detectRegexp.Match(content)
}

type style struct{ bold, italic, underline bool }

func isTimestampLine(line string) bool {
	return timestampRegexp.MatchString(strings.Trim(line, markupcore.PHPTrimSet))
}

func findDescriptor(lines []string, re *regexp.Regexp) (string, bool) {
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

func timeScale(header []string) int64 {
	value, ok := findDescriptor(header, timeScaleRegexp)
	if !ok || !timecode.IsDigits(value) {
		return 0
	}
	return phpkey.DigitsToInt(value)
}

func isRelative(header []string) bool {
	value, _ := findDescriptor(header, timeStampsRegexp)
	return markupcore.ASCIILower(value) == "relative"
}

// readLines applies the descriptors of lines to st and returns the text of the lines as core markup. Lines without
// text drop out. Descriptors other than {plain}, {bold}, {italic} and {underline} give no markup.
func readLines(lines []string, st *style) []string {
	var out []string
	for _, line := range lines {
		var runs []markupcore.Run
		at := 0
		for _, loc := range descriptorRegexp.FindAllStringIndex(line, -1) {
			runs = appendRun(runs, line[at:loc[0]], *st)
			switch markupcore.ASCIILower(strings.Trim(line[loc[0]:loc[1]], "{}")) {
			case "plain":
				*st = style{}
			case "bold":
				st.bold = true
			case "italic":
				st.italic = true
			case "underline":
				st.underline = true
			}
			at = loc[1]
		}
		runs = appendRun(runs, line[at:], *st)
		if text := strings.Trim(markupcore.StyleRuns(runs, true), markupcore.PHPTrimSet); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func appendRun(runs []markupcore.Run, text string, st style) []markupcore.Run {
	if text == "" {
		return runs
	}
	return append(runs, markupcore.Run{Text: text, Bold: st.bold, Italic: st.italic, Underline: st.underline})
}

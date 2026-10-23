package mpl2

import (
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var italicTagRegexp = regexp.MustCompile(`<(/?)[iI]>`)

// Write writes s as MPL2. It writes every cue, also one without text.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	for _, c := range s.All() {
		body = append(body, '[')
		body = strconv.AppendInt(body, writtenTenths(c.Start), 10)
		body = append(body, "]["...)
		body = strconv.AppendInt(body, writtenTenths(c.End), 10)
		body = append(body, ']')
		body = append(body, strings.Join(italicLines(c.Lines), "|")...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.MPL2, body, common, false)
}

// writtenTenths rounds d to tenths of a second, half away from zero on whole milliseconds. A time below 0 is 0.
func writtenTenths(d time.Duration) int64 {
	return (max(d, 0).Round(time.Millisecond).Milliseconds() + 50) / 100
}

// italicLines returns the visible text of each line. A line gets a / prefix when all its visible text is italic.
// The italic state runs on from one line to the next, also over a line without text.
func italicLines(lines []string) []string {
	var out []string
	italic := false
	for _, line := range lines {
		line = markupcore.RubyAsText(line)
		allItalic := true
		pos := 0
		for _, m := range italicTagRegexp.FindAllStringSubmatchIndex(line, -1) {
			allItalic = allItalic && (italic || !hasVisibleText(line[pos:m[0]]))
			italic = m[2] == m[3]
			pos = m[1]
		}
		allItalic = allItalic && (italic || !hasVisibleText(line[pos:]))

		text := markupcore.VisibleText(line)
		if text == "" {
			continue
		}
		if allItalic {
			text = "/" + text
		}
		out = append(out, text)
	}
	return out
}

func hasVisibleText(part string) bool {
	return strings.Trim(markupcore.StripAllTags(part), markupcore.PHPTrimSet) != ""
}

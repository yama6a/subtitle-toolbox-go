package mpl2

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var italicTagRegexp = regexp.MustCompile(`<(/?)[iI]>`)

// Write writes s as MPL2. It writes every cue, also one without text. A line break inside a line becomes |, and a |
// in the text becomes U+00A6 BROKEN BAR.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	for _, c := range s.All() {
		body = append(body, '[')
		body = strconv.AppendInt(body, timecode.Units(c.Start, tenth), 10)
		body = append(body, "]["...)
		body = strconv.AppendInt(body, timecode.Units(c.End, tenth), 10)
		body = append(body, ']')
		body = append(body, writing.PipeText(italicLines(c.Lines))...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.MPL2, body, common, false)
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

		for _, text := range markupcore.PlainLines([]string{line}) {
			if allItalic {
				text = "/" + text
			}
			out = append(out, text)
		}
	}
	return out
}

func hasVisibleText(part string) bool {
	return strings.Trim(markupcore.StripAllTags(part), markupcore.PHPTrimSet) != ""
}

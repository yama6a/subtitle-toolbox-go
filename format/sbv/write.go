package sbv

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as SBV. It skips a cue without text, because an empty line would end the SBV cue. A text line that
// the reader would read as a timing line starts with U+2060 WORD JOINER.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	first := true
	for _, c := range s.All() {
		lines := textLines(c)
		if len(lines) == 0 {
			continue
		}
		if !first {
			body = append(body, '\n')
		}
		first = false
		body = timecode.AppendClock(body, c.Start, 1, '.')
		body = append(body, ',')
		body = timecode.AppendClock(body, c.End, 1, '.')
		body = append(body, '\n')
		for _, line := range lines {
			body = append(body, line...)
			body = append(body, '\n')
		}
	}
	return writing.Write(w, model.SBV, body, common, false)
}

// textLines returns the visible lines of c that the reader reads as text lines. It drops a line that is empty in the
// form that the reader reads, such as a line of only U+FEFF, and puts a word joiner before a line that reads as a
// timing line.
func textLines(c model.Cue) []string {
	lines := markupcore.PlainLines(c.Lines)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		clean := blocks.CleanLine(line)
		switch {
		case clean == "":
			continue
		case isTimingLine(clean, true):
			line = writing.WordJoiner + line
		}
		out = append(out, line)
	}
	return out
}

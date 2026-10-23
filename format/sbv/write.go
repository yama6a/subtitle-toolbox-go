package sbv

import (
	"io"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as SBV. It skips a cue without text, because an empty line would end the SBV cue.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	first := true
	for _, c := range s.All() {
		lines := markupcore.PlainLines(c.Lines)
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
		body = append(body, strings.Join(lines, "\n")...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.SBV, body, common, false)
}

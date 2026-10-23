package srt

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/internal/subrip"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as SubRip. A text line that the reader would read as a timing line starts with U+2060 WORD JOINER.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	for i, c := range s.All() {
		if i > 0 {
			body = append(body, '\n')
		}
		body = subrip.AppendCue(body, i+1, c, common.StripTags)
	}
	return writing.Write(w, model.SubRip, body, common, true)
}

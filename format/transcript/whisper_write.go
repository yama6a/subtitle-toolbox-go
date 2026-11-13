package transcript

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteWhisper writes s to w as Whisper JSON.
func WriteWhisper(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	return nil
}

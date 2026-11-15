package ttml

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Write writes s as TTML. It keeps the namespace, head and attributes that Read stored.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.TTML, common)
	if err != nil {
		return WriteReport{}, err
	}
	body, err := ttmldoc.Document(s, common, ttmldoc.Options{Format: model.TTML})
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{}, writing.Write(w, model.TTML, body, common, false)
}

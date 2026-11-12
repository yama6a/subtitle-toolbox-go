package ttml

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ttmlwrite"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

// WriteOptions holds the settings of the TTML writer. It has no fields yet.
type WriteOptions struct{}

// Write writes s as TTML. It keeps the namespace, head and attributes that Read stored.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	body, err := ttmlwrite.Document(s, common, ttmlwrite.Options{Format: model.TTML})
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{}, writing.Write(w, model.TTML, body, common, false)
}

package webvtt

import (
	"io"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as WebVTT.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	_, err := WriteWithReport(w, s, common, opts)
	return err
}

// WriteWithReport writes s as WebVTT and returns what the output leaves out.
func WriteWithReport(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	var report WriteReport
	cw := &vtt.Writer{
		StripTags: common.StripTags,
		OnDroppedColor: func(cueIndex int, color, message string) {
			report.DroppedColors = append(report.DroppedColors, DroppedColor{CueIndex: cueIndex, Color: color, Message: message})
		},
	}
	if err := writing.Write(w, model.WebVTT, []byte(document(cw, s)), common, true); err != nil {
		return WriteReport{}, err
	}
	return report, nil
}

func document(cw *vtt.Writer, s *model.Subtitle) string {
	file, _ := s.FormatData(string(model.WebVTT))
	blocks := vtt.FileBlocks(file)
	comments := make([][]string, s.Len()+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, s.Len())
		comments[at] = append(comments[at], vtt.NoteBlock(c.Text))
	}
	for i, c := range s.All() {
		blocks = append(blocks, comments[i]...)
		blocks = append(blocks, cw.CueBlock(c, i))
	}
	blocks = append(blocks, comments[s.Len()]...)

	out := vtt.Header(file) + "\n\n"
	if len(blocks) > 0 {
		out += strings.Join(blocks, "\n\n") + "\n"
	}
	return out
}

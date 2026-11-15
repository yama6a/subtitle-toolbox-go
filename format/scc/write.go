package scc

import (
	"fmt"
	"io"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Write writes s as pop-on captions on CEA-608 data channel 1. A cue that breaks a limit of SCC gives an
// *model.UnwritableError, unless opts.Fit changes it. The error holds ErrWrapFirst or ErrSplitFirst for a cue that is
// too long.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.SCC, common)
	if err != nil {
		return WriteReport{}, err
	}
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	words, changes, err := caption.CEA608Words(model.SCC, s.Cues(), opts.Fit)
	if err != nil {
		return WriteReport{}, err
	}
	rate := model.FrameRate2997DF
	if !dropFrame(s, opts) {
		rate = model.FrameRate2997
	}
	if err := writing.Write(w, model.SCC, []byte(document(words, rate)), common, false); err != nil {
		return WriteReport{}, err
	}
	return WriteReport{Changes: changes}, nil
}

func dropFrame(s *model.Subtitle, opts WriteOptions) bool {
	switch opts.DropFrame {
	case model.DropFrameOn:
		return true
	case model.DropFrameOff:
		return false
	case model.DropFrameKeep:
	}
	file, _ := s.FormatData(formatDataKey)
	stored, ok := file.Bool("dropFrame")
	return stored || !ok
}

// document writes one line per run of consecutive frames, with an empty line between lines.
func document(words []caption.FrameWord, rate model.FrameRate) string {
	var b strings.Builder
	b.WriteString(header)
	for i, fw := range words {
		if i == 0 || fw.Frame != words[i-1].Frame+1 {
			fmt.Fprintf(&b, "\n\n%s\t%04x", rate.TimecodeOf(fw.Frame), fw.Word)
		} else {
			fmt.Fprintf(&b, " %04x", fw.Word)
		}
	}
	b.WriteString("\n")
	return b.String()
}

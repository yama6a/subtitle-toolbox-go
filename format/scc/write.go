package scc

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// WriteOptions holds the settings of the SCC writer.
type WriteOptions struct {
	// DropFrame selects the time code labels. DropFrameKeep takes the drop-frame mode that the reader stored, else drop-frame.
	DropFrame model.DropFrameMode
	// Fit changes the text and the timing that SCC cannot hold, in place of failing. The report lists each change.
	Fit bool
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown drop-frame mode.
func (o WriteOptions) Validate() error {
	if o.DropFrame > model.DropFrameOff {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.SCC.DropFrame",
			Msg:    "The SCC drop-frame mode " + strconv.Itoa(int(o.DropFrame)) + " is unknown.",
		}
	}
	return nil
}

// FitAction is the kind of a change that Fit made. The values are "transliterated", "wrapped", "delayed" and "dropped".
type FitAction = caption.FitAction

// The fit actions.
const (
	Transliterated = caption.Transliterated
	Wrapped        = caption.Wrapped
	Delayed        = caption.Delayed
	Dropped        = caption.Dropped
)

// FitChange is one change that Fit made to a cue.
type FitChange = caption.FitChange

// WriteReport lists the changes of a write.
type WriteReport struct {
	// Changes holds each change of Fit, in cue order. It is empty without Fit.
	Changes []FitChange
}

// Write writes s as pop-on captions on CEA-608 data channel 1. A cue that breaks a limit of SCC gives an
// *model.UnwritableError, unless opts.Fit changes it. The error holds ErrWrapFirst or ErrSplitFirst for a cue that is
// too long.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
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

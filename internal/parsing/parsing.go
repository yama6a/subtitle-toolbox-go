// Package parsing holds the state of one read and the text pre-pass that every text reader runs.
package parsing

import (
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const defaultLastCueDuration = 5 * time.Second

// State holds the options and the warnings of one read.
type State struct {
	// ReplaceNUL makes Prepare read each NUL as U+FFFD, as the WebVTT spec says.
	ReplaceNUL bool

	format   model.Format
	common   model.CommonReadOptions
	warnings []warning
}

// warning holds a warning until model.ParseWarning gets its fields in #17.
type warning struct {
	msg    string
	line   int
	action model.WarningAction
}

// NewState returns the state of one read of format f.
func NewState(f model.Format, common model.CommonReadOptions) *State {
	return &State{format: f, common: common}
}

// Lenient reports whether the read skips or repairs broken blocks.
func (s *State) Lenient() bool {
	return s.common.Lenient
}

// LastCueDuration returns the length of a last cue without an end, 5 s when the option is nil.
func (s *State) LastCueDuration() time.Duration {
	if s.common.LastCueDuration == nil {
		return defaultLastCueDuration
	}
	return *s.common.LastCueDuration
}

// Warnings returns the warnings of the read, in read order.
func (s *State) Warnings() []model.ParseWarning {
	if len(s.warnings) == 0 {
		return nil
	}
	return make([]model.ParseWarning, len(s.warnings))
}

func (s *State) warn(msg string, line int, action model.WarningAction) {
	s.warnings = append(s.warnings, warning{msg: msg, line: line, action: action})
}

// EndAtNextStart returns the first start after starts[i] that is later than starts[i].
// Without one, it returns starts[i] plus the last cue duration, rounded to whole milliseconds.
func (s *State) EndAtNextStart(starts []time.Duration, i int) time.Duration {
	for _, next := range starts[i+1:] {
		if next > starts[i] {
			return next
		}
	}
	return (starts[i] + s.LastCueDuration()).Round(time.Millisecond)
}

// NoCues returns the error "The file has text but no cues." at the line that textLine finds,
// when the read gave no cue and skipped no block. Lenient mode records a skipped warning instead.
// Only a reader that skips unknown text without an error calls it.
func (s *State) NoCues(cues int, textLine func() (int, bool)) error {
	if cues > 0 || slices.ContainsFunc(s.warnings, func(w warning) bool { return w.action == model.Skipped }) {
		return nil
	}
	line, ok := textLine()
	if !ok {
		return nil
	}
	const msg = "The file has text but no cues."
	if !s.Lenient() {
		return &model.ParseError{Format: s.format, Line: line, Msg: msg}
	}
	s.warn(msg, line, model.Skipped)
	return nil
}

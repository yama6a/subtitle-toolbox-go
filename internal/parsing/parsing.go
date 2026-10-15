// Package parsing holds the state of one read and the text pre-pass that every text reader runs.
package parsing

import (
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const defaultLastCueDuration = 5 * time.Second

// MaxSwappedDuration is the longest cue that lenient mode makes by a swap. A longer one is no typo.
const MaxSwappedDuration = 30 * time.Second

// State holds the options and the warnings of one read.
type State struct {
	// ReplaceNUL makes Prepare read each NUL as U+FFFD, as the WebVTT spec says.
	ReplaceNUL bool

	format   model.Format
	common   model.CommonReadOptions
	warnings []model.ParseWarning
	// badUTF8 and controls are the pre-pass warnings of a LineReader. They come first in the list,
	// also when the reader finds them after the warnings of earlier blocks.
	badUTF8, controls *model.ParseWarning
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

// Warnings returns a copy of the warnings of the read, in read order.
func (s *State) Warnings() []model.ParseWarning {
	if s.badUTF8 == nil && s.controls == nil {
		return slices.Clone(s.warnings)
	}
	out := make([]model.ParseWarning, 0, len(s.warnings)+2)
	for _, w := range []*model.ParseWarning{s.badUTF8, s.controls} {
		if w != nil {
			out = append(out, *w)
		}
	}
	return append(out, s.warnings...)
}

// Warn records a warning with a copy of lines. It does not check the mode, so strict reads can warn too.
func (s *State) Warn(msg string, line, block int, lines []string, action model.WarningAction) {
	s.warnings = append(s.warnings, newWarning(msg, line, block, lines, action))
}

func newWarning(msg string, line, block int, lines []string, action model.WarningAction) model.ParseWarning {
	return model.ParseWarning{
		Message: msg,
		Line:    line,
		Block:   block,
		Lines:   slices.Clone(lines),
		Action:  action,
	}
}

func (s *State) warn(msg string, line int, action model.WarningAction) {
	s.Warn(msg, line, model.NoBlock, nil, action)
}

// Fail returns err in strict mode, with the format of the read when err has none.
// In lenient mode it records a Skipped warning with the message of err and returns nil.
// With line NoLine, the message ends with the line of err, so the line is not lost.
func (s *State) Fail(err *model.ParseError, line, block int, lines []string) error {
	if !s.Lenient() {
		if err.Format != "" {
			return err
		}
		withFormat := *err
		withFormat.Format = s.format
		return &withFormat
	}
	msg := err.Msg
	if msg == "" && err.Err != nil {
		msg = err.Err.Error()
	}
	if line == model.NoLine && err.Line > 0 {
		msg += " (line " + strconv.Itoa(err.Line) + ")"
	}
	s.Warn(msg, line, block, lines, model.Skipped)
	return nil
}

// OrderedTimes returns start and end when end is not before start. Strict mode returns an error otherwise.
// Lenient mode swaps the times and warns when canSwap is true and the cue then lasts 30 s or less, else it
// returns the error too, for the caller to pass to Fail.
func (s *State) OrderedTimes(start, end time.Duration, line, block int, lines []string, canSwap bool) (time.Duration, time.Duration, error) {
	if end >= start {
		return start, end, nil
	}
	msg := "The cue ends at " + timecode.Seconds(end) + " s, before it starts at " + timecode.Seconds(start) + " s."
	if !s.Lenient() || !canSwap || start-end > MaxSwappedDuration {
		return 0, 0, &model.ParseError{Format: s.format, Line: line, Msg: msg}
	}
	s.Warn(msg+" The parser swapped the times.", line, block, lines, model.Repaired)
	return end, start, nil
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
	if cues > 0 || slices.ContainsFunc(s.warnings, func(w model.ParseWarning) bool { return w.Action == model.Skipped }) {
		return nil
	}
	line, ok := textLine()
	if !ok {
		return nil
	}
	return s.Fail(&model.ParseError{Format: s.format, Line: line, Msg: "The file has text but no cues."}, line, model.NoBlock, nil)
}

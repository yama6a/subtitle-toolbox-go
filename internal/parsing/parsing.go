// Package parsing holds the state of one read and the text pre-pass that every text reader runs.
package parsing

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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
	return LastCueDuration(s.common)
}

// LastCueDuration returns the length of a last cue without an end, 5 s when the option is nil.
func LastCueDuration(common model.CommonReadOptions) time.Duration {
	if common.LastCueDuration == nil {
		return defaultLastCueDuration
	}
	return *common.LastCueDuration
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

// Fail returns err unchanged when it holds no *model.ParseError. In strict mode it returns the parse error, with
// the format of the read when it has none. In lenient mode it records a Skipped warning with the message of the
// parse error as a sentence and returns nil. With line NoLine, the message ends with the line of the parse error.
func (s *State) Fail(err error, line, block int, lines []string) error {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return err
	}
	if !s.Lenient() {
		if perr.Format != "" {
			return perr
		}
		withFormat := *perr
		withFormat.Format = s.format
		return &withFormat
	}
	msg := Sentence(perr.Msg)
	if msg == "" && perr.Err != nil {
		msg = perr.Err.Error()
	}
	if line == model.NoLine && perr.Line > 0 {
		msg += " (line " + strconv.Itoa(perr.Line) + ")"
	}
	s.Warn(msg, line, block, lines, model.Skipped)
	return nil
}

// Capitalize returns msg with its first letter in upper case. A message that follows an earlier sentence of an
// error message starts this way.
func Capitalize(msg string) string {
	r, size := utf8.DecodeRuneInString(msg)
	if size == 0 {
		return msg
	}
	return string(unicode.ToUpper(r)) + msg[size:]
}

// Sentence returns the message of an error as the sentence of a warning: the first letter in upper case and a final
// period. A lenient read records the sentence, because ParseWarning.Message is a full sentence.
func Sentence(msg string) string {
	msg = Capitalize(msg)
	if msg != "" && !strings.HasSuffix(msg, ".") {
		msg += "."
	}
	return msg
}

// OrderedTimes returns start and end when end is not before start. Strict mode returns an error otherwise.
// Lenient mode swaps the times and warns when canSwap is true and the cue then lasts 30 s or less, else it
// returns the error too, for the caller to pass to Fail.
func (s *State) OrderedTimes(start, end time.Duration, line, block int, lines []string, canSwap bool) (time.Duration, time.Duration, error) {
	if end >= start {
		return start, end, nil
	}
	msg := "the cue ends at " + timecode.Seconds(end) + " s, before it starts at " + timecode.Seconds(start) + " s"
	if !s.Lenient() || !canSwap || start-end > MaxSwappedDuration {
		return 0, 0, &model.ParseError{Format: s.format, Line: line, Msg: msg}
	}
	s.Warn(Sentence(msg)+" The parser swapped the times.", line, block, lines, model.Repaired)
	return end, start, nil
}

// LastCueEnd returns the end of a last cue without an end: EndAfter of start and the last cue duration.
func (s *State) LastCueEnd(start time.Duration) time.Duration {
	return EndAfter(start, s.LastCueDuration())
}

// EndAfter returns the end of a cue that lasts d from start, rounded to whole milliseconds. An end at or beyond the
// time limit becomes timecode.MaxValidTime, so every writer output reads back.
func EndAfter(start, d time.Duration) time.Duration {
	end := start + d
	if d > 0 && end < start {
		end = math.MaxInt64
	}
	return min(end.Round(time.Millisecond), timecode.MaxValidTime)
}

// NoCues returns the error "the file has text but no cues" at the line that textLine finds,
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
	return s.Fail(&model.ParseError{Format: s.format, Line: line, Msg: "the file has text but no cues"}, line, model.NoBlock, nil)
}

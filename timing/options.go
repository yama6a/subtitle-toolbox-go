package timing

import (
	"errors"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Defaults of the frame counts of Options.
const (
	DefaultMinGapFrames      = 2
	DefaultMinDurationFrames = 20
)

// Options holds the settings of Apply.
type Options struct {
	// FrameRate is the frame rate of the video. It is required.
	FrameRate model.FrameRate
	// ShotChanges holds the shot change times. Empty: Apply only closes gaps and puts the times on frames.
	ShotChanges []time.Duration
	// SnapWindowFrames is the largest move to a shot change and the largest gap that closes, in frames. nil means DefaultSnapWindow(FrameRate).
	SnapWindowFrames *int
	// MinGapFrames is the frames between a cue and the next cue or shot change. nil means DefaultMinGapFrames.
	MinGapFrames *int
	// MinDurationFrames is the shortest cue that a move leaves, in frames. nil means DefaultMinDurationFrames.
	MinDurationFrames *int
	// KeepGaps keeps the gaps shorter than the snap window, which Apply closes by default.
	KeepGaps bool
}

// DefaultSnapWindow returns half of r in frames, rounded to the nearest frame, with a half frame rounded down.
// It returns 0 for a rate that is 1 frame per second or less.
func DefaultSnapWindow(r model.FrameRate) int {
	if r.Num <= r.Den || r.Den <= 0 {
		return 0
	}
	return int((r.Num - r.Den + 2*r.Den - 1) / (2 * r.Den))
}

// Validate returns an *model.InvalidArgumentError for a frame rate that is not valid and for a negative frame count.
func (o Options) Validate() error {
	if o.FrameRate.Num <= 0 || o.FrameRate.Den <= 0 {
		return invalid("FrameRate", "The frame rate of the video must be greater than 0.")
	}
	if err := o.FrameRate.Validate(); err != nil {
		return invalid("FrameRate", message(err))
	}
	for _, c := range []struct {
		field, name string
		value       *int
	}{
		{"SnapWindowFrames", "snap window", o.SnapWindowFrames},
		{"MinGapFrames", "minimum gap", o.MinGapFrames},
		{"MinDurationFrames", "minimum duration", o.MinDurationFrames},
	} {
		if c.value != nil && *c.value < 0 {
			return invalid(c.field, "The "+c.name+" must not be negative, got "+strconv.Itoa(*c.value)+".")
		}
	}
	return nil
}

func (o Options) window() int64 {
	if o.SnapWindowFrames == nil {
		return int64(DefaultSnapWindow(o.FrameRate))
	}
	return int64(*o.SnapWindowFrames)
}

func (o Options) minGap() int64 {
	if o.MinGapFrames == nil {
		return DefaultMinGapFrames
	}
	return int64(*o.MinGapFrames)
}

func (o Options) minDuration() int64 {
	if o.MinDurationFrames == nil {
		return DefaultMinDurationFrames
	}
	return int64(*o.MinDurationFrames)
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "timing.Options." + field, Msg: msg}
}

func message(err error) string {
	if aerr, ok := errors.AsType[*model.InvalidArgumentError](err); ok {
		return aerr.Msg
	}
	return err.Error()
}

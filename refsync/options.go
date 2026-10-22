package refsync

import (
	"fmt"
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// The defaults and limits of Options.
const (
	// DefaultMinOffset is the lowest offset that Apply tries when Options.MinOffset is nil.
	DefaultMinOffset = -60 * time.Second
	// DefaultMaxOffset is the highest offset that Apply tries when Options.MaxOffset is nil.
	DefaultMaxOffset = 60 * time.Second
	// DefaultSplitPenalty is the score that a split must add when Options.SplitPenalty is nil.
	DefaultSplitPenalty = 0.1
	// MaxOffset is the largest offset before or after, one day.
	MaxOffset = 86400 * time.Second
	// MaxOffsetRange is the largest distance between the minimum and the maximum offset. The split search keeps
	// a value for each 0.1 s of it.
	MaxOffsetRange = 7200 * time.Second
	// MaxSplits is the largest value of Options.MaxSplits.
	MaxSplits = 10
)

// Options holds the settings of Apply.
type Options struct {
	// Reference is the subtitle whose cue times the target gets. It is required.
	Reference *model.Subtitle
	// MinOffset is the lowest offset to try, rounded to whole milliseconds. Nil means DefaultMinOffset.
	MinOffset *time.Duration
	// MaxOffset is the highest offset to try, rounded to whole milliseconds. Nil means DefaultMaxOffset.
	MaxOffset *time.Duration
	// NoScale keeps the scale at 1 and finds only the offset. False also tries the frame rate factors.
	NoScale bool
	// MaxSplits is the most points where the offset jumps, from 0 to MaxSplits. 0 turns the split search off.
	MaxSplits int
	// SplitPenalty is the score from 0 to 1 that each split must add to stay. Nil means DefaultSplitPenalty.
	SplitPenalty *float64
}

// Validate returns an error that matches model.ErrInvalidArgument when a field of o is out of range.
func (o Options) Validate() error {
	if o.Reference == nil {
		return invalid("Reference", "The reference subtitle is missing.")
	}
	if err := checkOffsets(o.offsets()); err != nil {
		return err
	}
	if o.MaxSplits < 0 || o.MaxSplits > MaxSplits {
		return invalid("MaxSplits", fmt.Sprintf("The maximum number of splits must be from 0 to %d, got %d.",
			MaxSplits, o.MaxSplits))
	}
	if p := o.splitPenalty(); !(p >= 0) || math.IsInf(p, 1) {
		return invalid("SplitPenalty", "The split penalty must be a finite number of 0 or more, got "+timecode.Number(p)+".")
	}
	return nil
}

func checkOffsets(lo, hi time.Duration) error {
	for _, bound := range []struct {
		field, name string
		d           time.Duration
	}{{"MinOffset", "minimum", lo}, {"MaxOffset", "maximum", hi}} {
		if bound.d < -MaxOffset || bound.d > MaxOffset {
			return invalid(bound.field, fmt.Sprintf("The %s offset must be from -%d to %d seconds, got %s.",
				bound.name, MaxOffset/time.Second, MaxOffset/time.Second, timecode.Seconds(bound.d)))
		}
	}
	if lo > hi {
		return invalid("MinOffset", fmt.Sprintf("The minimum offset must not be greater than the maximum offset, got %s and %s.",
			timecode.Seconds(lo), timecode.Seconds(hi)))
	}
	if hi-lo > MaxOffsetRange {
		return invalid("MaxOffset", fmt.Sprintf("The minimum and the maximum offset must be at most %d seconds apart, got %s and %s.",
			MaxOffsetRange/time.Second, timecode.Seconds(lo), timecode.Seconds(hi)))
	}
	return nil
}

func (o Options) offsets() (lo, hi time.Duration) {
	lo, hi = DefaultMinOffset, DefaultMaxOffset
	if o.MinOffset != nil {
		lo = o.MinOffset.Round(time.Millisecond)
	}
	if o.MaxOffset != nil {
		hi = o.MaxOffset.Round(time.Millisecond)
	}
	return lo, hi
}

func (o Options) splitPenalty() float64 {
	if o.SplitPenalty == nil {
		return DefaultSplitPenalty
	}
	return *o.SplitPenalty
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "refsync.Options." + field, Msg: msg}
}

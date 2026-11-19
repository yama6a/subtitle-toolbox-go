package resegment

import (
	"cmp"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// DefaultMaxWordGap is the pause at which ByWords ends a cue.
const DefaultMaxWordGap = 600 * time.Millisecond

// Options holds the settings of SplitLong and ByWords.
type Options struct {
	// Limits holds the limits of each new cue. The zero value uses the defaults of model.CueLimits.
	Limits model.CueLimits
	// MaxWordGap makes ByWords end a cue before a word that starts this long or longer after the end of the word
	// before it. Nil means DefaultMaxWordGap. SplitLong ignores it.
	MaxWordGap *time.Duration
}

// Validate returns an *model.InvalidArgumentError for limits out of range and for a negative MaxWordGap.
func (o Options) Validate() error {
	_, err := o.resolve()
	return err
}

func (o Options) resolve() (settings, error) {
	if err := o.Limits.ValidateAt("resegment.Options.Limits"); err != nil {
		return settings{}, err
	}
	l := o.Limits
	s := settings{
		cpl:    cmp.Or(l.MaxCharactersPerLine, model.DefaultMaxCharactersPerLine),
		lines:  cmp.Or(l.MaxLinesPerCue, model.DefaultMaxLinesPerCue),
		minDur: model.DefaultMinDuration,
		maxDur: cmp.Or(l.MaxDuration, model.DefaultMaxDuration),
		cps:    l.MaxCharactersPerSecond,
		maxGap: DefaultMaxWordGap,
	}
	if l.MinDuration != nil {
		s.minDur = *l.MinDuration
	}
	if o.MaxWordGap != nil {
		s.maxGap = *o.MaxWordGap
	}
	if s.maxGap < 0 {
		return s, &model.InvalidArgumentError{
			Option: "resegment.Options.MaxWordGap",
			Msg:    "the maximum word gap must not be negative, got " + timecode.Seconds(s.maxGap),
		}
	}
	s.minDur, s.maxDur = s.minDur.Round(time.Millisecond), s.maxDur.Round(time.Millisecond)
	s.maxGap = s.maxGap.Round(time.Millisecond)
	return s, nil
}

package diff

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// DefaultTimeTolerance is the time tolerance that Compare uses when Options.TimeTolerance is nil.
const DefaultTimeTolerance = time.Millisecond

// Options holds the settings of Compare.
type Options struct {
	// TimeTolerance is the largest start or end difference that counts as the same time. Nil means DefaultTimeTolerance.
	// 0 compares the times exactly.
	TimeTolerance *time.Duration
	// IgnoreFormatting compares the text without tags and with entities decoded.
	IgnoreFormatting bool
	// IgnoreWhitespace compares the text without spaces, tabs and line breaks.
	IgnoreWhitespace bool
	// TextOnly reports no timing changes. The times still help to pair the cues.
	TextOnly bool
}

// Validate returns an error that matches model.ErrInvalidArgument when the time tolerance is negative.
func (o Options) Validate() error {
	if o.TimeTolerance != nil && *o.TimeTolerance < 0 {
		return &model.InvalidArgumentError{
			Option: "diff.Options.TimeTolerance",
			Msg:    "The time tolerance must not be negative, got " + timecode.Seconds(*o.TimeTolerance) + ".",
		}
	}
	return nil
}

func (o Options) timeTolerance() time.Duration {
	if o.TimeTolerance == nil {
		return DefaultTimeTolerance
	}
	return *o.TimeTolerance
}

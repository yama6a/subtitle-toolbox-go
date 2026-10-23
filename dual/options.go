package dual

import (
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Mode is how the secondary cues join the primary cues. The values are the CLI values.
type Mode string

// The modes of FromPair.
const (
	ModeStack     Mode = "stack"      // the secondary lines go below the primary lines of the cue they overlap most
	ModeTopBottom Mode = "top-bottom" // the secondary cues stay cues of their own at the secondary alignment
)

const (
	// DefaultTimeTolerance is the time tolerance that FromPair uses when Options.TimeTolerance is nil.
	DefaultTimeTolerance = 250 * time.Millisecond
	// DefaultSecondaryAlignment is the alignment that FromPair uses when Options.SecondaryAlignment is nil.
	DefaultSecondaryAlignment = model.TopCenter
)

// Options holds the settings of FromPair.
type Options struct {
	// Mode is how the secondary cues join the primary cues. The empty string acts as ModeStack.
	Mode Mode
	// TimeTolerance is the largest distance at which a secondary time moves to a primary time in ModeTopBottom.
	// Nil means DefaultTimeTolerance. 0 is legal and snaps only to an equal time.
	TimeTolerance *time.Duration
	// SecondaryStyle is a core markup tag around each secondary line, such as "i" or `font color="#ffff00"`.
	// The empty string adds none.
	SecondaryStyle string
	// SecondaryAlignment is the alignment of the secondary cues in ModeTopBottom. Nil means DefaultSecondaryAlignment.
	// Use a pointer to model.BottomCenter to put them at the bottom.
	SecondaryAlignment *model.Alignment
}

// Validate returns an error that matches model.ErrInvalidArgument when a field of o is out of range.
func (o Options) Validate() error {
	if o.TimeTolerance != nil && *o.TimeTolerance < 0 {
		return invalid("TimeTolerance", "The time tolerance "+timecode.Seconds(*o.TimeTolerance)+" must not be negative.")
	}
	if o.SecondaryStyle != "" && !markupcore.IsCoreTag(markupcore.TagName(o.SecondaryStyle)) {
		return invalid("SecondaryStyle", "The secondary style "+o.SecondaryStyle+
			` must be a core markup tag such as i or font color="#ffff00".`)
	}
	switch o.Mode {
	case "", ModeStack, ModeTopBottom:
	default:
		return invalid("Mode", `The mode "`+string(o.Mode)+`" is not stack or top-bottom.`)
	}
	if a := o.SecondaryAlignment; a != nil && *a > model.TopRight {
		return invalid("SecondaryAlignment", "The secondary alignment "+a.String()+" is not one of the 9 alignment constants.")
	}
	return nil
}

func (o Options) timeTolerance() time.Duration {
	if o.TimeTolerance == nil {
		return DefaultTimeTolerance
	}
	return *o.TimeTolerance
}

func (o Options) secondaryAlignment() model.Alignment {
	if o.SecondaryAlignment == nil {
		return DefaultSecondaryAlignment
	}
	return *o.SecondaryAlignment
}

// style wraps line in the secondary style, or returns it when there is none. The tag name is lower case in both tags,
// because WebVTT tags are case-sensitive.
func (o Options) style(line string) string {
	if o.SecondaryStyle == "" {
		return line
	}
	open := strings.Trim(o.SecondaryStyle, markupcore.PHPTrimSet)
	name := markupcore.TagName(open)
	return "<" + name + open[len(name):] + ">" + line + "</" + name + ">"
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "dual.Options." + field, Msg: msg}
}

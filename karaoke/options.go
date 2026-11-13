package karaoke

import (
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Mode selects which words the style marks.
type Mode uint8

// The modes of Apply.
const (
	ModeWord       Mode = iota // only the active word
	ModeCumulative             // every word up to the active word
)

// String returns "word" or "cumulative", else "Mode(n)".
func (m Mode) String() string {
	switch m {
	case ModeWord:
		return "word"
	case ModeCumulative:
		return "cumulative"
	}
	return "Mode(" + strconv.Itoa(int(m)) + ")"
}

// Options holds the settings of Apply. The zero value underlines the active word and shows all words.
type Options struct {
	// Style is the tag of the active word without its brackets, such as "b" or `font color="#ffff00"`. "" means "u".
	Style string
	// Mode selects which words the style marks.
	Mode Mode
	// MaxWordsPerCue is the number of words that one cue shows around the active word. 0 shows all words.
	MaxWordsPerCue int
}

// Validate returns a *model.InvalidArgumentError when a field of o is out of range.
func (o Options) Validate() error {
	style := o.style()
	switch {
	case !markupcore.IsStyleTag(markupcore.TagName(style)):
		return invalid("Style", "The style "+style+` must be one of the core markup tags b, i, u, s or font, for example u or font color="#ffff00".`)
	case strings.ContainsAny(style, "<>"):
		return invalid("Style", "The style "+style+" must not hold < or >. Pass the tag without its brackets, for example u.")
	case o.MaxWordsPerCue < 0:
		return invalid("MaxWordsPerCue", "The maximum of "+strconv.Itoa(o.MaxWordsPerCue)+" words per cue must be 1 or more.")
	}
	return nil
}

func (o Options) style() string {
	if o.Style == "" {
		return "u"
	}
	return o.Style
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "karaoke.Options." + field, Msg: msg}
}

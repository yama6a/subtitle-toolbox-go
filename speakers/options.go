package speakers

import (
	"regexp"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Options selects the steps of Apply. The zero value changes nothing.
type Options struct {
	// FromLabels reads labels such as "JOHN: " into speakers.
	FromLabels bool
	// AnyCaseLabels makes FromLabels also read labels with lower case letters, such as "Baker: ".
	AnyCaseLabels bool
	// Rename holds the new name of each speaker, for example {"SPEAKER_00": "Anna"}.
	Rename map[string]string
	// Style writes the speakers in this style and removes them as speakers.
	Style Style
	// KeepNameCase makes StylePrefix write the name as it is, not in upper case.
	KeepNameCase bool
	// Separator is the text after a StylePrefix name. Nil means ": ". The empty string is legal.
	Separator *string
	// DialogueDashStyle is the dash of StyleDialogueDashes.
	DialogueDashStyle model.DialogueDashStyle
	// Colors are the "#rrggbb" colors of StyleColors, used in turn. Empty means BBCColors.
	Colors []string
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// BBCColors returns white, yellow, cyan and green, the speaker colors of the BBC Subtitle Guidelines, in their order of use.
//
// Behaviour reference: BBC Subtitle Guidelines (https://www.bbc.co.uk/accessibility/forproducts/guides/subtitles/).
func BBCColors() []string {
	return []string{"#ffffff", "#ffff00", "#00ffff", "#00ff00"}
}

// Validate returns an error that matches model.ErrInvalidArgument when Style is not one of the constants,
// DialogueDashStyle is not one of the 8 constants, or a color is not of the form "#rrggbb".
func (o Options) Validate() error {
	if o.Style > StyleColors {
		return &model.InvalidArgumentError{Option: "speakers.Options.Style", Msg: "The speaker style must be one of the Style constants, got " + o.Style.String() + "."}
	}
	if o.DialogueDashStyle.Text() == "" {
		return &model.InvalidArgumentError{Option: "speakers.Options.DialogueDashStyle", Msg: "The dialogue dash style must be one of the 8 constants, got " + o.DialogueDashStyle.String() + "."}
	}
	for _, c := range o.Colors {
		if !colorPattern.MatchString(c) {
			return &model.InvalidArgumentError{Option: "speakers.Options.Colors", Msg: `The speaker colors must be a list of colors such as "#ffff00", got ` + strconv.Quote(c) + "."}
		}
	}
	return nil
}

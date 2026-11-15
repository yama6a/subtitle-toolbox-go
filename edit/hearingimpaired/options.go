package hearingimpaired

import (
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Options selects the annotations to remove. The zero value removes brackets, parentheses, upper case speaker
// labels and music-only lines.
type Options struct {
	// KeepSquareBrackets keeps text in square brackets, such as [DOOR SLAMS].
	KeepSquareBrackets bool
	// KeepParentheses keeps text in parentheses, such as (laughs).
	KeepParentheses bool
	// KeepSpeakerLabels keeps labels such as JOHN: at the start of a line.
	KeepSpeakerLabels bool
	// AnyCaseLabels also removes labels with lower case letters, such as Baker:, and so also Note:.
	AnyCaseLabels bool
	// KeepMusicLines keeps lines that hold only music symbols.
	KeepMusicLines bool
	// CustomBrackets removes the text between each pair too, before the square brackets and parentheses.
	CustomBrackets []BracketPair
	// Lyrics removes text between 2 music symbols, and lines that start or end with one.
	Lyrics bool
}

// BracketPair is an opening and a closing string, for example "{" and "}".
type BracketPair struct {
	Open, Close string
}

// Validate returns an error that matches model.ErrInvalidArgument when a custom bracket pair has an empty string.
func (o Options) Validate() error {
	for _, p := range o.CustomBrackets {
		if p.Open == "" || p.Close == "" {
			return &model.InvalidArgumentError{
				Option: "hearingimpaired.Options.CustomBrackets",
				Msg:    `each custom bracket pair must hold two non-empty strings, for example "{" and "}"`,
			}
		}
	}
	return nil
}

package profanity

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Options holds the settings of Apply.
type Options struct {
	// Words holds one word or phrase each. A * at the end matches any ending, so damn* matches damned.
	Words []string
	// Mask is how Apply writes a match. "" acts as MaskStars.
	Mask Mask
	// Padding widens each mute range on both sides.
	Padding time.Duration
}

// Validate returns an error that matches model.ErrInvalidArgument when a word is not valid, the word list is
// empty, Padding is negative or Mask is unknown.
func (o Options) Validate() error {
	for _, w := range o.Words {
		if !validWord(w) {
			return invalid("Words", "A profanity word must be a non-empty UTF-8 string with a * only at the end, got "+
				quoteString(w)+".")
		}
	}
	if len(o.Words) == 0 {
		return invalid("Words", "The profanity word list is empty.")
	}
	if o.Padding < 0 {
		return invalid("Padding", "The padding must not be negative, got "+timecode.Seconds(o.Padding)+".")
	}
	switch o.Mask {
	case "", MaskStars, MaskFirstLetter, MaskRemove, MaskNone:
		return nil
	default:
		return invalid("Mask", "The mask "+strconv.Quote(string(o.Mask))+" is not stars, first-letter, remove or none.")
	}
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "profanity.Options." + field, Msg: msg}
}

// validWord checks the pattern ^[^*\s](?:[^*]*[^*\s])?\*?$ with \s as Unicode white space.
func validWord(w string) bool {
	if !utf8.ValidString(w) {
		return false
	}
	w = strings.TrimSuffix(w, "*")
	if w == "" || strings.Contains(w, "*") {
		return false
	}
	first, _ := utf8.DecodeRuneInString(w)
	last, _ := utf8.DecodeLastRuneInString(w)
	return !markupcore.IsSpace(first) && !markupcore.IsSpace(last)
}

// quoteString writes s as a single-quoted string that escapes \ and '.
func quoteString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

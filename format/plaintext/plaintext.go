// Package plaintext writes the words of a subtitle as a readable transcript, without times or markup.
//
// Cues that follow each other closely join into one paragraph, and a pause of 2 s or more starts a new one.
// The format has no reader and no signature. The extension is .txt.
package plaintext

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// NoParagraphBreaks is the ParagraphGap that writes all cues as one paragraph.
const NoParagraphBreaks = time.Duration(math.MaxInt64)

const defaultParagraphGap = 2 * time.Second

// WriteOptions holds the settings of the plain text writer.
type WriteOptions struct {
	// KeepLineBreaks writes each line of a cue on its own line. False joins them with a space.
	KeepLineBreaks bool
	// OneCuePerLine writes each cue of a paragraph on its own line. False joins them with a space.
	OneCuePerLine bool
	// ParagraphGap is the gap after the latest end so far that starts a new paragraph. Nil means 2 s.
	ParagraphGap *time.Duration
	// WithTimes starts each paragraph with its start time, such as [00:01:02].
	WithTimes bool
}

// Validate returns an error that matches model.ErrInvalidArgument for a negative paragraph gap.
func (o WriteOptions) Validate() error {
	if o.ParagraphGap != nil && *o.ParagraphGap < 0 {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.PlainText.ParagraphGap",
			Msg:    "The paragraph gap must be 0 or more seconds, got " + timecode.Seconds(*o.ParagraphGap) + ".",
		}
	}
	return nil
}

func (o WriteOptions) gap() time.Duration {
	if o.ParagraphGap == nil {
		return defaultParagraphGap
	}
	return *o.ParagraphGap
}

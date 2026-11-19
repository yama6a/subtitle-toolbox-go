// Package plaintext writes the words of a subtitle as a readable transcript, without times or markup.
//
// Cues that follow each other closely join into one paragraph, and a pause of 2 s or more starts a new one.
// The format has no reader and no signature. The extension is .txt.
package plaintext

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/paragraphs"
)

// NoParagraphBreaks is the ParagraphGap that writes all cues as one paragraph.
const NoParagraphBreaks = time.Duration(math.MaxInt64)

// ReadOptions holds the settings of a plain text reader. Plain text has no reader, so it has no fields.
type ReadOptions struct{}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

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
	return paragraphs.ValidateGap(o.ParagraphGap, "WriteOptions.PlainText.ParagraphGap")
}

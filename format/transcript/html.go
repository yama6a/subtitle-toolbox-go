package transcript

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// NoParagraphBreaks is the ParagraphGap that writes all cues as one paragraph. It has the value of
// plaintext.NoParagraphBreaks.
const NoParagraphBreaks = time.Duration(math.MaxInt64)

const defaultParagraphGap = 2 * time.Second

// HTMLWriteOptions holds the settings of the HTML transcript writer.
type HTMLWriteOptions struct {
	// ParagraphGap is the gap after the latest end so far that starts a new paragraph. Nil means 2 s. 0 is legal.
	ParagraphGap *time.Duration
}

// Validate returns an error that matches model.ErrInvalidArgument for a negative paragraph gap.
func (o HTMLWriteOptions) Validate() error {
	if o.ParagraphGap != nil && *o.ParagraphGap < 0 {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.Transcript.HTML.ParagraphGap",
			Msg:    "The paragraph gap must be 0 or more seconds, got " + timecode.Seconds(*o.ParagraphGap) + ".",
		}
	}
	return nil
}

func (o HTMLWriteOptions) gap() time.Duration {
	if o.ParagraphGap == nil {
		return defaultParagraphGap
	}
	return *o.ParagraphGap
}

// DetectHTML reports whether content starts with "<" after any white space, and holds the start tags <cite and
// <time, each followed by white space or ">", in any case. The XML prolog items all start with "<", so they need
// no special rule. stbox.Detect runs this check last, because TTML, SAMI and YouTube XML can hold these names.
func DetectHTML(content []byte) bool {
	i := 0
	for i < len(content) && isHTMLSpace(content[i]) {
		i++
	}
	if i == len(content) || content[i] != '<' {
		return false
	}
	text := string(content[i:])
	return hasStartTag(text, "cite") && hasStartTag(text, "time")
}

func hasStartTag(s, name string) bool {
	for i := 0; i < len(s); {
		j := indexByteFrom(s, '<', i)
		if j < 0 {
			return false
		}
		if startTagAt(s, j, name) {
			return true
		}
		i = j + 1
	}
	return false
}

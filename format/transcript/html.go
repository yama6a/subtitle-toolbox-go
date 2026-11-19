package transcript

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/paragraphs"
)

// NoParagraphBreaks is the ParagraphGap that writes all cues as one paragraph. It has the value of
// plaintext.NoParagraphBreaks.
const NoParagraphBreaks = time.Duration(math.MaxInt64)

// HTMLWriteOptions holds the settings of the HTML transcript writer.
type HTMLWriteOptions struct {
	// ParagraphGap is the gap after the latest end so far that starts a new paragraph. Nil means 2 s. 0 is legal.
	ParagraphGap *time.Duration
}

// Validate returns an error that matches model.ErrInvalidArgument for a negative paragraph gap.
func (o HTMLWriteOptions) Validate() error {
	return paragraphs.ValidateGap(o.ParagraphGap, "WriteOptions.Transcript.HTML.ParagraphGap")
}

// DetectHTML reports whether content starts with "<" after any white space, and holds the start tags <cite and
// <time, each followed by white space or ">", in any case. The XML prolog items all start with "<", so they need
// no special rule. A detector must try this check last, because TTML, SAMI and YouTube XML can hold these names.
func DetectHTML(content []byte) bool {
	i := 0
	for i < len(content) && isHTMLSpace(content[i]) {
		i++
	}
	if i == len(content) || content[i] != '<' {
		return false
	}
	return hasStartTag(content[i:], "cite") && hasStartTag(content[i:], "time")
}

func hasStartTag[T string | []byte](s T, name string) bool {
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

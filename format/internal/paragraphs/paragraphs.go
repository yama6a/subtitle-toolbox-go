// Package paragraphs groups timed items into paragraphs for the plain text and HTML transcript writers.
package paragraphs

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Item is a value with the time range of its cue.
type Item[V any] struct {
	Start, End time.Duration
	Value      V
	// NewParagraph starts a paragraph at this item, whatever the gap.
	NewParagraph bool
}

// Paragraph holds the values of one paragraph and the start of its first item.
type Paragraph[V any] struct {
	Start  time.Duration
	Values []V
}

// DefaultGap is the paragraph gap of a nil ParagraphGap option.
const DefaultGap = 2 * time.Second

// Gap returns *gap, or DefaultGap for nil.
func Gap(gap *time.Duration) time.Duration {
	if gap == nil {
		return DefaultGap
	}
	return *gap
}

// ValidateGap returns an error that matches model.ErrInvalidArgument for a negative gap. option names the
// option field, such as "WriteOptions.PlainText.ParagraphGap".
func ValidateGap(gap *time.Duration, option string) error {
	if gap != nil && *gap < 0 {
		return &model.InvalidArgumentError{
			Option: option,
			Msg:    "the paragraph gap must be 0 or more seconds, got " + timecode.Seconds(*gap),
		}
	}
	return nil
}

// ByGap groups items, which are in start order, into paragraphs. An item starts a new paragraph when it is the
// first, when its start minus the latest end so far is gap or more, or when its NewParagraph is true.
func ByGap[V any](items []Item[V], gap time.Duration) []Paragraph[V] {
	var out []Paragraph[V]
	var latestEnd time.Duration
	for i, it := range items {
		if i == 0 || it.Start-latestEnd >= gap || it.NewParagraph {
			out = append(out, Paragraph[V]{Start: it.Start})
		}
		last := &out[len(out)-1]
		last.Values = append(last.Values, it.Value)
		if i == 0 {
			latestEnd = it.End
		}
		latestEnd = max(latestEnd, it.End)
	}
	return out
}

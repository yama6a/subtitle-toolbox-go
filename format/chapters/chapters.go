// Package chapters reads and writes chapter lists. A chapter list is a subtitle whose cues are chapters.
//
// Each cue holds the escaped title as its only line. A chapter without a title has no lines.
package chapters

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// ReadOptions holds the read settings of the chapter formats.
type ReadOptions struct {
	// MediaDuration is where a last chapter without its own end ends. A last chapter that starts after it ends at
	// its own start. 0 means unknown.
	MediaDuration time.Duration
}

// WriteOptions holds the write settings of the chapter formats. It has no fields.
type WriteOptions struct{}

// Validate returns a *model.InvalidArgumentError when MediaDuration is negative.
func (o ReadOptions) Validate() error {
	if o.MediaDuration < 0 {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.Chapters.MediaDuration",
			Msg:    fmt.Sprintf("The media duration must be 0 or more seconds, got %s.", timecode.Seconds(o.MediaDuration)),
		}
	}
	return nil
}

// endChapters sorts cues by start, stable, and sets each end. ends[i] is the end that the file gives for cues[i],
// or nil. ends can be nil or shorter than cues. A chapter ends at its own end from the file, else at the start of
// the next chapter, else, for the last one, at the later of its start and media.
func endChapters(cues []model.Cue, ends []*time.Duration, media time.Duration) []model.Cue {
	order := make([]int, len(cues))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(cues[a].Start, cues[b].Start) })
	out := make([]model.Cue, len(cues))
	for i, k := range order {
		c := cues[k]
		switch {
		case k < len(ends) && ends[k] != nil:
			c.End = *ends[k]
		case i+1 < len(order):
			c.End = cues[order[i+1]].Start
		default:
			c.End = min(max(c.Start, media), timecode.MaxValidTime)
		}
		out[i] = c
	}
	return out
}

// parseError returns a read error of format f. The chapter readers ignore Lenient for their own lines, so
// they always return it.
func parseError(f model.Format, line int, msg string) *model.ParseError {
	return &model.ParseError{Format: f, Line: line, Msg: msg}
}

// physicalLines splits content at LF, CR LF and CR. It returns one empty line for empty content.
func physicalLines(content string) []string {
	return strings.Split(lineEnds.Replace(content), "\n")
}

var lineEnds = strings.NewReplacer("\r\n", "\n", "\r", "\n") //nolint:gochecknoglobals // a replacer holds no state

// escapedCue returns a chapter cue at start with the title as escaped lines. A title with LF gives several lines.
func escapedCue(start time.Duration, title string) model.Cue {
	c := model.Cue{Start: start}
	if title != "" {
		c.Lines = strings.Split(markupcore.EscapeText(title), "\n")
	}
	return c
}

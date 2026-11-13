package model

import (
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

// Cue is one timed text block.
type Cue struct { //nolint:recvcheck // the API reads a copy with value receivers and changes the cue with pointer receivers
	// Start is the time the cue appears. It may be negative. Writers clamp it to 0.
	Start time.Duration
	// End is the time the cue disappears. Nothing checks that it is at or after Start.
	End time.Duration
	// Lines holds the text lines in core markup. Nil is a cue without text.
	Lines []string
	// ID is the identifier of the cue, such as a WebVTT cue identifier. The empty string means none.
	ID string
	// Position is the place of the cue on the screen.
	Position Position
	// Forced marks a forced narrative, which a player shows also when the viewer turned subtitles off.
	Forced bool
	// Image is the bitmap of an image cue. Nil is a text cue. An image cue may also hold lines.
	Image *Image
	// Speaker is the voice of the whole cue, "" for none. A cue with 2 or more voices keeps its <v> tags in the
	// lines and has no Speaker.
	Speaker string
	// Words holds the timed words of the text in order. Nil is a cue without word timing.
	// SetCue checks them with ValidateWords.
	Words []Word

	tagged     *taggedSource
	formatData map[string]*Data
}

// Text returns the lines joined with LF.
func (c Cue) Text() string {
	return strings.Join(c.Lines, "\n")
}

// SetText sets the lines to the parts of text between LF characters, cleaned, without the empty ones.
func (c *Cue) SetText(text string) {
	c.Lines = cleanLines(strings.Split(text, "\n"))
}

// AddLine cleans line and appends it. An empty result is ignored.
func (c *Cue) AddLine(line string) {
	if line = cleanLine(line); line != "" {
		c.Lines = append(c.Lines, line)
	}
}

// Clone returns a copy of c that shares no slice, no Offset, no image and no Confidence with c.
func (c Cue) Clone() Cue {
	c.Lines = slices.Clone(c.Lines)
	c.Words = cloneWords(c.Words)
	c.Position = c.Position.clone()
	c.Image = cloneImage(c.Image)
	return c
}

func (c Cue) normalized() Cue {
	c.Position = c.Position.clone()
	c.Image = cloneImage(c.Image)
	c.Start = c.Start.Round(time.Millisecond)
	c.End = c.End.Round(time.Millisecond)
	c.Lines = cleanLines(c.Lines)
	c.Words = roundWords(cloneWords(c.Words))
	return c
}

func cleanLines(lines []string) []string { return markupcore.CleanLines(lines) }

func cleanLine(s string) string { return markupcore.CleanLine(s) }

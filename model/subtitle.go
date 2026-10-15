package model

// Subtitle is a list of cues in start order, with comments, metadata and format data.
// The zero value is an empty subtitle.
type Subtitle struct{}

// Cue is one timed text block.
type Cue struct{}

// Comment is text that a writer puts before a cue, or after the last cue.
type Comment struct{}

// Alignment is the place of a cue on the screen. The zero value is BottomCenter.
type Alignment uint8

// The 9 alignments of the numeric keypad grid.
const (
	BottomCenter Alignment = iota
	BottomLeft
	BottomRight
	MiddleLeft
	MiddleCenter
	MiddleRight
	TopLeft
	TopCenter
	TopRight
)

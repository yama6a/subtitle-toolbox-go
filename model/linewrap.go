package model

import (
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/internal/linewrap"
)

// WrapLines breaks the lines of each cue that has more than maxLinesPerCue lines or a line longer than
// maxCharactersPerLine visible characters. Each dialogue turn wraps on lines of its own. WrapLines returns an
// error that matches ErrInvalidArgument when a limit is below 1.
func (s *Subtitle) WrapLines(maxCharactersPerLine, maxLinesPerCue int) error {
	if maxCharactersPerLine < 1 || maxLinesPerCue < 1 {
		return invalidArgument(fmt.Sprintf("The maximum characters per line and the maximum lines must be at least 1, got %d and %d.",
			maxCharactersPerLine, maxLinesPerCue))
	}
	s.EditLines(func(_ int, c Cue) ([]string, bool) {
		if linewrap.Fits(c.Lines, maxCharactersPerLine, maxLinesPerCue) {
			return nil, false
		}
		return linewrap.WrapTurns(c.Lines, maxCharactersPerLine, maxLinesPerCue), true
	}, nil)
	return nil
}

// UnwrapLines joins the lines of each cue. Each dialogue turn keeps a line of its own.
func (s *Subtitle) UnwrapLines() {
	s.EditLines(func(_ int, c Cue) ([]string, bool) { return linewrap.UnwrapTurns(c.Lines), true }, nil)
}

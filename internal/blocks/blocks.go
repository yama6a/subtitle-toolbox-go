// Package blocks holds the block rules that the line formats SubRip, WebVTT and SBV share.
package blocks

import (
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	// brokenTimeRegexp matches a first line that looks like a broken time or arrow, so its block stays apart.
	brokenTimeRegexp = regexp.MustCompile(`^\d+[\t\n\v\f\r ]*[:\x{FF1A}]\d`)
	numericRegexp    = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?[ \t\n\r\v\f]*$`)
)

// Rules says which line is a timing line and whether a cue number line comes before it.
type Rules struct {
	IsTimingLine  func(line string) bool
	HasCueNumbers bool
}

// Block is a run of non-empty lines and the 1-based number of its first line.
type Block struct {
	Line  int
	Lines []string
}

// HasTimingLine reports whether a line of lines is a timing line.
func (r Rules) HasTimingLine(lines []string) bool {
	return slices.ContainsFunc(lines, r.IsTimingLine)
}

// IsCueTextBlock reports whether lines are cue text: no timing line, and no cue number or time on the first line.
// A first line that looks like a broken time keeps the block apart, so the reader still skips it with a warning.
func (r Rules) IsCueTextBlock(lines []string) bool {
	return !r.HasTimingLine(lines) &&
		!(r.HasCueNumbers && IsNumeric(lines[0])) &&
		!brokenTimeRegexp.MatchString(lines[0]) && !strings.Contains(lines[0], "->")
}

// JoinCueTextBlocks joins a block of cue text to the block before it when that block has a timing line,
// so an empty line inside the cue text does not end the cue. It calls onJoin with the 0-based number of the
// joined block, the first line of the cue text and its lines. isOther marks a block that is no cue text,
// such as a WebVTT NOTE block. A nil isOther marks none.
func JoinCueTextBlocks(blocks iter.Seq[Block], r Rules, isOther func(lines []string) bool, onJoin func(block, line int, lines []string)) iter.Seq[Block] {
	return func(yield func(Block) bool) {
		var pending *Block
		index := 0
		for b := range blocks {
			if pending != nil && r.IsCueTextBlock(b.Lines) && (isOther == nil || !isOther(b.Lines)) && r.HasTimingLine(pending.Lines) {
				if onJoin != nil {
					onJoin(index, b.Line, b.Lines)
				}
				pending.Lines = slices.Concat(pending.Lines, b.Lines)
				continue
			}
			if pending != nil {
				if !yield(*pending) {
					return
				}
				index++
			}
			pending = &Block{Line: b.Line, Lines: b.Lines}
		}
		if pending != nil {
			yield(*pending)
		}
	}
}

// JoinWarning returns the lenient warning of a join by JoinCueTextBlocks.
func JoinWarning(block, line int) string {
	return "Block #" + strconv.Itoa(block) + " has an empty line before line " + strconv.Itoa(line) +
		" inside the cue. The parser kept the text after it in the cue."
}

// IsNumeric reports whether s is a PHP numeric string, such as "1", "+1", "1.5", ".5", "1." or "1e3",
// with optional white space before and after.
func IsNumeric(s string) bool {
	return numericRegexp.MatchString(s)
}

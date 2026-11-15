// Package textblock holds the rules for the blank-line separated blocks of SubRip, WebVTT, SBV and LRC.
package textblock

import (
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var (
	// brokenTimeRegexp matches a first line that looks like a broken time or arrow, so its block stays apart.
	// The pattern reads Unicode: the digits and the white space are those of Unicode.
	brokenTimeRegexp = regexp.MustCompile(`^\p{Nd}+[\t\n\v\f\r\x{85}\p{Z}]*[:\x{FF1A}]\p{Nd}`)
	numericRegexp    = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?[ \t\n\r\v\f]*$`)
)

// Rules says which line is a timing line and whether a cue number line comes before it.
type Rules struct {
	IsTimingLine  func(line string) bool
	HasCueNumbers bool
	// IsOtherBlock marks a block that is no cue text, such as a WebVTT NOTE block. Nil marks none.
	IsOtherBlock func(lines []string) bool
}

// Block is a run of non-empty lines and the 1-based number of its first line.
type Block struct {
	Line  int
	Lines []string
}

// hasTimingLine reports whether a line of lines is a timing line.
func (r Rules) hasTimingLine(lines []string) bool {
	return slices.ContainsFunc(lines, r.IsTimingLine)
}

// isCueTextBlock reports whether lines are cue text: no timing line, and no cue number or time on the first line.
// A first line that looks like a broken time keeps the block apart, so the reader still skips it with a warning.
func (r Rules) isCueTextBlock(lines []string) bool {
	return !r.hasTimingLine(lines) &&
		(!r.HasCueNumbers || !IsNumeric(lines[0])) &&
		!brokenTimeRegexp.MatchString(lines[0]) && !strings.Contains(lines[0], "->")
}

// JoinCueTextBlocks joins a block of cue text to the block before it when that block has a timing line,
// so an empty line inside the cue text does not end the cue. In lenient mode it warns JoinWarning for each join,
// as Repaired, with the 0-based number of the joined block and the lines of the cue text.
func JoinCueTextBlocks(blocks iter.Seq[Block], r Rules, st *parsing.State) iter.Seq[Block] {
	return func(yield func(Block) bool) {
		var pending *Block
		index := 0
		for b := range blocks {
			if pending != nil && r.joins(pending.Lines, b.Lines) {
				if st.Lenient() {
					st.Warn(JoinWarning(index, b.Line), b.Line, index, b.Lines, model.Repaired)
				}
				pending.Lines = append(pending.Lines, b.Lines...)
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

func (r Rules) joins(pending, lines []string) bool {
	return r.isCueTextBlock(lines) && (r.IsOtherBlock == nil || !r.IsOtherBlock(lines)) && r.hasTimingLine(pending)
}

// JoinWarning returns the lenient warning of a join by JoinCueTextBlocks.
func JoinWarning(block, line int) string {
	return "Block #" + strconv.Itoa(block) + " has an empty line before line " + strconv.Itoa(line) +
		" inside the cue. The parser kept the text after it in the cue."
}

// IsNumeric reports whether s is a decimal number, such as "1", "+1", "1.5", ".5", "1." or "1e3",
// with optional white space before and after.
func IsNumeric(s string) bool {
	return numericRegexp.MatchString(s)
}

// DropCut yields the blocks of blocks without the last one when an error cut the input inside it.
// cut is called once blocks has ended. It returns the 1-based number of the last line that was read, and
// true when an error ended the input. The last block is cut when it ends on that line, because no empty
// line came after it.
func DropCut(blocks iter.Seq[Block], cut func() (int, bool)) iter.Seq[Block] {
	return func(yield func(Block) bool) {
		var held Block
		has := false
		for b := range blocks {
			if has && !yield(held) {
				return
			}
			held, has = b, true
		}
		if !has {
			return
		}
		if line, ok := cut(); ok && held.Line+len(held.Lines)-1 == line {
			return
		}
		yield(held)
	}
}

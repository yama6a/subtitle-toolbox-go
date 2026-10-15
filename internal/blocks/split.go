package blocks

import (
	"iter"
	"strings"
)

const (
	utf8BOM    = "\ufeff"
	phpTrimSet = " \t\n\r\x00\v"
)

// SplitAtEmptyLines yields the runs of non-empty lines, keyed by the 0-based line number in lines.
// Each line loses its leading BOMs, so joined files work. Then each run of tabs and each run of spaces
// becomes 1 space, and the line is trimmed. A line that is then empty ends a block.
func SplitAtEmptyLines(lines iter.Seq2[int, string]) iter.Seq[Block] {
	return func(yield func(Block) bool) {
		var b Block
		for n, line := range lines {
			line = strings.Trim(collapseSpaces(trimBOMs(line)), phpTrimSet)
			if line != "" {
				if b.Lines == nil {
					b.Line = n + 1
				}
				b.Lines = append(b.Lines, line)
				continue
			}
			if b.Lines != nil {
				if !yield(b) {
					return
				}
				b = Block{}
			}
		}
		if b.Lines != nil {
			yield(b)
		}
	}
}

// SplitAtTimingLines splits lines before each timing line. With cue numbers, a numeric line just before a
// timing line goes with it, unless that line is the timing line before. The Line of each part is its 0-based
// offset in lines.
func (r Rules) SplitAtTimingLines(lines []string) []Block {
	starts := r.partStarts(lines)
	parts := make([]Block, 0, len(starts))
	for i, start := range starts {
		end := len(lines)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		parts = append(parts, Block{Line: start, Lines: lines[start:end:end]})
	}
	return parts
}

// partStarts returns 0 and the offset of each later part, in ascending order.
func (r Rules) partStarts(lines []string) []int {
	starts := []int{0}
	lastTiming := -1
	for i, line := range lines {
		if !r.IsTimingLine(line) {
			continue
		}
		start := i
		if r.HasCueNumbers && i > 0 && IsNumeric(lines[i-1]) && i-1 > lastTiming {
			start = i - 1
		}
		if start > 0 {
			starts = append(starts, start)
		}
		lastTiming = i
	}
	return starts
}

func trimBOMs(s string) string {
	for strings.HasPrefix(s, utf8BOM) {
		s = s[len(utf8BOM):]
	}
	return s
}

func collapseSpaces(s string) string {
	if !strings.Contains(s, "\t") && !strings.Contains(s, "  ") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		if c == '\t' {
			c = ' '
		}
		if c == ' ' && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

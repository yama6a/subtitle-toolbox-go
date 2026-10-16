package blocks

import (
	"errors"
	"iter"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	utf8BOM = "\ufeff"
)

// SplitAtEmptyLines yields the runs of non-empty lines. The keys of lines are 0-based line numbers, and each
// block holds the 1-based number of its first line. Each line loses its leading BOMs, so joined files work.
// Then each run of tabs and each run of spaces becomes 1 space, and the line is trimmed. A line that is then
// empty ends a block.
func SplitAtEmptyLines(lines iter.Seq2[int, string]) iter.Seq[Block] {
	return func(yield func(Block) bool) {
		var b Block
		for n, line := range lines {
			line = strings.Trim(collapseSpaces(trimBOMs(line)), markupcore.PHPTrimSet)
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

// Lines yields the lines of content and their 0-based numbers. LF, a run of CR before LF, and a lone CR end a line.
func Lines(content string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		n := 0
		for {
			i := strings.IndexAny(content, "\r\n")
			if i < 0 {
				yield(n, content)
				return
			}
			if !yield(n, content[:i]) {
				return
			}
			next := i + 1
			if content[i] == '\r' {
				j := i
				for j < len(content) && content[j] == '\r' {
					j++
				}
				if j < len(content) && content[j] == '\n' {
					next = j + 1
				}
			}
			content = content[next:]
			n++
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

// SplitWarning returns the lenient warning of a split by SplitAtTimingLines.
func SplitWarning(block, line int) string {
	return "Block #" + strconv.Itoa(block) + " has no empty line before line " + strconv.Itoa(line) +
		". The parser split the block there."
}

// PartReader reads the parts of a block that SplitAtTimingLines found.
type PartReader struct {
	Lenient bool
	// Split is called in lenient mode for each part after the first, with the warning message and the 1-based line.
	Split func(msg string, line int)
	// Parse reads the lines of one part, which starts on the 1-based line.
	Parse func(lines []string, line int) (model.Cue, error)
	// Fail handles a *model.ParseError of a part, on the line of the error or of the part.
	Fail func(err *model.ParseError, line int, lines []string) error
}

// ReadParts returns the cues of the parts of block b, number index. A part that fails with a *model.ParseError
// goes to p.Fail, which returns nil in lenient mode so that the read goes on. Any other error ends the read.
func (r Rules) ReadParts(b Block, index int, p PartReader) ([]model.Cue, error) {
	parts := r.SplitAtTimingLines(b.Lines)
	if p.Lenient {
		for _, part := range parts[1:] {
			line := b.Line + part.Line
			p.Split(SplitWarning(index, line), line)
		}
	}
	cues := make([]model.Cue, 0, len(parts))
	for _, part := range parts {
		line := b.Line + part.Line
		c, err := p.Parse(part.Lines, line)
		if err == nil {
			cues = append(cues, c)
			continue
		}
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return nil, err
		}
		if perr.Line > 0 {
			line = perr.Line
		}
		if err := p.Fail(perr, line, part.Lines); err != nil {
			return nil, err
		}
	}
	return cues, nil
}

// Blocks yields the blocks of content after the join of cue text blocks. onJoin is called for each join with the
// warning message, the 1-based line of the joined text, the 0-based block number and the lines. A nil onJoin
// ignores joins.
func (r Rules) Blocks(content string, onJoin func(msg string, line, block int, lines []string)) iter.Seq[Block] {
	var join func(block, line int, lines []string)
	if onJoin != nil {
		join = func(block, line int, lines []string) { onJoin(JoinWarning(block, line), line, block, lines) }
	}
	return JoinCueTextBlocks(SplitAtEmptyLines(Lines(content)), r, nil, join)
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

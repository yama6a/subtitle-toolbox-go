package linewrap

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

var (
	leadingTagRegexp = regexp.MustCompile(`^` + markupcore.TagPattern)
	embeddingRemover = strings.NewReplacer("\u202b", "", "\u202c", "") //nolint:gochecknoglobals // a replacer holds no state
)

// Wrap breaks the lines into at most maxLines lines of at most cpl visible characters where the words allow it.
// A line breaks at a space, or between 2 CJK characters where JIS X 4051 allows it. With keepDialogueLines, each
// line that starts with a dialogue dash starts a segment, and 2 or more segments come back unwrapped, 1 per line.
func Wrap(lines []string, cpl, maxLines int, keepDialogueLines bool) []string {
	return keepRightToLeftEmbedding(lines, func(lines []string) []string {
		return wrapPlain(lines, cpl, maxLines, keepDialogueLines)
	})
}

func wrapPlain(lines []string, cpl, maxLines int, keepDialogueLines bool) []string {
	segments := segments(lines, keepDialogueLines)
	switch len(segments) {
	case 0:
		return nil
	case 1:
		parts := breakableParts(segments[0])
		return joinLines(parts, findBreaks(parts, cpl, maxLines))
	}
	out := make([]string, len(segments))
	for i, segment := range segments {
		words := MeasuredWords(segment)
		texts := make([]string, len(words))
		for j, w := range words {
			texts[j] = w.Text
		}
		out[i] = strings.Join(texts, " ")
	}
	return out
}

// WrapTurns wraps each dialogue turn on lines of its own, with up to maxLines lines per turn, so the result can
// have more than maxLines lines. A turn starts at a line with a dialogue dash, or inside a line at a dialogue dash
// after a sentence end. With fewer than 2 turns, it wraps as Wrap does without keepDialogueLines.
func WrapTurns(lines []string, cpl, maxLines int) []string {
	return keepRightToLeftEmbedding(lines, func(lines []string) []string {
		return wrapTurnsPlain(lines, cpl, maxLines)
	})
}

func wrapTurnsPlain(lines []string, cpl, maxLines int) []string {
	turns := dialogueTurns(lines)
	if len(turns) < 2 {
		return wrapPlain(lines, cpl, maxLines, false)
	}

	var parts []part
	var lineStarts []int
	for _, turn := range turns {
		turnParts := breakableParts(turn)
		for _, start := range findBreaks(turnParts, cpl, maxLines) {
			lineStarts = append(lineStarts, len(parts)+start)
		}
		parts = append(parts, turnParts...)
	}
	return joinLines(parts, lineStarts)
}

// dialogueTurns joins the measured words of the lines into turns. A turn starts at the first word, at the first
// word of a line that starts with a dialogue dash, and at a dialogue dash after a sentence end inside a line.
func dialogueTurns(lines []string) []string {
	var turns []string
	for _, line := range lines {
		words := MeasuredWords(line)
		for i, w := range words {
			var startsTurn bool
			if i == 0 {
				startsTurn = len(turns) == 0 || markupcore.StartsDialogueTurn(markupcore.VisibleText(line))
			} else {
				startsTurn = i+1 < len(words) && startsInlineTurn(words[i-1], w, words[i+1])
			}
			last := len(turns) - 1
			switch {
			case startsTurn:
				turns = append(turns, w.Text)
			case i == 0:
				turns[last] = joinLine(turns[last], w.Text)
			default:
				turns[last] += " " + w.Text
			}
		}
	}
	return turns
}

func startsInlineTurn(previous, w, next Word) bool {
	text := markupcore.VisibleText(previous.Text)
	r, _ := utf8.DecodeLastRuneInString(text)
	return utf8.ValidString(text) && endsSentence(r) && markupcore.StartsDialogueTurn(markupcore.VisibleText(w.Text+" "+next.Text))
}

// endsSentence reports whether r is a full stop, a question mark, an exclamation mark or the horizontal ellipsis,
// in ASCII or as a CJK form.
func endsSentence(r rune) bool {
	switch r {
	case '.', '?', '!', 0x2026, 0x3002, 0xff01, 0xff1f, 0xff61:
		return true
	}
	return false
}

// UnwrapTurns joins the lines of each dialogue turn with a space, or without one between 2 CJK characters.
// A turn starts at the first line and at each line that starts with a dialogue dash.
func UnwrapTurns(lines []string) []string {
	return keepRightToLeftEmbedding(lines, unwrapTurnsPlain)
}

func unwrapTurnsPlain(lines []string) []string {
	var turns []string
	for _, line := range lines {
		if len(turns) > 0 && !markupcore.StartsDialogueTurn(markupcore.VisibleText(line)) {
			turns[len(turns)-1] = joinLine(turns[len(turns)-1], line)
		} else {
			turns = append(turns, line)
		}
	}
	return turns
}

// keepRightToLeftEmbedding calls fn without the marks of EmbedRightToLeft when a line holds its embedding, and
// embeds each new line again, because a line break ends every embedding in the Unicode bidi algorithm.
func keepRightToLeftEmbedding(lines []string, fn func([]string) []string) []string {
	if !markupcore.HoldsRightToLeftEmbedding(lines) {
		return fn(lines)
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = embeddingRemover.Replace(line)
	}
	out := fn(plain)
	for i, line := range out {
		out[i] = markupcore.EmbedRightToLeft(line)
	}
	return out
}

// segments joins the lines with words into 1 segment. With keepDialogueLines, each line that starts with a
// dialogue dash starts a new segment.
func segments(lines []string, keepDialogueLines bool) []string {
	var segments []string
	for _, line := range lines {
		if strings.Trim(line, " ") == "" {
			continue
		}
		if len(segments) == 0 || keepDialogueLines && startsWithDash(line) {
			segments = append(segments, line)
		} else {
			segments[len(segments)-1] = joinLine(segments[len(segments)-1], line)
		}
	}
	return segments
}

// startsWithDash reports whether a dash character follows the white space and tags at the start of line.
// A minus sign counts too.
func startsWithDash(line string) bool {
	if !utf8.ValidString(line) {
		return false
	}
	for line != "" {
		r, size := utf8.DecodeRuneInString(line)
		switch {
		case unicode.IsSpace(r):
			line = line[size:]
		case r == '<':
			loc := leadingTagRegexp.FindStringIndex(line)
			if loc == nil {
				return false
			}
			line = line[loc[1]:]
		default:
			return markupcore.IsDialogueDash(r)
		}
	}
	return false
}

func joinLine(text, line string) string {
	if endsWithCJK(markupcore.VisibleText(text)) && startsWithCJK(markupcore.VisibleText(line)) {
		return text + line
	}
	return text + " " + line
}

// ending is the best way to end a line after a number of parts: the sum of the overflow beyond cpl, the count
// of breaks inside a phrase, the sum of the squared line lengths, and the first part of the last line.
type ending struct {
	overflow, midPhrase, squares, start int
	ok                                  bool
}

func (e ending) less(o ending) bool {
	if e.overflow != o.overflow {
		return e.overflow < o.overflow
	}
	if e.midPhrase != o.midPhrase {
		return e.midPhrase < o.midPhrase
	}
	return e.squares < o.squares
}

// findBreaks returns the index of the first part of each line. It uses the fewest lines up to maxLines that fit,
// then the fewest breaks inside a phrase, then the most equal line lengths. When no line count fits, it uses
// min(maxLines, len(parts)) lines with the least overflow.
func findBreaks(parts []part, cpl, maxLines int) []int {
	n := len(parts)
	if n == 0 {
		return nil
	}
	b := breakSearch{parts: parts, cpl: cpl, starts: make([]int, n+1), ends: make([]int, n+1)}
	for i, p := range parts {
		b.ends[i+1] = b.starts[i] + p.length
		b.starts[i+1] = b.ends[i+1]
		if i+1 < n && parts[i+1].spaceBefore {
			b.starts[i+1]++
		}
	}

	best := [][]ending{make([]ending, n+1)}
	best[0][0].ok = true
	for count := 1; count <= min(maxLines, n); count++ {
		row := make([]ending, n+1)
		for end := count; end <= n; end++ {
			row[end] = b.bestLastLine(best[count-1], count-1, end)
		}
		best = append(best, row)
		if row[n].overflow == 0 {
			break
		}
	}

	lineStarts := make([]int, len(best)-1)
	end := n
	for count := len(best) - 1; count > 0; count-- {
		end = best[count][end].start
		lineStarts[count-1] = end
	}
	return lineStarts
}

// breakSearch holds the parts and their columns on one long line: part i starts at column starts[i], and part
// i-1 ends at column ends[i].
type breakSearch struct {
	parts        []part
	cpl          int
	starts, ends []int
}

// bestLastLine returns the best ending after the parts before end, with a last line that starts at firstStart
// or later. The earliest start wins a tie.
func (b breakSearch) bestLastLine(previous []ending, firstStart, end int) ending {
	var best ending
	for start := firstStart; start < end; start++ {
		if !previous[start].ok {
			continue
		}
		length := b.ends[end] - b.starts[start]
		e := ending{
			overflow:  previous[start].overflow + max(0, length-b.cpl),
			midPhrase: previous[start].midPhrase,
			squares:   previous[start].squares + length*length,
			start:     start,
			ok:        true,
		}
		if b.parts[start].midPhrase {
			e.midPhrase++
		}
		if !best.ok || e.less(best) {
			best = e
		}
	}
	return best
}

// joinLines joins the parts of each line. Each line opens the core tags that the line before it left open, and
// each line but the last closes its open core tags.
func joinLines(parts []part, lineStarts []int) []string {
	lines := make([]string, len(lineStarts))
	var open []markupcore.OpenTag
	for i, start := range lineStarts {
		end := len(parts)
		if i+1 < len(lineStarts) {
			end = lineStarts[i+1]
		}
		var b strings.Builder
		for _, t := range open {
			b.WriteString(t.Tag)
		}
		for j, p := range parts[start:end] {
			if j > 0 && p.spaceBefore {
				b.WriteByte(' ')
			}
			b.WriteString(p.text)
		}
		line := b.String()
		open = markupcore.OpenCoreTags(line, nil)
		if i+1 < len(lineStarts) {
			line += markupcore.CloseCoreTags(open)
		}
		lines[i] = line
	}
	return lines
}

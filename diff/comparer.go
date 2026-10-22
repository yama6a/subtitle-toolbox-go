package diff

import (
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/markup"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	minTextScore    = 0.7
	minOverlapShare = 0.5
	textWeight      = 2
)

// comparer holds the two cue lists of one Compare call. The starts and ends are seconds as float64, so the weights
// and the tie rules do not depend on the integer duration type.
type comparer struct {
	oldCues, newCues   []model.Cue
	oldTexts, newTexts []string
	oldStarts, oldEnds []float64
	newStarts, newEnds []float64
	tolerance          time.Duration
	textOnly           bool
}

func newComparer(oldCues, newCues []model.Cue, opts Options) *comparer {
	c := &comparer{oldCues: oldCues, newCues: newCues, tolerance: opts.timeTolerance(), textOnly: opts.TextOnly}
	c.oldTexts, c.oldStarts, c.oldEnds = describe(oldCues, opts)
	c.newTexts, c.newStarts, c.newEnds = describe(newCues, opts)
	return c
}

func describe(cues []model.Cue, opts Options) (texts []string, starts, ends []float64) {
	texts = make([]string, len(cues))
	starts = make([]float64, len(cues))
	ends = make([]float64, len(cues))
	for i, cue := range cues {
		texts[i] = comparedText(cue.Text(), opts)
		starts[i] = seconds(cue.Start)
		ends[i] = seconds(cue.End)
	}
	return texts, starts, ends
}

func seconds(d time.Duration) float64 {
	return float64(d.Milliseconds()) / 1000
}

func comparedText(text string, opts Options) string {
	if opts.IgnoreFormatting {
		text = markup.PlainText(text)
	}
	if opts.IgnoreWhitespace {
		text = removeWhitespace(text)
	}
	return text
}

// removeWhitespace drops the bytes of space, tab, LF, CR, FF and VT. It works on bytes, so invalid UTF-8 stays.
func removeWhitespace(text string) string {
	if !strings.ContainsAny(text, " \t\n\r\f\v") {
		return text
	}
	out := make([]byte, 0, len(text))
	for i := range len(text) {
		switch text[i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
		default:
			out = append(out, text[i])
		}
	}
	return string(out)
}

// sameTime reports whether the start and the end differ by at most the tolerance.
func (c *comparer) sameTime(i, j int) bool {
	o, n := &c.oldCues[i], &c.newCues[j]
	return abs(o.Start-n.Start) <= c.tolerance && abs(o.End-n.End) <= c.tolerance
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// weight returns 0 for cues that do not pair. Else it returns up to textWeight for the likeness of the texts,
// plus up to 1 for the likeness of the times.
func (c *comparer) weight(i, j int) float64 {
	return textWeight*textScore(c.oldTexts[i], c.newTexts[j]) + c.timeScore(i, j)
}

func (c *comparer) timeScore(i, j int) float64 {
	if c.sameTime(i, j) {
		return 1
	}
	oStart, oEnd, nStart, nEnd := c.oldStarts[i], c.oldEnds[i], c.newStarts[j], c.newEnds[j]
	overlap := min(oEnd, nEnd) - max(oStart, nStart)
	shorter := min(oEnd-oStart, nEnd-nStart)
	if overlap > 0 && overlap >= minOverlapShare*shorter {
		return overlap / (max(oEnd, nEnd) - min(oStart, nStart))
	}
	return 0
}

// textScore returns 1 for equal texts. Else it returns 1 minus the byte edit distance divided by the length of the
// longer text, or 0 when that is below minTextScore.
func textScore(a, b string) float64 {
	if a == b {
		return 1
	}
	longer := max(len(a), len(b))
	if longer == 0 || float64(min(len(a), len(b))) < minTextScore*float64(longer) {
		return 0
	}
	score := 1 - float64(levenshtein(a, b))/float64(longer)
	if score < minTextScore {
		return 0
	}
	return score
}

// levenshtein returns the edit distance of a and b in bytes. A count in runes would give other scores for
// non-ASCII text.
func levenshtein(a, b string) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	if len(b) == 0 {
		return len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := range len(a) {
		current[0] = i + 1
		for j := range len(b) {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}
			current[j+1] = min(previous[j+1]+1, current[j]+1, previous[j]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

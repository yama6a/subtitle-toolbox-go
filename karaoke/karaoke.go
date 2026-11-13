// Package karaoke writes one cue per timed word, with the active word styled.
package karaoke

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// MaxLineBytes is the most line text that the new cues of Apply may hold together.
const MaxLineBytes = 64 << 20

// Report counts the cues before and after.
type Report struct {
	CuesBefore int
	CuesAfter  int
}

// line is the items of a line, with the indexes of its tags and of its text runs.
type line struct {
	items []item
	tags  []int
	texts []int
}

// item is a tag or a text run of a line, with the number of its word. Word -1 is the text before the first word.
type item struct {
	text string
	tag  bool
	word int
}

// Apply replaces each cue that has words with one cue per word, in which the style marks the active word.
// A cue without words stays as it is. The word cues hold no Words.
// Apply fails with a *model.InvalidArgumentError when the new cues would hold more than MaxLineBytes of line text.
// On an error nothing changes.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	old := s.Cues()
	var (
		out   []model.Cue
		first = make([]int, len(old))
		bytes int
	)
	for i, c := range old {
		first[i] = len(out)
		start := len(out)
		out = expand(out, c, opts)
		for _, w := range out[start:] {
			for _, l := range w.Lines {
				bytes += len(l)
			}
		}
		if bytes > MaxLineBytes {
			return Report{}, &model.InvalidArgumentError{Option: "karaoke.Apply", Msg: "The word cues of cue " + strconv.Itoa(i+1) +
				" at " + timecode.Seconds(c.Start) + " s would bring the line text of the new cues above " + strconv.Itoa(MaxLineBytes>>20) +
				" MiB. Split the cue first with resegment.SplitLong, or set MaxWordsPerCue."}
		}
	}
	s.ReplaceCues(out, func(i int) int { return first[i] })
	return Report{CuesBefore: len(old), CuesAfter: len(out)}, nil
}

func expand(out []model.Cue, c model.Cue, opts Options) []model.Cue {
	texts := make([]string, len(c.Words))
	for i, w := range c.Words {
		texts[i] = w.Text
	}
	marks := markupcore.WordMarks(c.Lines, texts)
	n := len(marks)
	if n == 0 {
		return append(out, c.Clone())
	}
	lines := splitWords(c.Lines, marks)
	words := c.Words[:n]
	c.Words = nil

	end := max(c.Start, c.End)
	bounds := make([]time.Duration, 0, n+2)
	bounds = append(bounds, c.Start)
	for k := range n {
		bounds = append(bounds, max(bounds[k], min(end, words[k].Start)))
	}
	bounds = append(bounds, end)

	produced := false
	for w := -1; w < n; w++ {
		if bounds[w+2] <= bounds[w+1] {
			continue
		}
		cue := c.Clone()
		cue.Start, cue.End = bounds[w+1], bounds[w+2]
		cue.Lines = highlight(lines, w, n, opts)
		if produced {
			cue.ID = ""
		}
		produced = true
		out = append(out, cue)
	}
	if !produced {
		cue := c.Clone()
		cue.Lines = highlight(lines, -1, n, opts)
		out = append(out, cue)
	}
	return out
}

// numberer counts the marks at or before a position in the lines.
type numberer struct {
	marks []markupcore.Mark
	next  int
}

// word returns the word number at the offset of line l.
func (n *numberer) word(l, offset int) int {
	for n.next < len(n.marks) && (n.marks[n.next].Line < l || n.marks[n.next].Line == l && n.marks[n.next].Offset <= offset) {
		n.next++
	}
	return n.next - 1
}

// cut returns the end of the text run that starts at from and ends at end, cut at the next mark inside it.
func (n *numberer) cut(l, end int) int {
	if n.next < len(n.marks) && n.marks[n.next].Line == l && n.marks[n.next].Offset < end {
		return n.marks[n.next].Offset
	}
	return end
}

// splitWords splits each line into tags and text runs and numbers each item by the marks at or before its start.
func splitWords(lines []string, marks []markupcore.Mark) []line {
	result := make([]line, len(lines))
	num := numberer{marks: marks}
	for l, text := range lines {
		var cur line
		at := 0
		for i, part := range markupcore.SplitTags(text) {
			if i%2 == 1 {
				cur.tags = append(cur.tags, len(cur.items))
				cur.items = append(cur.items, item{part, true, num.word(l, at)})
				at += len(part)
				continue
			}
			for end := at + len(part); at < end; {
				w := num.word(l, at)
				to := num.cut(l, end)
				cur.texts = append(cur.texts, len(cur.items))
				cur.items = append(cur.items, item{text[at:to], false, w})
				at = to
			}
		}
		result[l] = cur
	}
	return result
}

// marks reports whether the style marks a text run of word when active is the active word.
func (o Options) marks(word, active int) bool {
	if word < 0 {
		return false
	}
	if o.Mode == ModeCumulative {
		return word <= active
	}
	return word == active
}

func window(active, n, limit int) (first, last int) {
	if limit == 0 || limit >= n {
		return -1, n - 1
	}
	first = max(0, min(max(0, active)-(limit-1)/2, n-limit))
	return first, first + limit - 1
}

// writer writes one line with the style around the styled text runs.
type writer struct {
	b                 strings.Builder
	openTag, closeTag string
	isOpen            bool
	pending           string
}

func (w *writer) other(text string) {
	w.closeStyle()
	w.b.WriteString(w.pending)
	w.b.WriteString(text)
	w.pending = ""
}

func (w *writer) styled(text string) {
	lead, core, trail := splitSpace(text)
	if core == "" {
		w.pending += text
		return
	}
	w.b.WriteString(w.pending)
	w.b.WriteString(lead)
	if !w.isOpen {
		w.b.WriteString(w.openTag)
	}
	w.b.WriteString(core)
	w.isOpen, w.pending = true, trail
}

func (w *writer) closeStyle() {
	if w.isOpen {
		w.b.WriteString(w.closeTag)
		w.isOpen = false
	}
}

func (w *writer) finish() string {
	w.closeStyle()
	w.b.WriteString(w.pending)
	return w.b.String()
}

func highlight(lines []line, active, n int, opts Options) []string {
	first, last := window(active, n, opts.MaxWordsPerCue)
	style := strings.Trim(opts.style(), markupcore.PHPTrimSet)

	result := make([]string, 0, len(lines))
	for _, ln := range lines {
		w := writer{openTag: "<" + style + ">", closeTag: "</" + markupcore.TagName(style) + ">"}
		for _, it := range ln.visible(first, last) {
			if !it.tag && opts.marks(it.word, active) {
				w.styled(it.text)
			} else {
				w.other(it.text)
			}
		}
		result = append(result, w.finish())
	}
	if opts.MaxWordsPerCue == 0 {
		return result
	}
	return cleanWindow(result)
}

// cleanWindow removes the tag pairs and the white space at the line edges that the hidden words leave, and the lines
// without visible text.
func cleanWindow(lines []string) []string {
	kept := lines[:0]
	for _, text := range lines {
		text = trimEdges(markupcore.RemoveEmptyTagPairs(text, false, true))
		if strings.TrimFunc(markupcore.StripAllTags(text), markupcore.IsSpace) != "" {
			kept = append(kept, text)
		}
	}
	return slices.Clip(kept)
}

// visible returns the tags and the text runs of words first to last, in order. Text runs of word -1 show when first
// is -1 or 0.
func (ln line) visible(first, last int) []item {
	lo := 0
	if first > 0 {
		lo = sort.Search(len(ln.texts), func(i int) bool { return ln.items[ln.texts[i]].word >= first })
	}
	hi := sort.Search(len(ln.texts), func(i int) bool { return ln.items[ln.texts[i]].word > last })
	if lo == 0 && hi == len(ln.texts) {
		return ln.items
	}
	texts := ln.texts[lo:hi]
	res := make([]item, 0, len(ln.tags)+len(texts))
	for i, j := 0, 0; i < len(ln.tags) || j < len(texts); {
		if j == len(texts) || i < len(ln.tags) && ln.tags[i] < texts[j] {
			res = append(res, ln.items[ln.tags[i]])
			i++
		} else {
			res = append(res, ln.items[texts[j]])
			j++
		}
	}
	return res
}

func splitSpace(s string) (lead, core, trail string) {
	core = strings.TrimLeftFunc(s, markupcore.IsSpace)
	lead = s[:len(s)-len(core)]
	core = strings.TrimRightFunc(core, markupcore.IsSpace)
	trail = s[len(lead)+len(core):]
	return lead, core, trail
}

// trimEdges removes white space after the opening tags at the line start and before the closing tags at the line end.
func trimEdges(line string) string {
	parts := markupcore.SplitTags(line)
	i := 0
	for i+1 < len(parts) && parts[i] == "" && !strings.HasPrefix(parts[i+1], "</") {
		i += 2
	}
	parts[i] = strings.TrimLeftFunc(parts[i], markupcore.IsSpace)
	j := len(parts) - 1
	for j-1 >= 0 && parts[j] == "" && strings.HasPrefix(parts[j-1], "</") {
		j -= 2
	}
	parts[j] = strings.TrimRightFunc(parts[j], markupcore.IsSpace)
	return strings.Join(parts, "")
}

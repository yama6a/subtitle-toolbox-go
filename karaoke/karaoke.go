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

// maxLineBytes is the most line text that the new cues of Apply may hold together.
const maxLineBytes = 64 << 20

// Report counts the cues before and after.
type Report struct {
	CuesBefore int
	CuesAfter  int
}

// item is a tag or a text run of a line, with the number of its word. Word -1 is the text before the first word.
type item struct {
	text string
	tag  bool
	word int
}

// node is an element of a list of tags that no empty tag pair removes. A list shares its tail with the lists of the
// tags before it, so each position of a line costs one node.
type node struct {
	tag     string
	name    string
	closing bool
	style   bool
	next    *node
}

// line is the items of a line, with the indexes of its tags and of its text runs. before[k] is the tags before
// tags[k] and after[k] the tags from tags[k] on, each with its empty pairs removed. Both are nil without a window.
type line struct {
	items         []item
	tags          []int
	texts         []int
	before, after []*node
	firstWord     int
	lastWord      int
}

// cuePlan is a cue with words, split into items and ready to write as word cues.
type cuePlan struct {
	cue       model.Cue
	lines     []line
	textLines []int
	n         int
	bounds    []time.Duration
	produced  int
}

// Apply replaces each cue that has words with one cue per word, in which the style marks the active word.
// A cue without words stays as it is. The word cues hold no Words.
// Apply fails with a *model.InvalidArgumentError when the new cues would hold more than 64 MiB of line text.
// On an error nothing changes.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	old := s.Cues()
	plans := make([]cuePlan, len(old))
	least := 0
	for i, c := range old {
		plans[i] = plan(c, opts)
		if opts.MaxWordsPerCue == 0 {
			least += plans[i].leastBytes()
			if least > maxLineBytes {
				return Report{}, tooLarge(i, c)
			}
		}
	}
	var (
		out   []model.Cue
		first = make([]int, len(old))
		bytes int
	)
	for i := range plans {
		first[i] = len(out)
		start := len(out)
		out = plans[i].expand(out, opts)
		for _, w := range out[start:] {
			for _, l := range w.Lines {
				bytes += len(l)
			}
		}
		if bytes > maxLineBytes {
			return Report{}, tooLarge(i, old[i])
		}
	}
	s.ReplaceCues(out, func(i int) int { return first[i] })
	return Report{CuesBefore: len(old), CuesAfter: len(out)}, nil
}

func tooLarge(i int, c model.Cue) error {
	return &model.InvalidArgumentError{Msg: "The word cues of cue " + strconv.Itoa(i+1) +
		" at " + timecode.Seconds(c.Start) + " s would bring the line text of the new cues above " + strconv.Itoa(maxLineBytes>>20) +
		" MiB. Split the cue first with resegment.SplitLong, or set MaxWordsPerCue."}
}

func plan(c model.Cue, opts Options) cuePlan {
	texts := make([]string, len(c.Words))
	for i, w := range c.Words {
		texts[i] = w.Text
	}
	marks := markupcore.WordMarks(c.Lines, texts)
	p := cuePlan{n: len(marks)}
	if p.n == 0 {
		p.cue = c
		return p
	}
	words := c.Words[:p.n]
	c.Words = nil
	p.cue = c
	p.lines = splitWords(c.Lines, marks, opts.MaxWordsPerCue > 0)
	for l, ln := range p.lines {
		if len(ln.texts) > 0 {
			p.textLines = append(p.textLines, l)
		}
	}

	end := max(c.Start, c.End)
	p.bounds = make([]time.Duration, 0, p.n+2)
	p.bounds = append(p.bounds, c.Start)
	for k := range p.n {
		p.bounds = append(p.bounds, max(p.bounds[k], min(end, words[k].Start)))
	}
	p.bounds = append(p.bounds, end)
	for w := -1; w < p.n; w++ {
		if p.bounds[w+2] > p.bounds[w+1] {
			p.produced++
		}
	}
	return p
}

// leastBytes returns fewer bytes than the cues of p hold. Without a window each word cue holds all the line text.
func (p cuePlan) leastBytes() int {
	size := 0
	for _, l := range p.cue.Lines {
		size += len(l)
	}
	return size * max(p.produced, 1)
}

func (p cuePlan) expand(out []model.Cue, opts Options) []model.Cue {
	if p.n == 0 {
		return append(out, p.cue.Clone())
	}
	produced := false
	for w := -1; w < p.n; w++ {
		if p.bounds[w+2] <= p.bounds[w+1] {
			continue
		}
		cue := p.cue.Clone()
		cue.Start, cue.End = p.bounds[w+1], p.bounds[w+2]
		cue.Lines = p.highlight(w, opts)
		if produced {
			cue.ID = ""
		}
		produced = true
		out = append(out, cue)
	}
	if !produced {
		cue := p.cue.Clone()
		cue.Lines = p.highlight(-1, opts)
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
func splitWords(lines []string, marks []markupcore.Mark, reduce bool) []line {
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
		if len(cur.texts) > 0 {
			cur.firstWord = cur.items[cur.texts[0]].word
			cur.lastWord = cur.items[cur.texts[len(cur.texts)-1]].word
		}
		if reduce {
			cur.reduceTags()
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

func (p cuePlan) highlight(active int, opts Options) []string {
	first, last := window(active, p.n, opts.MaxWordsPerCue)
	style := strings.Trim(opts.style(), markupcore.PHPTrimSet)

	write := func(ln *line) string {
		w := writer{openTag: "<" + style + ">", closeTag: "</" + markupcore.TagName(style) + ">"}
		for _, it := range ln.view(first, last) {
			if !it.tag && opts.marks(it.word, active) {
				w.styled(it.text)
			} else {
				w.other(it.text)
			}
		}
		return w.finish()
	}
	if opts.MaxWordsPerCue == 0 {
		result := make([]string, len(p.lines))
		for i := range p.lines {
			result[i] = write(&p.lines[i])
		}
		return result
	}
	from := max(first, -1)
	lo := sort.Search(len(p.textLines), func(i int) bool { return p.lines[p.textLines[i]].lastWord >= from })
	hi := sort.Search(len(p.textLines), func(i int) bool { return p.lines[p.textLines[i]].firstWord > last })
	result := make([]string, 0, max(hi-lo, 0))
	for i := lo; i < hi; i++ {
		result = append(result, write(&p.lines[p.textLines[i]]))
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

// view returns the items of words first to last in order, with the tags before and after them reduced. Text runs of
// word -1 show when first is -1 or 0.
func (ln *line) view(first, last int) []item {
	lo := 0
	if first > 0 {
		lo = sort.Search(len(ln.texts), func(i int) bool { return ln.items[ln.texts[i]].word >= first })
	}
	hi := sort.Search(len(ln.texts), func(i int) bool { return ln.items[ln.texts[i]].word > last })
	if lo == 0 && hi == len(ln.texts) {
		return ln.items
	}
	if lo >= hi {
		return nil
	}
	from, to := ln.texts[lo], ln.texts[hi-1]
	before := ln.before[sort.SearchInts(ln.tags, from)]
	after := ln.after[sort.SearchInts(ln.tags, to)]
	res := before.appendReversed(nil)
	slices.Reverse(res)
	res = append(res, ln.items[from:to+1]...)
	return after.appendReversed(res)
}

// appendReversed appends the tags of the list, from its head to its end.
func (n *node) appendReversed(res []item) []item {
	for ; n != nil; n = n.next {
		res = append(res, item{n.tag, true, -1})
	}
	return res
}

// reduceTags fills before and after. An empty pair is an opening style tag directly followed by its closing tag.
func (ln *line) reduceTags() {
	ln.before = make([]*node, len(ln.tags)+1)
	ln.after = make([]*node, len(ln.tags)+1)
	for k, idx := range ln.tags {
		ln.before[k+1] = push(ln.before[k], ln.items[idx].text, false)
	}
	for k := len(ln.tags) - 1; k >= 0; k-- {
		ln.after[k] = push(ln.after[k+1], ln.items[ln.tags[k]].text, true)
	}
}

// push adds tag to the list. Going forward, a closing tag removes the opening tag of its name at the head. Going
// backward, an opening tag removes the closing tag of its name at the head.
func push(head *node, tag string, backward bool) *node {
	name, closing, style := markupcore.StyleTag(tag)
	if style && head != nil && head.style && head.name == name && head.closing != closing && closing != backward {
		return head.next
	}
	return &node{tag, name, closing, style, head}
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

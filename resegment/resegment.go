// Package resegment splits long cues and builds new cues from word timings.
package resegment

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// DefaultMaxWordGap is the pause at which ByWords ends a cue.
const DefaultMaxWordGap = 600 * time.Millisecond

// Options holds the settings of SplitLong and ByWords.
type Options struct {
	// Limits holds the limits of each new cue. The zero value uses the defaults of model.CueLimits.
	Limits model.CueLimits
	// MaxWordGap makes ByWords end a cue before a word that starts this long or longer after the end of the word
	// before it. Nil means DefaultMaxWordGap. SplitLong ignores it.
	MaxWordGap *time.Duration
}

// Report counts the cues before and after.
type Report struct {
	CuesBefore int
	CuesAfter  int
}

// Validate returns an *model.InvalidArgumentError for limits out of range and for a negative MaxWordGap.
func (o Options) Validate() error {
	_, err := o.resolve()
	return err
}

type settings struct {
	cpl, lines     int
	minDur, maxDur time.Duration
	cps            float64
	maxGap         time.Duration
}

func (o Options) resolve() (settings, error) {
	if err := o.Limits.Validate(); err != nil {
		if e, ok := err.(*model.InvalidArgumentError); ok { //nolint:errorlint // Validate returns the type itself
			e.Option = "Options.Limits." + strings.TrimPrefix(e.Option, "CueLimits.")
		}
		return settings{}, err
	}
	l := o.Limits
	s := settings{
		cpl:    cmp.Or(l.MaxCharactersPerLine, model.DefaultMaxCharactersPerLine),
		lines:  cmp.Or(l.MaxLinesPerCue, model.DefaultMaxLinesPerCue),
		minDur: model.DefaultMinDuration,
		maxDur: cmp.Or(l.MaxDuration, model.DefaultMaxDuration),
		cps:    l.MaxCharactersPerSecond,
		maxGap: DefaultMaxWordGap,
	}
	if l.MinDuration != nil {
		s.minDur = *l.MinDuration
	}
	if o.MaxWordGap != nil {
		s.maxGap = *o.MaxWordGap
	}
	if s.maxGap < 0 {
		return s, &model.InvalidArgumentError{
			Option: "Options.MaxWordGap",
			Msg:    "The maximum word gap must not be negative, got " + timecode.Seconds(s.maxGap) + ".",
		}
	}
	s.minDur, s.maxDur = s.minDur.Round(time.Millisecond), s.maxDur.Round(time.Millisecond)
	s.maxGap = s.maxGap.Round(time.Millisecond)
	return s, nil
}

// fits reports whether lines shown from start to end keep the limits.
func (s settings) fits(lines []string, start, end time.Duration) bool {
	d := (end - start).Round(time.Millisecond)
	if d > s.maxDur || s.tooLong(linewrap.VisibleCharacters(lines)) {
		return false
	}
	if _, ok := linewrap.WrapToFit(lines, s.cpl, s.lines); !ok {
		return false
	}
	return linewrap.FitsCharactersPerSecond(lines, d, s.cps)
}

// tooLong reports whether n visible characters cannot fit in the lines, so that fits needs no wrap.
func (s settings) tooLong(n int) bool {
	return n > s.lines*s.cpl+s.lines
}

// wrap wraps lines as model.MergeShortCues does, or as WrapLines does when they do not fit. A source cue with the
// right-to-left embedding gives each new line its own embedding.
func (s settings) wrap(lines, source []string) []string {
	wrapped, ok := linewrap.WrapToFit(lines, s.cpl, s.lines)
	if !ok {
		wrapped = linewrap.Wrap(lines, s.cpl, s.lines, false)
	}
	if markupcore.HoldsRightToLeftEmbedding(source) {
		for i, line := range wrapped {
			wrapped[i] = markupcore.EmbedRightToLeft(line)
		}
	}
	return wrapped
}

// SplitLong splits each cue that breaks the limits at its best break points, until each part fits. The first part
// is the cue itself with its ID. The other parts have no ID. Image cues and cues that cannot break stay.
// SplitLong returns an *model.InvalidArgumentError when opts.Validate fails, and then changes nothing.
func SplitLong(s *model.Subtitle, opts Options) (Report, error) {
	set, err := opts.resolve()
	if err != nil {
		return Report{}, err
	}
	cues := s.Cues()
	out := make([]model.Cue, 0, len(cues))
	first := make([]int, len(cues))
	for i, c := range cues {
		first[i] = len(out)
		out = append(out, set.splitCue(c)...)
	}
	if len(out) != len(cues) {
		s.ReplaceCues(out, func(old int) int { return first[old] })
	}
	return Report{CuesBefore: len(cues), CuesAfter: len(out)}, nil
}

func (s settings) splitCue(c model.Cue) []model.Cue {
	if c.Image != nil || s.fits(c.Lines, c.Start, max(c.Start, c.End)) {
		return []model.Cue{c}
	}
	ps := newCuePieces(c)
	n := len(ps.pieces)
	if n == 0 {
		return []model.Cue{c}
	}
	times := ps.times(c.Start, c.End)
	breaks := s.findBreaks(&ps, times, 0, n, nil)
	if len(breaks) == 0 {
		return []model.Cue{c}
	}
	bounds := append(append([]int{0}, breaks...), n)
	parts := make([]model.Cue, 0, len(bounds)-1)
	for k := 1; k < len(bounds); k++ {
		a, b := bounds[k-1], bounds[k]
		part := c.Clone()
		if k > 1 {
			part.ID = ""
		}
		part.Start, part.End = times[a], times[b]
		part.Lines = s.wrap(ps.lines(a, b), c.Lines)
		part.Words = nil
		for _, w := range ps.wordIndexes(a, b) {
			part.Words = append(part.Words, c.Words[w])
		}
		part.SyncWords()
		parts = append(parts, part)
	}
	return parts
}

// findBreaks appends the indexes of the pieces that start a new cue, in order, until each part fits.
func (s settings) findBreaks(ps *cuePieces, times []time.Duration, first, after int, breaks []int) []int {
	if s.rangeFits(ps, times, first, after) {
		return breaks
	}
	best := s.bestBreak(ps, times, first, after)
	if best < 0 {
		return breaks
	}
	breaks = s.findBreaks(ps, times, first, best, breaks)
	breaks = append(breaks, best)
	return s.findBreaks(ps, times, best, after, breaks)
}

// bestBreak returns the piece with the lowest rank, then nearest to the middle, that leaves both parts at
// MinDuration, or -1.
func (s settings) bestBreak(ps *cuePieces, times []time.Duration, first, after int) int {
	best := -1
	var bestRank, bestDist int
	for i := first + 1; i < after; i++ {
		rank := ps.ranks[i]
		if rank < 0 ||
			(times[i]-times[first]).Round(time.Millisecond) < s.minDur ||
			(times[after]-times[i]).Round(time.Millisecond) < s.minDur {
			continue
		}
		dist := max(ps.positions[i]*2-ps.positions[first]-ps.positions[after], ps.positions[first]+ps.positions[after]-ps.positions[i]*2)
		if best < 0 || rank < bestRank || rank == bestRank && dist < bestDist {
			best, bestRank, bestDist = i, rank, dist
		}
	}
	return best
}

func (s settings) rangeFits(ps *cuePieces, times []time.Duration, first, after int) bool {
	if (times[after] - times[first]).Round(time.Millisecond) > s.maxDur {
		return false
	}
	if s.tooLong(ps.positions[after] - ps.positions[first]) {
		return false
	}
	return s.fits(ps.lines(first, after), times[first], times[after])
}

// timedPiece is a piece of a cue with the time span that ByWords gives it.
type timedPiece struct {
	cue        int
	index      int
	start, end time.Duration
}

// ByWords builds new cues from the word timings of the cues. It starts a new cue after a sentence end, at a pause
// of MaxWordGap, before a word that breaks a limit, and at a cue that cannot join. Image cues and cues without
// words stay as they are. ByWords returns an *model.InvalidArgumentError when opts.Validate fails, and then
// changes nothing.
func ByWords(s *model.Subtitle, opts Options) (Report, error) {
	set, err := opts.resolve()
	if err != nil {
		return Report{}, err
	}
	cues := s.Cues()
	b := wordBuilder{set: set, cues: cues, pieces: make([]cuePieces, len(cues)), newIndex: make([]int, len(cues))}
	for i, c := range cues {
		var ps cuePieces
		if c.Image == nil && len(c.Words) > 0 {
			ps = newCuePieces(c)
		}
		if !slices.ContainsFunc(ps.pieces, piece.hasTime) {
			b.close()
			b.newIndex[i] = len(b.out)
			b.out = append(b.out, c)
			continue
		}
		b.pieces[i] = ps
		b.addCue(i)
	}
	b.close()
	s.ReplaceCues(b.out, func(old int) int { return b.newIndex[old] })
	return Report{CuesBefore: len(cues), CuesAfter: len(b.out)}, nil
}

type wordBuilder struct {
	set      settings
	cues     []model.Cue
	pieces   []cuePieces
	newIndex []int
	group    []timedPiece
	out      []model.Cue
}

// pieceStarts returns the start of each piece of cue ci. A piece without a usable time starts where the piece before
// it starts.
func (b *wordBuilder) pieceStarts(ci int) []time.Duration {
	c, ps := b.cues[ci], &b.pieces[ci]
	start := c.Start
	starts := make([]time.Duration, len(ps.pieces))
	for i, p := range ps.pieces {
		if p.hasTime() && p.time >= start && p.time <= c.End {
			start = p.time
		}
		starts[i] = start
	}
	return starts
}

func (b *wordBuilder) addCue(ci int) {
	c, ps := b.cues[ci], &b.pieces[ci]
	starts := b.pieceStarts(ci)
	for i := range ps.pieces {
		end := max(c.End, starts[i])
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		tp := timedPiece{cue: ci, index: i, start: starts[i], end: end}
		if last := b.last(); last != nil && b.mustClose(*last, tp, c) {
			b.close()
		}
		b.group = append(b.group, tp)
		if ps.ends[i] {
			b.close()
		}
	}
}

// mustClose reports whether the group must end before tp, a piece of cue c.
func (b *wordBuilder) mustClose(last, tp timedPiece, c model.Cue) bool {
	if last.cue != tp.cue && !b.cues[last.cue].CanJoin(c) {
		return true
	}
	if (tp.start - last.end).Round(time.Millisecond) >= b.set.maxGap {
		return true
	}
	return !b.fits(append(slices.Clone(b.group), tp))
}

func (b *wordBuilder) last() *timedPiece {
	if len(b.group) == 0 {
		return nil
	}
	return &b.group[len(b.group)-1]
}

// lines returns the text of the group. The pieces of one source cue are consecutive.
func (b *wordBuilder) lines(group []timedPiece) []string {
	var texts []string
	first := 0
	for i, p := range group {
		if i+1 == len(group) || group[i+1].cue != p.cue {
			texts = append(texts, b.pieces[p.cue].text(group[first].index, p.index+1))
			first = i + 1
		}
	}
	return strings.Split(strings.Join(texts, "\n"), "\n")
}

func (b *wordBuilder) fits(group []timedPiece) bool {
	return b.set.fits(b.lines(group), group[0].start, group[len(group)-1].end)
}

// close builds one cue from the group and starts a new group.
func (b *wordBuilder) close() {
	g := b.group
	b.group = nil
	if len(g) == 0 {
		return
	}
	first := g[0]
	source := b.cues[first.cue]
	c := source.Clone()
	c.Words = nil
	for _, p := range g {
		for _, w := range b.pieces[p.cue].pieces[p.index].words {
			c.Words = append(c.Words, b.cues[p.cue].Words[w])
		}
		if p.index == 0 {
			b.newIndex[p.cue] = len(b.out)
		}
	}
	if first.index != 0 {
		c.ID = ""
	}
	c.Start, c.End = first.start, g[len(g)-1].end
	c.Lines = b.set.wrap(b.lines(g), source.Lines)
	c.SyncWords()
	b.out = append(b.out, c)
}

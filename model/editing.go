package model

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

var voiceTagRegexp = regexp.MustCompile(`(?i)` + markupcore.VoiceTagPattern)

// Append adds copies of the cues of other with their start, end and words moved by offset.
// A time that becomes negative becomes 0. The cues take their place in start order, and each comment stays before its cue.
// A comment of s after its last cue comes before the first copied cue. Metadata and file format data of s win
// over other, per key. Other stays as it is. Append returns an error that matches ErrInvalidArgument and changes
// nothing when a copied cue would end at 100,000 hours or later.
func (s *Subtitle) Append(other *Subtitle, offset time.Duration) error {
	r := retiming{shift: offset}
	if err := checkTimeLimit(other.cues, r, nil); err != nil {
		return err
	}

	n := len(s.cues)
	copies := other.Cues()
	for i := range copies {
		c := &copies[i]
		c.Start, c.End = r.at(c.Start), r.at(c.End)
		c.moveWords(r.at)
	}
	anchors := s.commentAnchors()
	if len(copies) > 0 {
		for k, a := range anchors {
			if a < 0 {
				anchors[k] = n
			}
		}
	}
	for _, a := range other.commentAnchors() {
		if a >= 0 {
			a += n
		}
		anchors = append(anchors, a)
	}
	s.comments = append(s.comments, other.comments...)
	s.addMissingKeys(other)
	s.place(append(s.cues, copies...), anchors)
	return nil
}

// addMissingKeys copies the metadata and the file format data of other for each key that s lacks.
func (s *Subtitle) addMissingKeys(other *Subtitle) {
	for k, v := range other.metadata {
		if _, ok := s.metadata[k]; !ok {
			s.SetMetadata(k, v)
		}
	}
	for k, d := range other.formatData {
		if _, ok := s.formatData[k]; ok {
			continue
		}
		if s.formatData == nil {
			s.formatData = map[string]*Data{}
		}
		s.formatData[k] = d
	}
}

// Slice returns a copy of s with the cues that overlap the range from to to, cut at both bounds. A cue that only
// touches the range drops. With moveToZero, the times and words of the copy move back by from, and a time
// below 0 becomes 0. The copy keeps the metadata, the file format data and the comments before the kept cues.
// A comment after the last cue stays when the last cue stays. Slice returns an error that matches
// ErrInvalidArgument when from is after to.
func (s *Subtitle) Slice(from, to time.Duration, moveToZero bool) (*Subtitle, error) {
	if from > to {
		return nil, invalidArgument(fmt.Sprintf("The slice start %s must not be after the slice end %s.",
			timecode.Seconds(from), timecode.Seconds(to)))
	}
	toZero := retiming{shift: -from}
	return s.copyWith(func(c *Cue) bool {
		if c.End <= from || c.Start >= to {
			return false
		}
		c.Start, c.End = max(c.Start, from), min(c.End, to)
		if !moveToZero {
			c.Start, c.End = c.Start.Round(time.Millisecond), c.End.Round(time.Millisecond)
			return true
		}
		// PHP subtracts from before it rounds, so a from of 2.0005 s turns a start of 2.0005 s into 0, not 0.001 s.
		c.Start, c.End = toZero.at(c.Start), toZero.at(c.End)
		c.moveWords(toZero.at)
		return true
	}), nil
}

// ForcedCuesOnly returns a copy of s with the forced cues. The copy keeps the metadata, the file format data and
// the comments before the kept cues. A comment after the last cue stays when the last cue is forced.
func (s *Subtitle) ForcedCuesOnly() *Subtitle {
	return s.copyWith(func(c *Cue) bool { return c.Forced })
}

// copyWith returns a copy of s with the copies of the cues for which keep returns true, after keep changed them.
// Keep must not change the start order.
func (s *Subtitle) copyWith(keep func(c *Cue) bool) *Subtitle {
	out := &Subtitle{metadata: maps.Clone(s.metadata), formatData: maps.Clone(s.formatData)}
	newIndex := make([]int, len(s.cues))
	for i, c := range s.cues {
		newIndex[i] = -1
		c = c.Clone()
		if keep(&c) {
			newIndex[i] = len(out.cues)
			out.cues = append(out.cues, c)
		}
	}

	keepsEnd := len(s.cues) > 0 && newIndex[len(s.cues)-1] >= 0
	var anchors []int
	for k, a := range s.commentAnchors() {
		if (a < 0 && keepsEnd) || (a >= 0 && newIndex[a] >= 0) {
			out.comments = append(out.comments, s.comments[k])
			anchors = append(anchors, a)
		}
	}
	out.rebuildComments(anchors, func(old int) int { return newIndex[old] })
	return out
}

// SplitCue splits the cue at index at the time at. The first cue keeps the lines 1 to splitAfterLine and ends at at.
// The second cue starts at at, holds the other lines, has no ID, and copies the alignment, the forced flag, the speaker and
// the format data. It takes its place in start order after the first cue. Words do not move.
// A word goes to the first cue when its text starts in one of the first lines, else to the second cue.
// SplitCue returns an error that matches ErrCueNotFound when no cue has the index, and an error that matches
// ErrInvalidArgument when at is not inside the cue or splitAfterLine leaves no line for one of the 2 cues.
func (s *Subtitle) SplitCue(index int, at time.Duration, splitAfterLine int) error {
	if index < 0 || index >= len(s.cues) {
		return cueDoesNotExist("split", index)
	}
	c := s.cues[index]
	if at <= c.Start || at >= c.End {
		return invalidArgument(fmt.Sprintf("Cannot split cue %d at %s: the time must be after the cue start %s and before the cue end %s.",
			index, timecode.Seconds(at), timecode.Seconds(c.Start), timecode.Seconds(c.End)))
	}
	if splitAfterLine < 1 || splitAfterLine >= len(c.Lines) {
		return invalidArgument(fmt.Sprintf("Cannot split cue %d after line %d: the cue has %d lines.", index, splitAfterLine, len(c.Lines)))
	}

	at = at.Round(time.Millisecond)
	second := c
	second.ID = ""
	second.Start = at
	second.Lines = slices.Clone(c.Lines[splitAfterLine:])
	second.Words, c.Words = splitWords(c, splitAfterLine)
	c.End = at
	c.Lines = slices.Clip(c.Lines[:splitAfterLine])
	c.SyncWords()
	second.SyncWords()
	s.cues[index] = c

	later := s.cues[index+1:]
	pos := index + 1 + sort.Search(len(later), func(j int) bool { return later[j].Start >= at })
	anchors := s.commentAnchors()
	s.cues = slices.Insert(s.cues, pos, second)
	s.index = nil
	s.rebuildComments(anchors, func(old int) int {
		if old >= pos {
			return old + 1
		}
		return old
	})
	return nil
}

// splitWords returns the words of c that come after the first lines, and the words that come before.
func splitWords(c Cue, after int) (later, earlier []Word) {
	marks := markupcore.WordMarks(c.Lines, wordTexts(c.Words))
	for i, w := range c.Words {
		if i < len(marks) && marks[i].Line < after {
			earlier = append(earlier, w)
		} else {
			later = append(later, w)
		}
	}
	return later, earlier
}

// JoinCues joins the cues first to last into cue first, with the lines and the words of all of them in order and the
// latest end. When the cues have different speakers, each speaker goes into a <v> tag of its own lines.
// The joined cue keeps the start, the ID, the alignment, the forced flag and the format data of cue first, and
// the comments before the joined cues move before it. JoinCues returns an error that matches ErrCueNotFound
// when no cue has the index first or last, and an error that matches ErrInvalidArgument when first is not below last
// or a cue of the range holds an image.
func (s *Subtitle) JoinCues(first, last int) error {
	for _, i := range []int{first, last} {
		if i < 0 || i >= len(s.cues) {
			return cueDoesNotExist("join", i)
		}
	}
	if first >= last {
		return invalidArgument(fmt.Sprintf("Cannot join cues %d to %d: the first index must be lower than the last index.", first, last))
	}

	if i := slices.IndexFunc(s.cues[first:last+1], func(c Cue) bool { return c.Image != nil }); i >= 0 {
		return invalidArgument(fmt.Sprintf("Cannot join cues %d to %d: cue %d holds an image, and a join would drop it.", first, last, first+i))
	}

	joined := joinParts(s.cues[first], s.cues[first:last+1])
	for _, c := range s.cues[first : last+1] {
		joined.End = max(joined.End, c.End)
	}

	anchors := s.commentAnchors()
	s.cues[first] = joined
	s.cues = slices.Delete(s.cues, first+1, last+1)
	s.index = nil
	s.rebuildComments(anchors, func(old int) int {
		switch {
		case old > last:
			return old - (last - first)
		case old > first:
			return first
		default:
			return old
		}
	})
	return nil
}

// RemoveDuplicateCues never joins an image cue. It joins each run of neighbouring cues with the same text, speaker and
// words, tags included, that overlap, touch or are at most maxGap apart. The cues of a run also have the same alignment,
// forced flag, speakers and format data. A run joins into its first cue, which keeps its lines and words and ends at
// the latest end of the run.
// The comments before the joined cues move before it. RemoveDuplicateCues rounds maxGap to whole milliseconds,
// and returns an error that matches ErrInvalidArgument when maxGap is below 0.
func (s *Subtitle) RemoveDuplicateCues(maxGap time.Duration) error {
	if maxGap < 0 {
		return invalidArgument(fmt.Sprintf("The maximum gap must be 0 or more seconds, got %s.", timecode.Seconds(maxGap)))
	}
	maxGap = maxGap.Round(time.Millisecond)
	s.joinRuns(func(first Cue, runEnd time.Duration, c Cue) bool {
		return textKey(c) == textKey(first) && canJoin(first, c) && sameFormatData(first.formatData, c.formatData) &&
			c.Start-runEnd <= maxGap && first.Start-c.End <= maxGap
	}, func(run []Cue) []Cue { return run[:1] })
	return nil
}

// MergeSameTimeCues never joins an image cue. It joins each run of neighbouring cues whose start and end both differ by at most timeTolerance
// from the first cue of the run. The cues of a run also have the same alignment and forced flag. The joined cue
// holds the lines and the words of the run in order, without the lines and the words of a cue whose text, speaker
// and words came before in the run. It keeps the
// start, the ID and the format data of the first cue and ends at the latest end of the run. The comments before
// the joined cues move before it. MergeSameTimeCues rounds timeTolerance to whole milliseconds, and returns
// an error that matches ErrInvalidArgument when timeTolerance is below 0.
func (s *Subtitle) MergeSameTimeCues(timeTolerance time.Duration) error {
	if timeTolerance < 0 {
		return invalidArgument(fmt.Sprintf("The time tolerance must be 0 or more seconds, got %s.", timecode.Seconds(timeTolerance)))
	}
	tol := timeTolerance.Round(time.Millisecond)
	s.joinRuns(func(first Cue, _ time.Duration, c Cue) bool {
		return first.Image == nil && c.Image == nil && c.Forced == first.Forced && c.Position.Alignment.Numpad() == first.Position.Alignment.Numpad() &&
			(c.Start-first.Start).Abs() <= tol && (c.End-first.End).Abs() <= tol
	}, func(run []Cue) []Cue {
		var parts []Cue
		seen := make(map[string]bool, len(run))
		for _, c := range run {
			if key := textKey(c); !seen[key] {
				seen[key] = true
				parts = append(parts, c)
			}
		}
		return parts
	})
	return nil
}

// joinRuns joins each run of neighbouring cues into the first cue of the run, which ends at the latest end of the run
// and gets the lines, the words and the speaker of the cues that partsOf returns for the run, as joinParts joins them. A cue continues the run when continues returns true for the
// first cue of the run, the latest end of the run so far, and the cue. The comments before the joined cues
// move before the first cue.
func (s *Subtitle) joinRuns(continues func(first Cue, runEnd time.Duration, c Cue) bool, partsOf func(run []Cue) []Cue) {
	n := len(s.cues)
	newIndex := make([]int, n)
	out := make([]Cue, 0, n)
	for i := 0; i < n; {
		joined := s.cues[i]
		j := i + 1
		for j < n && continues(s.cues[i], joined.End, s.cues[j]) {
			joined.End = max(joined.End, s.cues[j].End)
			j++
		}
		if j-i > 1 {
			joined = joinParts(joined, partsOf(s.cues[i:j]))
		}
		for k := i; k < j; k++ {
			newIndex[k] = len(out)
		}
		out = append(out, joined)
		i = j
	}
	if len(out) == n {
		return
	}

	anchors := s.commentAnchors()
	s.cues = out
	s.index = nil
	s.rebuildComments(anchors, func(old int) int { return newIndex[old] })
}

// canJoin reports whether a and b hold no image and have the same alignment, the same forced flag and the same speakers.
func canJoin(a, b Cue) bool {
	return a.Image == nil && b.Image == nil && a.Position.Alignment.Numpad() == b.Position.Alignment.Numpad() && a.Forced == b.Forced && slices.Equal(speakers(a), speakers(b))
}

// speakers returns Speaker and the names of the <v> tags of c, trimmed, without repeats, in byte order.
func speakers(c Cue) []string {
	matches := voiceTagRegexp.FindAllStringSubmatch(c.Text(), -1)
	names := make([]string, 0, len(matches)+1)
	if c.Speaker != "" {
		names = append(names, c.Speaker)
	}
	for _, m := range matches {
		names = append(names, strings.Trim(m[2], markupcore.PHPTrimSet))
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// textKey identifies the text of c for the comparison of cues: the text, the speaker, and the text and start of each word.
func textKey(c Cue) string {
	var b strings.Builder
	b.WriteString(c.Text())
	b.WriteByte(0)
	b.WriteString(c.Speaker)
	for _, w := range c.Words {
		b.WriteByte(0)
		b.WriteString(w.Text)
		b.WriteByte(0)
		b.WriteString(strconv.FormatInt(w.Start.Milliseconds(), 10))
	}
	return b.String()
}

// joinParts returns first with the lines and the words of the parts in order. It leaves out a word that starts before
// the last kept word. When all parts have the same speaker, the joined cue keeps it. Else each part writes its voice
// into its own lines as a <v> tag, so a joined cue with 2 voices holds 2 <v> tags and a single voice becomes the
// speaker again.
func joinParts(first Cue, parts []Cue) Cue {
	first.Lines, first.Words = nil, nil
	sameSpeaker := true
	for _, p := range parts {
		sameSpeaker = sameSpeaker && p.Speaker == parts[0].Speaker
	}
	for _, p := range parts {
		lines := p.Lines
		if !sameSpeaker {
			lines = markupcore.TagWords(lines, nil, p.voice())
		}
		first.Lines = append(first.Lines, lines...)
		for _, w := range p.Words {
			if n := len(first.Words); n == 0 || w.Start >= first.Words[n-1].Start {
				first.Words = append(first.Words, w)
			}
		}
	}
	first.Words = cloneWords(first.Words)
	if sameSpeaker {
		first.Speaker = parts[0].Speaker
		return first
	}
	first.Speaker = ""
	first.ExtractWordTags()
	return first
}

func sameFormatData(a, b map[string]*Data) bool {
	return maps.EqualFunc(a, b, sameData)
}

func sameData(a, b *Data) bool {
	if a == nil || b == nil {
		return a.Len() == 0 && b.Len() == 0
	}
	if !slices.Equal(a.keys, b.keys) {
		return false
	}
	for _, k := range a.keys {
		if !sameValue(a.values[k], b.values[k]) {
			return false
		}
	}
	return true
}

func sameValue(a, b any) bool {
	switch x := a.(type) {
	case []any:
		y, ok := b.([]any)
		return ok && slices.EqualFunc(x, y, sameValue)
	case *Data:
		y, ok := b.(*Data)
		return ok && sameData(x, y)
	default:
		return a == b
	}
}

func cueDoesNotExist(action string, i int) error {
	return fmt.Errorf("%w: Cannot %s cue %d: the cue does not exist.", ErrCueNotFound, action, i) //nolint:revive,staticcheck // the message is the PHP sentence, as docs/errors.md says
}

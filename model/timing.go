package model

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

// FixOverlaps ends each cue at least minGap before the next later start, but never before its own start.
// Cues with the same start all end before the next later start, and the cues of the last start keep their ends.
// FixOverlaps returns an error that matches ErrInvalidArgument when minGap is below 0.
func (s *Subtitle) FixOverlaps(minGap time.Duration) error {
	if err := checkMinGap(minGap); err != nil {
		return err
	}
	groupStart, laterStart := time.Duration(0), time.Duration(0)
	hasGroup, hasLater := false, false
	for i := len(s.cues) - 1; i >= 0; i-- {
		c := &s.cues[i]
		if !hasGroup || c.Start != groupStart {
			laterStart, hasLater = groupStart, hasGroup
			groupStart, hasGroup = c.Start, true
		}
		if !hasLater {
			continue
		}
		if latest := addClamped(laterStart, -minGap).Round(time.Millisecond); c.End > latest {
			c.End = max(c.Start, latest)
		}
	}
	s.index = nil
	return nil
}

// ExtendShortCues makes each cue last at least minDuration, but ends it at least minGap before the start of
// the next cue. It never shortens a cue, the last cue extends freely, and a cue with the same start as the cue
// before it keeps its end. ExtendShortCues returns an error that matches ErrInvalidArgument and changes nothing
// when minDuration is not above 0, when minGap is below 0, or when a cue would end at 100,000 hours or later.
func (s *Subtitle) ExtendShortCues(minDuration, minGap time.Duration) error {
	if minDuration <= 0 {
		return invalidArgument(fmt.Sprintf("The minimum duration must be greater than 0, got %s.", timecode.Seconds(minDuration)))
	}
	if err := checkMinGap(minGap); err != nil {
		return err
	}
	ends := make([]time.Duration, len(s.cues))
	for i, c := range s.cues {
		ends[i] = c.End
		if i > 0 && s.cues[i-1].Start == c.Start {
			continue
		}
		next, hasNext := time.Duration(0), i+1 < len(s.cues)
		if hasNext {
			next = addClamped(s.cues[i+1].Start, -minGap)
		}
		end, err := extendedEnd(i, c.End, c.Start, minDuration, next, hasNext)
		if err != nil {
			return err
		}
		ends[i] = end
	}
	s.setEnds(ends)
	return nil
}

// extendedEnd returns from + d, capped at maxEnd when capped is true and rounded to whole milliseconds, or oldEnd
// when that is later. It returns the time limit error of retiming when the new end of cue i would be
// 100,000 hours or later.
func extendedEnd(i int, oldEnd, from, d, maxEnd time.Duration, capped bool) (time.Duration, error) {
	end, seconds := addClamped(from, d), retiming{shift: d}.seconds(from)
	if capped && maxEnd < end {
		end, seconds = maxEnd, maxEnd.Round(time.Millisecond).Seconds()
	}
	if end = end.Round(time.Millisecond); end <= oldEnd {
		return oldEnd, nil
	}
	if seconds >= timecode.MaxTime.Seconds() {
		return 0, timeLimitError(i, seconds)
	}
	return end, nil
}

func (s *Subtitle) setEnds(ends []time.Duration) {
	for i, end := range ends {
		s.cues[i].End = end
	}
	s.index = nil
}

// AddLeadInOut shows each cue leadIn earlier and leadOut longer, but at least minGap away from its neighbours.
// The lead-out comes first, so it takes the space between 2 cues before the lead-in of the next cue.
// A start or end inside another cue does not move, and no start goes below 0. AddLeadInOut returns an error that
// matches ErrInvalidArgument and changes nothing when leadIn, leadOut or minGap is below 0, or when a cue would end
// at 100,000 hours or later.
func (s *Subtitle) AddLeadInOut(leadIn, leadOut, minGap time.Duration) error {
	if leadIn < 0 {
		return invalidArgument(fmt.Sprintf("The lead-in must not be negative, got %s.", timecode.Seconds(leadIn)))
	}
	if leadOut < 0 {
		return invalidArgument(fmt.Sprintf("The lead-out must not be negative, got %s.", timecode.Seconds(leadOut)))
	}
	if err := checkMinGap(minGap); err != nil {
		return err
	}

	starts, ends := make([]time.Duration, len(s.cues)), make([]time.Duration, len(s.cues))
	for i, c := range s.cues {
		starts[i], ends[i] = c.Start, c.End
	}
	startInside, endInside := insideOtherCues(starts, ends)
	if leadOut > 0 {
		newEnds, err := s.leadOutEnds(starts, ends, endInside, leadOut, minGap)
		if err != nil {
			return err
		}
		s.setEnds(newEnds)
	}
	if leadIn > 0 {
		s.addLeadIn(starts, startInside, leadIn, minGap)
	}
	s.index = nil
	if !startsInOrder(s.cues) {
		s.place(s.cues, s.commentAnchors())
	}
	return nil
}

// leadOutEnds returns the ends after the lead-out. It caps each new end at minGap before the first cue that
// starts at or after the old end of the cue.
func (s *Subtitle) leadOutEnds(starts, ends []time.Duration, endInside []bool, leadOut, minGap time.Duration) ([]time.Duration, error) {
	n := len(s.cues)
	newEnds := slices.Clone(ends)
	for i, c := range s.cues {
		if endInside[i] {
			continue
		}
		next := sort.Search(n, func(j int) bool { return starts[j] >= ends[i] })
		if next == i {
			next++
		}
		maxEnd := time.Duration(0)
		if next < n {
			maxEnd = addClamped(starts[next], -minGap)
		}
		end, err := extendedEnd(i, c.End, ends[i], leadOut, maxEnd, next < n)
		if err != nil {
			return nil, err
		}
		newEnds[i] = end
	}
	return newEnds, nil
}

// addLeadIn floors each new start at minGap after the latest end at or before the old start of the cue.
func (s *Subtitle) addLeadIn(starts []time.Duration, startInside []bool, leadIn, minGap time.Duration) {
	n := len(s.cues)
	byEnd := make([]int, n)
	for i := range byEnd {
		byEnd[i] = i
	}
	slices.SortStableFunc(byEnd, func(a, b int) int { return cmp.Compare(s.cues[a].End, s.cues[b].End) })
	for i := range s.cues {
		if startInside[i] {
			continue
		}
		start := max(0, addClamped(starts[i], -leadIn))
		prev := sort.Search(n, func(j int) bool { return s.cues[byEnd[j]].End > starts[i] }) - 1
		if prev >= 0 && byEnd[prev] == i {
			prev--
		}
		if prev >= 0 {
			start = max(start, addClamped(s.cues[byEnd[prev]].End, minGap))
		}
		if start = start.Round(time.Millisecond); start < starts[i] {
			s.cues[i].Start = start
		}
	}
}

// insideOtherCues reports for each cue whether its start and its end lie inside another cue. The starts are in
// ascending order. A start is inside when an earlier cue ends after it, or a later cue with the same start ends
// after it. An end is inside when an earlier cue ends at or after it, or a later cue that starts before it
// ends at or after it.
func insideOtherCues(starts, ends []time.Duration) (startInside, endInside []bool) {
	n := len(starts)
	startInside, endInside = make([]bool, n), make([]bool, n)
	maxEnd := time.Duration(math.MinInt64)
	for i := range n {
		startInside[i] = i > 0 && maxEnd > starts[i]
		endInside[i] = i > 0 && maxEnd >= ends[i]
		for k := i + 1; k < n && starts[k] < ends[i] && !endInside[i]; k++ {
			endInside[i] = ends[k] >= ends[i]
		}
		for k := i + 1; k < n && starts[k] == starts[i] && !startInside[i]; k++ {
			startInside[i] = ends[k] > starts[i]
		}
		maxEnd = max(maxEnd, ends[i])
	}
	return startInside, endInside
}

// LimitLongCues ends each cue at most maxDuration after its start. It never lengthens a cue.
// LimitLongCues returns an error that matches ErrInvalidArgument when maxDuration is not above 0.
func (s *Subtitle) LimitLongCues(maxDuration time.Duration) error {
	if maxDuration <= 0 {
		return invalidArgument(fmt.Sprintf("The maximum duration must be greater than 0, got %s.", timecode.Seconds(maxDuration)))
	}
	for i := range s.cues {
		c := &s.cues[i]
		if end := addClamped(c.Start, maxDuration).Round(time.Millisecond); end < c.End {
			c.End = end
		}
	}
	s.index = nil
	return nil
}

func checkMinGap(minGap time.Duration) error {
	if minGap < 0 {
		return invalidArgument(fmt.Sprintf("The minimum gap must not be negative, got %s.", timecode.Seconds(minGap)))
	}
	return nil
}

// addClamped returns a + b, or the nearest time.Duration when the sum overflows.
func addClamped(a, b time.Duration) time.Duration {
	sum := a + b
	switch {
	case b > 0 && sum < a:
		return math.MaxInt64
	case b < 0 && sum > a:
		return math.MinInt64
	}
	return sum
}

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
// before it keeps its end. ExtendShortCues returns an error that matches ErrInvalidArgument when minDuration is
// not above 0 or minGap is below 0.
func (s *Subtitle) ExtendShortCues(minDuration, minGap time.Duration) error {
	if minDuration <= 0 {
		return invalidArgument(fmt.Sprintf("The minimum duration must be greater than 0, got %s.", timecode.Seconds(minDuration)))
	}
	if err := checkMinGap(minGap); err != nil {
		return err
	}
	for i := range s.cues {
		c := &s.cues[i]
		if i > 0 && s.cues[i-1].Start == c.Start {
			continue
		}
		end := addClamped(c.Start, minDuration)
		if i+1 < len(s.cues) {
			end = min(end, addClamped(s.cues[i+1].Start, -minGap))
		}
		if end = end.Round(time.Millisecond); end > c.End {
			c.End = end
		}
	}
	s.index = nil
	return nil
}

// AddLeadInOut shows each cue leadIn earlier and leadOut longer, but at least minGap away from its neighbours.
// The lead-out comes first, so it takes the space between 2 cues before the lead-in of the next cue.
// A start or end inside another cue does not move, and no start goes below 0. AddLeadInOut returns an error that
// matches ErrInvalidArgument when leadIn, leadOut or minGap is below 0.
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
		s.addLeadOut(starts, ends, endInside, leadOut, minGap)
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

// addLeadOut caps each new end at minGap before the first cue that starts at or after the old end of the cue.
func (s *Subtitle) addLeadOut(starts, ends []time.Duration, endInside []bool, leadOut, minGap time.Duration) {
	n := len(s.cues)
	for i := range s.cues {
		if endInside[i] {
			continue
		}
		end := addClamped(ends[i], leadOut)
		next := sort.Search(n, func(j int) bool { return starts[j] >= ends[i] })
		if next == i {
			next++
		}
		if next < n {
			end = min(end, addClamped(starts[next], -minGap))
		}
		if end = end.Round(time.Millisecond); end > ends[i] {
			s.cues[i].End = end
		}
	}
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

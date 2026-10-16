package refsync

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// segmentGap is the least time between the end of an earlier cue and the first cue of a later segment.
const segmentGap = time.Millisecond

// apply retimes s with the segments of a report. It checks the time limit first and changes nothing on an error.
func apply(s *model.Subtitle, segments []Segment) error {
	if len(segments) == 1 {
		return applyOne(s, segments[0])
	}

	cues := s.Cues()
	segmentOf := make([]int, len(cues))
	for i, c := range cues {
		k := len(segments) - 1
		for k > 0 && c.Start < segments[k].From {
			k--
		}
		segmentOf[i] = k
	}
	if err := checkTimeLimit(cues, func(i int, t time.Duration) float64 { return segments[segmentOf[i]].millis(t) }); err != nil {
		return err
	}

	members := make([][]int, len(segments))
	for i := range cues {
		seg := segments[segmentOf[i]]
		c := &cues[i]
		c.Start, c.End = seg.at(c.Start), seg.at(c.End)
		c.MapWordTimestamps(seg.at)
		members[segmentOf[i]] = append(members[segmentOf[i]], i)
	}
	endBeforeLaterSegments(cues, members)
	s.ReplaceCues(cues, func(old int) int { return old })
	return nil
}

// endBeforeLaterSegments ends each cue that starts before the first cue of a later segment at least segmentGap
// before that cue. members holds the cue indexes of each segment.
func endBeforeLaterSegments(cues []model.Cue, members [][]int) {
	for k := 1; k < len(members); k++ {
		if len(members[k]) == 0 {
			continue
		}
		laterStart := cues[members[k][0]].Start
		for _, i := range members[k] {
			laterStart = min(laterStart, cues[i].Start)
		}
		for _, earlier := range members[:k] {
			for _, i := range earlier {
				if c := &cues[i]; c.Start < laterStart && c.End > laterStart-segmentGap {
					c.End = max(c.Start, laterStart-segmentGap)
				}
			}
		}
	}
}

// applyOne scales and then shifts s, as PHP does. The check comes first, so a shift that fails after the scale
// cannot leave s half retimed.
func applyOne(s *model.Subtitle, seg Segment) error {
	if seg.Scale != 1 && seg.Offset > 0 {
		offset := float64(seg.Offset / time.Millisecond)
		if err := checkTimeLimit(s.Cues(), func(_ int, t time.Duration) float64 {
			return math.Round(float64(t/time.Millisecond)*seg.Scale) + offset
		}); err != nil {
			return err
		}
	}
	if seg.Scale != 1 {
		if err := s.Scale(seg.Scale); err != nil {
			return err
		}
	}
	if seg.Offset != 0 {
		return s.Shift(seg.Offset)
	}
	return nil
}

// checkTimeLimit returns the error of model retiming when millis moves the end of a cue to 100,000 hours or later.
func checkTimeLimit(cues []model.Cue, millis func(i int, t time.Duration) float64) error {
	limit := timecode.MaxTime.Seconds()
	for i, c := range cues {
		if end := millis(i, max(c.Start, c.End)) / 1000; !(end < limit) { // also true for NaN
			return &model.InvalidArgumentError{Msg: timecode.TimeLimitMessage(i+1, end)}
		}
	}
	return nil
}

// millis returns the new time of t in whole milliseconds, not clamped.
func (seg Segment) millis(t time.Duration) float64 {
	return math.Round(float64(t/time.Millisecond)*seg.Scale + float64(seg.Offset/time.Millisecond))
}

func (seg Segment) at(t time.Duration) time.Duration {
	ms := seg.millis(t)
	switch {
	case ms <= 0:
		return 0
	case ms >= float64(timecode.MaxRetimed/time.Millisecond):
		return timecode.MaxRetimed
	}
	return time.Duration(ms) * time.Millisecond
}

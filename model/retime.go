package model

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

// SyncPoint is a time in the subtitle and the time where it belongs, for SyncByPoints.
type SyncPoint struct {
	// Old is the time in the subtitle now.
	Old time.Duration
	// New is the time where Old belongs.
	New time.Duration
}

// maxRetimed is the largest time.Duration in whole milliseconds. A word timestamp that a retiming moves
// beyond it stops there.
const maxRetimed = time.Duration(math.MaxInt64 / int64(time.Millisecond) * int64(time.Millisecond))

// Shift moves every cue and its word timestamps by d. See ShiftRange for the rules.
func (s *Subtitle) Shift(d time.Duration) error {
	return s.ShiftRange(d, nil, nil)
}

// ShiftRange moves each cue with from <= Start < to, and its word timestamps, by d. A nil bound sets no limit on that side.
// The start decides, so a cue across to moves as a whole. A time that becomes negative becomes 0, and the cue stays.
// Each sum is rounded to whole milliseconds, and the moved cues take their place in start order with their comments.
// ShiftRange returns an error that matches ErrInvalidArgument and changes nothing when to is not after from,
// or when a cue would end at 100,000 hours or later.
func (s *Subtitle) ShiftRange(d time.Duration, from, to *time.Duration) error {
	if from != nil && to != nil && *to <= *from {
		return invalidArgument(fmt.Sprintf("The end time of the shift must be after its start time, got %s to %s.",
			timecode.Seconds(*from), timecode.Seconds(*to)))
	}
	return s.retime(retiming{shift: d}, func(start time.Duration) bool {
		return (from == nil || start >= *from) && (to == nil || start < *to)
	})
}

// Scale multiplies the start, the end and the word timestamps of every cue by factor, rounded to whole milliseconds.
// It returns an error that matches ErrInvalidArgument and changes nothing when factor is not a finite number above 0,
// or when a cue would end at 100,000 hours or later.
func (s *Subtitle) Scale(factor float64) error {
	if !positiveFinite(factor) {
		return invalidArgument(fmt.Sprintf("The scale factor must be greater than 0, got %s.", timecode.Number(factor)))
	}
	return s.retime(retiming{segments: []segment{{factor: factor}}}, nil)
}

// ConvertFrameRate moves the cues of a video at the frame rate from to the same video at the frame rate to,
// in frames per second. It calls Scale(from / to), so 25 to 23.976 makes every time longer by 25/23.976.
// It returns an error that matches ErrInvalidArgument when a frame rate is not a finite number above 0.
func (s *Subtitle) ConvertFrameRate(from, to float64) error {
	for _, rate := range []float64{from, to} {
		if !positiveFinite(rate) {
			return invalidArgument(fmt.Sprintf("The frame rate must be greater than 0, got %s.", timecode.Number(rate)))
		}
	}
	return s.Scale(from / to)
}

// SyncByPoints moves the Old time of each point to its New time and maps all times in between on the line through
// the neighbouring points. Times before the first point and after the last point follow the nearest 2 points.
// 1 point shifts every cue by New - Old. Start, end and each word timestamp map on their own, and a time
// that becomes negative becomes 0. SyncByPoints returns an error that matches ErrInvalidArgument and changes nothing
// when points is empty, when the Old and New times do not both increase strictly, or when a cue would end at
// 100,000 hours or later.
func (s *Subtitle) SyncByPoints(points []SyncPoint) error {
	if len(points) == 0 {
		return invalidArgument("The sync points must not be empty.")
	}
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if b.Old <= a.Old || b.New <= a.New {
			return invalidArgument(fmt.Sprintf("The old and new times of the sync points must both increase, got %s=%s before %s=%s.",
				timecode.Seconds(a.Old), timecode.Seconds(a.New), timecode.Seconds(b.Old), timecode.Seconds(b.New)))
		}
	}
	if len(points) == 1 {
		return s.retime(retiming{shift: points[0].New - points[0].Old}, nil)
	}

	segments := make([]segment, len(points)-1)
	for i := range segments {
		a, b := points[i], points[i+1]
		factor := float64(b.New-a.New) / float64(b.Old-a.Old)
		segments[i] = segment{from: a.Old, factor: factor, offset: milliseconds(a.New) - milliseconds(a.Old)*factor}
	}
	return s.retime(retiming{segments: segments}, nil)
}

// retiming maps a time either by adding shift, when segments is nil, or through the line of the last segment
// that starts at or before the time. A time before the first segment uses the first segment.
type retiming struct {
	shift    time.Duration
	segments []segment
}

// segment maps a time t in milliseconds to t * factor + offset.
type segment struct {
	from   time.Duration
	factor float64
	offset float64
}

// seconds returns the new time of t in seconds, unclamped and unrounded, so a check sees values beyond time.Duration.
func (r retiming) seconds(t time.Duration) float64 {
	if r.segments == nil {
		if sum, ok := add(t, r.shift); ok {
			return sum.Seconds()
		}
		return t.Seconds() + r.shift.Seconds()
	}
	return r.milliseconds(t) / 1000
}

// at returns the new time of t, clamped to 0 and rounded to whole milliseconds.
func (r retiming) at(t time.Duration) time.Duration {
	if r.segments == nil {
		sum, ok := add(t, r.shift)
		switch {
		case !ok && r.shift > 0:
			return maxRetimed
		case !ok:
			return 0
		}
		return min(max(0, sum.Round(time.Millisecond)), maxRetimed)
	}
	ms := math.Round(r.milliseconds(t))
	switch {
	case ms <= 0:
		return 0
	case ms >= float64(maxRetimed/time.Millisecond):
		return maxRetimed
	}
	return time.Duration(ms) * time.Millisecond
}

func (r retiming) milliseconds(t time.Duration) float64 {
	i := max(0, sort.Search(len(r.segments), func(i int) bool { return r.segments[i].from > t })-1)
	return milliseconds(t)*r.segments[i].factor + r.segments[i].offset
}

// retime checks every cue whose start affected accepts, or every cue when affected is nil, against the time limit,
// and only then maps the times of these cues through r and sorts the cues once.
func (s *Subtitle) retime(r retiming, affected func(start time.Duration) bool) error {
	limit := timecode.MaxTime.Seconds()
	for i, c := range s.cues {
		if affected != nil && !affected(c.Start) {
			continue
		}
		if end := r.seconds(max(c.Start, c.End)); end >= limit {
			return invalidArgument(fmt.Sprintf("The retimed cue %d would end at %s s. Cue times must be below %d hours.",
				i+1, timecode.Number(end), timecode.MaxHours))
		}
	}

	for i := range s.cues {
		c := &s.cues[i]
		if affected != nil && !affected(c.Start) {
			continue
		}
		c.Start, c.End = r.at(c.Start), r.at(c.End)
		c.MapWordTimestamps(r.at)
	}
	s.index = nil
	if !startsInOrder(s.cues) {
		s.place(s.cues, s.anchors(nil))
	}
	return nil
}

// add returns t + d and false when the sum overflows.
func add(t, d time.Duration) (time.Duration, bool) {
	sum := t + d
	return sum, (d > 0) == (sum > t) || d == 0
}

// milliseconds is exact for a whole number of milliseconds, also above 2^53 nanoseconds.
func milliseconds(t time.Duration) float64 {
	return float64(t/time.Millisecond) + float64(t%time.Millisecond)/float64(time.Millisecond)
}

func positiveFinite(f float64) bool {
	return f > 0 && !math.IsInf(f, 1)
}

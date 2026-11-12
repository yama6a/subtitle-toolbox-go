package timing

import (
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Report counts the cue times that moved by 1 frame or more.
type Report struct {
	MovedStarts int
	MovedEnds   int
}

// snapper holds the frame numbers of one Apply call.
type snapper struct {
	shots        []int64
	starts, ends []int64
	window       int64
	minGap       int64
	minDuration  int64
}

// Apply moves the cue times of s to the shot changes, closes short gaps and puts every time on a frame.
// It returns an *model.InvalidArgumentError for bad options and then changes nothing.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	cues := s.Cues()
	if len(cues) == 0 {
		return Report{}, nil
	}
	r := opts.FrameRate
	sn := &snapper{
		shots:       make([]int64, len(opts.ShotChanges)),
		starts:      make([]int64, len(cues)),
		ends:        make([]int64, len(cues)),
		window:      opts.window(),
		minGap:      opts.minGap(),
		minDuration: opts.minDuration(),
	}
	for i, d := range opts.ShotChanges {
		sn.shots[i] = r.Frames(d)
	}
	slices.Sort(sn.shots)
	sn.shots = slices.Compact(sn.shots)
	for i, c := range cues {
		sn.starts[i], sn.ends[i] = r.Frames(c.Start), r.Frames(c.End)
	}
	originalStarts, originalEnds := slices.Clone(sn.starts), slices.Clone(sn.ends)

	sn.snapEnds()
	sn.snapStarts()
	if !opts.KeepGaps {
		sn.chain()
	}

	var report Report
	for i := range cues {
		if sn.starts[i] != originalStarts[i] {
			report.MovedStarts++
		}
		if sn.ends[i] != originalEnds[i] {
			report.MovedEnds++
		}
		cues[i].Start = onFrame(r, sn.starts[i])
		cues[i].End = onFrame(r, sn.ends[i])
	}
	s.ReplaceCues(cues, func(old int) int { return old })
	return report, nil
}

// onFrame returns the time of frame n in whole milliseconds. It stops at the largest cue time, because the last frame
// below 100000 h can round up to it.
func onFrame(r model.FrameRate, n int64) time.Duration {
	return min(r.Duration(n).Round(time.Millisecond), timecode.MaxValidTime)
}

func (sn *snapper) snapEnds() {
	for i := range sn.ends {
		at, _ := slices.BinarySearch(sn.shots, sn.ends[i])
		if at == len(sn.shots) || sn.shots[at]-sn.ends[i] > sn.window {
			continue
		}
		if end := sn.shots[at] - sn.minGap; sn.allowed(i, sn.starts[i], end) {
			sn.ends[i] = end
		}
	}
}

func (sn *snapper) snapStarts() {
	for i := range sn.starts {
		at, found := slices.BinarySearch(sn.shots, sn.starts[i])
		if found {
			at++
		}
		if at == 0 || sn.starts[i]-sn.shots[at-1] > sn.window {
			continue
		}
		if shot := sn.shots[at-1]; sn.allowed(i, shot, sn.ends[i]) {
			sn.starts[i] = shot
		}
	}
}

// allowed reports whether cue i can take the times start and end.
func (sn *snapper) allowed(i int, start, end int64) bool {
	duration := end - start
	if duration <= 0 || duration < sn.minDuration && duration < sn.ends[i]-sn.starts[i] {
		return false
	}
	if i+1 < len(sn.starts) && end > sn.starts[i+1]-sn.minGap && end > sn.ends[i] {
		return false
	}
	return i == 0 || start >= sn.ends[i-1]+sn.minGap || start >= sn.starts[i]
}

// chain ends each cue minGap frames before the next cue when the gap is longer than minGap and shorter than the
// window, and no shot change lies inside the gap.
func (sn *snapper) chain() {
	for i := 0; i+1 < len(sn.starts); i++ {
		gap := sn.starts[i+1] - sn.ends[i]
		if gap <= sn.minGap || gap >= sn.window {
			continue
		}
		at, _ := slices.BinarySearch(sn.shots, sn.ends[i]+1)
		if at == len(sn.shots) || sn.shots[at] >= sn.starts[i+1] {
			sn.ends[i] = sn.starts[i+1] - sn.minGap
		}
	}
}

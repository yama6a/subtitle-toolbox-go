// Package refsync syncs a subtitle to a reference subtitle by the cue times alone, so the reference can be in another
// language. It finds the frame rate scale, the offset and the points where the offset jumps, then retimes the subtitle.
package refsync

import (
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Report is the result of Apply.
type Report struct {
	// Offset is added after the scale. With splits, it is the offset of the first segment.
	Offset time.Duration
	// Scale is the factor applied first: 1, or the ratio of 2 of the frame rates 23.976, 24 and 25.
	Scale float64
	// Score is the time that both files cover after the sync divided by the time that either file covers, from 0 to 1.
	Score float64
	// Segments holds the parts of the subtitle in time order. Without a split, 1 segment from 0 with no end.
	Segments []Segment
}

// Segment is a part of the subtitle that Apply retimes with one scale and offset.
type Segment struct {
	// From is the cue start before the sync where the segment starts, 0 for the first segment.
	From time.Duration
	// To is the From of the next segment, nil for the last segment.
	To *time.Duration
	// Scale is the factor applied first.
	Scale float64
	// Offset is added after the scale.
	Offset time.Duration
}

// params holds the settings of one search in seconds.
type params struct {
	lo, hi       float64
	noScale      bool
	maxSplits    int
	splitPenalty float64
}

// Apply finds the scale, the offset and the splits that make the cue times of s match the cue times of
// opts.Reference, then retimes s in place. A caller who only wants the report passes s.Clone().
// Apply returns an error that matches model.ErrInvalidArgument and changes nothing when opts is invalid or when
// a cue would end at 100,000 hours or later.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	lo, hi := opts.offsets()
	r := find(s, opts.Reference, params{
		lo: seconds(lo.Milliseconds()), hi: seconds(hi.Milliseconds()),
		noScale: opts.NoScale, maxSplits: opts.MaxSplits, splitPenalty: opts.splitPenalty(),
	})
	if err := apply(s, r.Segments); err != nil {
		return Report{}, err
	}
	return r, nil
}

// scales returns 1 and the factors between the frame rates 24 and 23.976, 25 and 24, 25 and 23.976. A video
// converted between 2 of these plays at another speed, so its subtitles drift.
func scales(noScale bool) []float64 {
	if noScale {
		return []float64{1}
	}
	out := []float64{1}
	for _, pair := range [][2]float64{{24, 23.976}, {25, 24}, {25, 23.976}} {
		out = append(out, pair[0]/pair[1], pair[1]/pair[0])
	}
	return out
}

func find(target, reference *model.Subtitle, p params) Report {
	targetSpans, referenceSpans := spans(target), spans(reference)
	best := report(0, 1, 0, nil)
	if len(targetSpans) == 0 || len(referenceSpans) == 0 {
		return best
	}
	targetTime, referenceTime := totalTime(targetSpans), totalTime(referenceSpans)
	var split splitChoice
	for _, scale := range scales(p.noScale) {
		scaled := make([]span, len(targetSpans))
		for i, sp := range targetSpans {
			scaled[i] = span{sp.start * scale, sp.end * scale}
		}
		score := func(v float64) float64 { return v / (targetTime*scale + referenceTime - v) }

		coarseBest := p.lo + float64(firstMax(overlapPerOffset(scaled, referenceSpans, p.lo, p.hi)))*coarseStep
		offset, v := refine(scaled, referenceSpans, coarseBest, p.lo, p.hi)
		if sc := score(v); sc > best.Score+epsilon {
			best = report(milliseconds(offset), scale, sc, nil)
		}
		if p.maxSplits > 0 {
			split.consider(segmentCandidates(targetSpans, scaled, referenceSpans, p), scale, score, p.splitPenalty)
		}
	}
	if split.ok && split.value > best.Score+epsilon {
		return split.report
	}
	return best
}

// splitChoice is the best result with splits so far. Its value is the score minus the penalty of its splits.
type splitChoice struct {
	report Report
	value  float64
	ok     bool
}

func (c *splitChoice) consider(candidates []candidate, scale float64, score func(float64) float64, penalty float64) {
	for _, cand := range candidates {
		sc := score(cand.overlap)
		value := sc - float64(len(cand.segments)-1)*penalty
		if !c.ok || value > c.value+epsilon {
			*c = splitChoice{report: report(cand.segments[0].offset, scale, sc, cand.segments), value: value, ok: true}
		}
	}
}

// report builds a Report with the score clamped to 0 to 1. Nil segments give 1 segment from 0.
func report(offsetMillis int64, scale, score float64, segments []segmentResult) Report {
	if segments == nil {
		segments = []segmentResult{{offset: offsetMillis}}
	}
	r := Report{
		Offset:   time.Duration(offsetMillis) * time.Millisecond,
		Scale:    scale,
		Score:    min(1, max(0, score)),
		Segments: make([]Segment, len(segments)),
	}
	for i, sr := range segments {
		r.Segments[i] = Segment{
			From: time.Duration(sr.from) * time.Millisecond, Scale: scale, Offset: time.Duration(sr.offset) * time.Millisecond,
		}
		if i > 0 {
			to := r.Segments[i].From
			r.Segments[i-1].To = &to
		}
	}
	return r
}

// milliseconds rounds seconds to whole milliseconds, as the PHP offsets are rounded.
func milliseconds(sec float64) int64 {
	return int64(math.Round(sec * 1000))
}

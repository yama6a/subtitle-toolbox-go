package refsync

import (
	"cmp"
	"math"
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	epsilon    = 1e-9
	coarseStep = 0.1
	fineStep   = 0.01
)

// span is a time interval in seconds that at least 1 cue covers.
type span struct{ start, end float64 }

// seconds gives the same float64 as the PHP time of a whole number of milliseconds. d.Seconds() can differ in the
// last bit.
func seconds(ms int64) float64 {
	return float64(ms) / 1000
}

// spans returns the cue times of s as sorted spans without overlaps, so time that 2 cues share counts once.
func spans(s *model.Subtitle) []span {
	var out []span
	for _, c := range s.All() {
		if c.End > c.Start {
			out = append(out, span{seconds(c.Start.Milliseconds()), seconds(c.End.Milliseconds())})
		}
	}
	slices.SortFunc(out, func(a, b span) int {
		return cmp.Or(cmp.Compare(a.start, b.start), cmp.Compare(a.end, b.end))
	})
	merged := out[:0]
	for _, sp := range out {
		if last := len(merged) - 1; last >= 0 && sp.start <= merged[last].end {
			merged[last].end = max(merged[last].end, sp.end)
			continue
		}
		merged = append(merged, sp)
	}
	return merged
}

func totalTime(spans []span) float64 {
	var sum float64
	for _, sp := range spans {
		sum += sp.end - sp.start
	}
	return sum
}

// overlap returns the time that target, moved by offset, shares with reference.
func overlap(target, reference []span, offset float64) float64 {
	var sum float64
	sweep(target, reference, offset, func(_ int, d float64) { sum += d })
	return sum
}

// overlapPerSpan returns the time that each span of target, moved by offset, shares with reference.
func overlapPerSpan(target, reference []span, offset float64) []float64 {
	out := make([]float64, len(target))
	sweep(target, reference, offset, func(i int, d float64) { out[i] += d })
	return out
}

func sweep(target, reference []span, offset float64, add func(i int, d float64)) {
	for i, j := 0, 0; i < len(target) && j < len(reference); {
		targetEnd := target[i].end + offset
		from := max(target[i].start+offset, reference[j].start)
		to := min(targetEnd, reference[j].end)
		if to > from {
			add(i, to-from)
		}
		if targetEnd < reference[j].end {
			i++
		} else {
			j++
		}
	}
}

// coarseCount returns the index of the last coarse offset from lo to hi.
func coarseCount(lo, hi float64) int {
	return int(math.Floor((hi-lo)/coarseStep + epsilon))
}

// overlapPerOffset returns the overlap for each offset from lo to hi in steps of coarseStep. The overlap of a target
// span and a reference span is the sum of 4 ramps w * max(0, o - k), which add up in second differences on the grid.
func overlapPerOffset(target, reference []span, lo, hi float64) []float64 {
	last := coarseCount(lo, hi)
	maxGridOffset := lo + float64(last)*coarseStep
	secondDiffs := make([]float64, last+2)
	var firstValue, firstSlope float64

	addRamp := func(weight, corner float64) {
		position := (corner - lo) / coarseStep
		if position < 0 {
			firstValue += weight * (lo - corner)
			firstSlope += weight * coarseStep
			return
		}
		index := int(math.Floor(position))
		if index >= last {
			return
		}
		fraction := position - float64(index)
		if index == 0 {
			firstSlope += weight * (1 - fraction) * coarseStep
		} else {
			secondDiffs[index] += weight * (1 - fraction) * coarseStep
		}
		secondDiffs[index+1] += weight * fraction * coarseStep
	}

	firstCandidate := 0
	for _, t := range target {
		for firstCandidate < len(reference) && reference[firstCandidate].end-t.start <= lo {
			firstCandidate++
		}
		for j := firstCandidate; j < len(reference) && reference[j].start-t.end < maxGridOffset; j++ {
			r := reference[j]
			addRamp(1, r.start-t.end)
			addRamp(-1, r.start-t.start)
			addRamp(-1, r.end-t.end)
			addRamp(1, r.end-t.start)
		}
	}

	return integrate(firstValue, firstSlope, secondDiffs, last)
}

// integrate returns the values on the grid from the first value, the first slope and the second differences.
func integrate(firstValue, firstSlope float64, secondDiffs []float64, last int) []float64 {
	values := make([]float64, 1, last+1)
	values[0] = firstValue
	if last >= 1 {
		values = append(values, firstValue+firstSlope)
	}
	for n := 1; n < last; n++ {
		values = append(values, 2*values[n]-values[n-1]+secondDiffs[n])
	}
	return values
}

// refine returns the offset with the highest overlap within coarseStep of coarseBest, in steps of fineStep, and its
// overlap.
func refine(target, reference []span, coarseBest, lo, hi float64) (offset, best float64) {
	offset, best = coarseBest, overlap(target, reference, coarseBest)
	from, to := max(lo, coarseBest-coarseStep), min(hi, coarseBest+coarseStep)
	steps := int(math.Floor((to-from)/fineStep + epsilon))
	for n := 0; n <= steps; n++ {
		candidate := from + float64(n)*fineStep
		if v := overlap(target, reference, candidate); v > best+epsilon {
			offset, best = candidate, v
		}
	}
	return offset, best
}

// firstMax returns the index of the first maximum of values.
func firstMax(values []float64) int {
	best := 0
	for i, v := range values {
		if v > values[best] {
			best = i
		}
	}
	return best
}

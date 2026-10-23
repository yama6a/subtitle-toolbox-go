package dual_test

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func randomSubtitle(rng *rand.Rand, prefix string) *model.Subtitle {
	var s model.Subtitle
	for i := range rng.IntN(25) {
		start := time.Duration(rng.IntN(60)) * 500 * time.Millisecond
		length := time.Duration(rng.IntN(12)) * 500 * time.Millisecond
		if rng.IntN(8) == 0 {
			length = time.Duration(rng.IntN(40)) * time.Second
		}
		s.AddCue(model.Cue{Start: start, End: start + length, Lines: []string{prefix + strconv.Itoa(i)}})
	}
	return &s
}

// naiveStack is the search of the PHP code: every secondary cue against every primary cue.
func naiveStack(primary, secondary *model.Subtitle) []got {
	cues := primary.Cues()
	partners := make([][]model.Cue, len(cues))
	var unpaired []model.Cue
	for _, c := range secondary.Cues() {
		best, bestOverlap := -1, time.Duration(0)
		for i, p := range cues {
			if overlap := min(p.End, c.End) - max(p.Start, c.Start); overlap > bestOverlap {
				best, bestOverlap = i, overlap
			}
		}
		if best < 0 {
			unpaired = append(unpaired, c)
		} else {
			partners[best] = append(partners[best], c)
		}
	}
	for i, group := range partners {
		for _, c := range group {
			cues[i].Lines = append(cues[i].Lines, c.Lines...)
			cues[i].Start = min(cues[i].Start, c.Start)
			cues[i].End = max(cues[i].End, c.End)
		}
	}
	all := slices.Concat(cues, unpaired)
	slices.SortStableFunc(all, func(a, b model.Cue) int { return int(a.Start - b.Start) })
	out := make([]got, len(all))
	for i, c := range all {
		out[i] = got{c.Start, c.End, c.Text(), c.Alignment}
	}
	return out
}

// naiveSnap is the loop of the PHP code over every primary time.
func naiveSnap(t time.Duration, times []time.Duration, tolerance time.Duration) time.Duration {
	snapped, distance := t, time.Duration(-1)
	for _, c := range times {
		d := max(c-t, t-c)
		if d <= tolerance && (distance < 0 || d < distance) {
			snapped, distance = c, d
		}
	}
	return snapped
}

func naiveTopBottom(primary, secondary *model.Subtitle, tol time.Duration) []got {
	times := make([]time.Duration, 0, 2*primary.Len())
	for _, c := range primary.Cues() {
		times = append(times, c.Start, c.End)
	}
	slices.Sort(times)
	all := primary.Cues()
	for _, c := range secondary.Cues() {
		start, end := naiveSnap(c.Start, times, tol), naiveSnap(c.End, times, tol)
		if end > start {
			c.Start, c.End = start, end
		}
		c.Alignment = model.TopCenter
		all = append(all, c)
	}
	slices.SortStableFunc(all, func(a, b model.Cue) int { return int(a.Start - b.Start) })
	out := make([]got, len(all))
	for i, c := range all {
		out[i] = got{c.Start, c.End, c.Text(), c.Alignment}
	}
	return out
}

func TestFromPairMatchesTheNaiveSearch(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(52, 1)) //nolint:gosec // a fixed seed makes the test repeat
	for range 500 {
		primary, secondary := randomSubtitle(rng, "p"), randomSubtitle(rng, "s")

		stacked, err := dual.FromPair(primary, secondary, dual.Options{})
		require.NoError(t, err)
		assert.Equal(t, naiveStack(primary, secondary), describe(stacked))

		tol := time.Duration(rng.IntN(5)) * 250 * time.Millisecond
		apart, err := dual.FromPair(primary, secondary, dual.Options{Mode: dual.ModeTopBottom, TimeTolerance: &tol})
		require.NoError(t, err)
		assert.Equal(t, naiveTopBottom(primary, secondary, tol), describe(apart), "tolerance %s", tol)
	}
}

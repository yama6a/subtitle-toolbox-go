package dual_test

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// longCuePair returns n primary cues of 1 s every 2 s, one primary cue over the whole film, and n secondary cues
// that start 0.5 s later. The long primary cue overlaps every secondary cue.
func longCuePair(n int) (primary, secondary *model.Subtitle) {
	primary, secondary = &model.Subtitle{}, &model.Subtitle{}
	cues, others := make([]model.Cue, 0, n+1), make([]model.Cue, 0, n)
	cues = append(cues, model.Cue{Start: 0, End: time.Duration(2*n) * time.Second, Lines: []string{"title"}})
	for i := range n {
		start := time.Duration(2*i) * time.Second
		cues = append(cues, model.Cue{Start: start, End: start + time.Second, Lines: []string{"first"}})
		others = append(others, model.Cue{Start: start + 500*time.Millisecond, End: start + 1500*time.Millisecond, Lines: []string{"erste"}})
	}
	primary.AddCues(cues...)
	secondary.AddCues(others...)
	return primary, secondary
}

// TestFromPairGrowth times FromPair at n and 10n cues, best of 3 each, and fails above a ratio of 30.
func TestFromPairGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	for _, mode := range []dual.Mode{dual.ModeStack, dual.ModeTopBottom} {
		best := func(n int) time.Duration {
			primary, secondary := longCuePair(n)
			fastest := time.Duration(math.MaxInt64)
			for range 3 {
				begin := time.Now()
				_, err := dual.FromPair(primary, secondary, dual.Options{Mode: mode})
				require.NoError(t, err)
				fastest = min(fastest, time.Since(begin))
			}
			return fastest
		}
		small, large := best(2000), best(20000)
		ratio := float64(large) / float64(max(small, time.Nanosecond))
		assert.LessOrEqual(t, ratio, 30.0, "%s: 2000 cues took %v, 20000 cues took %v", mode, small, large)
	}
}

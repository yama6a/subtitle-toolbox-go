package markupcore

import (
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

// removeWordTagsByPasses removes the word timestamp tags with one full scan per nesting level.
func removeWordTagsByPasses(lines []string) []string {
	for {
		removed := false
		out := make([]string, len(lines))
		for i, line := range lines {
			parts := SplitTags(line)
			var b strings.Builder
			for j, part := range parts {
				if j%2 == 1 {
					if d, ok := WordTimestamp(part); ok && d < timecode.MaxTime {
						removed = true
						continue
					}
				}
				b.WriteString(part)
			}
			out[i] = b.String()
		}
		if out = CleanLines(out); !removed {
			return out
		}
		lines = out
	}
}

func TestRemoveWordTagsMatchesThePassLoop(t *testing.T) {
	t.Parallel()

	pieces := []string{"<", ">", "<", ">", "00:00:01.000", "00:00:00.000", "<00:00:02.500>", "99:59:59.999", "a", " ", "\t", "\r", "&amp;", "</b>", "<b>", "1000000:00:00.000"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed makes the test repeat
	for range 20000 {
		lines := make([]string, 1+rng.IntN(3))
		for i := range lines {
			var b strings.Builder
			for range rng.IntN(12) {
				b.WriteString(pieces[rng.IntN(len(pieces))])
			}
			lines[i] = b.String()
		}
		assert.Equal(t, removeWordTagsByPasses(lines), removeWordTags(lines), "%q", lines)
	}
}

// nestedWordTags returns a line of n levels, each a "<" that a removed tag lets close against the next timestamp.
func nestedWordTags(n int) string {
	return strings.Repeat("<", n) + "<00:00:00.000>" + strings.Repeat("00:00:00.000>", n)
}

func BenchmarkRemoveWordTagsNested(b *testing.B) {
	lines := []string{nestedWordTags(70000)}
	b.SetBytes(int64(len(lines[0])))
	for b.Loop() {
		removeWordTags(lines)
	}
}

// TestRemoveWordTagsGrowth times removeWordTags at n and 10n levels, best of 3 each, and fails above a ratio of 30.
func TestRemoveWordTagsGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	best := func(n int) time.Duration {
		lines := []string{nestedWordTags(n)}
		fastest := time.Duration(math.MaxInt64)
		for range 3 {
			begin := time.Now()
			removeWordTags(lines)
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	small, large := best(7000), best(70000)
	ratio := float64(large) / float64(max(small, time.Nanosecond))
	assert.LessOrEqual(t, ratio, 30.0, "7000 levels took %v, 70000 levels took %v", small, large)
}

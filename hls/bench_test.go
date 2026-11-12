package hls_test

import (
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/hls"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// BenchmarkSegment cuts the 30 cues of node-webvtt-subs1.vtt, repeated every 132 s to 51000 cues, and writes every
// segment.
func BenchmarkSegment(b *testing.B) {
	const copies = 1700
	cues := readFixture(b).Cues()
	input := &model.Subtitle{}
	all := make([]model.Cue, 0, copies*len(cues))
	for i := range copies {
		shift := time.Duration(i) * 132 * time.Second
		for _, c := range cues {
			c.Start, c.End = c.Start+shift, c.End+shift
			all = append(all, c)
		}
	}
	input.AddCues(all...)

	b.Run("cues="+strconv.Itoa(input.Len()), func(b *testing.B) {
		b.ReportAllocs()
		var count int
		for b.Loop() {
			r, err := hls.Segment(input, hls.SegmentOptions{})
			require.NoError(b, err)
			count = 0
			require.NoError(b, r.EachSegment(func(string, []byte) error {
				count++
				return nil
			}))
		}
		assert.Equal(b, 37399, count)
	})
}

// TestSegmentGrowth times Segment and EachSegment at n and 10n cues, best of 3 each, and fails above a ratio of 30.
func TestSegmentGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	cues := readFixture(t).Cues()
	best := func(copies int) time.Duration {
		input := &model.Subtitle{}
		all := make([]model.Cue, 0, copies*len(cues))
		for i := range copies {
			shift := time.Duration(i) * 132 * time.Second
			for _, c := range cues {
				c.Start, c.End = c.Start+shift, c.End+shift
				all = append(all, c)
			}
		}
		input.AddCues(all...)
		fastest := time.Duration(math.MaxInt64)
		for range 3 {
			begin := time.Now()
			r, err := hls.Segment(input, hls.SegmentOptions{})
			require.NoError(t, err)
			require.NoError(t, r.EachSegment(func(string, []byte) error { return nil }))
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	small, large := best(170), best(1700)
	ratio := float64(large) / float64(max(small, time.Nanosecond))
	assert.LessOrEqual(t, ratio, 30.0, "5100 cues took %v, 51000 cues took %v", small, large)
}

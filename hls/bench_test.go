package hls_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/hls"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
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

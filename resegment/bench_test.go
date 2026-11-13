package resegment_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
)

func bakeryCopies(tb testing.TB, copies int) *model.Subtitle {
	tb.Helper()

	bakery, _, err := srt.Read(strings.NewReader(string(testfiles.Read(tb, "statistics/own_bakery.srt"))), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(tb, err)
	var s model.Subtitle
	for i := range copies {
		for _, c := range bakery.All() {
			c.Start += time.Duration(i) * 12 * time.Second
			c.End += time.Duration(i) * 12 * time.Second
			s.AddCue(c)
		}
	}
	return &s
}

func TestSplitLongLeavesCuesThatFit(t *testing.T) {
	t.Parallel()

	s := bakeryCopies(t, 50)
	before := describe(s)
	n := s.Len()

	report, err := resegment.SplitLong(s, resegment.Options{})
	require.NoError(t, err)

	assert.Equal(t, resegment.Report{CuesBefore: n, CuesAfter: n}, report)
	assert.Equal(t, before, describe(s))
}

func BenchmarkSplitLong(b *testing.B) {
	b.Run("cues=40000", func(b *testing.B) {
		s := bakeryCopies(b, 10000)
		b.ReportAllocs()
		for b.Loop() {
			_, err := resegment.SplitLong(s, resegment.Options{})
			require.NoError(b, err)
		}
	})
	b.Run("cueBytes=1000000", func(b *testing.B) {
		words := strings.Repeat("alpha beta gamma delta. ", 1_000_000/24)
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			var s model.Subtitle
			s.AddCue(model.Cue{End: 3 * time.Hour, Lines: []string{words}})
			b.StartTimer()
			_, err := resegment.SplitLong(&s, resegment.Options{})
			require.NoError(b, err)
		}
	})
}

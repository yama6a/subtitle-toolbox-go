package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// moveSecondHalfFirst shifts the cues of the second half of s before the first cue, so all cues sort again.
func moveSecondHalfFirst(tb testing.TB, s *model.Subtitle) {
	tb.Helper()

	middle, ok := s.Cue(s.Len() / 2)
	require.True(tb, ok)
	require.NoError(tb, s.ShiftRange(-middle.Start, new(middle.Start), nil))
}

// syncPoints returns n points 30 s apart that stretch the times by 0.1 %.
func syncPoints(n int) []model.SyncPoint {
	points := make([]model.SyncPoint, n)
	for i := range points {
		points[i] = model.SyncPoint{Old: time.Duration(i) * 30 * time.Second, New: time.Duration(i) * 30030 * time.Millisecond}
	}
	return points
}

func TestShiftRangeGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := repeatHarbourTour(t, n)
		return func() { moveSecondHalfFirst(t, s) }
	})
}

func TestSyncByPointsGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 1200, func(n int) func() {
		s := repeatHarbourTour(t, n)
		points := syncPoints(n / 6)
		return func() { require.NoError(t, s.SyncByPoints(points)) }
	})
}

func BenchmarkShiftRange(b *testing.B) {
	full := repeatHarbourTour(b, 20000)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		s := full.Clone()
		b.StartTimer()
		moveSecondHalfFirst(b, s)
	}
}

func BenchmarkSyncByPoints(b *testing.B) {
	full := repeatHarbourTour(b, 20000)
	points := syncPoints(20000 / 6)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		s := full.Clone()
		b.StartTimer()
		require.NoError(b, s.SyncByPoints(points))
	}
}

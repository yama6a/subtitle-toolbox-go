package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestRemoveDuplicateCuesGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := duplicatePairs(t, n)
		return func() { require.NoError(t, s.RemoveDuplicateCues(0)) }
	})
}

func TestMergeSameTimeCuesGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := sameTimePairs(t, n)
		return func() { require.NoError(t, s.MergeSameTimeCues(0)) }
	})
}

func BenchmarkRemoveDuplicateCues(b *testing.B) {
	benchmarkJoin(b, duplicatePairs(b, 20000), func(s *model.Subtitle) error { return s.RemoveDuplicateCues(0) })
}

func BenchmarkMergeSameTimeCues(b *testing.B) {
	benchmarkJoin(b, sameTimePairs(b, 20000), func(s *model.Subtitle) error { return s.MergeSameTimeCues(0) })
}

func benchmarkJoin(b *testing.B, full *model.Subtitle, join func(*model.Subtitle) error) {
	b.Helper()

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		s := full.Clone()
		b.StartTimer()
		require.NoError(b, join(s))
	}
}

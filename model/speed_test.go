package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const speedCueCount = 20000

const speedBudget = 5 * time.Second

// repeatHarbourTour repeats the cues and comments of harbourTour every 30 s up to speedCueCount cues,
// with one AddCue and one AddComment call for each cue.
func repeatHarbourTour(tb testing.TB) *model.Subtitle {
	tb.Helper()

	harbour := harbourTour(tb)
	cues, comments := harbour.Cues(), harbour.Comments()
	s := &model.Subtitle{}
	for i := range speedCueCount {
		c := cues[i%len(cues)]
		offset := time.Duration(i/len(cues)) * 30 * time.Second
		c.Start += offset
		c.End += offset
		s.AddCue(c)
		require.NoError(tb, s.AddComment(comments[i%len(comments)].Text, i))
	}
	return s
}

func skipSpeedTest(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("speed test")
	}
}

func TestAddCueInALoopIsFast(t *testing.T) {
	t.Parallel()
	skipSpeedTest(t)

	begin := time.Now()
	s := repeatHarbourTour(t)

	assert.Less(t, time.Since(begin), speedBudget)
	assert.Equal(t, speedCueCount, s.Len())
	assert.Len(t, s.Comments(), speedCueCount)
	assert.IsNonDecreasing(t, starts(s))
}

func TestRemoveCueInALoopIsFast(t *testing.T) {
	t.Parallel()
	skipSpeedTest(t)

	s := repeatHarbourTour(t)
	want := s.Cues()[500:]

	begin := time.Now()
	for range 500 {
		require.NoError(t, s.RemoveCue(0))
	}

	assert.Less(t, time.Since(begin), speedBudget)
	assert.Equal(t, want, s.Cues())
	comments := s.Comments()
	for i, c := range comments {
		require.Equal(t, max(0, i-500), c.BeforeCue, i)
	}
}

func TestSetCueThenCueIndexAtInALoopIsFast(t *testing.T) {
	t.Parallel()
	skipSpeedTest(t)

	s := repeatHarbourTour(t)
	_, _ = s.CueIndexAt(100 * time.Second)

	begin := time.Now()
	for i := range s.Len() {
		c, _ := s.Cue(i)
		c.Start += time.Millisecond
		c.End += time.Millisecond
		require.NoError(t, s.SetCue(i, c))
		got, ok := s.CueIndexAt(c.Start)
		require.True(t, ok)
		require.Equal(t, i, got)
	}

	assert.Less(t, time.Since(begin), speedBudget)
}

func BenchmarkAddCue(b *testing.B) {
	for b.Loop() {
		repeatHarbourTour(b)
	}
}

func BenchmarkRemoveCue(b *testing.B) {
	full := repeatHarbourTour(b)
	for b.Loop() {
		b.StopTimer()
		s := full.Clone()
		b.StartTimer()
		for range 500 {
			require.NoError(b, s.RemoveCue(0))
		}
	}
}

func BenchmarkSetCueThenCueIndexAt(b *testing.B) {
	s := repeatHarbourTour(b)
	for b.Loop() {
		for i := range s.Len() {
			c, _ := s.Cue(i)
			c.End += time.Millisecond
			require.NoError(b, s.SetCue(i, c))
			_, _ = s.CueIndexAt(c.Start)
		}
	}
}

package model_test

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// repeatHarbourTour repeats the cues and comments of harbourTour every 30 s up to n cues,
// with one AddCue and one AddComment call for each cue.
func repeatHarbourTour(tb testing.TB, n int) *model.Subtitle {
	tb.Helper()

	harbour := harbourTour(tb)
	cues, comments := harbour.Cues(), harbour.Comments()
	s := &model.Subtitle{}
	for i := range n {
		s.AddCue(shifted(cues[i%len(cues)], i/len(cues)))
		require.NoError(tb, s.AddComment(comments[i%len(comments)].Text, i))
	}
	return s
}

func shifted(c model.Cue, copyIndex int) model.Cue {
	offset := time.Duration(copyIndex) * 30 * time.Second
	c.Start += offset
	c.End += offset
	return c
}

func shiftByAMillisecond(tb testing.TB, s *model.Subtitle, i int) model.Cue {
	tb.Helper()

	c, ok := s.Cue(i)
	require.True(tb, ok)
	c.Start += time.Millisecond
	c.End += time.Millisecond
	require.NoError(tb, s.SetCue(i, c))
	return c
}

func TestAddCueInALoop(t *testing.T) {
	t.Parallel()

	s := repeatHarbourTour(t, 600)

	assert.Equal(t, 600, s.Len())
	assert.Len(t, s.Comments(), 600)
	assert.IsNonDecreasing(t, starts(s))
	for i, c := range s.Comments() {
		require.Equal(t, i, c.BeforeCue)
	}
}

func TestRemoveCueInALoop(t *testing.T) {
	t.Parallel()

	s := repeatHarbourTour(t, 600)
	want := s.Cues()[50:]
	for range 50 {
		require.NoError(t, s.RemoveCue(0))
	}

	assert.Equal(t, want, s.Cues())
	for i, c := range s.Comments() {
		require.Equal(t, max(0, i-50), c.BeforeCue, i)
	}
}

func TestEditThenLookupInALoop(t *testing.T) {
	t.Parallel()

	s := repeatHarbourTour(t, 600)
	_, _ = s.CueIndexAt(100 * time.Second)
	for i := range s.Len() {
		c := shiftByAMillisecond(t, s, i)
		got, ok := s.CueIndexAt(c.Start)
		require.True(t, ok)
		require.Equal(t, i, got)
	}
}

// assertGrowth times the operation that setup returns at n and 10n, best of 3 each, and fails above a ratio of 30.
// Linear time gives about 10, quadratic time about 100.
func assertGrowth(t *testing.T, n int, setup func(n int) func()) {
	t.Helper()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	best := func(n int) time.Duration {
		fastest := time.Duration(1<<63 - 1)
		for range 3 {
			run := setup(n)
			begin := time.Now()
			run()
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	small, large := best(n), best(10*n)
	ratio := float64(large) / float64(max(small, time.Nanosecond))
	assert.LessOrEqual(t, ratio, 30.0, "n=%d took %v, 10n took %v", n, small, large)
}

func TestAddCueGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		return func() { repeatHarbourTour(t, n) }
	})
}

func TestRemoveCueGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := repeatHarbourTour(t, n)
		return func() {
			for range 500 {
				require.NoError(t, s.RemoveCue(0))
			}
		}
	})
}

func TestEditThenLookupGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := repeatHarbourTour(t, n)
		_, _ = s.CueIndexAt(0)
		return func() {
			for i := range n {
				c := shiftByAMillisecond(t, s, i)
				_, _ = s.CueIndexAt(c.Start)
			}
		}
	})
}

func BenchmarkAddCue(b *testing.B) {
	harbour := harbourTour(b)
	cues := make([]model.Cue, 20000)
	for i := range cues {
		cues[i] = shifted(harbour.Cues()[i%harbour.Len()], i/harbour.Len())
	}
	b.ReportAllocs()
	for b.Loop() {
		s := &model.Subtitle{}
		for _, c := range cues {
			s.AddCue(c)
		}
	}
}

func BenchmarkRemoveCue(b *testing.B) {
	full := repeatHarbourTour(b, 5000)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		s := full.Clone()
		b.StartTimer()
		for range 500 {
			_ = s.RemoveCue(0)
		}
	}
}

func BenchmarkEditThenLookup(b *testing.B) {
	s := repeatHarbourTour(b, 20000)
	b.ReportAllocs()
	for b.Loop() {
		for i := range s.Len() {
			c := shiftByAMillisecond(b, s, i)
			_, _ = s.CueIndexAt(c.Start)
		}
	}
}

// dotsTester builds n cues from the 3 cues of tests/files/srt/real/language_subtitles_dots_tester.srt of
// subtitle-toolbox-php, copy k shifted by 10 k seconds.
func dotsTester(n int) *model.Subtitle {
	source := []model.Cue{
		{Start: sec(2.501), End: sec(4.5), Lines: []string{"Tester for dots instead of commas in timestamps 1.0 by ale5000", "<b><i>Use VLC 1.0.3 or higher as reference</i></b>"}},
		{Start: sec(4.501), End: sec(8.5), Lines: []string{"{\\an8}Dots instead of commas"}},
		{Start: sec(6.501), End: sec(8.5), Lines: []string{"{\\an8}OK"}},
	}
	cues := make([]model.Cue, n)
	for i := range cues {
		c := source[i%len(source)]
		offset := time.Duration(i/len(source)) * 10 * time.Second
		c.Start += offset
		c.End += offset
		cues[i] = c
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func BenchmarkFindCuesAt(b *testing.B) {
	for _, n := range []int{100, 2000, 20000} {
		s := dotsTester(n)
		last, _ := s.Cue(n - 1)
		rng := rand.New(rand.NewPCG(185, uint64(n))) //nolint:gosec // reproducible test data needs a seeded generator
		times := make([]time.Duration, 1000)
		for i := range times {
			times[i] = time.Duration(rng.Int64N(last.End.Milliseconds()+1)) * time.Millisecond
		}

		b.Run(fmt.Sprintf("cues=%d/alone", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, at := range times {
					_ = s.CuesAt(at)
				}
			}
		})
		b.Run(fmt.Sprintf("cues=%d/after-edit", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for k, at := range times {
					i := k * 7 % n
					c, _ := s.Cue(i)
					c.End += time.Millisecond
					if k%2 == 1 {
						c.End -= 2 * time.Millisecond
					}
					_ = s.SetCue(i, c)
					_ = s.CuesAt(at)
				}
			}
		})
	}
}

// letterWords returns n distinct words of 5 letters, or of 5 digits for digits true.
func letterWords(n int, digits bool) []string {
	base, first := 26, byte('a')
	if digits {
		base, first = 10, '0'
	}
	words := make([]string, n)
	for i := range n {
		w := []byte("00000")
		for j, v := 4, i; j >= 0; j, v = j-1, v/base {
			w[j] = first + byte(v%base)
		}
		words[i] = string(w)
	}
	return words
}

// wordCue returns a cue whose line holds n words, each timed 1 s from its own number.
func wordCue(n int, tagged bool) model.Cue {
	texts := letterWords(n, false)
	words := make([]model.Word, n)
	var line strings.Builder
	for i, w := range texts {
		words[i] = model.Word{Text: w, Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second}
		if tagged {
			line.WriteString("<" + fmt.Sprintf("%02d:%02d:%02d.000", i/3600, i/60%60, i%60) + ">")
		}
		line.WriteString(w + " ")
	}
	return model.Cue{End: time.Duration(n) * time.Second, Lines: []string{strings.TrimSpace(line.String())}, Words: words}
}

func syncCase(n int, missing bool) model.Cue {
	c := wordCue(n, false)
	if missing {
		c.Lines = []string{strings.Join(letterWords(n, true), " ")}
		return c
	}
	c.Lines = []string{strings.ToUpper(c.Lines[0])}
	return c
}

func BenchmarkSyncWords(b *testing.B) {
	for _, name := range []string{"found", "missing"} {
		b.Run(name, func(b *testing.B) {
			base := syncCase(10_000, name == "missing")
			for b.Loop() {
				c := base.Clone()
				c.SyncWords()
			}
		})
	}
}

func TestWordsGrowth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(n int) func()
	}{
		{"ExtractWordTags", func(n int) func() {
			c := wordCue(n, true)
			return func() { c.ExtractWordTags() }
		}},
		{"TaggedLines", func(n int) func() {
			c := wordCue(n, false)
			return func() { c.TaggedLines() }
		}},
		{"ValidateWords", func(n int) func() {
			c := wordCue(n, false)
			return func() { _ = c.ValidateWords() }
		}},
		{"SyncWords found", func(n int) func() {
			c := syncCase(n, false)
			return func() { c.SyncWords() }
		}},
		{"SyncWords repeated missing text", func(n int) func() {
			c := syncCase(n, true)
			for i := range c.Words {
				c.Words[i].Text = "zzzzz"
			}
			return func() { c.SyncWords() }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertGrowth(t, 1000, tt.run)
		})
	}
}

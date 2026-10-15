package model_test

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestCuesAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subtitle func(testing.TB) *model.Subtitle
		at       float64
		want     []int
	}{
		{"overlapping signs at 5 s", func(testing.TB) *model.Subtitle { return weatherSigns() }, 5, []int{1, 2, 3}},
		{"overlapping signs at 8.5 s", func(testing.TB) *model.Subtitle { return weatherSigns() }, 8.5, []int{2, 3, 4}},
		{"on screen at the start", harbourTour, 1, []int{0}},
		{"off screen at the end", harbourTour, 4, []int{1}},
		{"before the first cue", harbourTour, 0.999, nil},
		{"in a gap", harbourTour, 6.2, nil},
		{"after the last cue", harbourTour, 20, nil},
		{"just before the end", harbourTour, 19.999, []int{5}},
		{
			"a cue without length is never on screen",
			func(testing.TB) *model.Subtitle { return fromTimes([2]float64{1, 1}, [2]float64{1, 2}) },
			1,
			[]int{1},
		},
		{
			"negative times",
			func(testing.TB) *model.Subtitle { return fromTimes([2]float64{-2, -1}, [2]float64{-1, 1}) },
			-1,
			[]int{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.subtitle(t)
			assert.Equal(t, tt.want, s.CuesAt(sec(tt.at)))
			i, ok := s.CueIndexAt(sec(tt.at))
			assert.Equal(t, len(tt.want) > 0, ok)
			if ok {
				assert.Equal(t, tt.want[0], i)
			}
		})
	}
}

func TestCuesBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from, to float64
		want     []int
	}{
		{5, 13, []int{1, 2}},
		{13, 17, []int{3, 4}},
		{17, 18, nil},
		{0, 1, nil},
		{0, 1.001, []int{0}},
		{3, 3, []int{0}},
		{20, 30, nil},
	}
	s := harbourTour(t)
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v to %v", tt.from, tt.to), func(t *testing.T) {
			t.Parallel()

			got, err := s.Clone().CuesBetween(sec(tt.from), sec(tt.to))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCuesBetweenReturnsUncutCues(t *testing.T) {
	t.Parallel()

	s := harbourTour(t)
	got, err := s.CuesBetween(sec(5), sec(13))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, got)
	c, ok := s.Cue(1)
	require.True(t, ok)
	assert.Equal(t, sec(4), c.Start)
	c, ok = s.Cue(2)
	require.True(t, ok)
	assert.Equal(t, sec(12.5), c.End)
}

func TestCuesBetweenRejectsAReversedRange(t *testing.T) {
	t.Parallel()

	got, err := (&model.Subtitle{}).CuesBetween(sec(10), sec(5))
	_, fractionErr := (&model.Subtitle{}).CuesBetween(sec(100.25), sec(2.5))
	require.ErrorContains(t, fractionErr, "The range start 100.25 must not be after the range end 2.5.")
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	var argErr *model.InvalidArgumentError
	require.ErrorAs(t, err, &argErr)
	assert.Equal(t, "The range start 10 must not be after the range end 5.", argErr.Msg)
	assert.Nil(t, got)
}

func TestLookupSeesChanges(t *testing.T) {
	t.Parallel()

	at := func(s *model.Subtitle, f float64) int {
		i, ok := s.CueIndexAt(sec(f))
		if !ok {
			return -1
		}
		return i
	}

	t.Run("start change after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 0, at(s, 2))
		got, err := s.CuesBetween(sec(1), sec(5))
		require.NoError(t, err)
		assert.Equal(t, []int{0, 1}, got)

		c, _ := s.Cue(0)
		c.Start = sec(2.5)
		require.NoError(t, s.SetCue(0, c))

		assert.Equal(t, -1, at(s, 2))
		assert.Equal(t, 0, at(s, 2.5))
		got, err = s.CuesBetween(sec(1), sec(2.5))
		require.NoError(t, err)
		assert.Empty(t, got)
		got, err = s.CuesBetween(sec(1), sec(2.6))
		require.NoError(t, err)
		assert.Equal(t, []int{0}, got)
	})

	t.Run("end change after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 0, at(s, 2))

		c, _ := s.Cue(0)
		c.End = sec(1.5)
		require.NoError(t, s.SetCue(0, c))

		assert.Equal(t, -1, at(s, 2))
		assert.Equal(t, 0, at(s, 1.4))
	})

	t.Run("cue moves after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 5, at(s, 19))

		c, _ := s.Cue(5)
		c.Start, c.End = sec(0), sec(0.5)
		require.NoError(t, s.SetCue(5, c))

		assert.Equal(t, 0, at(s, 0.2))
		assert.Equal(t, -1, at(s, 19))
		assert.Equal(t, 1, at(s, 2))
	})

	t.Run("cue added after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, -1, at(s, 30))

		s.AddCue(model.Cue{Start: sec(29), End: sec(31), Lines: []string{"late"}})

		assert.Equal(t, 6, at(s, 30))
	})

	t.Run("cue removed after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 0, at(s, 2))

		require.NoError(t, s.RemoveCue(0))

		assert.Equal(t, -1, at(s, 2))
		assert.Equal(t, 0, at(s, 5))
	})

	t.Run("cues removed by a predicate after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 0, at(s, 2))

		s.RemoveCuesWhere(func(c model.Cue) bool { return c.Start < sec(2) })

		assert.Equal(t, -1, at(s, 2))
	})

	t.Run("cues replaced after a lookup", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		assert.Equal(t, 0, at(s, 2))

		s.ReplaceCues([]model.Cue{{Start: sec(40), End: sec(41)}}, nil)

		assert.Equal(t, -1, at(s, 2))
		assert.Equal(t, 0, at(s, 40))
	})
}

// largeSubtitle has a cue every 2.5 s for 2 s, a 30 s sign every 100 cues and one cue from start to end, 10,101 cues.
func largeSubtitle() *model.Subtitle {
	cues := []model.Cue{{Start: 0, End: sec(25000), Lines: []string{"background"}}}
	for i := range 10000 {
		start := time.Duration(i) * 2500 * time.Millisecond
		cues = append(cues, model.Cue{Start: start, End: start + 2*time.Second, Lines: []string{fmt.Sprintf("line %d", i)}})
		if i%100 == 0 {
			cues = append(cues, model.Cue{Start: start + time.Second, End: start + 31*time.Second, Lines: []string{fmt.Sprintf("sign %d", i)}})
		}
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func TestLookupOnALargeSubtitle(t *testing.T) {
	t.Parallel()

	s := largeSubtitle()
	require.Equal(t, 10101, s.Len())

	tests := []struct {
		at   float64
		want []string
	}{
		{10001.5, []string{"background", "line 4000", "sign 4000"}},
		{10027.5, []string{"background", "sign 4000", "line 4011"}},
		{24997.5, []string{"background", "line 9999"}},
		{24999.9, []string{"background"}},
		{25000, []string{}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, textsAt(t, s, s.CuesAt(sec(tt.at))), tt.at)
	}

	got, err := s.CuesBetween(sec(9999), sec(10002.5))
	require.NoError(t, err)
	assert.Equal(t, []string{"background", "line 3999", "line 4000", "sign 4000"}, textsAt(t, s, got))

	sign := s.CuesWhere(func(c model.Cue) bool { return c.Text() == "sign 4000" })
	require.Len(t, sign, 1)
	c, _ := s.Cue(sign[0])
	c.Start = sec(10001.6)
	require.NoError(t, s.SetCue(sign[0], c))

	assert.Equal(t, []string{"background", "line 4000"}, textsAt(t, s, s.CuesAt(sec(10001.5))))
	assert.Equal(t, []string{"background", "line 4000", "sign 4000"}, textsAt(t, s, s.CuesAt(sec(10001.6))))
}

func scanAt(cues []model.Cue, t time.Duration) []int {
	var out []int
	for i, c := range cues {
		if c.Start <= t && t < c.End {
			out = append(out, i)
		}
	}
	return out
}

func scanBetween(cues []model.Cue, from, to time.Duration) []int {
	var out []int
	for i, c := range cues {
		if c.Start < to && c.End > from {
			out = append(out, i)
		}
	}
	return out
}

func TestLookupMatchesALinearScan(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(81, 20000)) //nolint:gosec // reproducible test data needs a seeded generator
	randomTime := func(maxMillis int) time.Duration {
		return time.Duration(rng.IntN(maxMillis+2000)-1000) * time.Millisecond
	}
	cues := make([]model.Cue, 20000)
	for i := range cues {
		start := randomTime(4_000_000)
		cues[i] = model.Cue{Start: start, End: start + time.Duration(rng.IntN(60_000)-1000)*time.Millisecond}
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	cues = s.Cues()

	for range 500 {
		at := randomTime(4_000_000)
		assert.Equal(t, scanAt(cues, at), s.CuesAt(at), at)

		from, to := randomTime(4_000_000), randomTime(4_000_000)
		from, to = min(from, to), max(from, to)
		got, err := s.CuesBetween(from, to)
		require.NoError(t, err)
		assert.Equal(t, scanBetween(cues, from, to), got, "%v to %v", from, to)
	}
}

func TestLookupAfterEachEditMatchesALinearScan(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(934, 200)) //nolint:gosec // reproducible test data needs a seeded generator
	tenths := func(n int) time.Duration { return time.Duration(rng.IntN(n)) * 100 * time.Millisecond }
	s := &model.Subtitle{}
	for range 200 {
		a, b := tenths(4001), tenths(4001)
		s.AddCue(model.Cue{Start: min(a, b), End: max(a, b) + 500*time.Millisecond, Lines: []string{"cue"}})
	}

	for edit := range 400 {
		i := rng.IntN(200)
		c, ok := s.Cue(i)
		require.True(t, ok)
		if edit%3 == 0 {
			c.Start = max(0, c.Start+tenths(101)-5*time.Second)
		}
		c.End = c.Start + tenths(300) + 100*time.Millisecond
		require.NoError(t, s.SetCue(i, c))

		at := tenths(4001)
		require.Equal(t, scanAt(s.Cues(), at), s.CuesAt(at), "edit %d at %v", edit, at)
	}
}

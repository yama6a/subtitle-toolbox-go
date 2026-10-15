package model_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestFixOverlaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		times  [][2]float64
		minGap time.Duration
		want   [][2]float64
	}{
		{"2 frames at 24 fps", [][2]float64{{1, 5.2}, {5.1, 7}}, time.Second * 2 / 24, [][2]float64{{1, 5.017}, {5.1, 7}}},
		{"without gap", [][2]float64{{1, 5.2}, {5.1, 7}, {7, 8}}, 0, [][2]float64{{1, 5.1}, {5.1, 7}, {7, 8}}},
		{"widens a gap that is too small", [][2]float64{{1, 5.05}, {5.1, 7}}, 100 * time.Millisecond, [][2]float64{{1, 5}, {5.1, 7}}},
		{
			"never moves a start",
			[][2]float64{{5, 6}, {5, 7}, {5.05, 8}},
			100 * time.Millisecond,
			[][2]float64{{5, 5}, {5, 5}, {5.05, 8}},
		},
		{
			"same start ends before the next later start",
			[][2]float64{{1, 4}, {1, 3}, {2, 5}, {6, 7}, {6, 8}},
			0,
			[][2]float64{{1, 2}, {1, 2}, {2, 5}, {6, 7}, {6, 8}},
		},
		{"gap larger than every time", [][2]float64{{1, 2}, {3, 4}}, math.MaxInt64, [][2]float64{{1, 1}, {3, 4}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.times...)
			require.NoError(t, s.FixOverlaps(tt.minGap))
			assert.Equal(t, secs(tt.want...), times(s))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestExtendShortCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		times               [][2]float64
		minDuration, minGap time.Duration
		want                [][2]float64
	}{
		{"extends", [][2]float64{{1, 1.3}, {3.3, 5}}, 833 * time.Millisecond, 0, [][2]float64{{1, 1.833}, {3.3, 5}}},
		{
			"stops before the next cue",
			[][2]float64{{1, 1.3}, {1.5, 1.6}, {1.6, 3}},
			time.Second, 100 * time.Millisecond,
			[][2]float64{{1, 1.4}, {1.5, 1.6}, {1.6, 3}},
		},
		{
			"never shortens a cue",
			[][2]float64{{1, 1.3}, {1.2, 1.4}, {2, 5}},
			time.Second, 100 * time.Millisecond,
			[][2]float64{{1, 1.3}, {1.2, 1.9}, {2, 5}},
		},
		{
			"skips cues with the same start",
			[][2]float64{{1, 1.2}, {1, 1.3}, {5, 6}},
			time.Second, 0,
			[][2]float64{{1, 1.2}, {1, 1.3}, {5, 6}},
		},
		{"extends the last cue freely", [][2]float64{{1, 1.2}}, 2 * time.Second, 500 * time.Millisecond, [][2]float64{{1, 3}}},
		{"rounds the result", [][2]float64{{1, 1.2}}, 1500400 * time.Microsecond, 0, [][2]float64{{1, 2.5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.times...)
			require.NoError(t, s.ExtendShortCues(tt.minDuration, tt.minGap))
			assert.Equal(t, secs(tt.want...), times(s))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestAddLeadInOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		times                   [][2]float64
		leadIn, leadOut, minGap float64
		want                    [][2]float64
	}{
		{"free space", [][2]float64{{1, 2}, {5, 6}}, 0.2, 0.3, 0, [][2]float64{{0.8, 2.3}, {4.8, 6.3}}},
		{"small gap", [][2]float64{{1, 2}, {2.1, 3}}, 0.2, 0.3, 0.04, [][2]float64{{0.8, 2.06}, {2.1, 3.3}}},
		{"never below 0", [][2]float64{{0.1, 1}}, 0.2, 0, 0, [][2]float64{{0, 1}}},
		{"overlap kept", [][2]float64{{1, 3}, {2, 4}}, 0.2, 0.2, 0, [][2]float64{{0.8, 3}, {2, 4.2}}},
		{"lead-out first", [][2]float64{{1, 2}, {2.2, 3}}, 0.3, 0.3, 0, [][2]float64{{0.7, 2.2}, {2.2, 3.3}}},
		{"gap below minimum", [][2]float64{{1, 2}, {2.02, 3}}, 0.2, 0.2, 0.04, [][2]float64{{0.8, 2}, {2.02, 3.2}}},
		{"same start", [][2]float64{{1, 2}, {1, 3}, {5, 6}}, 0.2, 0.2, 0, [][2]float64{{1, 2}, {1, 3.2}, {4.8, 6.2}}},
		{"inside another cue", [][2]float64{{1, 5}, {2, 3}}, 0.2, 0.2, 0, [][2]float64{{0.8, 5.2}, {2, 3}}},
		{"zero leads", [][2]float64{{1, 2}, {5, 6}}, 0, 0, 0.5, [][2]float64{{1, 2}, {5, 6}}},
		{"zero-length cue", [][2]float64{{1, 1}, {3, 4}}, 0.2, 0.2, 0, [][2]float64{{0.8, 1.2}, {2.8, 4.2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.times...)
			require.NoError(t, s.AddLeadInOut(sec(tt.leadIn), sec(tt.leadOut), sec(tt.minGap)))
			assert.Equal(t, secs(tt.want...), times(s))
		})
	}
}

func TestTimingFixesOnAnEmptySubtitle(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	require.NoError(t, s.FixOverlaps(0))
	require.NoError(t, s.ExtendShortCues(time.Second, 0))
	require.NoError(t, s.AddLeadInOut(time.Second, time.Second, 0))
	require.NoError(t, s.LimitLongCues(time.Second))
	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))
	assert.Equal(t, 0, s.Len())
}

// Adapted from testAddLeadInOutCreatesNoOverlapAndNoNegativeStart in tests/FixesTest.php of subtitle-toolbox-php.
func TestAddLeadInOutCreatesNoOverlapAndNoNegativeStart(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(461, 0)) //nolint:gosec // a fixed seed makes the rounds repeatable
	ms := func(n int) time.Duration { return time.Duration(rng.IntN(n+1)) * time.Millisecond }
	for round := range 200 {
		cues := make([]model.Cue, 12)
		for i := range cues {
			start := ms(6000)
			cues[i] = model.Cue{Start: start, End: start + ms(1500), Lines: []string{fmt.Sprintf("text%d", i)}}
		}
		s := &model.Subtitle{}
		s.AddCues(cues...)
		before := byText(s)
		minGap := ms(100)

		require.NoError(t, s.AddLeadInOut(ms(500), ms(500), minGap))

		after := byText(s)
		require.Len(t, after, len(before))
		for text, c := range after {
			old := before[text]
			assert.GreaterOrEqual(t, c.Start, time.Duration(0), "round %d, %s", round, text)
			assert.LessOrEqual(t, c.Start, old.Start, "round %d, %s", round, text)
			assert.GreaterOrEqual(t, c.End, old.End, "round %d, %s", round, text)
			for otherText, other := range after {
				if otherText != text && old.End <= before[otherText].Start {
					assert.LessOrEqual(t, c.End, other.Start, "round %d, %s before %s", round, text, otherText)
				}
			}
		}
	}
}

func byText(s *model.Subtitle) map[string]model.Cue {
	out := map[string]model.Cue{}
	for _, c := range s.All() {
		out[c.Text()] = c
	}
	return out
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestLimitLongCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		times       [][2]float64
		maxDuration time.Duration
		want        [][2]float64
	}{
		{"too long", [][2]float64{{1, 26}}, 7 * time.Second, [][2]float64{{1, 8}}},
		{"short", [][2]float64{{1, 3}}, 7 * time.Second, [][2]float64{{1, 3}}},
		{"exactly the limit", [][2]float64{{1, 8}}, 7 * time.Second, [][2]float64{{1, 8}}},
		{"below 1 ms", [][2]float64{{1.001, 9}}, 7000400 * time.Microsecond, [][2]float64{{1.001, 8.001}}},
		{"overlapping cues", [][2]float64{{1, 10}, {2, 4}}, 2500 * time.Millisecond, [][2]float64{{1, 3.5}, {2, 4}}},
		{"never lengthens", [][2]float64{{1, 2}}, math.MaxInt64, [][2]float64{{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.times...)
			require.NoError(t, s.LimitLongCues(tt.maxDuration))
			assert.Equal(t, secs(tt.want...), times(s))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestTimingFixArgumentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(s *model.Subtitle) error
		want string
	}{
		{
			"overlaps with a negative gap", func(s *model.Subtitle) error { return s.FixOverlaps(-time.Second) },
			"The minimum gap must not be negative, got -1.",
		},
		{
			"extend with a negative gap", func(s *model.Subtitle) error { return s.ExtendShortCues(time.Second, -100*time.Millisecond) },
			"The minimum gap must not be negative, got -0.1.",
		},
		{
			"extend with a zero duration", func(s *model.Subtitle) error { return s.ExtendShortCues(0, 0) },
			"The minimum duration must be greater than 0, got 0.",
		},
		{
			"extend with a negative duration", func(s *model.Subtitle) error { return s.ExtendShortCues(-time.Second, -time.Second) },
			"The minimum duration must be greater than 0, got -1.",
		},
		{
			"negative lead-in", func(s *model.Subtitle) error { return s.AddLeadInOut(-100*time.Millisecond, 0, 0) },
			"The lead-in must not be negative, got -0.1.",
		},
		{
			"negative lead-out", func(s *model.Subtitle) error { return s.AddLeadInOut(0, -time.Second, 0) },
			"The lead-out must not be negative, got -1.",
		},
		{
			"lead with a negative gap", func(s *model.Subtitle) error {
				return s.AddLeadInOut(200*time.Millisecond, 200*time.Millisecond, -time.Second)
			},
			"The minimum gap must not be negative, got -1.",
		},
		{
			"zero maximum duration", func(s *model.Subtitle) error { return s.LimitLongCues(0) },
			"The maximum duration must be greater than 0, got 0.",
		},
		{
			"negative maximum duration", func(s *model.Subtitle) error { return s.LimitLongCues(-7 * time.Second) },
			"The maximum duration must be greater than 0, got -7.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes([2]float64{1, 5.2}, [2]float64{5.1, 5.3})
			err := tt.edit(s)

			require.ErrorIs(t, err, model.ErrInvalidArgument)
			require.EqualError(t, err, "stbox: invalid argument: "+tt.want)
			assert.Equal(t, secs([2]float64{1, 5.2}, [2]float64{5.1, 5.3}), times(s))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestTimingFixFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want string
		edit     func(s *model.Subtitle) error
	}{
		{
			name: "overlaps and short cues", in: "fixes/own_overlaps_and_short_cues.srt", want: "fixes/own_overlaps_and_short_cues_fixed.srt",
			edit: func(s *model.Subtitle) error {
				return errors.Join(s.FixOverlaps(83*time.Millisecond), s.ExtendShortCues(833*time.Millisecond, 83*time.Millisecond), s.WrapLines(42, 2))
			},
		},
		{
			name: "lead-in and lead-out of speech-to-text timing", in: "fixes/own_asr_tight_timing.srt", want: "fixes/own_asr_tight_timing_lead.srt",
			edit: func(s *model.Subtitle) error {
				return s.AddLeadInOut(200*time.Millisecond, 300*time.Millisecond, 83*time.Millisecond)
			},
		},
		{
			name: "long cues", in: "fixes/own_long_cues.srt", want: "fixes/own_long_cues_limited.srt",
			edit: func(s *model.Subtitle) error { return s.LimitLongCues(7 * time.Second) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readSubRip(t, tt.in)
			require.NoError(t, tt.edit(s))
			want := readSubRip(t, tt.want)
			assert.Equal(t, timesAndLines(want), timesAndLines(s))
			assert.Equal(t, want.Comments(), s.Comments())
		})
	}
}

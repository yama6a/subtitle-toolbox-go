package refsync_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/markup"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/refsync"
)

const frameRateScale = 25 / 23.976

func load(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	s, _, err := stbox.Load(testfiles.Path(tb, "sync/"+name), stbox.ReadOptions{})
	require.NoError(tb, err)
	return s
}

// randomSubtitle builds n cues with random gaps of 0.5 to 4 s and random lengths of 1 to 5 s from 1 s on, as the
// PHP fixture does. The same seed gives the same cues.
func randomSubtitle(n int, seed uint64) *model.Subtitle {
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // test data, not a secret
	t := time.Second
	cues := make([]model.Cue, n)
	for i := range cues {
		t += time.Duration(500+rng.IntN(3501)) * time.Millisecond
		length := time.Duration(1000+rng.IntN(4001)) * time.Millisecond
		cues[i] = model.Cue{Start: t, End: t + length, Lines: []string{"text" + strconv.Itoa(i)}}
		t += length
	}
	var s model.Subtitle
	s.AddCues(cues...)
	return &s
}

type cueTimes struct {
	Start, End time.Duration
	Lines      []string
}

func timesAndLines(s *model.Subtitle) []cueTimes {
	var out []cueTimes
	for _, c := range s.All() {
		out = append(out, cueTimes{Start: c.Start, End: c.End, Lines: c.Lines})
	}
	return out
}

func times(s *model.Subtitle) [][2]time.Duration {
	var out [][2]time.Duration
	for _, c := range s.All() {
		out = append(out, [2]time.Duration{c.Start, c.End})
	}
	return out
}

// matchedStarts counts the cues of s that start less than 1 s from a cue of reference, and checks that each of them
// starts at most maxDistance from it.
func matchedStarts(t *testing.T, s, reference *model.Subtitle, maxDistance time.Duration) int {
	t.Helper()

	matched := 0
	for _, c := range s.All() {
		distance := time.Duration(1<<63 - 1)
		for _, r := range reference.All() {
			distance = min(distance, (c.Start - r.Start).Abs())
		}
		if distance < time.Second {
			assert.LessOrEqual(t, distance, maxDistance, "cue at %s", c.Start)
			matched++
		}
	}
	return matched
}

func durationPtr(d time.Duration) *time.Duration { return &d }

// Adapted from ReferenceSyncTest::testRealFilesParse and ReferenceSyncSplitTest::testRealFileWithAdBreakParses.
func TestFixturesParse(t *testing.T) {
	t.Parallel()

	type cue struct {
		index      int
		start, end time.Duration
		text       string
	}
	ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
	tests := []struct {
		file  string
		count int
		cues  []cue
	}{
		{
			file: "own_reference_en.srt", count: 60,
			cues: []cue{
				{0, ms(77689), ms(79205), "The bakery opens at six every morning."},
				{59, ms(800651), ms(804214), "Good morning, the usual please."},
			},
		},
		{
			file: "own_target_de_25fps.srt", count: 59,
			cues: []cue{{0, ms(76738), ms(78148), ""}, {58, ms(770055), ms(773450), "Guten Morgen, wie immer bitte."}},
		},
		{
			file: "own_reference_en_tv_break.srt", count: 60,
			cues: []cue{
				{0, ms(77689), ms(79205), "The bakery opens at six every morning."},
				{27, ms(348029), ms(350893), ""},
				{28, ms(579717), ms(581104), ""},
				{59, ms(950651), ms(954214), "Good morning, the usual please."},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := load(t, tt.file)
			require.Equal(t, tt.count, s.Len())
			for _, want := range tt.cues {
				c, ok := s.Cue(want.index)
				require.True(t, ok)
				assert.Equal(t, [2]time.Duration{want.start, want.end}, [2]time.Duration{c.Start, c.End})
				if want.text != "" {
					assert.Equal(t, want.text, c.Text())
				}
			}
		})
	}
}

// Adapted from ReferenceSyncTest and ReferenceSyncSplitTest of subtitle-toolbox-php.
func TestApplyRealFiles(t *testing.T) {
	t.Parallel()

	split := func(penalty *float64) refsync.Options {
		return refsync.Options{
			MinOffset: durationPtr(-180 * time.Second), MaxOffset: durationPtr(180 * time.Second), MaxSplits: 2,
			SplitPenalty: penalty,
		}
	}
	tests := []struct {
		name      string
		reference string
		opts      refsync.Options
		want      string
		segments  int
		check     func(t *testing.T, r refsync.Report)
	}{
		{
			name:      "offset and frame rate scale with missing and extra cues",
			reference: "own_reference_en.srt",
			want:      "own_target_de_synced.srt",
			segments:  1,
			check: func(t *testing.T, r refsync.Report) {
				t.Helper()
				assert.InDelta(t, -2.3, r.Offset.Seconds(), 0.02)
				assert.InDelta(t, frameRateScale, r.Scale, 0.00001)
				assert.Greater(t, r.Score, 0.8)
			},
		},
		{
			name:      "an ad break gives a split",
			reference: "own_reference_en_tv_break.srt",
			opts:      split(nil),
			want:      "own_target_de_split_synced.srt",
			segments:  2,
			check: func(t *testing.T, r refsync.Report) {
				t.Helper()
				splitAt := 414320 * time.Millisecond
				assert.Equal(t, time.Duration(0), r.Segments[0].From)
				assert.Equal(t, &splitAt, r.Segments[0].To)
				assert.Equal(t, splitAt, r.Segments[1].From)
				assert.Nil(t, r.Segments[1].To)
				assert.InDelta(t, -2.3, r.Segments[0].Offset.Seconds(), 0.02)
				assert.InDelta(t, 147.7, r.Segments[1].Offset.Seconds(), 0.02)
				assert.InDelta(t, frameRateScale, r.Segments[1].Scale, 0.00001)
				assert.Equal(t, r.Segments[0].Offset, r.Offset)
				assert.Greater(t, r.Score, 0.8)
			},
		},
		{
			name:      "a penalty above the gain of the split keeps 1 segment",
			reference: "own_reference_en_tv_break.srt",
			opts:      split(new(5.0)),
			want:      "own_target_de_split_penalty_synced.srt",
			segments:  1,
		},
		{
			name:      "a high penalty keeps 1 segment",
			reference: "own_reference_en_tv_break.srt",
			opts:      split(new(0.5)),
			segments:  1,
		},
		{
			name:      "without splits the ad break file does not sync",
			reference: "own_reference_en_tv_break.srt",
			opts:      refsync.Options{MinOffset: durationPtr(-180 * time.Second), MaxOffset: durationPtr(180 * time.Second)},
			segments:  1,
			check: func(t *testing.T, r refsync.Report) {
				t.Helper()
				assert.Less(t, r.Score, 0.5)
			},
		},
		{
			name:      "without the scale search the files do not match",
			reference: "own_reference_en.srt",
			opts:      refsync.Options{NoScale: true},
			segments:  1,
			check: func(t *testing.T, r refsync.Report) {
				t.Helper()
				assert.Equal(t, 1.0, r.Scale) //nolint:testifylint // the scale is exactly 1
				assert.Less(t, r.Score, 0.5)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reference := load(t, tt.reference)
			target := load(t, "own_target_de_25fps.srt")
			before := times(target)
			tt.opts.Reference = reference

			r, err := refsync.Apply(target, tt.opts)
			require.NoError(t, err)
			require.Len(t, r.Segments, tt.segments)
			assert.NotEqual(t, before, times(target))
			if tt.check != nil {
				tt.check(t, r)
			}
			if tt.want == "" {
				return
			}
			assert.Equal(t, timesAndLines(load(t, tt.want)), timesAndLines(target))
			if tt.segments == 2 || tt.want == "own_target_de_synced.srt" {
				assert.Equal(t, 57, matchedStarts(t, target, reference, 50100*time.Microsecond))
			}
		})
	}
}

// Adapted from ReferenceSyncSplitTest::testFileWithoutSplitKeepsTheResultOfNoSplitSearch.
func TestApplyWithoutASplitIgnoresMaxSplits(t *testing.T) {
	t.Parallel()

	reference := load(t, "own_reference_en.srt")
	plain, withSplits := load(t, "own_target_de_25fps.srt"), load(t, "own_target_de_25fps.srt")

	want, err := refsync.Apply(plain, refsync.Options{Reference: reference})
	require.NoError(t, err)
	got, err := refsync.Apply(withSplits, refsync.Options{Reference: reference, MaxSplits: 2})
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, timesAndLines(plain), timesAndLines(withSplits))
}

// Adapted from ReferenceSyncTest and ReferenceSyncSplitTest of subtitle-toolbox-php, with Go random cues.
func TestApplyRandomFiles(t *testing.T) {
	t.Parallel()

	t.Run("unrelated files score below 0.5", func(t *testing.T) {
		t.Parallel()

		for _, splits := range []int{0, 2} {
			r, err := refsync.Apply(randomSubtitle(300, 1), refsync.Options{Reference: randomSubtitle(300, 2), MaxSplits: splits})
			require.NoError(t, err)
			assert.Less(t, r.Score, 0.5, "max splits %d", splits)
		}
	})

	t.Run("the offset range limits the search", func(t *testing.T) {
		t.Parallel()

		reference := randomSubtitle(200, 3)
		target := randomSubtitle(200, 3)
		require.NoError(t, target.Shift(75*time.Second))

		r, err := refsync.Apply(target.Clone(), refsync.Options{Reference: reference})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, r.Offset, -60*time.Second)

		r, err = refsync.Apply(target, refsync.Options{
			Reference: reference, MinOffset: durationPtr(-90 * time.Second), MaxOffset: durationPtr(-60 * time.Second),
		})
		require.NoError(t, err)
		assert.Equal(t, -75*time.Second, r.Offset)
		assert.Equal(t, 1.0, r.Scale) //nolint:testifylint // the scale is exactly 1
		assert.InDelta(t, 1, r.Score, 0.000001)
		assert.Equal(t, times(reference), times(target))
	})

	t.Run("an identical copy keeps its times", func(t *testing.T) {
		t.Parallel()

		s := randomSubtitle(20, 5)
		before := times(s)
		r, err := refsync.Apply(s, refsync.Options{Reference: s.Clone()})
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), r.Offset)
		assert.Equal(t, 1.0, r.Scale) //nolint:testifylint // the scale is exactly 1
		assert.Equal(t, before, times(s))
	})
}

func TestApplyWithoutSpans(t *testing.T) {
	t.Parallel()

	var zeroLength model.Subtitle
	zeroLength.AddCue(model.Cue{Start: 5 * time.Second, End: 5 * time.Second, Lines: []string{"flash"}})
	tests := []struct {
		name              string
		target, reference *model.Subtitle
	}{
		{"an empty target", &model.Subtitle{}, randomSubtitle(10, 4)},
		{"an empty reference", randomSubtitle(10, 4), &model.Subtitle{}},
		{"cues of length 0", &zeroLength, randomSubtitle(10, 4)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := tt.target.Clone()
			before := times(target)
			r, err := refsync.Apply(target, refsync.Options{Reference: tt.reference, MaxSplits: 2})
			require.NoError(t, err)
			assert.Equal(t, refsync.Report{Scale: 1, Segments: []refsync.Segment{{Scale: 1}}}, r)
			assert.Equal(t, before, times(target))
		})
	}
}

// Adapted from ReferenceSyncTest::testTwoThousandCuesFinishUnderFiveSeconds and
// ReferenceSyncSplitTest::testTwoThousandCuesWithTwoSplitsFinishUnderFifteenSeconds, without the time limits.
func TestApplyTwoThousandCues(t *testing.T) {
	t.Parallel()

	target := func(t *testing.T) *model.Subtitle {
		t.Helper()

		s := randomSubtitle(2000, 6)
		require.NoError(t, s.Scale(23.976/25))
		require.NoError(t, s.Shift(12400*time.Millisecond))
		return s
	}
	offset := -12.4 * frameRateScale

	t.Run("without splits", func(t *testing.T) {
		t.Parallel()

		r, err := refsync.Apply(target(t), refsync.Options{Reference: randomSubtitle(2000, 6)})
		require.NoError(t, err)
		assert.InDelta(t, frameRateScale, r.Scale, 0.00001)
		assert.InDelta(t, offset, r.Offset.Seconds(), 0.02)
	})

	t.Run("with 2 splits", func(t *testing.T) {
		t.Parallel()

		reference := randomSubtitle(2000, 6)
		at := func(s *model.Subtitle, i int) *time.Duration {
			c, ok := s.Cue(i)
			require.True(t, ok)
			return &c.Start
		}
		require.NoError(t, reference.ShiftRange(30*time.Second, at(reference, 700), nil))
		require.NoError(t, reference.ShiftRange(-20*time.Second, at(reference, 1400), nil))
		s := target(t)
		splits := []time.Duration{0, *at(s, 700), *at(s, 1400)}

		r, err := refsync.Apply(s, refsync.Options{Reference: reference, MaxSplits: 2})
		require.NoError(t, err)
		froms := make([]time.Duration, 0, len(r.Segments))
		for _, seg := range r.Segments {
			froms = append(froms, seg.From)
		}
		assert.Equal(t, splits, froms)
		require.Len(t, r.Segments, 3)
		assert.InDelta(t, frameRateScale, r.Scale, 0.00001)
		for i, shift := range []float64{0, 30, 10} {
			assert.InDelta(t, offset+shift, r.Segments[i].Offset.Seconds(), 0.02, "segment %d", i)
		}
		assert.InDelta(t, 1, r.Score, 0.001)
	})
}

// Adapted from WordTimestampRetimingTest::testReferenceSyncWithSplitsMovesWordTimestampsWithTheirPart.
func TestApplyMovesWordTimestampsWithTheirSegment(t *testing.T) {
	t.Parallel()

	target := load(t, "own_target_de_25fps.srt")
	last := target.Len() - 1
	for _, i := range []int{0, last} {
		c, ok := target.Cue(i)
		require.True(t, ok)
		ts := c.Start + 500*time.Millisecond
		c.Lines[0] = fmt.Sprintf("<%02d:%02d:%02d.%03d>", int(ts.Hours()), int(ts.Minutes())%60, int(ts.Seconds())%60,
			ts.Milliseconds()%1000) + c.Lines[0]
		require.NoError(t, target.SetCue(i, c))
	}

	r, err := refsync.Apply(target, refsync.Options{
		Reference: load(t, "own_reference_en_tv_break.srt"), MinOffset: durationPtr(-180 * time.Second),
		MaxOffset: durationPtr(180 * time.Second), MaxSplits: 2,
	})
	require.NoError(t, err)
	require.Len(t, r.Segments, 2)
	for _, i := range []int{0, last} {
		c, ok := target.Cue(i)
		require.True(t, ok)
		var stamps []time.Duration
		markup.MapWordTimestamps(c.Lines[0], func(d time.Duration) time.Duration {
			stamps = append(stamps, d)
			return d
		})
		require.Len(t, stamps, 1)
		ts := stamps[0]
		assert.InDelta(t, (c.Start + time.Duration(0.5*r.Scale*float64(time.Second))).Seconds(), ts.Seconds(), 0.0015)
	}
}

func TestApplyTimeLimit(t *testing.T) {
	t.Parallel()

	var target, reference model.Subtitle
	target.AddCue(model.Cue{Start: 359999990 * time.Second, End: 359999999 * time.Second, Lines: []string{"late"}})
	reference.AddCue(model.Cue{Start: 360000020 * time.Second, End: 360000029 * time.Second, Lines: []string{"later"}})
	before := times(&target)

	_, err := refsync.Apply(&target, refsync.Options{Reference: &reference, NoScale: true})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Equal(t, before, times(&target))
}

func TestValidate(t *testing.T) {
	t.Parallel()

	ref := &model.Subtitle{}
	tests := []struct {
		name   string
		opts   refsync.Options
		msg    string
		option string
	}{
		{name: "defaults", opts: refsync.Options{Reference: ref}},
		{
			name: "offsets and penalty 0 at the limits",
			opts: refsync.Options{
				Reference: ref, MinOffset: durationPtr(0), MaxOffset: durationPtr(refsync.MaxOffsetRange),
				MaxSplits: refsync.MaxSplits, SplitPenalty: new(0.0),
			},
		},
		{name: "no reference", opts: refsync.Options{}, msg: "The reference subtitle is missing.", option: "Reference"},
		{
			name:   "minimum offset below the limit",
			opts:   refsync.Options{Reference: ref, MinOffset: durationPtr(-86401 * time.Second)},
			msg:    "The minimum offset must be from -86400 to 86400 seconds, got -86401.",
			option: "MinOffset",
		},
		{
			name:   "maximum offset above the limit",
			opts:   refsync.Options{Reference: ref, MaxOffset: durationPtr(86400500 * time.Millisecond)},
			msg:    "The maximum offset must be from -86400 to 86400 seconds, got 86400.5.",
			option: "MaxOffset",
		},
		{
			name:   "minimum above maximum",
			opts:   refsync.Options{Reference: ref, MinOffset: durationPtr(10 * time.Second), MaxOffset: durationPtr(-10 * time.Second)},
			msg:    "The minimum offset must not be greater than the maximum offset, got 10 and -10.",
			option: "MinOffset",
		},
		{
			name:   "range too wide",
			opts:   refsync.Options{Reference: ref, MinOffset: durationPtr(-3600 * time.Second), MaxOffset: durationPtr(3600001 * time.Millisecond)},
			msg:    "The minimum and the maximum offset must be at most 7200 seconds apart, got -3600 and 3600.001.",
			option: "MaxOffset",
		},
		{
			name:   "negative splits",
			opts:   refsync.Options{Reference: ref, MaxSplits: -1},
			msg:    "The maximum number of splits must be from 0 to 10, got -1.",
			option: "MaxSplits",
		},
		{
			name:   "too many splits",
			opts:   refsync.Options{Reference: ref, MaxSplits: 11},
			msg:    "The maximum number of splits must be from 0 to 10, got 11.",
			option: "MaxSplits",
		},
		{
			name:   "negative penalty",
			opts:   refsync.Options{Reference: ref, SplitPenalty: new(-0.5)},
			msg:    "The split penalty must be a finite number of 0 or more, got -0.5.",
			option: "SplitPenalty",
		},
		{
			name:   "NaN penalty",
			opts:   refsync.Options{Reference: ref, SplitPenalty: new(nan())},
			msg:    "The split penalty must be a finite number of 0 or more, got NAN.",
			option: "SplitPenalty",
		},
		{
			name:   "infinite penalty",
			opts:   refsync.Options{Reference: ref, SplitPenalty: new(inf())},
			msg:    "The split penalty must be a finite number of 0 or more, got INF.",
			option: "SplitPenalty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			_, applyErr := refsync.Apply(&model.Subtitle{}, tt.opts)
			if tt.msg == "" {
				require.NoError(t, err)
				require.NoError(t, applyErr)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			e, ok := errors.AsType[*model.InvalidArgumentError](err)
			require.True(t, ok)
			assert.Equal(t, tt.msg, e.Msg)
			assert.Equal(t, "refsync.Options."+tt.option, e.Option)
			assert.Equal(t, err, applyErr)
		})
	}
}

func nan() float64 { return math.NaN() }

func inf() float64 { return math.Inf(1) }

func BenchmarkApply(b *testing.B) {
	reference := randomSubtitle(2000, 6)
	target := randomSubtitle(2000, 6)
	require.NoError(b, target.Scale(23.976/25))
	require.NoError(b, target.Shift(12400*time.Millisecond))
	for _, splits := range []int{0, 2} {
		b.Run("cues=2000/splits="+strconv.Itoa(splits), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := refsync.Apply(target.Clone(), refsync.Options{Reference: reference, MaxSplits: splits}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

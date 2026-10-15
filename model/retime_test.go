package model_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/markup"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

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

func secs(pairs ...[2]float64) [][2]time.Duration {
	out := make([][2]time.Duration, len(pairs))
	for i, p := range pairs {
		out[i] = [2]time.Duration{sec(p[0]), sec(p[1])}
	}
	return out
}

// readFixture reads the cue blocks and NOTE comments of a SubRip or WebVTT file in testdata.
// It reads only what the retiming fixtures hold, because package model cannot use the format readers.
func readFixture(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(tb, err)
	text := strings.ReplaceAll(strings.TrimPrefix(string(b), "\ufeff"), "\r\n", "\n")

	var cues []model.Cue
	var comments []model.Comment
	for block := range strings.SplitSeq(strings.TrimSpace(text), "\n\n") {
		lines := strings.Split(block, "\n")
		switch {
		case strings.HasPrefix(lines[0], "WEBVTT"):
		case strings.HasPrefix(lines[0], "NOTE "):
			comments = append(comments, model.Comment{Text: strings.TrimPrefix(block, "NOTE "), BeforeCue: len(cues)})
		default:
			if !strings.Contains(lines[0], "-->") {
				lines = lines[1:]
			}
			fields := strings.Fields(lines[0])
			require.GreaterOrEqual(tb, len(fields), 3, block)
			cues = append(cues, model.Cue{Start: fixtureTime(tb, fields[0]), End: fixtureTime(tb, fields[2]), Lines: lines[1:]})
		}
	}

	s := &model.Subtitle{}
	s.AddCues(cues...)
	for _, c := range comments {
		require.NoError(tb, s.AddComment(c.Text, c.BeforeCue))
	}
	return s
}

func fixtureTime(tb testing.TB, field string) time.Duration {
	tb.Helper()

	d, ok := markup.WordTimestamp("<" + strings.Replace(field, ",", ".", 1) + ">")
	require.True(tb, ok, field)
	return d
}

func TestRetimeFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want string
		retime   func(*model.Subtitle) error
	}{
		{
			name: "shift without a range moves every cue and word timestamp",
			in:   "editing/own_ferry_drift.vtt", want: "editing/own_ferry_drift_shifted_all.vtt",
			retime: func(s *model.Subtitle) error { return s.Shift(-1500 * time.Millisecond) },
		},
		{
			name: "scale moves every cue and word timestamp",
			in:   "editing/own_ferry_drift.vtt", want: "editing/own_ferry_drift_scaled.vtt",
			retime: func(s *model.Subtitle) error { return s.Scale(1.001) },
		},
		{
			name: "shift with a range moves the cues that start in it",
			in:   "editing/own_ferry_drift.vtt", want: "editing/own_ferry_drift_shifted.vtt",
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(2*time.Second, new(6*time.Second), new(600*time.Second))
			},
		},
		{
			name: "shift with a range sorts a moved cue before the earlier cues",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_shifted_back.vtt",
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(-12500*time.Millisecond, new(17500*time.Millisecond), nil)
			},
		},
		{
			name: "sync by points maps each segment linearly and moves word timestamps",
			in:   "editing/own_ferry_drift.vtt", want: "editing/own_ferry_drift_synced.vtt",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{
					{Old: sec(10), New: sec(12)}, {Old: sec(600), New: sec(610)}, {Old: sec(1200), New: sec(1205)},
				})
			},
		},
		{
			name: "convert the frame rate of a real file",
			in:   "sync/own_target_de_25fps.srt", want: "sync/own_target_de_23976fps.srt",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(25, 23.976) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFixture(t, tt.in)
			require.NoError(t, tt.retime(s))
			want := readFixture(t, tt.want)
			assert.Equal(t, timesAndLines(want), timesAndLines(s))
			assert.Equal(t, want.Comments(), s.Comments())
		})
	}
}

func TestRetimeTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     [][2]float64
		retime func(*model.Subtitle) error
		want   [][2]float64
	}{
		{
			name:   "shift moves all cues",
			in:     [][2]float64{{10, 12.5}, {20.25, 22}},
			retime: func(s *model.Subtitle) error { return s.Shift(-2500 * time.Millisecond) },
			want:   [][2]float64{{7.5, 10}, {17.75, 19.5}},
		},
		{
			name: "shift range with only a start moves the cues from that time",
			in:   [][2]float64{{10, 12.5}, {20, 22}, {30, 31}},
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(1250*time.Millisecond, new(20*time.Second), nil)
			},
			want: [][2]float64{{10, 12.5}, {21.25, 23.25}, {31.25, 32.25}},
		},
		{
			name:   "shift range with only an end moves the cues that start before it",
			in:     [][2]float64{{10, 12.5}, {20, 22}, {30, 31}},
			retime: func(s *model.Subtitle) error { return s.ShiftRange(-time.Second, nil, new(20*time.Second)) },
			want:   [][2]float64{{9, 11.5}, {20, 22}, {30, 31}},
		},
		{
			name:   "shift range moves a cue across the end as a whole",
			in:     [][2]float64{{10, 25}, {20, 22}, {30, 31}},
			retime: func(s *model.Subtitle) error { return s.ShiftRange(time.Second, nil, new(20*time.Second)) },
			want:   [][2]float64{{11, 26}, {20, 22}, {30, 31}},
		},
		{
			name:   "shift clamps negative times to 0 and keeps the cues",
			in:     [][2]float64{{0.5, 1.5}, {1, 3}, {5, 6}},
			retime: func(s *model.Subtitle) error { return s.Shift(-2 * time.Second) },
			want:   [][2]float64{{0, 0}, {0, 1}, {3, 4}},
		},
		{
			name:   "shift rounds the sum half away from zero",
			in:     [][2]float64{{1, 2}},
			retime: func(s *model.Subtitle) error { return s.Shift(-500 * time.Microsecond) },
			want:   [][2]float64{{1, 2}},
		},
		{
			name:   "shift by half a millisecond forward rounds up",
			in:     [][2]float64{{1, 2}},
			retime: func(s *model.Subtitle) error { return s.Shift(500 * time.Microsecond) },
			want:   [][2]float64{{1.001, 2.001}},
		},
		{
			name:   "scale",
			in:     [][2]float64{{1.5, 2.25}},
			retime: func(s *model.Subtitle) error { return s.Scale(2) },
			want:   [][2]float64{{3, 4.5}},
		},
		{
			name:   "scale by 1.001",
			in:     [][2]float64{{1, 2}},
			retime: func(s *model.Subtitle) error { return s.Scale(1.001) },
			want:   [][2]float64{{1.001, 2.002}},
		},
		{
			name:   "scale rounds an exact half millisecond away from zero",
			in:     [][2]float64{{0.009, 0.011}},
			retime: func(s *model.Subtitle) error { return s.Scale(1.5) },
			want:   [][2]float64{{0.014, 0.017}},
		},
		{
			name:   "scale rounds a binary result below a decimal half millisecond down",
			in:     [][2]float64{{0.5, 1}},
			retime: func(s *model.Subtitle) error { return s.Scale(1.001) },
			want:   [][2]float64{{0.5, 1.001}},
		},
		{
			name:   "convert the frame rate from PAL to film",
			in:     [][2]float64{{100, 102.5}},
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(25, 23.976) },
			want:   [][2]float64{{104.271, 106.878}},
		},
		{
			name: "sync by 2 points",
			in:   [][2]float64{{10, 12}, {3130, 3132}, {6260, 6262}},
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(12)}, {Old: sec(6260), New: sec(6005)}})
			},
			want: [][2]float64{{12, 13.918}, {3003.706, 3005.623}, {6005, 6006.918}},
		},
		{
			name: "sync by 1 point shifts all cues",
			in:   [][2]float64{{10, 12.5}, {20.25, 22}},
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(12)}})
			},
			want: [][2]float64{{12, 14.5}, {22.25, 24}},
		},
		{
			name: "sync by points clamps negative times to 0",
			in:   [][2]float64{{1, 2}, {10, 11}},
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(5)}, {Old: sec(20), New: sec(15)}})
			},
			want: [][2]float64{{0, 0}, {5, 6}},
		},
		{
			name: "sync by 3 points maps a time between the first 2 points through the first segment",
			in:   [][2]float64{{300, 300}},
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{
					{Old: sec(10), New: sec(12)}, {Old: sec(600), New: sec(610)}, {Old: sec(1200), New: sec(1205)},
				})
			},
			want: [][2]float64{{305.932, 305.932}},
		},
		{
			name: "sync by points maps a cue across a point through 2 segments",
			in:   [][2]float64{{590, 610}, {1300, 1310}},
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{
					{Old: sec(10), New: sec(12)}, {Old: sec(600), New: sec(610)}, {Old: sec(1200), New: sec(1205)},
				})
			},
			want: [][2]float64{{599.864, 619.917}, {1304.167, 1314.083}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.in...)
			require.NoError(t, tt.retime(s))
			assert.Equal(t, secs(tt.want...), times(s))
			assert.Equal(t, "text0", texts(s)[0])
		})
	}
}

func TestShiftRangeReSortsAndMovesTheComments(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{10, 12}, [2]float64{20, 22}, [2]float64{30, 32})
	require.NoError(t, s.AddComment("before 1", 1))
	require.NoError(t, s.AddComment("before 2", 2))
	require.NoError(t, s.AddComment("at the end", 3))

	require.NoError(t, s.ShiftRange(-25*time.Second, new(30*time.Second), nil))

	assert.Equal(t, []string{"text2", "text0", "text1"}, texts(s))
	assert.Equal(t, secs([2]float64{5, 7}, [2]float64{10, 12}, [2]float64{20, 22}), times(s))
	assert.Equal(t, []model.Comment{
		{Text: "before 2", BeforeCue: 0},
		{Text: "before 1", BeforeCue: 2},
		{Text: "at the end", BeforeCue: 3},
	}, s.Comments())
}

func TestRetimeKeepsEqualStartsInOrder(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2}, [2]float64{3, 4}, [2]float64{5, 6})
	require.NoError(t, s.Shift(-10*time.Second))

	assert.Equal(t, []string{"text0", "text1", "text2"}, texts(s))
}

func TestRetimeUpdatesTheLookups(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2}, [2]float64{3, 4})
	require.Equal(t, []int{0}, s.CuesAt(sec(1.5)))

	require.NoError(t, s.Shift(10*time.Second))

	assert.Empty(t, s.CuesAt(sec(1.5)))
	assert.Equal(t, []int{1}, s.CuesAt(sec(13.5)))
}

func TestRetimeWordTimestamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		retime func(*model.Subtitle) error
		cue    int
		want   []string
	}{
		{
			name:   "shift moves the word timestamps of the first cue",
			retime: func(s *model.Subtitle) error { return s.Shift(10 * time.Second) },
			cue:    0,
			want:   []string{"<00:00:11.000>Turn <00:00:11.400>the <00:00:11.600>damn <00:00:12.000>radio <00:00:12.500>off."},
		},
		{
			name:   "shift moves the word timestamps of each line",
			retime: func(s *model.Subtitle) error { return s.Shift(10 * time.Second) },
			cue:    2,
			want:   []string{"<00:00:15.500>It <00:00:15.700>sounds <00:00:16.100>like", "<00:00:16.300>hell <00:00:16.700>to <00:00:16.800>me."},
		},
		{
			name:   "shift range keeps the word timestamps of earlier cues",
			retime: func(s *model.Subtitle) error { return s.ShiftRange(-time.Second, new(5*time.Second), nil) },
			cue:    0,
			want:   []string{"<00:00:01.000>Turn <00:00:01.400>the <00:00:01.600>damn <00:00:02.000>radio <00:00:02.500>off."},
		},
		{
			name:   "shift range moves the word timestamps of later cues",
			retime: func(s *model.Subtitle) error { return s.ShiftRange(-time.Second, new(5*time.Second), nil) },
			cue:    3,
			want:   []string{"<00:00:06.500>Fine, <00:00:07.000>damnit."},
		},
		{
			name:   "shift clamps word timestamps at 0",
			retime: func(s *model.Subtitle) error { return s.Shift(-1500 * time.Millisecond) },
			cue:    0,
			want:   []string{"<00:00:00.000>Turn <00:00:00.000>the <00:00:00.100>damn <00:00:00.500>radio <00:00:01.000>off."},
		},
		{
			name: "sync by points moves word timestamps",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(1), New: sec(2)}, {Old: sec(7.5), New: sec(15)}})
			},
			cue:  0,
			want: []string{"<00:00:02.000>Turn <00:00:02.800>the <00:00:03.200>damn <00:00:04.000>radio <00:00:05.000>off."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFixture(t, "profanity/radio.vtt")
			require.NoError(t, tt.retime(s))
			c, ok := s.Cue(tt.cue)
			require.True(t, ok)
			assert.Equal(t, tt.want, c.Lines)
		})
	}
}

func TestRetimeSaturatesAWordTimestampBeyondTheDurationRange(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: sec(1), End: sec(2), Lines: []string{"<2000000:00:00.000>far"}})

	require.NoError(t, s.Scale(10))

	c, _ := s.Cue(0)
	assert.Equal(t, []string{"<2562047:47:16.854>far"}, c.Lines)
}

func TestRetimeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		retime func(*model.Subtitle) error
		want   string
	}{
		{
			name: "shift range with an end equal to the start",
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(2*time.Second, new(600*time.Second), new(600*time.Second))
			},
			want: "The end time of the shift must be after its start time, got 600 to 600.",
		},
		{
			name: "shift range with an end before the start",
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(2*time.Second, new(900*time.Second), new(600*time.Second))
			},
			want: "The end time of the shift must be after its start time, got 900 to 600.",
		},
		{
			name:   "shift range with fractions",
			retime: func(s *model.Subtitle) error { return s.ShiftRange(time.Second, new(sec(2.5)), new(sec(1.5))) },
			want:   "The end time of the shift must be after its start time, got 2.5 to 1.5.",
		},
		{
			name:   "scale by 0",
			retime: func(s *model.Subtitle) error { return s.Scale(0) },
			want:   "The scale factor must be greater than 0, got 0.",
		},
		{
			name:   "scale by a negative factor",
			retime: func(s *model.Subtitle) error { return s.Scale(-1.5) },
			want:   "The scale factor must be greater than 0, got -1.5.",
		},
		{
			name:   "scale by NaN",
			retime: func(s *model.Subtitle) error { return s.Scale(math.NaN()) },
			want:   "The scale factor must be greater than 0, got NAN.",
		},
		{
			name:   "scale by infinity",
			retime: func(s *model.Subtitle) error { return s.Scale(math.Inf(1)) },
			want:   "The scale factor must be greater than 0, got INF.",
		},
		{
			name:   "frame rate 0",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(25, 0) },
			want:   "The frame rate must be greater than 0, got 0.",
		},
		{
			name:   "frame rate NaN",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(math.NaN(), 25) },
			want:   "The frame rate must be greater than 0, got NAN.",
		},
		{
			name:   "frame rate minus infinity",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(25, math.Inf(-1)) },
			want:   "The frame rate must be greater than 0, got -INF.",
		},
		{
			name:   "frame rates whose ratio is infinite",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(1e300, 1e-300) },
			want:   "The scale factor must be greater than 0, got INF.",
		},
		{
			name:   "no sync points",
			retime: func(s *model.Subtitle) error { return s.SyncByPoints(nil) },
			want:   "The sync points must not be empty.",
		},
		{
			name:   "an empty list of sync points",
			retime: func(s *model.Subtitle) error { return s.SyncByPoints([]model.SyncPoint{}) },
			want:   "The sync points must not be empty.",
		},
		{
			name: "same old times",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(12)}, {Old: sec(10), New: sec(15)}})
			},
			want: "The old and new times of the sync points must both increase, got 10=12 before 10=15.",
		},
		{
			name: "old times reverse",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(600), New: sec(610)}, {Old: sec(10), New: sec(12)}})
			},
			want: "The old and new times of the sync points must both increase, got 600=610 before 10=12.",
		},
		{
			name: "new times reverse",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(20)}, {Old: sec(30), New: sec(15)}})
			},
			want: "The old and new times of the sync points must both increase, got 10=20 before 30=15.",
		},
		{
			name: "same new times",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(10), New: sec(12)}, {Old: sec(20), New: sec(30)}, {Old: sec(40), New: sec(30)}})
			},
			want: "The old and new times of the sync points must both increase, got 20=30 before 40=30.",
		},
		{
			name:   "scale beyond the time limit",
			retime: func(s *model.Subtitle) error { return s.Scale(1e300) },
			want:   "The retimed cue 1 would end at 8e+300 s. Cue times must be below 100000 hours.",
		},
		{
			name:   "frame rate beyond the time limit",
			retime: func(s *model.Subtitle) error { return s.ConvertFrameRate(1, 1e-300) },
			want:   "The retimed cue 1 would end at 8e+300 s. Cue times must be below 100000 hours.",
		},
		{
			name:   "shift beyond the time limit",
			retime: func(s *model.Subtitle) error { return s.Shift(360000000 * time.Second) },
			want:   "The retimed cue 1 would end at 360000008 s. Cue times must be below 100000 hours.",
		},
		{
			name: "shift from a time beyond the time limit",
			retime: func(s *model.Subtitle) error {
				return s.ShiftRange(359998500*time.Second, new(1000*time.Second), nil)
			},
			want: "The retimed cue 5 would end at 360000003 s. Cue times must be below 100000 hours.",
		},
		{
			name:   "shift by the largest duration",
			retime: func(s *model.Subtitle) error { return s.Shift(math.MaxInt64) },
			want:   "The retimed cue 1 would end at 9223372044.8548 s. Cue times must be below 100000 hours.",
		},
		{
			name: "sync by 2 points beyond the time limit",
			retime: func(s *model.Subtitle) error {
				return s.SyncByPoints([]model.SyncPoint{{Old: sec(5), New: sec(5)}, {Old: sec(10), New: sec(2000000)}})
			},
			want: "The retimed cue 4 would end at 478798808 s. Cue times must be below 100000 hours.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFixture(t, "editing/own_ferry_drift.vtt")
			before := s.Clone()

			err := tt.retime(s)

			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var argErr *model.InvalidArgumentError
			require.ErrorAs(t, err, &argErr)
			assert.Equal(t, tt.want, argErr.Msg)
			assert.Equal(t, before.Cues(), s.Cues())
			assert.Equal(t, before.Comments(), s.Comments())
		})
	}
}

func TestRetimeAtTheTimeLimit(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2})
	require.NoError(t, s.Shift(100000*time.Hour-2*time.Second-time.Millisecond))
	assert.Equal(t, [][2]time.Duration{{100000*time.Hour - time.Second - time.Millisecond, 100000*time.Hour - time.Millisecond}}, times(s))

	require.ErrorIs(t, s.Shift(time.Millisecond), model.ErrInvalidArgument)
}

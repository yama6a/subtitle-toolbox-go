package timing_test

import (
	"bytes"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
	"github.com/yama6a/subtitle-toolbox-go/v2/timing"
)

func ptr(n int) *int { return &n }

func rate(tb testing.TB, fps float64) model.FrameRate {
	tb.Helper()
	r, err := model.FrameRateFromFloat(fps)
	require.NoError(tb, err)
	return r
}

var fourRates = []float64{23.976, 24, 25, 29.97}

type frames [][2]int64

func subtitleOf(r model.FrameRate, f frames) *model.Subtitle {
	s := &model.Subtitle{}
	for _, c := range f {
		s.AddCue(model.Cue{Start: r.Duration(c[0]).Round(time.Millisecond), End: r.Duration(c[1]).Round(time.Millisecond), Lines: []string{"x"}})
	}
	return s
}

func framesOf(r model.FrameRate, s *model.Subtitle) frames {
	f := make(frames, 0, s.Len())
	for _, c := range s.Cues() {
		f = append(f, [2]int64{r.Frames(c.Start), r.Frames(c.End)})
	}
	return f
}

func shotTimes(r model.FrameRate, shots []int64) []time.Duration {
	out := make([]time.Duration, 0, len(shots))
	for _, n := range shots {
		out = append(out, r.Duration(n).Round(time.Millisecond))
	}
	return out
}

func TestApplyExamples(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		before frames
		shots  []int64
		opts   timing.Options
		after  frames
		// every is true when the row holds at 23.976, 24, 25 and 29.97 fps.
		every bool
	}{
		{"in-time after a shot change", frames{{1505, 1560}}, []int64{1500}, timing.Options{}, frames{{1500, 1560}}, true},
		{"out-time before a shot change", frames{{1620, 1674}, {1700, 1760}}, []int64{1680}, timing.Options{}, frames{{1620, 1678}, {1700, 1760}}, true},
		{"chain closes 7 frames to 2", frames{{200, 240}, {247, 300}}, nil, timing.Options{}, frames{{200, 245}, {247, 300}}, true},
		{"keep gaps", frames{{200, 240}, {247, 300}}, nil, timing.Options{KeepGaps: true}, frames{{200, 240}, {247, 300}}, true},
		{
			"chain up to window minus 1",
			frames{{100, 140}, {142, 180}, {183, 220}, {231, 300}, {312, 400}},
			nil,
			timing.Options{SnapWindowFrames: ptr(12)},
			frames{{100, 140}, {142, 181}, {183, 229}, {231, 300}, {312, 400}},
			false,
		},
		{"outside the window", frames{{513, 560}, {600, 627}}, []int64{500, 640}, timing.Options{}, frames{{513, 560}, {600, 627}}, false},
		{"last shot before the start", frames{{110, 160}}, []int64{104, 100, 112}, timing.Options{}, frames{{104, 160}}, false},
		{
			"too short after the move",
			frames{{100, 119}, {200, 215}},
			[]int64{120, 198},
			timing.Options{},
			frames{{100, 119}, {198, 215}},
			false,
		},
		{"min duration option", frames{{100, 119}}, []int64{120}, timing.Options{MinDurationFrames: ptr(10)}, frames{{100, 118}}, false},
		{
			"too close to a neighbour",
			frames{{100, 150}, {153, 229}, {230, 260}},
			[]int64{158, 228},
			timing.Options{KeepGaps: true},
			frames{{100, 150}, {153, 229}, {230, 260}},
			false,
		},
		{"chain stops at a shot", frames{{140, 152}, {158, 200}}, []int64{153}, timing.Options{}, frames{{140, 152}, {158, 200}}, false},
		{
			"min gap option",
			frames{{200, 240}, {247, 300}, {400, 450}},
			[]int64{455},
			timing.Options{MinGapFrames: ptr(3)},
			frames{{200, 244}, {247, 300}, {400, 452}},
			false,
		},
		{"window 0 needs the same frame", frames{{101, 150}, {200, 250}}, []int64{100, 200}, timing.Options{SnapWindowFrames: ptr(0)}, frames{{101, 150}, {200, 250}}, false},
	}
	for _, tt := range tests {
		fps := []float64{24}
		if tt.every {
			fps = fourRates
		}
		for _, f := range fps {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				r := rate(t, f)
				s := subtitleOf(r, tt.before)
				tt.opts.FrameRate = r
				tt.opts.ShotChanges = shotTimes(r, tt.shots)

				_, err := timing.Apply(s, tt.opts)

				require.NoError(t, err)
				assert.Equal(t, tt.after, framesOf(r, s))
			})
		}
	}
}

func TestApplyWindowAt2997Fps(t *testing.T) {
	t.Parallel()
	r := rate(t, 29.97)
	s := subtitleOf(r, frames{{515, 560}, {1016, 1060}})

	_, err := timing.Apply(s, timing.Options{FrameRate: r, ShotChanges: shotTimes(r, []int64{500, 1000})})

	require.NoError(t, err)
	assert.Equal(t, frames{{500, 560}, {1016, 1060}}, framesOf(r, s))
}

func TestApplyTimes(t *testing.T) {
	t.Parallel()
	sec := func(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
	tests := []struct {
		name  string
		rate  model.FrameRate
		cues  [][2]time.Duration
		shots []time.Duration
		want  [][2]time.Duration
	}{
		{"netflix in-time", rate(t, 24), [][2]time.Duration{{sec(62.708), sec(65)}}, []time.Duration{sec(62.5)}, [][2]time.Duration{{sec(62.5), sec(65)}}},
		{"netflix out-time", rate(t, 24), [][2]time.Duration{{sec(67), sec(69.75)}}, []time.Duration{sec(70)}, [][2]time.Duration{{sec(67), sec(69.917)}}},
		{
			"netflix gap", rate(t, 24),
			[][2]time.Duration{{sec(8), sec(10)}, {sec(10.292), sec(12)}},
			nil,
			[][2]time.Duration{{sec(8), sec(10.208)}, {sec(10.292), sec(12)}},
		},
		{"half frame at 25 fps rounds up", rate(t, 25), [][2]time.Duration{{sec(0.58), sec(2)}}, nil, [][2]time.Duration{{sec(0.6), sec(2)}}},
		{
			"time limit", rate(t, 25),
			[][2]time.Duration{{359999999900 * time.Millisecond, 359999999990 * time.Millisecond}},
			nil,
			[][2]time.Duration{{359999999920 * time.Millisecond, 359999999999 * time.Millisecond}},
		},
		{"half frame at 2997/125 rounds up", model.FrameRate{Num: 2997, Den: 125}, [][2]time.Duration{{sec(62.5), sec(70)}}, nil, [][2]time.Duration{{sec(62.521), sec(69.987)}}},
		{"half frame at 2997/100 rounds up", model.FrameRate{Num: 2997, Den: 100}, [][2]time.Duration{{sec(50), sec(60)}}, nil, [][2]time.Duration{{sec(50.017), sec(59.993)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &model.Subtitle{}
			for _, c := range tt.cues {
				s.AddCue(model.Cue{Start: c[0], End: c[1], Lines: []string{"x"}})
			}

			_, err := timing.Apply(s, timing.Options{FrameRate: tt.rate, ShotChanges: tt.shots})

			require.NoError(t, err)
			got := make([][2]time.Duration, 0, s.Len())
			for _, c := range s.Cues() {
				got = append(got, [2]time.Duration{c.Start, c.End})
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultSnapWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fps  float64
		want int
	}{{23.976, 12}, {24, 12}, {25, 12}, {29.97, 15}, {30, 15}, {50, 25}, {59.94, 30}, {60, 30}, {0.5, 0}}
	for _, tt := range tests {
		t.Run(rate(t, tt.fps).String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, timing.DefaultSnapWindow(rate(t, tt.fps)))
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	r := rate(t, 24)
	tests := []struct {
		name   string
		opts   timing.Options
		option string
		msg    string
	}{
		{"zero rate", timing.Options{}, "timing.Options.FrameRate", "The frame rate of the video must be greater than 0."},
		{
			"drop-frame at 24",
			timing.Options{FrameRate: model.FrameRate{Num: 24, Den: 1, DropFrame: true}},
			"timing.Options.FrameRate",
			"The frame rate must be 30000/1001, 2997/100, 60000/1001 or 2997/50 for drop-frame time codes, got 24.",
		},
		{"window", timing.Options{FrameRate: r, SnapWindowFrames: ptr(-1)}, "timing.Options.SnapWindowFrames", "The snap window must not be negative, got -1."},
		{"gap", timing.Options{FrameRate: r, MinGapFrames: ptr(-2)}, "timing.Options.MinGapFrames", "The minimum gap must not be negative, got -2."},
		{"duration", timing.Options{FrameRate: r, MinDurationFrames: ptr(-3)}, "timing.Options.MinDurationFrames", "The minimum duration must not be negative, got -3."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := subtitleOf(r, frames{{100, 150}})

			report, err := timing.Apply(s, tt.opts)

			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, tt.option, aerr.Option)
			assert.Equal(t, tt.msg, aerr.Msg)
			assert.Equal(t, timing.Report{}, report)
			assert.Equal(t, frames{{100, 150}}, framesOf(r, s))
			assert.EqualError(t, tt.opts.Validate(), err.Error())
		})
	}
}

func TestApplyEmptyAndComments(t *testing.T) {
	t.Parallel()
	r := rate(t, 24)

	report, err := timing.Apply(&model.Subtitle{}, timing.Options{FrameRate: r})
	require.NoError(t, err)
	assert.Equal(t, timing.Report{}, report)

	s := subtitleOf(r, frames{{100, 150}, {1000, 1050}})
	require.NoError(t, s.AddComment("note", 1))
	_, err = timing.Apply(s, timing.Options{FrameRate: r, ShotChanges: shotTimes(r, []int64{1004})})
	require.NoError(t, err)
	assert.Equal(t, []model.Comment{{Text: "note", BeforeCue: 1}}, s.Comments())
}

func readSubRip(tb testing.TB, rel string) *model.Subtitle {
	tb.Helper()
	s, _, err := stbox.Parse(bytes.NewReader(testfiles.Read(tb, rel)), stbox.ReadOptions{Format: stbox.SubRip})
	require.NoError(tb, err)
	return s
}

func TestApplyRealFile(t *testing.T) {
	t.Parallel()
	for _, shots := range []string{"own_ffmpeg_showinfo.log", "own_scenes.txt"} {
		t.Run(shots, func(t *testing.T) {
			t.Parallel()
			content := testfiles.Read(t, "shot-changes/"+shots)
			times, err := timing.ShotChangesFromFFmpegLog(content)
			if shots == "own_scenes.txt" {
				times, err = timing.ShotChangesFromText(content)
			}
			require.NoError(t, err)
			s := readSubRip(t, "shot-changes/own_garden_24fps.srt")

			report, err := timing.Apply(s, timing.Options{FrameRate: rate(t, 24), ShotChanges: times})

			require.NoError(t, err)
			assert.Equal(t, timing.Report{MovedStarts: 3, MovedEnds: 5}, report)
			assert.Equal(t, readSubRip(t, "shot-changes/own_garden_24fps_timed.srt").Cues(), s.Cues())
		})
	}
}

func TestApplyAllTimesOnFrames(t *testing.T) {
	t.Parallel()
	for _, f := range fourRates {
		t.Run(rate(t, f).String(), func(t *testing.T) {
			t.Parallel()
			r := rate(t, f)
			rng := rand.New(rand.NewPCG(7, 7)) //nolint:gosec // a fixed seed makes the test repeat
			ms := func(lo, hi int) time.Duration { return time.Duration(lo+rng.IntN(hi-lo+1)) * time.Millisecond }
			s := &model.Subtitle{}
			shots := make([]time.Duration, 0, 200)
			now := time.Second
			for range 200 {
				start := now + ms(0, 900)
				now = start + ms(500, 4000)
				shots = append(shots, start+ms(-600, 600))
				s.AddCue(model.Cue{Start: start, End: now, Lines: []string{"x"}})
			}

			_, err := timing.Apply(s, timing.Options{FrameRate: r, ShotChanges: shots})

			require.NoError(t, err)
			var previousEnd time.Duration
			for _, c := range s.Cues() {
				for _, d := range []time.Duration{c.Start, c.End} {
					assert.Equal(t, r.Duration(r.Frames(d)).Round(time.Millisecond), d)
				}
				assert.Greater(t, c.End, c.Start)
				assert.GreaterOrEqual(t, c.Start, previousEnd)
				previousEnd = c.End
			}
		})
	}
}

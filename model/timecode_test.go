package model_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func seconds(f float64) time.Duration { return time.Duration(math.Round(f * 1e9)) }

func TestNamedFrameRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rate   model.FrameRate
		want   model.FrameRate
		labels int
	}{
		{"23976", model.FrameRate23976, model.FrameRate{Num: 24000, Den: 1001}, 24},
		{"24", model.FrameRate24, model.FrameRate{Num: 24, Den: 1}, 24},
		{"25", model.FrameRate25, model.FrameRate{Num: 25, Den: 1}, 25},
		{"2997", model.FrameRate2997, model.FrameRate{Num: 30000, Den: 1001}, 30},
		{"2997DF", model.FrameRate2997DF, model.FrameRate{Num: 30000, Den: 1001, DropFrame: true}, 30},
		{"30", model.FrameRate30, model.FrameRate{Num: 30, Den: 1}, 30},
		{"50", model.FrameRate50, model.FrameRate{Num: 50, Den: 1}, 50},
		{"5994", model.FrameRate5994, model.FrameRate{Num: 60000, Den: 1001}, 60},
		{"5994DF", model.FrameRate5994DF, model.FrameRate{Num: 60000, Den: 1001, DropFrame: true}, 60},
		{"60", model.FrameRate60, model.FrameRate{Num: 60, Den: 1}, 60},
		{"2997/125", model.FrameRate{Num: 2997, Den: 125}, model.FrameRate{Num: 2997, Den: 125}, 24},
		{"25/2", model.FrameRate{Num: 25, Den: 2}, model.FrameRate{Num: 25, Den: 2}, 13},
		{"zero", model.FrameRate{}, model.FrameRate{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.rate)
			assert.Equal(t, tt.labels, tt.rate.Labels())
		})
	}
}

func TestDropFrameModeString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "keep", model.DropFrameKeep.String())
	assert.Equal(t, "on", model.DropFrameOn.String())
	assert.Equal(t, "off", model.DropFrameOff.String())
	assert.Equal(t, "DropFrameMode(7)", model.DropFrameMode(7).String())
}

func TestFrameRateNear(t *testing.T) {
	t.Parallel()

	assert.True(t, model.FrameRate{Num: 2997, Den: 125}.Near(model.FrameRate23976))
	assert.True(t, model.FrameRate{Num: 5993, Den: 200}.Near(model.FrameRate2997))
	assert.False(t, model.FrameRate{Num: 2998, Den: 100}.Near(model.FrameRate30))
	assert.True(t, model.FrameRate2997DF.Near(model.FrameRate2997))
}

func TestFrameConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rate   model.FrameRate
		d      time.Duration
		frames int64
	}{
		{"20 ms at 25 is the half frame 0.5", model.FrameRate25, ms(20), 1},
		{"-20 ms", model.FrameRate25, ms(-20), -1},
		{"0.1 s at 25", model.FrameRate25, ms(100), 3},
		{"250 frames at 25 is 10 s", model.FrameRate25, 10 * time.Second, 250},
		{"1 s at 30000/1001", model.FrameRate2997, time.Second, 30},
		{"0.4 ms", model.FrameRate25, 400 * time.Microsecond, 0},
		{"30000/1001 from a float, a Den of 1e14", model.FrameRate{Num: 2997002997002997, Den: 100000000000000}, time.Second, 30},
		{"99999 h at 60000/1001", model.FrameRate5994, 99999 * time.Hour, 21578205794},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.frames, tt.rate.Frames(tt.d))
		})
	}

	assert.Equal(t, seconds(1.001), model.FrameRate2997.Duration(30))
	assert.Equal(t, 500500*time.Microsecond, model.FrameRate2997.Duration(15))
	assert.Equal(t, ms(41708), model.FrameRate{Num: 2997, Den: 125}.Duration(1000).Round(time.Millisecond))
	assert.Equal(t, 10*time.Second, model.FrameRate25.Duration(250))

	const frames = 21578205794
	back := model.FrameRate5994.Duration(model.FrameRate5994.Frames(99999 * time.Hour))
	assert.InDelta(t, float64(99999*time.Hour), float64(back), float64(model.FrameRate5994.Duration(1)))
	assert.Equal(t, int64(frames), model.FrameRate5994.Frames(model.FrameRate5994.Duration(frames)))
}

func TestFrameRateZeroValue(t *testing.T) {
	t.Parallel()

	var r model.FrameRate
	assert.Zero(t, r.Frames(time.Hour))
	assert.Zero(t, r.Duration(100))
	assert.Equal(t, model.Timecode{}, r.TimecodeOf(100))
	assert.Equal(t, int64(5), r.FrameOf(model.Timecode{Frames: 5}))
	assert.Equal(t, model.Timecode{}, r.ClockTimecode(time.Hour))
	assert.Equal(t, time.Minute, r.ClockDuration(model.Timecode{Minutes: 1, Frames: 7}))
}

// TestFramesMatchTheFloatFormula compares Frames with round(ms / 1000 * fps) of PHP for every millisecond of the first
// 2 minutes and every 997th millisecond up to 1 hour. It skips the times where the float product lies within 1e-6 of
// a half frame. There the float error decides the rounding, while Frames always rounds the exact half up.
func TestFramesMatchTheFloatFormula(t *testing.T) {
	t.Parallel()

	rates := []model.FrameRate{
		model.FrameRate23976, model.FrameRate24, model.FrameRate25, model.FrameRate2997,
		model.FrameRate30, model.FrameRate50, model.FrameRate5994, model.FrameRate60,
		{Num: 2997, Den: 125},
	}
	for _, r := range rates {
		t.Run(r.String(), func(t *testing.T) {
			t.Parallel()

			var diffs []string
			check := func(n int64) {
				x := float64(n) / 1000 * r.Float64()
				if math.Abs(x-math.Floor(x)-0.5) < 1e-6 {
					return
				}
				if want, got := int64(math.Round(x)), r.Frames(ms(n)); want != got {
					diffs = append(diffs, fmt.Sprintf("%d ms: float %d, exact %d", n, want, got))
				}
			}
			for n := range int64(120_001) {
				check(n)
			}
			for n := int64(120_000); n <= 3_600_000; n += 997 {
				check(n)
			}
			assert.Empty(t, diffs)
		})
	}
}

func TestTimecodeAndFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rate  model.FrameRate
		frame int64
		label string
	}{
		{model.FrameRate2997DF, 0, "00:00:00;00"},
		{model.FrameRate2997DF, 1799, "00:00:59;29"},
		{model.FrameRate2997DF, 1800, "00:01:00;02"},
		{model.FrameRate2997DF, 17981, "00:09:59;29"},
		{model.FrameRate2997DF, 17982, "00:10:00;00"},
		{model.FrameRate2997DF, 19782, "00:11:00;02"},
		{model.FrameRate2997DF, 107892, "01:00:00;00"},
		{model.FrameRate5994DF, 3600, "00:01:00;04"},
		{model.FrameRate{Num: 2997, Den: 100, DropFrame: true}, 1800, "00:01:00;02"},
		{model.FrameRate2997, 1800, "00:01:00:00"},
		{model.FrameRate2997, 108000, "01:00:00:00"},
		{model.FrameRate25, 360_000 * 25, "100:00:00:00"},
		{model.FrameRate23976, 59, "00:00:02:11"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s frame %d", tt.rate, tt.frame), func(t *testing.T) {
			t.Parallel()

			label, err := model.ParseTimecode(tt.label)
			require.NoError(t, err)
			assert.Equal(t, label, tt.rate.TimecodeOf(tt.frame))
			assert.Equal(t, tt.label, tt.rate.TimecodeOf(tt.frame).String())
			assert.Equal(t, tt.frame, tt.rate.FrameOf(label))
		})
	}

	inverse := []struct {
		name  string
		rate  model.FrameRate
		label string
		frame int64
	}{
		{"a skipped label gives the label before it", model.FrameRate2997DF, "00:01:00;00", 1798},
		{"the rate decides, not the semicolon", model.FrameRate2997, "00:01:00;02", 1802},
		{"the colon with a drop-frame rate", model.FrameRate2997DF, "00:01:00:02", 1800},
		{"fields are not range checked", model.FrameRate25, "00:00:99:99", 99*25 + 99},
	}
	for _, tt := range inverse {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			label, err := model.ParseTimecode(tt.label)
			require.NoError(t, err)
			assert.Equal(t, tt.frame, tt.rate.FrameOf(label))
		})
	}

	assert.Equal(t, model.Timecode{DropFrame: true}, model.FrameRate2997DF.TimecodeOf(-5))
}

func TestFramesThenTimecode(t *testing.T) {
	t.Parallel()

	times := []time.Duration{0, 400 * time.Microsecond, 500 * time.Microsecond, ms(1996), seconds(59.9996), seconds(3599.9996), 360_000 * time.Second}
	tests := []struct {
		rate   model.FrameRate
		labels []string
	}{
		{model.FrameRate23976, []string{"00:00:00:00", "00:00:00:00", "00:00:00:00", "00:00:02:00", "00:00:59:23", "00:59:56:10", "99:54:00:09"}},
		{model.FrameRate25, []string{"00:00:00:00", "00:00:00:00", "00:00:00:00", "00:00:02:00", "00:01:00:00", "01:00:00:00", "100:00:00:00"}},
		{model.FrameRate2997DF, []string{"00:00:00;00", "00:00:00;00", "00:00:00;00", "00:00:02;00", "00:00:59;28", "01:00:00;00", "100:00:00;11"}},
		{model.FrameRate30, []string{"00:00:00:00", "00:00:00:00", "00:00:00:00", "00:00:02:00", "00:01:00:00", "01:00:00:00", "100:00:00:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.rate.String(), func(t *testing.T) {
			t.Parallel()

			for i, d := range times {
				assert.Equal(t, tt.labels[i], tt.rate.TimecodeOf(tt.rate.Frames(d)).String(), "time %v", d)
			}
		})
	}
}

func TestClockTimecodeAndDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rate  model.FrameRate
		d     time.Duration
		label string
		back  time.Duration
	}{
		{"whole seconds", model.FrameRate23976, 100 * time.Second, "00:01:40:00", 100 * time.Second},
		{"a frame that rolls into the next second", model.FrameRate23976, ms(1996), "00:00:02:00", 2 * time.Second},
		{"half a second at 23.976", model.FrameRate23976, ms(1500), "00:00:01:12", ms(1500) + 500*time.Microsecond},
		{"59.9996 s", model.FrameRate23976, seconds(59.9996), "00:01:00:00", time.Minute},
		{"25 fps", model.FrameRate25, ms(1480), "00:00:01:12", ms(1480)},
		{"25 fps, 1 hour", model.FrameRate25, seconds(3723.48), "01:02:03:12", seconds(3723.48)},
		{"30000/1001 never drops", model.FrameRate2997, time.Minute, "00:01:00:00", time.Minute},
		{"drop-frame rate, still no drop", model.FrameRate2997DF, time.Minute, "00:01:00:00", time.Minute},
		{"negative", model.FrameRate25, -time.Second, "00:00:00:00", 0},
		{"100 h", model.FrameRate25, 100 * time.Hour, "100:00:00:00", 100 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			label := tt.rate.ClockTimecode(tt.d)
			assert.Equal(t, tt.label, label.String())
			assert.False(t, label.DropFrame)
			assert.Equal(t, tt.back, tt.rate.ClockDuration(label))
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	const (
		shape  = `The timecode "%s" is not h:mm:ss.mmm, m:ss.mmm or h:mm:ss:ff.`
		limit  = `The timecode "%s" is not below 100000 hours.`
		noRate = `The timecode "%s" counts frames and needs a frame rate.`
	)
	tests := []struct {
		in   string
		rate model.FrameRate
		want time.Duration
		err  string
	}{
		{in: "00:01:02,500", want: ms(62500)},
		{in: "0:01:02.5", want: ms(62500)},
		{in: "00:00:00", want: 0},
		{in: "100:00:00", want: 360_000 * time.Second},
		{in: "99999:59:59.999", want: 99999*time.Hour + 59*time.Minute + 59999*time.Millisecond},
		{in: "01:02.5", want: ms(62500)},
		{in: "75:00,25", want: ms(4500250)},
		{in: "00:00:00.0005", want: ms(1)},
		{in: "00:00:00.005", want: ms(5)},
		{in: "00:00:01.0004", want: ms(1000)},
		{in: "00:01:02:12", rate: model.FrameRate25, want: ms(62480)},
		{in: "00:00:01:12", rate: model.FrameRate23976, want: ms(1501)},
		{in: "00:00:01:12", rate: model.FrameRate{Num: 2997002997002997, Den: 100000000000000}, want: ms(1400)},
		{in: "00:00:01:012", rate: model.FrameRate25, want: ms(1480)},
		{in: "0099999:00:00:00", rate: model.FrameRate25, want: 99999 * time.Hour},
		{in: "99999:59:59.9996", err: limit},
		{in: "99999:59:59:24", rate: model.FrameRate25, want: 99999*time.Hour + 59*time.Minute + 59960*time.Millisecond},
		{in: "99999:59:59:9996", rate: model.FrameRate{Num: 10000, Den: 1}, err: limit},
		{in: "62.5", err: shape},
		{in: "", err: shape},
		{in: "-00:00:01", err: shape},
		{in: " 00:00:01", err: shape},
		{in: "00:00:01 ", err: shape},
		{in: "00:00:01;12", err: shape},
		{in: "0:0:01", err: shape},
		{in: "00:60:00", err: shape},
		{in: "00:00:60", err: shape},
		{in: "00:00:02.", err: shape},
		{in: "00:00:02,5,5", err: shape},
		{in: "00:01:02:1", rate: model.FrameRate25, err: shape},
		{in: "00:01:02;12", rate: model.FrameRate2997DF, err: shape},
		{in: "100000:00:00", err: limit},
		{in: "6000000:00", err: limit},
		{in: "00099999999999999999999999999999999999999999:00:00", err: limit},
		{in: "100000:00:00:00", rate: model.FrameRate25, err: limit},
		{in: "00:01:02:12", err: noRate},
		{in: "00:01:02:12", rate: model.FrameRate{Num: 0, Den: 1}, err: noRate},
		{in: "00:01:02:25", rate: model.FrameRate25, err: `The timecode "%s" has a frame number that is not below 25.`},
		{in: "00:01:02:24", rate: model.FrameRate23976, err: `The timecode "%s" has a frame number that is not below 24.`},
		{in: "00:01:02:99999999999999999999999", rate: model.FrameRate25, err: `The timecode "%s" has a frame number that is not below 25.`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := model.ParseTime(tt.in, tt.rate)
			if tt.err != "" {
				require.ErrorIs(t, err, model.ErrInvalidArgument)
				var ia *model.InvalidArgumentError
				require.ErrorAs(t, err, &ia)
				assert.Empty(t, ia.Option)
				assert.Equal(t, fmt.Sprintf(tt.err, tt.in), ia.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTimecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want model.Timecode
		err  bool
	}{
		{in: "01:00:00;00", want: model.Timecode{Hours: 1, DropFrame: true}},
		{in: "00:01:02:12", want: model.Timecode{Minutes: 1, Seconds: 2, Frames: 12}},
		{in: "100:00:00:00", want: model.Timecode{Hours: 100}},
		{in: "99:99:99:99", want: model.Timecode{Hours: 99, Minutes: 99, Seconds: 99, Frames: 99}},
		{in: "1:00:00:00", err: true},
		{in: "00:00:00,00", err: true},
		{in: "00:00:00:0", err: true},
		{in: "00:00:00:00\n", err: true},
		{in: "99999999999999999999:00:00:00", err: true},
		{in: "", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := model.ParseTimecode(tt.in)
			if tt.err {
				var ia *model.InvalidArgumentError
				require.ErrorAs(t, err, &ia)
				assert.Equal(t, `The time code "`+tt.in+`" is not HH:MM:SS:FF or HH:MM:SS;FF.`, ia.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.in, got.String())
		})
	}
}

func FuzzTimecode(f *testing.F) {
	f.Add(int64(1800), "00:01:02,500")
	f.Add(int64(107892), "00:01:02:12")
	f.Add(int64(0), "01:00:00;00")
	f.Add(int64(21578205794), "99999999999999999999:00:00:00")

	rates := []model.FrameRate{
		model.FrameRate23976, model.FrameRate24, model.FrameRate25, model.FrameRate2997, model.FrameRate2997DF,
		model.FrameRate30, model.FrameRate50, model.FrameRate5994, model.FrameRate5994DF, model.FrameRate60,
	}
	f.Fuzz(func(t *testing.T, n int64, s string) {
		for _, r := range rates {
			frame := n % (360_000_000 * int64(r.Labels()))
			if frame < 0 {
				frame = -frame
			}
			if got := r.FrameOf(r.TimecodeOf(frame)); got != frame {
				t.Fatalf("%s: FrameOf(TimecodeOf(%d)) = %d", r, frame, got)
			}
			_, _ = model.ParseTime(s, r)
		}
		_, _ = model.ParseTimecode(s)
	})
}

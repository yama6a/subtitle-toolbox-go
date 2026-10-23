package timecode_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

func TestSeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   time.Duration
		want string
	}{
		{3 * time.Second, "3"},
		{1500 * time.Millisecond, "1.5"},
		{time.Millisecond, "0.001"},
		{0, "0"},
		{-2500 * time.Millisecond, "-2.5"},
		{timecode.MaxTime, "360000000"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.Seconds(tt.in))
		})
	}
}

func TestFromSeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want time.Duration
	}{
		{0, 0},
		{1.0005, 1001 * time.Millisecond},
		{-1.0005, -1001 * time.Millisecond},
		{72 / 23.976, 3003 * time.Millisecond},
		{8999999999.0 / 25, 359999999960 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(timecode.Number(tt.in), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.FromSeconds(tt.in))
		})
	}
}

func TestNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{-1, "-1"},
		{1.001, "1.001"},
		{360000008, "360000008"},
		{0.0001, "0.0001"},
		{1e-5, "1e-05"},
		{8e300, "8e+300"},
		{7.999999999999999e300, "8e+300"},
		{1.0 / 3, "0.33333333333333"},
		{99999999999999, "99999999999999"},
		{1e14, "1e+14"},
		{100.25, "100.25"},
		{math.NaN(), "NAN"},
		{math.Inf(1), "INF"},
		{math.Inf(-1), "-INF"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.Number(tt.in))
		})
	}
}

func TestTimeLimitMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n    int
		end  float64
		want string
	}{
		{1, 360000001.5, "The retimed cue 1 would end at 360000001.5 s. Cue times must be below 100000 hours."},
		{12, 8e300, "The retimed cue 12 would end at 8e+300 s. Cue times must be below 100000 hours."},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.TimeLimitMessage(tt.n, tt.end))
		})
	}
}

// Ports the loose time cases of tests/Parsers/LooseTimestampsTest.php of subtitle-toolbox-php.
func TestLoose(t *testing.T) {
	t.Parallel()

	ms := time.Millisecond
	tests := []struct {
		name          string
		in            string
		separators    string
		hoursOptional bool
		want          time.Duration
		wantOK        bool
	}{
		{name: "1 fraction digit", in: "00:00:01.5", separators: ".,", hoursOptional: true, want: 1500 * ms, wantOK: true},
		{name: "2 fraction digits", in: "00:00:01.25", separators: ".,", hoursOptional: true, want: 1250 * ms, wantOK: true},
		{name: "comma", in: "00:00:01,500", separators: ".,", hoursOptional: true, want: 1500 * ms, wantOK: true},
		{name: "4 fraction digits round down", in: "00:28:41,1000", separators: ",.:", want: 1721100 * ms, wantOK: true},
		{name: "4 fraction digits round half up", in: "00:28:42,0005", separators: ",.:", want: 1722001 * ms, wantOK: true},
		{name: "4 fraction digits round up", in: "00:28:42,0006", separators: ",.:", want: 1722001 * ms, wantOK: true},
		{name: "colon before the fraction", in: "00:00:01:105", separators: ",.:", want: 1105 * ms, wantOK: true},
		{name: "1-digit fields", in: "0:0:1,500", separators: ",.:", want: 1500 * ms, wantOK: true},
		{name: "1-digit seconds", in: "00:00:0,000", separators: ",.:", want: 0, wantOK: true},
		{name: "no fraction", in: "00:00:04", separators: ",.:", want: 4 * time.Second, wantOK: true},
		{name: "4 hour digits", in: "0000:00:01,000", separators: ",.:", want: time.Second, wantOK: true},
		{name: "leading zeros before 5 hour digits", in: "0099999:59:59.999", separators: ".,", want: 99999*time.Hour + 59*time.Minute + 59999*ms, wantOK: true},
		{name: "rounds up to MaxTime", in: "99999:59:59,9995", separators: ",.:", want: timecode.MaxTime, wantOK: true},
		{name: "6 hour digits", in: "100000:00:00.000", separators: ".,"},
		{name: "no hours", in: "07:03,920", separators: ",.:", hoursOptional: true, want: 423920 * ms, wantOK: true},
		{name: "no hours when they are required", in: "07:03,920", separators: ",.:"},
		{name: "hours first with a colon fraction", in: "00:01:50", separators: ",.:", hoursOptional: true, want: 110 * time.Second, wantOK: true},
		{name: "minutes of 60", in: "00:60:02,000", separators: ".,", hoursOptional: true},
		{name: "seconds of 75", in: "00:75.000", separators: ".,", hoursOptional: true},
		{name: "3 minute digits", in: "00:001:02.000", separators: ".,", hoursOptional: true},
		{name: "5 fraction digits", in: "00:00:01.50000", separators: ".,", hoursOptional: true},
		{name: "separator not in the set", in: "00:00:01:500", separators: ".,", hoursOptional: true},
		{name: "separator without fraction", in: "00:00:01.", separators: ".,", hoursOptional: true},
		{name: "letters", in: "00:00:0G,000", separators: ",.:"},
		{name: "empty", in: "", separators: ".,", hoursOptional: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := timecode.Loose(tt.in, tt.separators, tt.hoursOptional)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppendClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        time.Duration
		hourWidth int
		sep       byte
		want      string
	}{
		{"zero, 2 hour digits", 0, 2, ',', "00:00:00,000"},
		{"zero, 1 hour digit", 0, 1, '.', "0:00:00.000"},
		{"all fields", 3*time.Hour + 4*time.Minute + 5*time.Second + 6*time.Millisecond, 2, ',', "03:04:05,006"},
		{"more hour digits than the width", 100*time.Hour + 1500*time.Millisecond, 2, ',', "100:00:01,500"},
		{"negative", -time.Second, 1, '.', "0:00:00.000"},
		{"rounds half away from zero", 1500500 * time.Nanosecond, 1, '.', "0:00:00.002"},
		{"rounds down", 1499999 * time.Nanosecond, 1, '.', "0:00:00.001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "x"+tt.want, string(timecode.AppendClock([]byte("x"), tt.in, tt.hourWidth, tt.sep)))
		})
	}
}

func TestUnits(t *testing.T) {
	t.Parallel()

	const centisecond = 10 * time.Millisecond
	const tenth = 100 * time.Millisecond

	tests := []struct {
		name string
		in   time.Duration
		unit time.Duration
		want int64
	}{
		{"1.240 s in tenths", 1240 * time.Millisecond, tenth, 12},
		{"1.250 s in tenths rounds up", 1250 * time.Millisecond, tenth, 13},
		{"1.249 s in tenths rounds down", 1249 * time.Millisecond, tenth, 12},
		{"1.005 s in centiseconds rounds up", 1005 * time.Millisecond, centisecond, 101},
		{"1.500 s in seconds rounds up", 1500 * time.Millisecond, time.Second, 2},
		{"1.499 s in seconds rounds down", 1499 * time.Millisecond, time.Second, 1},
		{"1 ms unit keeps the milliseconds", 1234 * time.Millisecond, time.Millisecond, 1234},
		{"0", 0, tenth, 0},
		{"below 0", -3 * time.Second, tenth, 0},
		{"99999:59:59.900 in tenths", timecode.MaxTime - 100*time.Millisecond, tenth, 3599999999},
		{"99999:59:59.949 in tenths", timecode.MaxTime - 51*time.Millisecond, tenth, 3599999999},
		{"99999:59:59.950 in tenths clamps", timecode.MaxTime - 50*time.Millisecond, tenth, 3599999999},
		{"99999:59:59.999 in tenths clamps", timecode.MaxValidTime, tenth, 3599999999},
		{"99999:59:59.990 in centiseconds", timecode.MaxTime - 10*time.Millisecond, centisecond, 35999999999},
		{"99999:59:59.995 in centiseconds clamps", timecode.MaxTime - 5*time.Millisecond, centisecond, 35999999999},
		{"99999:59:59.499 in seconds", timecode.MaxTime - 501*time.Millisecond, time.Second, 359999999},
		{"99999:59:59.500 in seconds clamps", timecode.MaxTime - 500*time.Millisecond, time.Second, 359999999},
		{"99999:59:59.999 in seconds clamps", timecode.MaxValidTime, time.Second, 359999999},
		{"99999:59:59.999 in milliseconds", timecode.MaxValidTime, time.Millisecond, 359999999999},
		{"the limit clamps", timecode.MaxTime, tenth, 3599999999},
		{"the largest duration clamps", time.Duration(math.MaxInt64), time.Second, 359999999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.Units(tt.in, tt.unit))
		})
	}
}

func TestFrames(t *testing.T) {
	t.Parallel()

	const limit = timecode.MaxTime
	tests := []struct {
		name string
		in   time.Duration
		fps  float64
		want float64
	}{
		{"1 s at 25 fps", time.Second, 25, 25},
		{"1.020 s at 25 fps rounds up", 1020 * time.Millisecond, 25, 26},
		{"1.019 s at 25 fps rounds down", 1019 * time.Millisecond, 25, 25},
		{"below 0", -time.Second, 25, 0},
		{"99999:59:59.900 at 25 fps", limit - 100*time.Millisecond, 25, 8999999998},
		{"99999:59:59.999 at 25 fps clamps", timecode.MaxValidTime, 25, 8999999999},
		{"99999:59:59.995 at 25 fps clamps", limit - 5*time.Millisecond, 25, 8999999999},
		{"99999:59:59.999 at 23.976 fps clamps", timecode.MaxValidTime, 23.976, 8631359999},
		{"99999:59:59.999 at 1 fps clamps", timecode.MaxValidTime, 1, 359999999},
		{"99999:59:59.999 at 0.5 fps clamps", timecode.MaxValidTime, 0.5, 179999999},
		{"99999:59:59.999 at 0.001 fps clamps", timecode.MaxValidTime, 0.001, 359999},
		{"the limit at 25 fps clamps", limit, 25, 8999999999},
		{"100000:00:01 at 25 fps clamps", limit + time.Second, 25, 8999999999},
		{"the limit at 1000 fps clamps", limit, 1000, 359999999999},
		{"the limit at 1e6 fps clamps", limit, 1e6, 359999999999000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := timecode.Frames(tt.in, tt.fps)
			assert.InDelta(t, tt.want, got, 0)
			assert.Less(t, timecode.FromFrames(got, tt.fps), timecode.MaxTime)
		})
	}
}

func TestFromFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    float64
		fps  float64
		want time.Duration
	}{
		{"25 frames at 25 fps", 25, 25, time.Second},
		{"1 frame at 24 fps rounds to the millisecond", 1, 24, 42 * time.Millisecond},
		{"8999999999 frames at 25 fps", 8999999999, 25, 359999999960 * time.Millisecond},
		{"9000000000 frames at 25 fps is the limit", 9000000000, 25, timecode.MaxTime},
		{"beyond the limit", 1e300, 25, timecode.MaxTime},
		{"rounds up to the limit", 359999999.9996, 1, timecode.MaxTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.FromFrames(tt.n, tt.fps))
		})
	}
}

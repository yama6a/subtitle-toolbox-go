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

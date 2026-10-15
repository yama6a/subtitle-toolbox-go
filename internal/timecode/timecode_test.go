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

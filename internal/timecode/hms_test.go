package timecode_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
)

func TestHMS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		hours, minutes, second string
		want                   time.Duration
	}{
		{"plain", "1", "02", "03", time.Hour + 2*time.Minute + 3*time.Second},
		{"leading zeros", "0000001", "00", "00", time.Hour},
		{"empty hours", "", "01", "02", time.Minute + 2*time.Second},
		{"6 digits", "099999", "59", "59", 99999*time.Hour + 59*time.Minute + 59*time.Second},
		{"7 digits", "1000000", "00", "00", timecode.MaxTime},
		{"7 digits of 9", "9999999", "00", "00", timecode.MaxTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, timecode.HMS(tt.hours, tt.minutes, tt.second))
		})
	}
}

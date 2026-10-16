package timecode_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

func TestClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{"01:02:03.456", time.Hour + 2*time.Minute + 3456*time.Millisecond, true},
		{"00:00:02,500", 2500 * time.Millisecond, true},
		{"0:01:02.5", 62500 * time.Millisecond, true},
		{"01:02.5", 62500 * time.Millisecond, true},
		{"00:00", 0, true},
		{"75:00", 75 * time.Minute, true},
		{"00:00:01.0005", 1001 * time.Millisecond, true},
		{"00:00:01.0004999", time.Second, true},
		{"00:00:00.9996", time.Second, true},
		{"99999:59:59.999", timecode.MaxTime - time.Millisecond, true},
		{"0000000000000099999:00:00", 99999 * time.Hour, true},
		{"100000:00:00", 0, false},
		{"5999999:59.9996", 0, false},
		{"99999999999999999999:00:00", 0, false},
		{"5124096:00:00", 0, false},
		{"307445735:00", 0, false},
		{"9999999:00:00", 0, false},
		{"5999999:59", 359999999 * time.Second, true},
		{"6000000:00", 0, false},
		{"00:00:01:12", 0, false},
		{"1:2:3:4:5", 0, false},
		{"-00:01", 0, false},
		{"00:60", 0, false},
		{"00:5", 0, false},
		{"00:00:05.", 0, false},
		{"00:00:05.5.5", 0, false},
		{"00:00:05.x", 0, false},
		{":00", 0, false},
		{"00", 0, false},
		{"", 0, false},
		{"2.5", 0, false},
		{" 00:01", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := timecode.Clock(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

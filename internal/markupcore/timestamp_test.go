package markupcore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

func TestCoreTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00.000"},
		{62500 * time.Millisecond, "00:01:02.500"},
		{time.Hour + 600*time.Microsecond, "01:00:00.001"},
		{1999600 * time.Microsecond, "00:00:02.000"},
		{500 * time.Microsecond, "00:00:00.001"},
		{100 * time.Hour, "100:00:00.000"},
		{-1500 * time.Millisecond, "00:00:00.000"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.CoreTimestamp(tt.d))
		})
	}
}

func TestWordTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		want   time.Duration
		wantOK bool
	}{
		{"<00:01:02.500>", 62500 * time.Millisecond, true},
		{"<01:02:03.500>", time.Hour + 2*time.Minute + 3500*time.Millisecond, true},
		{"<100:00:00.000>", 100 * time.Hour, true},
		{"<100:59:59.999>", 100*time.Hour + 59*time.Minute + 59999*time.Millisecond, true},
		{"<01:02.500>", 0, false},
		{"<00:60:00.000>", 0, false},
		{"<00:00:01.50>", 0, false},
		{"<0:00:01.500>", 0, false},
		{"<00:00:01.5000>", 0, false},
		{"<i>", 0, false},
		{"x<00:00:01.000>", 0, false},
		{"<00:00:01.000>x", 0, false},
		{"<2562046:59:59.999>", 2562046*time.Hour + 59*time.Minute + 59999*time.Millisecond, true},
		{"<2562047:00:00.000>", 0, false},
		{"<99999999999999999999:00:00.000>", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.WordTimestamp(tt.tag)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

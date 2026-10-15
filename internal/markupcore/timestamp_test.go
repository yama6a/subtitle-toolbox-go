package markupcore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
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

func TestMapWordTimestamps(t *testing.T) {
	t.Parallel()

	plus := func(d time.Duration) func(time.Duration) time.Duration {
		return func(t time.Duration) time.Duration { return t + d }
	}
	tests := []struct {
		name string
		text string
		fn   func(time.Duration) time.Duration
		want string
	}{
		{"docs example", "<00:00:01.000>Hi", plus(2 * time.Second), "<00:00:03.000>Hi"},
		{"below 0 becomes 0", "<<00:00:01.000>a", plus(-5 * time.Second), "<<00:00:00.000>a"},
		{"rounds half away from zero", "<00:00:01.000>a", plus(500 * time.Microsecond), "<00:00:01.001>a"},
		{"each timestamp", "<b>One</b> <00:00:01.500>two <100:59:59.999>", plus(time.Millisecond), "<b>One</b> <00:00:01.501>two <101:00:00.000>"},
		{"not word timestamps", "<01:02.500> <00:60:00.000> <00:00:01.50> <0:00:01.500>", plus(time.Second), "<01:02.500> <00:60:00.000> <00:00:01.50> <0:00:01.500>"},
		{"no tag", "plain", plus(time.Second), "plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.MapWordTimestamps(tt.text, tt.fn))
		})
	}
}

func TestInsertWordTimestamps(t *testing.T) {
	t.Parallel()

	at := func(d time.Duration) *time.Duration { return &d }
	tests := []struct {
		name  string
		text  string
		words []markupcore.TimedWord
		want  string
	}{
		{
			"docs example",
			"Hi there",
			[]markupcore.TimedWord{{Text: "Hi", Start: at(time.Second)}, {Text: "there", Start: at(1400 * time.Millisecond)}},
			"<00:00:01.000>Hi <00:00:01.400>there",
		},
		{
			"escapes and searches from the previous word",
			"Tom & Tom left",
			[]markupcore.TimedWord{
				{Text: "Tom", Start: at(time.Second)},
				{Text: "Tom", Start: at(1500 * time.Millisecond)},
				{Text: "", Start: at(2 * time.Second)},
				{Text: "missing", Start: at(2100 * time.Millisecond)},
				{Text: "left", Start: at(2250 * time.Millisecond)},
			},
			"<00:00:01.000>Tom &amp; <00:00:01.500>Tom <00:00:02.250>left",
		},
		{"words without a start", "a < b", []markupcore.TimedWord{{Text: "a"}, {Text: "b"}}, "a &lt; b"},
		{"start at 0", "a", []markupcore.TimedWord{{Text: "a", Start: at(0)}}, "<00:00:00.000>a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.InsertWordTimestamps(tt.text, tt.words))
		})
	}
}

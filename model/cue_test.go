package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestCueText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no lines", nil, ""},
		{"one line", []string{"Hello"}, "Hello"},
		{"two lines", []string{"this is a line", "with a linebreak"}, "this is a line\nwith a linebreak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, model.Cue{Lines: tt.lines}.Text())
		})
	}
}

func TestCueSetText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{"one line per LF", "this is a line\nwith a linebreak", []string{"this is a line", "with a linebreak"}},
		{
			"CRLF, double spaces and tabs",
			"this is a       line\r\n\r\nwith a \t\t   linebreak",
			[]string{"this is a line", "with a linebreak"},
		},
		{"empty", "", nil},
		{"blank lines only", "\n \n\t", nil},
		{"lone CR stays inside one line", "a\rb", []string{"a\nb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Lines: []string{"old"}}
			c.SetText(tt.text)
			assert.Equal(t, tt.want, c.Lines)
		})
	}
}

func TestCueAddLine(t *testing.T) {
	t.Parallel()

	var c model.Cue
	c.AddLine("first line")
	c.AddLine("  ")
	c.AddLine("second  \tline ")

	assert.Equal(t, []string{"first line", "second line"}, c.Lines)
}

func TestCueMapWordTimestamps(t *testing.T) {
	t.Parallel()

	double := func(d time.Duration) time.Duration { return 2*d - 3*time.Second }
	tests := []struct {
		name  string
		lines []string
		fn    func(time.Duration) time.Duration
		want  []string
	}{
		{
			"below 0 becomes 0",
			[]string{"One <00:00:01.000>two <00:00:02.500>three"},
			double,
			[]string{"One <00:00:00.000>two <00:00:02.000>three"},
		},
		{
			"each line",
			[]string{"<00:00:01.000>Hi", "<00:59:59.500>there"},
			func(d time.Duration) time.Duration { return d + 2*time.Second },
			[]string{"<00:00:03.000>Hi", "<01:00:01.500>there"},
		},
		{
			"rounds to whole milliseconds",
			[]string{"<00:00:01.000>Hi"},
			func(d time.Duration) time.Duration { return d + 500*time.Microsecond },
			[]string{"<00:00:01.001>Hi"},
		},
		{
			"more than 2 hour digits",
			[]string{"<100:59:59.999>end"},
			func(d time.Duration) time.Duration { return d + time.Millisecond },
			[]string{"<101:00:00.000>end"},
		},
		{
			"not word timestamps",
			[]string{"<01:02.500> <00:60:00.000> <00:00:01.50> <0:00:01.500> <b>x</b>"},
			func(time.Duration) time.Duration { return 0 },
			[]string{"<01:02.500> <00:60:00.000> <00:00:01.50> <0:00:01.500> <b>x</b>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Lines: tt.lines}
			c.MapWordTimestamps(tt.fn)
			assert.Equal(t, tt.want, c.Lines)
		})
	}
}

func TestCueClone(t *testing.T) {
	t.Parallel()

	c := model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"a", "b"}, ID: "intro", Alignment: model.TopLeft, Forced: true}
	clone := c.Clone()
	assert.Equal(t, c, clone)

	clone.Lines[0] = "changed"
	assert.Equal(t, []string{"a", "b"}, c.Lines)
	assert.Nil(t, model.Cue{}.Clone().Lines)
}

package markup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/markup"
)

func TestInsertWordTimestamps(t *testing.T) {
	t.Parallel()

	at := func(d time.Duration) *time.Duration { return &d }
	tests := []struct {
		name  string
		text  string
		words []markup.TimedWord
		want  string
	}{
		{
			"escapes and searches from the previous word",
			"Tom & Tom left",
			[]markup.TimedWord{
				{Text: "Tom", Start: at(time.Second)},
				{Text: "Tom", Start: at(1500 * time.Millisecond)},
				{Text: "", Start: at(2 * time.Second)},
				{Text: "missing", Start: at(2100 * time.Millisecond)},
				{Text: "left", Start: at(2250 * time.Millisecond)},
			},
			"<00:00:01.000>Tom &amp; <00:00:01.500>Tom <00:00:02.250>left",
		},
		{"words without a start", "a < b", []markup.TimedWord{{Text: "a"}, {Text: "b"}}, "a &lt; b"},
		{"no words", "a & b", nil, "a &amp; b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markup.InsertWordTimestamps(tt.text, tt.words))
		})
	}
}

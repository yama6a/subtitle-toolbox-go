package readingspeed_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/inspect/internal/readingspeed"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestMeasure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cue        model.Cue
		lengths    []int
		characters int
		words      int
		cps, wpm   float64
	}{
		{
			name:       "two lines with a tag",
			cue:        model.Cue{Start: 0, End: 2 * time.Second, Lines: []string{"<i>Hello</i> there", "my friend"}},
			lengths:    []int{11, 9},
			characters: 20,
			words:      4,
			cps:        10,
			wpm:        120,
		},
		{
			name: "no lines",
			cue:  model.Cue{Start: 0, End: time.Second},
			cps:  0,
			wpm:  0,
		},
		{
			name:       "duration of 0",
			cue:        model.Cue{Start: time.Second, End: time.Second, Lines: []string{"Hi"}},
			lengths:    []int{2},
			characters: 2,
			words:      1,
			cps:        math.Inf(1),
			wpm:        math.Inf(1),
		},
		{
			name:    "line with only a tag",
			cue:     model.Cue{Start: 0, End: time.Second, Lines: []string{"<i></i>", "Go"}},
			lengths: []int{0, 2}, characters: 2, words: 1,
			cps: 2,
			wpm: 60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := readingspeed.Measure(tt.cue)
			assert.Len(t, m.LineLengths, len(tt.cue.Lines))
			assert.Len(t, m.PlainLines, len(tt.cue.Lines))
			if tt.lengths != nil {
				assert.Equal(t, tt.lengths, m.LineLengths)
			}
			assert.Equal(t, tt.characters, m.Characters)
			assert.Len(t, m.Words, tt.words)
			assert.InDelta(t, tt.cps, m.CharactersPerSecond(), 1e-9)
			assert.InDelta(t, tt.wpm, m.WordsPerMinute(), 1e-9)
		})
	}
}

package chapters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestEndChapters(t *testing.T) {
	t.Parallel()

	s := time.Second
	at := func(d time.Duration) *time.Duration { return &d }
	tests := []struct {
		name  string
		start []time.Duration
		ends  []*time.Duration
		media time.Duration
		want  []time.Duration
	}{
		{"unsorted", []time.Duration{30 * s, 0, 10 * s}, nil, 0, []time.Duration{10 * s, 30 * s, 30 * s}},
		{"media after last start", []time.Duration{0, 10 * s}, nil, 50 * s, []time.Duration{10 * s, 50 * s}},
		{"media before last start", []time.Duration{0, 10 * s}, nil, 5 * s, []time.Duration{10 * s, 10 * s}},
		{"ends from the file move with their chapter", []time.Duration{20 * s, 0}, []*time.Duration{at(25 * s), nil}, 40 * s, []time.Duration{20 * s, 25 * s}},
		{"equal starts keep their order", []time.Duration{5 * s, 5 * s}, nil, 0, []time.Duration{5 * s, 5 * s}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cues := make([]model.Cue, len(tt.start))
			for i, d := range tt.start {
				cues[i] = model.Cue{Start: d, Lines: []string{string(rune('a' + i))}}
			}
			var got []time.Duration
			for _, c := range endChapters(cues, tt.ends, tt.media) {
				got = append(got, c.End)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

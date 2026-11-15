package csv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testALastCueWithoutEndLastsFiveSecondsByDefault and testALastCueDurationOf0KeepsTheLastCue
// of tests/ReadOptionsTest.php of subtitle-toolbox-php, which no 1.0 reader runs.
func TestEndAtNextStart(t *testing.T) {
	t.Parallel()

	s := time.Second
	tests := []struct {
		name   string
		last   *time.Duration
		starts []time.Duration
		i      int
		want   time.Duration
	}{
		{name: "next start", starts: []time.Duration{1 * s, 2 * s}, i: 0, want: 2 * s},
		{name: "last cue lasts 5 s by default", starts: []time.Duration{1 * s, 2 * s}, i: 1, want: 7 * s},
		{name: "last cue lasts the option", last: new(7 * s), starts: []time.Duration{3600 * s}, i: 0, want: 3607 * s},
		{name: "last cue duration 0 keeps the start", last: new(time.Duration(0)), starts: []time.Duration{32800 * time.Millisecond}, i: 0, want: 32800 * time.Millisecond},
		{name: "same start does not end the cue", starts: []time.Duration{1 * s, 1 * s, 4 * s}, i: 0, want: 4 * s},
		{name: "2 cues at the same start at the end", starts: []time.Duration{1 * s, 1 * s}, i: 0, want: 6 * s},
		{name: "second of 2 cues at the same start", starts: []time.Duration{1 * s, 1 * s}, i: 1, want: 6 * s},
		{name: "an earlier start is skipped", starts: []time.Duration{3 * s, 1 * s, 5 * s}, i: 0, want: 5 * s},
		{name: "rounds to whole milliseconds", last: new(1000500 * time.Microsecond), starts: []time.Duration{0}, i: 0, want: 1001 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := parsing.NewState(model.CSV, model.CommonReadOptions{LastCueDuration: tt.last})
			assert.Equal(t, tt.want, endAtNextStart(st, tt.starts, tt.i))
		})
	}
}

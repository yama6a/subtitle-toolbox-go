package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpeakers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"no voice tag", []string{"Hello"}, []string{}},
		{"a voice tag without a name", []string{"<v>Hello"}, []string{}},
		{"one speaker", []string{"<v Anna>Hello"}, []string{"Anna"}},
		{"upper-case tag", []string{"<V Anna>Hello"}, []string{"Anna"}},
		{"classes", []string{"<v.loud.angry Anna>Hello"}, []string{"Anna"}},
		{"trimmed name", []string{"<v \tAnna \t>Hello"}, []string{"Anna"}},
		{"no repeats", []string{"<v Ben>a", "<v Anna>b", "<v Ben >c"}, []string{"Anna", "Ben"}},
		{"byte order", []string{"<v anna>a", "<v Zoe>b", "<v Émile>c"}, []string{"Zoe", "anna", "Émile"}},
		{"2 tags in one line", []string{"<v Ben>a</v> <v Anna>b"}, []string{"Anna", "Ben"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, speakers(Cue{Lines: tt.lines}))
		})
	}
}

func TestCanJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b Cue
		want bool
	}{
		{"same speakers in another order", Cue{Lines: []string{"<v Anna>a", "<v Ben>b"}}, Cue{Lines: []string{"<V Ben>c", "<v Anna >d"}}, true},
		{"other speakers", Cue{Lines: []string{"<v Anna>a"}}, Cue{Lines: []string{"<v Ben>a"}}, false},
		{"a speaker and none", Cue{Lines: []string{"<v Anna>a"}}, Cue{Lines: []string{"a"}}, false},
		{"other alignment", Cue{}, Cue{Alignment: TopCenter}, false},
		{"an alignment outside the grid counts as bottom center", Cue{}, Cue{Alignment: Alignment(42)}, true},
		{"other forced flag", Cue{}, Cue{Forced: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, canJoin(tt.a, tt.b))
		})
	}
}

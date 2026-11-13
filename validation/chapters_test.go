package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/validation"
)

func TestCheckYouTubeChapters(t *testing.T) {
	t.Parallel()

	zero, three, ten := 0.0, 3.0, 10.0
	tests := []struct {
		name string
		cues [][2]int
		want []validation.Violation
	}{
		{"valid, last length unknown", [][2]int{{0, 10000}, {10000, 20000}, {20000, 20000}}, nil},
		{"every rule", [][2]int{{5000, 9500}, {9500, 30000}}, []validation.Violation{
			{Cue: 0, Rule: validation.RuleFirstChapterAtZero, Value: 5, Limit: &zero},
			{Cue: validation.NoCue, Rule: validation.RuleMinChapters, Value: 2, Limit: &three},
			{Cue: 0, Rule: validation.RuleMinDuration, Value: 4.5, Limit: &ten},
		}},
		{"no chapters", nil, []validation.Violation{{Cue: validation.NoCue, Rule: validation.RuleMinChapters, Value: 0, Limit: &three}}},
		{"first start below 1 s", [][2]int{{900, 10900}, {10900, 21000}, {21000, 40000}}, nil},
		{"known last chapter too short", [][2]int{{0, 10000}, {10000, 20000}, {20000, 25000}}, []validation.Violation{
			{Cue: 2, Rule: validation.RuleMinDuration, Value: 5, Limit: &ten},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle()
			for _, c := range tt.cues {
				s.AddCue(cue(c[0], c[1]))
			}
			assert.Equal(t, tt.want, validation.CheckYouTubeChapters(s))
		})
	}
}

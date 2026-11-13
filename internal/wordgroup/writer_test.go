package wordgroup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestCueHelpers(t *testing.T) {
	t.Parallel()
	cue := &model.Cue{Lines: []string{"<i>Tea</i>  &amp; <ruby>toast<rt>x</rt></ruby>", "later\u00a0now "}}
	assert.Equal(t, "Tea & toast (x) later now", wordgroup.CueText(cue))
	assert.True(t, wordgroup.SameText(cue, parseJSON(t, `" Tea & toast (x)\tlater now"`)))
	assert.False(t, wordgroup.SameText(cue, parseJSON(t, `"Tea"`)))
	assert.False(t, wordgroup.SameText(cue, parseJSON(t, `5`)))

	assert.True(t, wordgroup.SameTime(parseJSON(t, `1.2504`), 1e9, ms(1250)))
	assert.True(t, wordgroup.SameTime(parseJSON(t, `0`), 1e9, ms(-500)))
	assert.False(t, wordgroup.SameTime(parseJSON(t, `"x"`), 1e9, 0))
	assert.False(t, wordgroup.SameTime(parseJSON(t, `1.3`), 1e9, ms(1250)))

	voiced := &model.Cue{Lines: []string{"<v Ann>Hi</v>", "<v Bob>Yo</v>"}}
	assert.Equal(t, "Ann", wordgroup.CueSpeaker(voiced))
	voiced.Speaker = "Cy"
	assert.Equal(t, "Cy", wordgroup.CueSpeaker(voiced))
	assert.Empty(t, wordgroup.CueSpeaker(cue))
}

func TestTaggedLines(t *testing.T) {
	t.Parallel()
	c := &model.Cue{Lines: []string{"<i>The beach</i>", "was quiet."}, Words: []model.Word{
		{Text: "The", Start: ms(0), End: ms(200)}, {Text: "beach", Start: ms(240), End: ms(500)}, {Text: "quiet", Start: ms(950), End: ms(1200)},
	}}
	assert.Equal(t, []string{"<00:00:00.000><i>The <00:00:00.240>beach</i>", "was <00:00:00.950>quiet."}, wordgroup.TaggedLines(c))
	assert.Equal(t, []string{"a"}, wordgroup.TaggedLines(&model.Cue{Lines: []string{"a"}}))
}

func TestSpans(t *testing.T) {
	t.Parallel()
	type span struct {
		text       string
		start, end int
		word       int // index in Cue.Words, -1 for the leading span
	}
	tests := []struct {
		name string
		cue  model.Cue
		want []span
	}{
		{"beach", model.Cue{Lines: []string{"The beach was quiet."}, Words: []model.Word{
			{Text: "The", End: ms(200)}, {Text: "beach", Start: ms(240), End: ms(650)}, {Text: "quiet", Start: ms(950), End: ms(1200)},
		}}, []span{{"The", 0, 200, 0}, {"beach was", 240, 650, 1}, {"quiet.", 950, 1200, 2}}},
		{"leading span and a word the text lacks", model.Cue{Start: ms(100), Lines: []string{"Well, the end"}, Words: []model.Word{
			{Text: "the", Start: ms(500), End: ms(600)}, {Text: "gone", Start: ms(700), End: ms(800)}, {Text: "end", Start: ms(900), End: ms(1000)},
		}}, []span{{"Well,", 0, 500, -1}, {"the", 500, 600, 0}, {"end", 900, 1000, 2}}},
		{
			"leading end is at least the cue start",
			model.Cue{Start: ms(800), Lines: []string{"x a"}, Words: []model.Word{{Text: "a", Start: ms(500), End: ms(600)}}},
			[]span{{"x", 0, 800, -1}, {"a", 500, 600, 0}},
		},
		{"no word found", model.Cue{Lines: []string{"abc"}, Words: []model.Word{{Text: "z", End: ms(1)}}}, []span{{"abc", 0, 0, -1}}},
		{"no words", model.Cue{Lines: []string{"abc"}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []span
			for _, sp := range wordgroup.Spans(&tt.cue) {
				idx := -1
				if sp.Word != nil {
					for k := range tt.cue.Words {
						if sp.Word == &tt.cue.Words[k] {
							idx = k
						}
					}
				}
				got = append(got, span{sp.Text, int(sp.Start.Milliseconds()), int(sp.End.Milliseconds()), idx})
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

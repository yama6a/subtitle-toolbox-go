package resegment

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func word(text string, start time.Duration) model.Word {
	return model.Word{Text: text, Start: start, End: start}
}

func TestPieces(t *testing.T) {
	t.Parallel()

	ms := time.Millisecond
	tests := []struct {
		name       string
		cue        model.Cue
		texts      []string
		separators []string
		positions  []int
		times      []time.Duration
		ranks      []int
		ends       []bool
	}{
		{
			name:  "a word marker splits a word",
			cue:   model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"Welcome home"}, Words: []model.Word{word("Wel", time.Second), word("come", 1400*ms)}},
			texts: []string{"Wel", "come", "home"}, separators: []string{"", "", " "},
			positions: []int{0, 3, 8, 12}, times: []time.Duration{1000 * ms, 1400 * ms, 2289 * ms, 3000 * ms},
			ranks: []int{-1, 2, 2}, ends: []bool{false, false, false},
		},
		{
			name:  "closing tag before a lower case word",
			cue:   model.Cue{End: 2 * time.Second, Lines: []string{"<i>Hi.</i> there"}},
			texts: []string{"<i>Hi.</i>", "there"}, separators: []string{"", " "},
			positions: []int{0, 4, 9}, times: []time.Duration{0, 889 * ms, 2000 * ms},
			ranks: []int{-1, 2}, ends: []bool{false, false},
		},
		{
			name:  "tag around a sentence end",
			cue:   model.Cue{End: 2 * time.Second, Lines: []string{"<i>Hi. There</i>"}},
			texts: []string{"<i>Hi.</i>", "<i>There</i>"}, separators: []string{"", " "},
			positions: []int{0, 4, 9}, times: []time.Duration{0, 889 * ms, 2000 * ms},
			ranks: []int{-1, 0}, ends: []bool{true, false},
		},
		{
			name:  "tags only after a space join the piece before",
			cue:   model.Cue{End: time.Second, Lines: []string{"<i>Hi</i> </i>"}},
			texts: []string{"<i>Hi</i> </i>"}, separators: []string{""},
			positions: []int{0, 2}, times: []time.Duration{0, time.Second}, ranks: []int{-1}, ends: []bool{false},
		},
		{
			name:  "lines",
			cue:   model.Cue{End: 2 * time.Second, Lines: []string{"One", "two"}},
			texts: []string{"One", "two"}, separators: []string{"", "\n"},
			positions: []int{0, 4, 7}, times: []time.Duration{0, 1143 * ms, 2000 * ms},
			ranks: []int{-1, 2}, ends: []bool{false, false},
		},
		{name: "empty", cue: model.Cue{End: time.Second}, positions: []int{0}, times: []time.Duration{time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newCuePieces(tt.cue)
			var texts, separators []string
			for i, p := range ps.pieces {
				texts = append(texts, ps.text(i, i+1))
				separators = append(separators, p.separator)
			}
			assert.Equal(t, tt.texts, texts)
			assert.Equal(t, tt.separators, separators)
			assert.Equal(t, tt.positions, ps.positions)
			assert.Equal(t, tt.times, ps.times(tt.cue.Start, tt.cue.End))
			if len(tt.ranks) > 0 {
				assert.Equal(t, tt.ranks, ps.ranks)
				assert.Equal(t, tt.ends, ps.ends)
			}
		})
	}
}

func TestPiecesOfCJKPunctuation(t *testing.T) {
	t.Parallel()

	jp := strings.Split(strings.TrimSpace(string(testfiles.Read(t, "resegmenting/cjk_texts.txt"))), "\n")[0]
	ps := newCuePieces(model.Cue{End: 10 * time.Second, Lines: []string{jp}})

	require.Len(t, ps.pieces, 5)
	texts := make([]string, 0, len(ps.pieces))
	for i := range ps.pieces {
		texts = append(texts, ps.text(i, i+1))
	}
	assert.Equal(t, jp, strings.Join(texts, ""))
	for _, p := range ps.pieces[1:] {
		assert.Empty(t, p.separator)
	}
	assert.Equal(t, []int{-1, 0, 1, 0, 0}, ps.ranks)
}

package wordgroup_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

type cueRow struct {
	start, end int
	text       string
}

func TestGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		words []wordgroup.Word
		want  []cueRow
	}{
		{"sentence end", []wordgroup.Word{word("One.", 0, 300), word("Two", 400, 700)}, []cueRow{{0, 300, "One."}, {400, 700, "Two"}}},
		{"question mark and quote", []wordgroup.Word{word("Why?\"", 0, 300), word("Two", 400, 700)}, []cueRow{{0, 300, "Why?\""}, {400, 700, "Two"}}},
		{"lower case after a dot", []wordgroup.Word{word("e.g.", 0, 300), word("this", 400, 700)}, []cueRow{{0, 700, "e.g. this"}}},
		{
			"CJK full stop",
			[]wordgroup.Word{word("\u3067\u3059\u3002", 0, 300), word("\u306f\u3044", 400, 700)},
			[]cueRow{{0, 300, "\u3067\u3059\u3002"}, {400, 700, "\u306f\u3044"}},
		},
		{"gap of 1 s", []wordgroup.Word{word("wait", 0, 300), word("then", 1300, 1600)}, []cueRow{{0, 300, "wait"}, {1300, 1600, "then"}}},
		{"gap below 1 s", []wordgroup.Word{word("wait", 0, 300), word("then", 1299, 1600)}, []cueRow{{0, 1600, "wait then"}}},
		{"overlap never splits", []wordgroup.Word{word("wait", 0, 900), word("then", 100, 1600)}, []cueRow{{0, 1600, "wait then"}}},
		{
			"speaker change",
			[]wordgroup.Word{spoken(word("Hi", 0, 300), "A"), spoken(word("there", 400, 700), "B")},
			[]cueRow{{0, 300, "Hi"}, {400, 700, "there"}},
		},
		{
			"missing speaker against empty",
			[]wordgroup.Word{word("Hi", 0, 300), spoken(word("there", 400, 700), "")},
			[]cueRow{{0, 300, "Hi"}, {400, 700, "there"}},
		},
		{"missing speakers match", []wordgroup.Word{word("Hi", 0, 300), word("there", 400, 700)}, []cueRow{{0, 700, "Hi there"}}},
		{
			"84 characters",
			[]wordgroup.Word{word(repeat("a", 41), 0, 300), word(repeat("b", 42), 300, 600), word("c", 600, 900)},
			[]cueRow{{0, 600, repeat("a", 41) + " " + repeat("b", 42)}, {600, 900, "c"}},
		},
		{
			"84 characters in UTF-8",
			[]wordgroup.Word{word(repeat("\u00e9", 41), 0, 300), word(repeat("\u00e8", 42), 300, 600)},
			[]cueRow{{0, 600, repeat("\u00e9", 41) + " " + repeat("\u00e8", 42)}},
		},
		{
			"84 characters with marks",
			[]wordgroup.Word{word(repeat("e\u0301", 41), 0, 300), word(repeat("a\u0308", 42), 300, 600), word("c", 600, 900)},
			[]cueRow{{0, 600, repeat("e\u0301", 41) + " " + repeat("a\u0308", 42)}, {600, 900, "c"}},
		},
		{
			"85 characters",
			[]wordgroup.Word{word(repeat("a", 41), 0, 300), word(repeat("b", 43), 300, 600)},
			[]cueRow{{0, 300, repeat("a", 41)}, {300, 600, repeat("b", 43)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := wordgroup.NewReader(parsing.NewState("test", model.CommonReadOptions{}), wordgroup.Options{}, "test")
			cues, err := r.CuesFromWords(tt.words, "words", nil)
			require.NoError(t, err)
			var got []cueRow
			for _, c := range cues {
				got = append(got, cueRow{int(c.Start.Milliseconds()), int(c.End.Milliseconds()), c.Text()})
			}
			assert.Equal(t, tt.want, got)
			joined := 0
			for _, g := range wordgroup.Group(tt.words) {
				joined += len(g)
			}
			assert.Len(t, tt.words, joined)
		})
	}
}

func FuzzGroup(f *testing.F) {
	f.Add("One. Two e.g. this\nA", uint16(300))
	f.Add("a\xffb. \u300d\u3002 \u00e9", uint16(1000))
	f.Fuzz(func(t *testing.T, s string, gap uint16) {
		fields := strings.Fields(s)
		words := make([]wordgroup.Word, 0, len(fields))
		for i, text := range fields {
			w := word(text, i*int(gap), i*int(gap)+int(gap)/2)
			if i%3 == 0 {
				w.Speaker, w.HasSpeaker = text[:1], true
			}
			words = append(words, w)
		}
		flat := []wordgroup.Word{}
		for _, g := range wordgroup.Group(words) {
			require.NotEmpty(t, g)
			flat = append(flat, g...)
		}
		if len(words) == 0 {
			require.Empty(t, flat)
			return
		}
		require.Equal(t, words, flat)
	})
}

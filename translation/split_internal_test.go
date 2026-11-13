package translation

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func pieces(translation string, weights []int, tags []tagPair) []string {
	parts := split(tokenize(translation), weights)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, restore(p, tags))
	}
	return out
}

func TestSplit(t *testing.T) {
	t.Parallel()

	italic := []tagPair{{"<i>", "</i>"}}
	tests := []struct {
		name        string
		translation string
		weights     []int
		tags        []tagPair
		want        []string
	}{
		{
			name:        "cut at the space nearest to the share",
			translation: "Er nimmt regelmässig an Veranstaltungen der Patientenorganisation teil.",
			weights:     []int{36, 25},
			want:        []string{"Er nimmt regelmässig an Veranstaltungen der", "Patientenorganisation teil."},
		},
		{
			name:        "Han text is cut between characters and keeps its tag",
			translation: "<x1>\u6211\u4eec\u660e\u5929\u65e9\u4e0a\u53bb\u5317\u4eac</x1>",
			weights:     []int{3, 3},
			tags:        italic,
			want:        []string{"<i>\u6211\u4eec\u660e\u5929</i>", "<i>\u65e9\u4e0a\u53bb\u5317\u4eac</i>"},
		},
		{
			name:        "more pieces than spaces leave the last piece empty",
			translation: "Bonjour tout",
			weights:     []int{1, 1, 1},
			want:        []string{"Bonjour", "tout", ""},
		},
		{
			name:        "the prolonged sound mark joins the kana that follow it",
			translation: "A\u30fc\u30b3",
			weights:     []int{1, 1},
			want:        []string{"A\u30fc", "\u30b3"},
		},
		{
			name:        "6 spaces and 4 pieces",
			translation: "a b c d e f g",
			weights:     []int{1, 1, 1, 1},
			want:        []string{"a b", "c d", "e", "f g"},
		},
		{
			name:        "a tag over 2 pieces closes and opens again",
			translation: "<x1>one two</x1> three",
			weights:     []int{1, 2},
			tags:        italic,
			want:        []string{"<i>one</i>", "<i>two</i> three"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, pieces(tt.translation, tt.weights, tt.tags))
		})
	}
}

func splitInput(words int) []token {
	return []token{{kind: textToken, text: strings.Repeat("word ", words)}}
}

func weightsFor(words int) []int {
	weights := make([]int, words/4)
	for i := range weights {
		weights[i] = 1
	}
	return weights
}

func BenchmarkSplit(b *testing.B) {
	for _, words := range []int{4000, 40000} {
		tokens, weights := splitInput(words), weightsFor(words)
		b.Run("words="+strconv.Itoa(words), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				split(tokens, weights)
			}
		})
	}
}

func TestSplitGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	best := func(words int) time.Duration {
		tokens, weights := splitInput(words), weightsFor(words)
		fastest := time.Duration(1<<63 - 1)
		for range 3 {
			begin := time.Now()
			split(tokens, weights)
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	small, large := best(4000), best(40000)
	ratio := float64(large) / float64(max(small, time.Nanosecond))
	assert.LessOrEqual(t, ratio, 30.0, "4000 words took %v, 40000 took %v", small, large)
}

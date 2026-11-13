package karaoke_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// megabyteCue returns one cue whose text is about 1 MB in n words.
func megabyteCue(n int) model.Cue {
	text := strings.Repeat("a", (1<<20)/n-1)
	c := model.Cue{End: time.Duration(n) * time.Second, Lines: []string{strings.TrimSpace(strings.Repeat(text+" ", n))}}
	for i := range n {
		c.Words = append(c.Words, model.Word{Text: text, Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
	}
	return c
}

func BenchmarkApply(b *testing.B) {
	for _, tt := range []struct {
		words, window int
	}{{40, 0}, {170000, 3}} {
		b.Run("words="+strconv.Itoa(tt.words)+"/window="+strconv.Itoa(tt.window), func(b *testing.B) {
			cue := megabyteCue(tt.words)
			b.ReportAllocs()
			for b.Loop() {
				var s model.Subtitle
				s.AddCue(cue)
				_, err := karaoke.Apply(&s, karaoke.Options{MaxWordsPerCue: tt.window})
				require.NoError(b, err)
			}
		})
	}
}

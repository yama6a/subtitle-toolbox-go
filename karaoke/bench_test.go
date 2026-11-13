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

// taggedCue returns one cue of about 1 MB with n words, each in its own <i> pair.
func taggedCue(n int) model.Cue {
	var text strings.Builder
	c := model.Cue{End: time.Duration(n) * time.Second}
	for i := range n {
		text.WriteString("<i>word</i> ")
		c.Words = append(c.Words, model.Word{Text: "word", Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
	}
	c.Lines = []string{strings.TrimSpace(text.String())}
	return c
}

func BenchmarkApplyTags(b *testing.B) {
	cue := taggedCue(80000)
	b.ReportAllocs()
	for b.Loop() {
		var s model.Subtitle
		s.AddCue(cue)
		_, err := karaoke.Apply(&s, karaoke.Options{MaxWordsPerCue: 3})
		require.NoError(b, err)
	}
}

// BenchmarkApplyTooLarge times the failure of a cue of 256 KB with 1,000 words, which would make 306 MiB.
func BenchmarkApplyTooLarge(b *testing.B) {
	text := strings.Repeat("a", 255)
	cue := model.Cue{End: 1000 * time.Second, Lines: []string{strings.TrimSpace(strings.Repeat(text+" ", 1000))}}
	for i := range 1000 {
		cue.Words = append(cue.Words, model.Word{Text: text, Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
	}
	b.ReportAllocs()
	for b.Loop() {
		var s model.Subtitle
		s.AddCue(cue)
		_, err := karaoke.Apply(&s, karaoke.Options{})
		require.Error(b, err)
	}
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

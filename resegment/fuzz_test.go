package resegment_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
)

func FuzzResegment(f *testing.F) {
	f.Add("One two. Three four, five six seven.\nEight nine", uint8(3), uint8(7))
	f.Add("<i>Hi.</i> there <b>and</b> more words here", uint8(1), uint8(20))
	f.Add("Welcome home e.g. this and that: now", uint8(0), uint8(9))
	f.Add(cjkTexts(f)[0], uint8(5), uint8(4))
	f.Add("a &amp; b <v Ann>c</v> d", uint8(2), uint8(2))
	f.Fuzz(checkResegment)
}

func checkResegment(t *testing.T, text string, gapStep, size uint8) {
	if strings.ContainsAny(text, string(rune(0x202b))+string(rune(0x202c))) {
		t.Skip("wrapping rewrites right-to-left embeddings")
	}
	c := model.Cue{End: 30 * time.Second}
	c.SetText(text)
	fields := strings.Fields(c.PlainText())
	words := make([]model.Word, 0, len(fields))
	at := time.Duration(0)
	for _, w := range fields {
		words = append(words, model.Word{Text: w, Start: at, End: at + 200*time.Millisecond})
		at += time.Duration(gapStep%8+1) * 100 * time.Millisecond
	}
	c.Words = words
	if c.ValidateWords() != nil {
		c.Words = nil
	}
	opts := resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: int(size%30) + 1, MaxLinesPerCue: 2, MaxDuration: 3 * time.Second}}
	zero := time.Duration(0)
	opts.Limits.MinDuration = &zero

	for _, run := range []func(*model.Subtitle, resegment.Options) (resegment.Report, error){resegment.SplitLong, resegment.ByWords} {
		var s model.Subtitle
		s.AddCue(c)
		before := plain(&s)
		_, err := run(&s, opts)
		require.NoError(t, err)
		cues := s.Cues()
		assert.True(t, slices.IsSortedFunc(cues, func(a, b model.Cue) int { return int(a.Start - b.Start) }))
		for _, got := range cues {
			require.NoError(t, got.ValidateWords())
		}
		assert.Equal(t, before, plain(&s))
	}
}

func plain(s *model.Subtitle) string {
	var parts []string
	for _, c := range s.All() {
		parts = append(parts, c.PlainText())
	}
	// CleanLines trims NUL at a line end, and a break can put a NUL there.
	return strings.ReplaceAll(strings.Join(strings.Fields(strings.Join(parts, " ")), ""), "\x00", "")
}

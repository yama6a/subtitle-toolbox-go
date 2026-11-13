package karaoke_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzApply(f *testing.F) {
	f.Add("The beach was quiet.", "<b>Hi</b> there\nyou", uint16(300), uint16(900))
	f.Add("a < b > c", "<i>x</i> <u>y", uint16(0), uint16(0))
	f.Add("", "", uint16(1), uint16(2))
	f.Fuzz(func(t *testing.T, text, more string, gap, shift uint16) {
		c := model.Cue{Start: time.Duration(shift) * time.Millisecond, End: 5 * time.Second}
		c.SetText(text + "\n" + more)
		at := time.Duration(0)
		for _, w := range strings.Fields(c.PlainText()) {
			c.Words = append(c.Words, model.Word{Text: w, Start: at, End: at})
			at += time.Duration(gap) * time.Millisecond
		}
		if c.ValidateWords() != nil {
			return
		}
		ownTag := strings.Contains(strings.ToLower(text+more), "u>")
		for _, mode := range []karaoke.Mode{karaoke.ModeWord, karaoke.ModeCumulative} {
			for window := range 4 {
				var s model.Subtitle
				s.AddCue(c)
				_, err := karaoke.Apply(&s, karaoke.Options{Mode: mode, MaxWordsPerCue: window})
				require.NoError(t, err)
				prev := time.Duration(-1 << 62)
				for _, got := range s.All() {
					assert.GreaterOrEqual(t, got.Start, prev)
					prev = got.Start
					assert.Nil(t, got.Words)
					for _, l := range got.Lines {
						if ownTag {
							continue
						}
						assert.Equal(t, strings.Count(l, "<u>"), strings.Count(l, "</u>"), l)
					}
				}
			}
		}
	})
}

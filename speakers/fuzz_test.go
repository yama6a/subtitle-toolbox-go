package speakers_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/speakers"
)

var voiceToken = regexp.MustCompile(`^(<v(\.[^ \t\n\r\f\v>]*)?([ \t\n\r\f\v][^>]*)?>|</v[ \t\n\r\f\v]*>)$`)

func FuzzApply(f *testing.F) {
	for _, seed := range []string{
		"JOHN: Hi.\nMARY: Bye.",
		"- JOHN: Hi.\n- MARY: Bye.",
		"<v Anna>Where? <v Ben>Home.",
		"<v Anna><i>Where? <v Ben>Home.</i>",
		"<v Anna>Hi</v> there",
		"<v.loud O&#39;Neil>Hi.\n<b>x",
		"Baker: <i>JOHN:</i>\nHi",
	} {
		f.Add([]byte(seed))
	}
	styles := []speakers.Style{speakers.StylePrefix, speakers.StyleDialogueDashes, speakers.StyleColors}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, style := range styles {
			for _, anyCase := range []bool{false, true} {
				var s model.Subtitle
				for i, chunk := range strings.Split(string(data), "\n\n") {
					c := model.Cue{Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second}
					c.SetText(chunk)
					if len(chunk)%2 == 1 {
						c.Speaker = "Anna"
					}
					s.AddCue(c)
				}
				_, err := speakers.Apply(&s, speakers.Options{
					FromLabels: true, AnyCaseLabels: anyCase, Style: style, Rename: map[string]string{"Anna": "Zoë"},
				})
				require.NoError(t, err)
				for i, c := range s.All() {
					require.Empty(t, c.Speaker, "cue %d", i)
					require.NoError(t, c.ValidateWords(), "cue %d", i)
					if !onlyTags(data) {
						continue
					}
					for _, line := range c.Lines {
						for k, tok := range markupcore.SplitTags(line) {
							require.False(t, k%2 == 1 && voiceToken.MatchString(tok), "cue %d keeps %q", i, tok)
						}
					}
				}
			}
		}
	})
}

// onlyTags reports whether each "<" of data starts a tag. Removing a voice tag from the middle of another "<" can
// join the text around it into a new voice tag, as in "<v <v x>>", so the check for left-over voice tags needs this.
func onlyTags(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Count(line, "<") != len(markupcore.SplitTags(line))/2 {
			return false
		}
	}
	return true
}

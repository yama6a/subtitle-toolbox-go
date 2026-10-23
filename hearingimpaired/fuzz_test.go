package hearingimpaired_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzApply(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "hearing-impaired"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".srt") {
			f.Add(testfiles.Read(f, "hearing-impaired/"+e.Name()))
		}
	}
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000\n- [gasps] No!\n- MARY: Yes.\n\n2\n00:00:00,500 --> 00:00:03,000\n<i>[x]</i>",
		"1\n00:00:01,000 --> 00:00:02,000\n\u266a The rain\nkeeps falling \u266a\nHi. *sighs* {laughs}",
		"1\n00:00:01,000 --> 00:00:02,000\n\xe9t\xe9 [x]\n<I>[y]</i> &amp; (z",
	} {
		f.Add([]byte(seed))
	}
	all := hearingimpaired.Options{
		AnyCaseLabels: true,
		Lyrics:        true,
		CustomBrackets: []hearingimpaired.BracketPair{
			{Open: "{", Close: "}"}, {Open: "*", Close: "*"}, {Open: "<<", Close: ">>"},
		},
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, opts := range []hearingimpaired.Options{{}, all} {
			s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: true}, srt.ReadOptions{})
			if err != nil {
				t.Skip()
			}

			_, err = hearingimpaired.Apply(s, opts)
			require.NoError(t, err)
			cues := s.Cues()
			for i, c := range cues {
				require.False(t, i > 0 && c.Start < cues[i-1].Start, "cue %d starts before cue %d", i, i-1)
				for _, line := range c.Lines {
					require.NotEmpty(t, strings.Trim(line, markupcore.PHPTrimSet), "cue %d", i)
				}
			}
		}
	})
}

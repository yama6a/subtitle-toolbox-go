package fixing_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/fixing"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzApply(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "fixing"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".srt") {
			f.Add(testfiles.Read(f, "fixing/"+e.Name()))
		}
	}
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000\n<i>Hi. - Hello.\nlt's |t 0nly\n\n2\n00:00:00,500 --> 00:00:03,000\n# i think #",
		"1\n00:00:01,000 --> 00:00:02,000\nWell. . . Really ?.\nStop.Now  -Hi\n\n2\n00:00:02,000 --> 00:00:03,000\nyts",
		"1\n00:00:01,000 --> 00:00:02,000\n\xe9t\xe9 |\n</b>caf\xe9 l'm",
	} {
		f.Add([]byte(seed))
	}
	all := fixing.Options{Language: "en", Enable: fixing.OptionalRules(), Ellipsis: fixing.EllipsisUnicode}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, opts := range []fixing.Options{{}, all, {Language: "fr"}, {Language: "tr", Enable: fixing.OptionalRules()}} {
			s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: true}, srt.ReadOptions{})
			if err != nil {
				t.Skip()
			}

			_, err = fixing.Apply(s, opts)
			require.NoError(t, err)
			cues := s.Cues()
			for i, c := range cues {
				require.False(t, i > 0 && c.Start < cues[i-1].Start, "cue %d starts before cue %d", i, i-1)
			}
		}
	})
}

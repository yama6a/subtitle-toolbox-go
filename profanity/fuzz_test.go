package profanity_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/profanity"
)

func FuzzApply(f *testing.F) {
	for _, name := range []string{"keys.srt", "keys_stars.srt", "keys_first_letter.srt", "keys_removed.srt"} {
		f.Add(testfiles.Read(f, "profanity/"+name))
	}
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000\nDamn <00:00:01.500>this <00:00:01.200>hell\n<i>hell</i> &amp; damned",
		"1\n00:00:05,000 --> 00:00:06,000\nhell\n\n2\n00:00:01,000 --> 00:00:09,000\n\xe9 hell <00:00:10.000>damn",
		"1\n00:00:01,000 --> 00:00:02,000\nHELL\u00e9 hell's \u03a3hell",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, mask := range []profanity.Mask{profanity.MaskStars, profanity.MaskFirstLetter, profanity.MaskRemove, profanity.MaskNone} {
			s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: true}, srt.ReadOptions{})
			if err != nil {
				t.Skip()
			}

			report, err := profanity.Apply(s, profanity.Options{Words: []string{"damn*", "hell"}, Mask: mask})
			require.NoError(t, err)
			for i, r := range report.MuteRanges {
				require.GreaterOrEqual(t, r.Start, time.Duration(0), "range %d", i)
				require.LessOrEqual(t, r.Start, r.End, "range %d", i)
				require.False(t, i > 0 && r.Start <= report.MuteRanges[i-1].End, "range %d overlaps range %d", i, i-1)
			}
			cues := s.Cues()
			for i, c := range cues {
				require.False(t, i > 0 && c.Start < cues[i-1].Start, "cue %d starts before cue %d", i, i-1)
			}
		}
	})
}

package stbox_test

import (
	"bytes"
	"testing"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
)

const benchCues = 20000

// BenchmarkParse ports tests/Parsers/ParseSpeedTest.php of subtitle-toolbox-php: 20,000 cues of
// srt/real/own_escaping.srt, copy k shifted by k periods, written once in each format.
func BenchmarkParse(b *testing.B) {
	base, _, err := stbox.Load("testdata/srt/real/own_escaping.srt", stbox.ReadOptions{Format: stbox.SubRip})
	if err != nil {
		b.Fatal(err)
	}
	last, _ := base.Cue(base.Len() - 1)
	period := last.End.Truncate(time.Second) + time.Second
	if last.End%time.Second != 0 {
		period += time.Second
	}
	s := &stbox.Subtitle{}
	cues := make([]stbox.Cue, 0, benchCues)
	for k := 0; len(cues) < benchCues; k++ {
		for _, c := range base.All() {
			if len(cues) == benchCues {
				break
			}
			c = c.Clone()
			c.Start += time.Duration(k) * period
			c.End += time.Duration(k) * period
			cues = append(cues, c)
		}
	}
	s.AddCues(cues...)

	for _, f := range []stbox.Format{stbox.SubRip, stbox.TTML, stbox.WebVTT, stbox.ASS, stbox.SAMI, stbox.MicroDVD, stbox.SBV, stbox.LRC, stbox.CSV, stbox.SCC, stbox.Whisper, stbox.PodcastTranscript, stbox.YouTubeTimedText} {
		var content bytes.Buffer
		write := stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}, WriteFrameRateLine: true}, SCC: scc.WriteOptions{Fit: true}}
		if _, err := stbox.Write(&content, s, f, write); err != nil {
			b.Fatal(err)
		}
		b.Run(string(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := parseBytes(content.Bytes(), stbox.ReadOptions{Format: f}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

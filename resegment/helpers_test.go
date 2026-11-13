package resegment_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// part is a cue in the form that the tests compare. Text holds the lines with their word timestamp tags.
type part struct {
	Start, End float64
	Text       string
}

func secs(s float64) time.Duration { return time.Duration(math.Round(s*1000)) * time.Millisecond }

// newCue builds a cue from text that may hold word timestamp tags and a voice tag.
func newCue(start, end float64, text string) model.Cue {
	c := model.Cue{Start: secs(start), End: secs(end), Lines: strings.Split(text, "\n")}
	c.ExtractWordTags()
	return c
}

func build(cues ...model.Cue) *model.Subtitle {
	var s model.Subtitle
	s.AddCues(cues...)
	return &s
}

func describe(s *model.Subtitle) []part {
	out := make([]part, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, part{c.Start.Seconds(), c.End.Seconds(), strings.Join(c.TaggedLines(), "\n")})
	}
	return out
}

func readRTL(tb testing.TB) *model.Subtitle {
	tb.Helper()

	s, _, err := srt.Read(strings.NewReader(string(testfiles.Read(tb, "transforms/own_rtl_long.srt"))), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(tb, err)
	return s
}

func cjkTexts(tb testing.TB) []string {
	tb.Helper()

	return strings.Split(strings.TrimSpace(string(testfiles.Read(tb, "resegmenting/cjk_texts.txt"))), "\n")
}

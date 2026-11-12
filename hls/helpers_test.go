package hls_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/hls"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const defaultMap = "X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000"

// cue is a cue in the form that the tests compare.
type cue struct {
	Start, End time.Duration
	Text       string
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func build(cues ...cue) *model.Subtitle {
	var s model.Subtitle
	for _, c := range cues {
		s.AddCue(model.Cue{Start: c.Start, End: c.End, Lines: strings.Split(c.Text, "\n")})
	}
	return &s
}

// ferry is a subtitle of 3 cues that ends at 20 s.
func ferry() *model.Subtitle {
	return build(
		cue{ms(1000), ms(3000), "The ferry leaves at noon."},
		cue{ms(5000), ms(7500), "Please keep your tickets ready."},
		cue{ms(18500), ms(20000), "Thank you for travelling with us."},
	)
}

func summarize(s *model.Subtitle) []cue {
	out := make([]cue, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cue{c.Start, c.End, c.Text()})
	}
	return out
}

func readVTT(tb testing.TB, content string) *model.Subtitle {
	tb.Helper()

	s, _, err := webvtt.Read(strings.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(tb, err)
	return s
}

func readFixture(tb testing.TB) *model.Subtitle {
	tb.Helper()

	return readVTT(tb, string(testfiles.Read(tb, "hls/node-webvtt-subs1.vtt")))
}

// segments returns the file names and the contents of every segment of r.
func segments(tb testing.TB, r *hls.Rendition) (names, contents []string) {
	tb.Helper()

	err := r.EachSegment(func(name string, content []byte) error {
		names = append(names, name)
		contents = append(contents, string(content))
		return nil
	})
	require.NoError(tb, err)
	return names, contents
}

func readFixtureText(tb testing.TB, name string) []byte {
	tb.Helper()

	return testfiles.Read(tb, "hls/"+name)
}

package karaoke_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func seconds(f float64) time.Duration {
	return time.Duration(math.Round(f*1000)) * time.Millisecond
}

// word is a word text and its start in seconds.
type word struct {
	text  string
	start float64
}

// cue builds a cue from start to end in seconds, with one word entry per timed word.
func cue(start, end float64, lines []string, words ...word) model.Cue {
	c := model.Cue{Start: seconds(start), End: seconds(end), Lines: lines}
	for _, w := range words {
		c.Words = append(c.Words, model.Word{Text: w.text, Start: seconds(w.start), End: seconds(w.start)})
	}
	return c
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	var s model.Subtitle
	s.AddCues(cues...)
	return &s
}

// out is a cue in the form that the tests compare.
type out struct {
	Start, End time.Duration
	Lines      []string
}

func describe(s *model.Subtitle) []out {
	res := make([]out, 0, s.Len())
	for _, c := range s.All() {
		res = append(res, out{c.Start, c.End, c.Lines})
	}
	return res
}

func rows(start, end float64, lines ...string) out {
	return out{seconds(start), seconds(end), lines}
}

func readFile(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	data := bytes.TrimPrefix(testfiles.Read(tb, name), []byte("\xef\xbb\xbf"))
	var (
		s   *model.Subtitle
		err error
	)
	switch name[len(name)-4:] {
	case ".srt":
		s, _, err = srt.Read(bytes.NewReader(data), model.CommonReadOptions{}, srt.ReadOptions{})
	case ".vtt":
		s, _, err = webvtt.Read(bytes.NewReader(data), model.CommonReadOptions{}, webvtt.ReadOptions{})
	case ".lrc":
		s, _, err = lrc.Read(bytes.NewReader(data), model.CommonReadOptions{}, lrc.ReadOptions{})
	case ".ass":
		s, _, err = ass.Read(bytes.NewReader(data), model.CommonReadOptions{}, ass.ReadOptions{})
	default:
		tb.Fatalf("no reader for %s", name)
	}
	require.NoError(tb, err)
	return s
}

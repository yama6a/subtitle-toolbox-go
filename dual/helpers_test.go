package dual_test

import (
	"bytes"
	"io"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// row is a cue in seconds.
type row struct {
	start, end float64
	text       string
}

// got is a cue in the form that the table tests compare.
type got struct {
	Start, End time.Duration
	Text       string
	Alignment  model.Alignment
}

func seconds(f float64) time.Duration {
	return time.Duration(math.Round(f*1000)) * time.Millisecond
}

func subtitle(rows ...row) *model.Subtitle {
	var s model.Subtitle
	for _, r := range rows {
		s.AddCue(model.Cue{Start: seconds(r.start), End: seconds(r.end), Lines: []string{r.text}})
	}
	return &s
}

func describe(s *model.Subtitle) []got {
	out := make([]got, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, got{c.Start, c.End, c.Text(), c.Alignment})
	}
	return out
}

func bottom(start, end float64, text string) got {
	return got{seconds(start), seconds(end), text, model.BottomCenter}
}

func at(a model.Alignment, start, end float64, text string) got {
	return got{seconds(start), seconds(end), text, a}
}

func tolerance(d time.Duration) *time.Duration { return &d }

func alignment(a model.Alignment) *model.Alignment { return &a }

func load(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	content := bytes.TrimPrefix(testfiles.Read(tb, "dual/"+name), []byte("\xef\xbb\xbf"))
	s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(tb, err)
	return s
}

// format is a writer and the reader of the same format.
type format struct {
	name  string
	ext   string
	write func(io.Writer, *model.Subtitle) error
	read  func(io.Reader) (*model.Subtitle, error)
}

func formats() []format {
	return []format{
		{
			"SubRip", "srt",
			func(w io.Writer, s *model.Subtitle) error {
				return srt.Write(w, s, model.CommonWriteOptions{}, srt.WriteOptions{})
			},
			func(r io.Reader) (*model.Subtitle, error) {
				s, _, err := srt.Read(r, model.CommonReadOptions{}, srt.ReadOptions{})
				return s, err
			},
		},
		{
			"WebVTT", "vtt",
			func(w io.Writer, s *model.Subtitle) error {
				return webvtt.Write(w, s, model.CommonWriteOptions{}, webvtt.WriteOptions{})
			},
			func(r io.Reader) (*model.Subtitle, error) {
				s, _, err := webvtt.Read(r, model.CommonReadOptions{}, webvtt.ReadOptions{})
				return s, err
			},
		},
		{
			"ASS", "ass",
			func(w io.Writer, s *model.Subtitle) error {
				return ass.Write(w, s, model.CommonWriteOptions{}, ass.WriteOptions{})
			},
			func(r io.Reader) (*model.Subtitle, error) {
				s, _, err := ass.Read(r, model.CommonReadOptions{}, ass.ReadOptions{})
				return s, err
			},
		},
	}
}

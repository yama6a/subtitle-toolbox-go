package chapters_test

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

type span struct {
	start, end time.Duration
	title      string
}

type reader func(io.Reader, model.CommonReadOptions, chapters.ReadOptions) (*model.Subtitle, []model.ParseWarning, error)

type writer func(io.Writer, *model.Subtitle, model.CommonWriteOptions, chapters.WriteOptions) error

// sec returns f seconds rounded to whole milliseconds.
func sec(f float64) time.Duration {
	return time.Duration(math.Round(f*1000)) * time.Millisecond
}

func spansOf(s *model.Subtitle) []span {
	var out []span
	for _, c := range s.All() {
		out = append(out, span{c.Start, c.End, c.Text()})
	}
	return out
}

func readSubtitle(tb testing.TB, rd reader, content string, opts chapters.ReadOptions) *model.Subtitle {
	tb.Helper()

	s, warnings, err := rd(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(tb, err)
	assert.Empty(tb, warnings)
	return s
}

// readError returns the read error of content, in strict and in lenient mode. Both must be the same error.
func readError(tb testing.TB, rd reader, content string) *model.ParseError {
	tb.Helper()

	var first *model.ParseError
	for _, lenient := range []bool{false, true} {
		_, _, err := rd(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, chapters.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(tb, err, &perr)
		if first != nil {
			assert.Equal(tb, first, perr)
		}
		first = perr
	}
	return first
}

// chaptersOf returns a subtitle with one cue per row: start, end and the lines.
func chaptersOf(rows ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(rows...)
	return s
}

func writeString(tb testing.TB, wr writer, s *model.Subtitle, common model.CommonWriteOptions) string {
	tb.Helper()

	var buf bytes.Buffer
	require.NoError(tb, wr(&buf, s, common, chapters.WriteOptions{}))
	return buf.String()
}

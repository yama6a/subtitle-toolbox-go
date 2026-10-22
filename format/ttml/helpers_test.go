package ttml_test

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	realDir = "ttml/real"
	// header is the root start tag of the PHP parser tests, without its closing >.
	header = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling"` +
		` xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"`
	// bare is the root start tag of the PHP lenient tests.
	bare = `<tt xmlns="http://www.w3.org/ns/ttml">`
)

type cueRow struct {
	Start, End time.Duration
	Text       string
}

type warningRow struct {
	Line, Block int
	Action      model.WarningAction
	Message     string
}

// sec returns seconds as a duration in whole milliseconds, for the times of the PHP tests.
func sec(s float64) time.Duration {
	return time.Duration(math.Round(s*1000)) * time.Millisecond
}

func read(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return ttml.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ttml.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(t, content, lenient)
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string, lenient bool) *model.ParseError {
	t.Helper()

	_, _, err := read(t, content, lenient)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.TTML, perr.Format)
	return perr
}

// document returns the TTML of the PHP parser tests: the header with extra root attributes, a head and a body.
func document(body, rootAttributes, head string) string {
	return header + " " + rootAttributes + "><head>" + head + "</head><body>" + body + "</body></tt>"
}

// paragraphCue reads a paragraph inside a <div> and returns its cue.
func paragraphCue(t *testing.T, p, rootAttributes, head string) model.Cue {
	t.Helper()

	s, _ := mustRead(t, document("<div>"+p+"</div>", rootAttributes, head), false)
	require.Positive(t, s.Len())
	c, _ := s.Cue(0)
	return c
}

func cueRows(s *model.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{c.Start, c.End, c.Text()})
	}
	return out
}

func warningRows(warnings []model.ParseWarning) []warningRow {
	out := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, warningRow{w.Line, w.Block, w.Action, w.Message})
	}
	return out
}

func readFile(t *testing.T, rel string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := ttml.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{Lenient: lenient}, ttml.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

func fileData(t *testing.T, s *model.Subtitle) *model.Data {
	t.Helper()

	d, ok := s.FormatData("ttml")
	require.True(t, ok)
	return d
}

func cueData(t *testing.T, c model.Cue) *model.Data {
	t.Helper()

	d, ok := c.FormatData("ttml")
	require.True(t, ok)
	return d
}

func attrs(fields ...string) *model.Data {
	d := model.NewData()
	for i := 0; i+1 < len(fields); i += 2 {
		d.Set(fields[i], fields[i+1])
	}
	return d
}

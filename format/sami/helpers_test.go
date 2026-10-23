package sami_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "sami/real"
)

type cueRow struct {
	Start, End time.Duration
	Lines      []string
}

type warningRow struct {
	Message     string
	Line, Block int
	Lines       []string
	Action      model.WarningAction
}

func read(t *testing.T, content string, common model.CommonReadOptions, opts sami.ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return sami.Read(strings.NewReader(content), common, opts)
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(t, content, model.CommonReadOptions{Lenient: lenient}, sami.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string) *model.ParseError {
	t.Helper()

	_, _, err := read(t, content, model.CommonReadOptions{}, sami.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.SAMI, perr.Format)
	return perr
}

// readBody reads a file of the given SYNC blocks, as parseBody of tests/Parsers/SamiParserTest.php builds it.
func readBody(t *testing.T, body string) *model.Subtitle {
	t.Helper()

	s, _ := mustRead(t, "<SAMI>\n<BODY>\n"+body+"</BODY>\n</SAMI>\n", false)
	return s
}

func readFile(t *testing.T, file string, opts sami.ReadOptions) *model.Subtitle {
	t.Helper()

	s, warnings, err := sami.Read(bytes.NewReader(testfiles.Read(t, realDir+"/"+file)), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, sami.Write(&out, s, common, sami.WriteOptions{}))
	return out.String()
}

func cueRows(s *model.Subtitle) []cueRow {
	var rows []cueRow
	for _, c := range s.All() {
		rows = append(rows, cueRow{c.Start, c.End, c.Lines})
	}
	return rows
}

func lines(s *model.Subtitle) [][]string {
	var out [][]string
	for _, c := range s.All() {
		out = append(out, c.Lines)
	}
	return out
}

func warningRows(warnings []model.ParseWarning) []warningRow {
	rows := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		rows = append(rows, warningRow{w.Message, w.Line, w.Block, w.Lines, w.Action})
	}
	return rows
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

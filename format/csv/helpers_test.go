package csv_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "csv/real"
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

func read(content string, common model.CommonReadOptions, opts csv.ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	return csv.Read(strings.NewReader(content), common, opts)
}

func mustRead(t *testing.T, content string, common model.CommonReadOptions, opts csv.ReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, common, opts)
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string, common model.CommonReadOptions, opts csv.ReadOptions) *model.ParseError {
	t.Helper()

	_, _, err := read(content, common, opts)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.CSV, perr.Format)
	return perr
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts csv.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, csv.Write(&out, s, common, opts))
	return out.String()
}

// noBOM is the common write options without the BOM that CSV writes by default.
var noBOM = model.CommonWriteOptions{BOM: model.BOMRemove}

func cueRows(s *model.Subtitle) []cueRow {
	var rows []cueRow
	for _, c := range s.All() {
		rows = append(rows, cueRow{c.Start, c.End, c.Lines})
	}
	return rows
}

func warningRows(warnings []model.ParseWarning) []warningRow {
	rows := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		rows = append(rows, warningRow{w.Message, w.Line, w.Block, w.Lines, w.Action})
	}
	return rows
}

func messages(warnings []model.ParseWarning) []string {
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, w.Message)
	}
	return out
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func fileData(t *testing.T, s *model.Subtitle) *model.Data {
	t.Helper()

	d, ok := s.FormatData("csv")
	require.True(t, ok)
	return d
}

package mpsub_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "mpsub/real"
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

func read(content string, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	return mpsub.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mpsub.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, lenient)
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string) *model.ParseError {
	t.Helper()

	_, _, err := read(content, false)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.MPSub, perr.Format)
	return perr
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts mpsub.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, mpsub.Write(&out, s, common, opts))
	return out.String()
}

// writeBare writes s without a BOM, with the default options.
func writeBare(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	return write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{})
}

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

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

// fileData returns the stored mpsub format data as a map.
func fileData(s *model.Subtitle) map[string]any {
	d, _ := s.FormatData("mpsub")
	out := map[string]any{}
	for _, k := range d.Keys() {
		out[k], _ = d.Get(k)
	}
	return out
}

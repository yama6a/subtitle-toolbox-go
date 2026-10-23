package subviewer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "subviewer/real"
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

func read(content string, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	return subviewer.Read(strings.NewReader(content), common, subviewer.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, model.CommonReadOptions{Lenient: lenient})
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string) *model.ParseError {
	t.Helper()

	_, _, err := read(content, model.CommonReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.SubViewer, perr.Format)
	return perr
}

func write(t *testing.T, s *model.Subtitle, opts subviewer.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, subviewer.Write(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, opts))
	return out.String()
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

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

// fileData returns the stored subviewer format data as a map. An object inside it is a map too.
func fileData(s *model.Subtitle) map[string]any {
	d, _ := s.FormatData("subviewer")
	return plain(d).(map[string]any)
}

func plain(v any) any {
	d, ok := v.(*model.Data)
	if !ok {
		return v
	}
	out := map[string]any{}
	for _, k := range d.Keys() {
		item, _ := d.Get(k)
		out[k] = plain(item)
	}
	return out
}

// header returns the stored header tags as a map.
func header(s *model.Subtitle) map[string]string {
	d, _ := s.FormatData("subviewer")
	out := map[string]string{}
	if h, ok := d.Object("header"); ok {
		for _, k := range h.Keys() {
			out[k], _ = h.String(k)
		}
	}
	return out
}

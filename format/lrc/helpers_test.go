package lrc_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	bom     = "\xEF\xBB\xBF"
	realDir = "lrc/real"
)

type cueRow struct {
	Start, End time.Duration
	Text       string
}

type warningRow struct {
	Message     string
	Line, Block int
	Lines       []string
	Action      model.WarningAction
}

func read(content string, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	return lrc.Read(strings.NewReader(content), common, lrc.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, model.CommonReadOptions{Lenient: lenient})
	require.NoError(t, err)
	return s, warnings
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := lrc.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, lrc.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, lrc.Write(&out, s, common, lrc.WriteOptions{}))
	return out.String()
}

func cueRows(s *model.Subtitle) []cueRow {
	var rows []cueRow
	for _, c := range s.All() {
		rows = append(rows, cueRow{c.Start, c.End, c.Text()})
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

// fileData returns the lrc file data of s as a JSON-like value, or nil.
func fileData(t *testing.T, s *model.Subtitle) map[string]any {
	t.Helper()

	d, ok := s.FormatData("lrc")
	if !ok {
		return nil
	}
	return dataMap(d)
}

func dataMap(d *model.Data) map[string]any {
	out := map[string]any{}
	for _, k := range d.Keys() {
		v, _ := d.Get(k)
		if inner, ok := v.(*model.Data); ok {
			v = dataMap(inner)
		}
		out[k] = v
	}
	return out
}

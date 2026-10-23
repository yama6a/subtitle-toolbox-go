package tmplayer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "tmplayer/real"
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
	return tmplayer.Read(strings.NewReader(content), common, tmplayer.ReadOptions{})
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
	require.Equal(t, model.TMPlayer, perr.Format)
	return perr
}

func readFile(t *testing.T, rel string, common model.CommonReadOptions) *model.Subtitle {
	t.Helper()

	s, warnings, err := tmplayer.Read(bytes.NewReader(testfiles.Read(t, rel)), common, tmplayer.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, tmplayer.Write(&out, s, common, tmplayer.WriteOptions{}))
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

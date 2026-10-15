package srt_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "srt/real"
)

type cueRow struct {
	Start, End time.Duration
	Lines      []string
}

type warningRow struct {
	Message     string
	Line, Block int
	Action      model.WarningAction
}

func read(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return srt.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, srt.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(t, content, lenient)
	require.NoError(t, err)
	return s, warnings
}

func readError(t *testing.T, content string) *model.ParseError {
	t.Helper()

	_, _, err := read(t, content, false)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	require.Equal(t, model.SubRip, perr.Format)
	return perr
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := srt.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, srt.Write(&out, s, common, srt.WriteOptions{}))
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
	if len(warnings) == 0 {
		return nil
	}
	rows := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		rows = append(rows, warningRow{w.Message, w.Line, w.Block, w.Action})
	}
	return rows
}

func firstCue(t *testing.T, s *model.Subtitle) model.Cue {
	t.Helper()

	c, ok := s.Cue(0)
	require.True(t, ok)
	return c
}

func subtitle(t *testing.T, cues ...model.Cue) *model.Subtitle {
	t.Helper()

	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

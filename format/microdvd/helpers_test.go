package microdvd_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "microdvd/real"
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

func read(content string, lenient bool, fps float64) (*model.Subtitle, []model.ParseWarning, error) {
	return microdvd.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, microdvd.ReadOptions{FrameRate: fps})
}

func mustRead(t *testing.T, content string, lenient bool, fps float64) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, lenient, fps)
	require.NoError(t, err)
	return s, warnings
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts microdvd.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, microdvd.Write(&out, s, common, opts))
	return out.String()
}

func cueRows(s *model.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{Start: c.Start, End: c.End, Lines: c.Lines})
	}
	return out
}

func warningRows(warnings []model.ParseWarning) []warningRow {
	out := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, warningRow{Message: w.Message, Line: w.Line, Block: w.Block, Action: w.Action})
	}
	return out
}

// subtitle returns a subtitle with one cue for each entry of lines, cue i from i+1 s to i+2 s.
func subtitle(lines ...[]string) *model.Subtitle {
	s := &model.Subtitle{}
	for i, l := range lines {
		start := time.Duration(2*i+1) * time.Second
		s.AddCue(model.Cue{Start: start, End: start + time.Second, Lines: l})
	}
	return s
}

func cueLines(t *testing.T, s *model.Subtitle, i int) []string {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok)
	return c.Lines
}

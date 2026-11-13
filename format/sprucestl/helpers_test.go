package sprucestl_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const ms = time.Millisecond

var (
	rate25 = model.FrameRate{Num: 25, Den: 1}
	rate30 = model.FrameRate{Num: 30, Den: 1}
)

type cueRow struct {
	Start, End time.Duration
	Lines      []string
	Alignment  model.Alignment
}

func read(content string, lenient bool, fps model.FrameRate) (*model.Subtitle, []model.ParseWarning, error) {
	return sprucestl.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, sprucestl.ReadOptions{FrameRate: fps})
}

func mustRead(t *testing.T, content string) *model.Subtitle {
	t.Helper()

	s, _, err := read(content, false, model.FrameRate{})
	require.NoError(t, err)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts sprucestl.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	_, err := sprucestl.Write(&out, s, common, opts)
	require.NoError(t, err)
	return out.String()
}

func cueRows(s *model.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{Start: c.Start, End: c.End, Lines: c.Lines, Alignment: c.Position.Alignment})
	}
	return out
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

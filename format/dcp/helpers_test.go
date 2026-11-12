package dcp_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const ms = time.Millisecond

// smpteReel wraps subtitles in an SMPTE root at 25 fps, as the PHP parser tests do.
func smpteReel(subtitles string) string {
	return `<SubtitleReel xmlns="http://www.smpte-ra.org/schemas/428-7/2014/DCST"><EditRate>25 1</EditRate><TimeCodeRate>25</TimeCodeRate>` +
		`<SubtitleList>` + subtitles + `</SubtitleList></SubtitleReel>`
}

func interopReel(subtitles string) string {
	return `<DCSubtitle Version="1.0">` + subtitles + `</DCSubtitle>`
}

func read(content string, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	return dcp.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, dcp.ReadOptions{})
}

func mustRead(t *testing.T, content string) *model.Subtitle {
	t.Helper()

	s, _, err := read(content, false)
	require.NoError(t, err)
	return s
}

func readFile(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	return mustRead(t, string(testfiles.Read(t, "dcp/real/"+name)))
}

func write(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	var out bytes.Buffer
	_, err := dcp.Write(&out, s, model.CommonWriteOptions{}, dcp.WriteOptions{})
	require.NoError(t, err)
	return out.String()
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

type cueRow struct {
	Start, End time.Duration
	Lines      []string
	Alignment  model.Alignment
	Offset     *model.Offset
}

func cueRows(s *model.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{Start: c.Start, End: c.End, Lines: c.Lines, Alignment: c.Position.Alignment, Offset: c.Position.Offset})
	}
	return out
}

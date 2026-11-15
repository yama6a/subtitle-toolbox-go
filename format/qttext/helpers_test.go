package qttext_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

const (
	ms      = time.Millisecond
	realDir = "qttext/real"
	// subtitleEditHeader is the header that Subtitle Edit writes, and the writer writes for a new file.
	subtitleEditHeader = "{QTtext} {font:Tahoma}\n{plain} {size:20}\n{timeScale:30}\n{width:160} {height:32}\n{timestamps:absolute} {language:0}\n"
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
	return qttext.Read(strings.NewReader(content), common, qttext.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(content, model.CommonReadOptions{Lenient: lenient})
	require.NoError(t, err)
	return s, warnings
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := qttext.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, qttext.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	_, err := qttext.Write(&out, s, common, qttext.WriteOptions{})
	require.NoError(t, err)
	return out.String()
}

func cueRows(s *model.Subtitle) []cueRow {
	rows := make([]cueRow, 0, s.Len())
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

// withHeader returns a subtitle with the cues and the stored qttext header lines.
func withHeader(t *testing.T, header []string, paired bool, cues ...model.Cue) *model.Subtitle {
	t.Helper()

	list := make([]any, len(header))
	for i, l := range header {
		list[i] = l
	}
	data := model.NewData(model.Field{Key: "header", Value: list})
	if paired {
		data.Set("pairedTimestamps", true)
	}
	s := formattest.Subtitle(cues...)
	require.NoError(t, s.SetFormatData("qttext", data))
	return s
}

// fileData returns the qttext file data of s as a JSON-like value, or nil.
func fileData(s *model.Subtitle) map[string]any {
	d, ok := s.FormatData("qttext")
	if !ok {
		return nil
	}
	out := map[string]any{}
	for _, k := range d.Keys() {
		out[k], _ = d.Get(k)
	}
	return out
}

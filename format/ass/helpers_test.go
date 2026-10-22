package ass_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms      = time.Millisecond
	realDir = "ass/real"
	// eventsHeader is the [Events] section start of the PHP tests.
	eventsHeader = "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"
	// defaultHeader is the header that the writer writes for cues of another format.
	defaultHeader = "\xEF\xBB\xBF[Script Info]\n" +
		"ScriptType: v4.00+\n" +
		"PlayResX: 384\n" +
		"PlayResY: 288\n" +
		"ScaledBorderAndShadow: yes\n" +
		"\n" +
		"[V4+ Styles]\n" +
		"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, " +
		"Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, " +
		"Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,16,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1\n" +
		"\n" +
		"[Events]\n" +
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"
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

	return ass.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ass.ReadOptions{})
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
	require.Equal(t, model.ASS, perr.Format)
	return perr
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := ass.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, ass.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

// event returns the first cue of an [Events] section with one event of text.
func event(t *testing.T, text string) model.Cue {
	t.Helper()

	s, _ := mustRead(t, eventsHeader+"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,"+text+"\n", false)
	require.Equal(t, 1, s.Len())
	c, _ := s.Cue(0)
	return c
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts ass.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, ass.Write(&out, s, common, opts))
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

func subtitle(t *testing.T, cues ...model.Cue) *model.Subtitle {
	t.Helper()

	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

func cueAt(t *testing.T, s *model.Subtitle, i int) model.Cue {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok, i)
	return c
}

func setCue(t *testing.T, s *model.Subtitle, i int, change func(*model.Cue)) {
	t.Helper()

	c := cueAt(t, s, i)
	change(&c)
	require.NoError(t, s.SetCue(i, c))
}

func fileData(t *testing.T, s *model.Subtitle) *model.Data {
	t.Helper()

	d, ok := s.FormatData("ass")
	require.True(t, ok)
	return d
}

func cueData(t *testing.T, c model.Cue) *model.Data {
	t.Helper()

	d, ok := c.FormatData("ass")
	require.True(t, ok)
	return d
}

func strs(t *testing.T, d *model.Data, key string) []string {
	t.Helper()

	v, ok := d.Strings(key)
	require.True(t, ok, key)
	return v
}

func object(t *testing.T, d *model.Data, key string) *model.Data {
	t.Helper()

	v, ok := d.Object(key)
	require.True(t, ok, key)
	return v
}

func str(t *testing.T, d *model.Data, key string) string {
	t.Helper()

	v, ok := d.String(key)
	require.True(t, ok, key)
	return v
}

var errRead = errors.New("disk gone")

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errRead }

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errRead }

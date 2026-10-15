package webvtt_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ms       = time.Millisecond
	sec      = time.Second
	vttDir   = "vtt"
	realDir  = "vtt/real"
	lenDir   = "lenient"
	noWebVTT = "The file does not start with WEBVTT."
	noEmpty  = "The WEBVTT header has no empty line before the first cue."
)

func fixture(t *testing.T, rel string) string {
	t.Helper()

	return string(testfiles.Read(t, rel))
}

func parse(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	return read(t, []byte(content), lenient)
}

func parseError(t *testing.T, content string, lenient bool) *model.ParseError {
	t.Helper()

	_, _, err := webvtt.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, model.WebVTT, perr.Format)
	return perr
}

func cueTexts(s *model.Subtitle) []string {
	out := []string{}
	for _, c := range s.All() {
		out = append(out, c.Text())
	}
	return out
}

func vttData(fields ...model.Field) *model.Data {
	return model.NewData(fields...)
}

func cueData(t *testing.T, c model.Cue) *model.Data {
	t.Helper()

	d, _ := c.FormatData("vtt")
	return d
}

func firstCue(t *testing.T, s *model.Subtitle) model.Cue {
	t.Helper()

	c, ok := s.Cue(0)
	require.True(t, ok)
	return c
}

func messages(warnings []model.ParseWarning) []string {
	out := []string{}
	for _, w := range warnings {
		out = append(out, w.Message)
	}
	return out
}

// Ports the error cases of tests/Parsers/WebVttParserTest.php and tests/Parsers/LenientWebVttTest.php of subtitle-toolbox-php.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		lenient bool
		msg     string
		line    int
	}{
		{name: "no empty line after the header", content: fixture(t, filepath.Join(vttDir, "no_empty_line_after_webvtt_header.vtt")), msg: noEmpty, line: 3},
		{name: "header line with an arrow", content: "WEBVTT\nKind: captions\n00:01.000 --> 00:02.000\nText\n", msg: noEmpty, line: 3},
		{name: "no signature", content: fixture(t, filepath.Join(vttDir, "missing_webvtt_header.vtt")), msg: noWebVTT, line: 1},
		{name: "minutes of 99", content: fixture(t, filepath.Join(vttDir, "exceeded_minutes.vtt")), msg: `The time "55:99:00.456" is not valid.`, line: 4},
		{name: "seconds of 99", content: fixture(t, filepath.Join(vttDir, "exceeded_seconds.vtt")), msg: `The time "34:00:99.123" is not valid.`, line: 4},
		{name: "4 fraction digits", content: fixture(t, filepath.Join(vttDir, "exceeded_milli_accuracy.vtt")), msg: `The time "34:00:00.1234" is not valid.`, line: 4},
		{name: "no timing line", content: fixture(t, filepath.Join(vttDir, "missing_timestamps.vtt")), msg: `Block #3 is not a WebVTT cue, comment, style or region. The line is "3".`, line: 11},
		{name: "a line of text", content: "WEBVTT\n\nstray line", msg: `Block #1 is not a WebVTT cue, comment, style or region. The line is "stray line".`, line: 3},
		{name: "no dot before the fraction", content: "WEBVTT\n\n00:00:01x000 --> 00:00:04.000\nText\n", msg: `The time "00:00:01x000" is not valid.`, line: 3},
		{name: "text before the arrow", content: "WEBVTT\n\n00:00:01.000 align:left --> 00:00:04.000\nText\n", msg: `The time "00:00:01.000 align:left" is not valid.`, line: 3},
		{name: "4th fraction digit is no setting", content: "WEBVTT\n\nid\n00:01.000 --> 00:01.0005\nText\n", msg: `The time "00:01.0005" is not valid.`, line: 4},
		{name: "long arrow", content: "WEBVTT\n\n00:01.000 ---> 00:02.000\nText\n", msg: `The time "00:01.000 -" is not valid.`, line: 3},
		{name: "short arrow", content: "WEBVTT\n\n00:01.000 -> 00:02.000\nText\n", msg: `Block #1 is not a WebVTT cue, comment, style or region. The line is "00:01.000 -> 00:02.000".`, line: 3},
		{name: "minutes of 60", content: "WEBVTT\n\n00:00:01.000 --> 00:60:02.000\nText\n", msg: `The time "00:60:02.000" is not valid.`, line: 3},
		{name: "100000 hours", content: "WEBVTT\n\n100000:00:00.000 --> 100000:00:01.000\nText\n", msg: `The time "100000:00:00.000" is not below 100000 hours.`, line: 3},
		{name: "hours beyond int64", content: "WEBVTT\n\n00:01.000 --> 99999999999999999999:00:00.000\nText\n", msg: `The time "99999999999999999999:00:00.000" is not below 100000 hours.`, line: 3},
		{name: "word timestamp of 100000 hours", content: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\nTwo <100000:00:00.000>three\n", msg: `The time "100000:00:00.000" is not below 100000 hours.`, line: 5},
		{name: "end before start", content: "WEBVTT\n\n00:05.000 --> 00:01.000\nText\n", msg: "The cue ends at 1 s, before it starts at 5 s.", line: 3},
		{name: "WEBVTTX", content: "WEBVTTX\n\n00:01.000 --> 00:02.000\nHi\n", msg: noWebVTT, line: 1},
		{name: "empty lines before a missing signature", content: "\n\nHello\n", msg: noWebVTT, line: 3},
		{name: "lenient SubRip timing line", content: "1\n00:00:01,000 --> 00:00:02,000\ntext\n", lenient: true, msg: noWebVTT, line: 1},
		{name: "lenient no timing line", content: "Hello\n\nWorld\n", lenient: true, msg: noWebVTT, line: 1},
		{name: "lenient timing line with a digit after the end", content: "\nHello\n00:00:01.000 --> 00:00:02.0001\n", lenient: true, msg: noWebVTT, line: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := parseError(t, tt.content, tt.lenient)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
		})
	}
}

func TestReadSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		header  *model.Data
		wantErr bool
	}{
		{name: "WEBVTT", content: "WEBVTT"},
		{name: "WEBVTT and a cue", content: "WEBVTT\n\n00:01.000 --> 00:02.000\nHi\n"},
		{name: "title", content: "WEBVTT Title\n\n00:01.000 --> 00:02.000\nHi\n", header: vttData(model.Field{Key: "header", Value: "Title"})},
		{name: "title after a tab", content: "WEBVTT\t- Title \n", header: vttData(model.Field{Key: "header", Value: "- Title"})},
		{name: "WEBVTTX", content: "WEBVTTX\n", wantErr: true},
		{name: "a BOM pair", content: "\ufeff\ufeffWEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nHi\r\n"},
		{name: "CR line ends", content: "WEBVTT\r\r00:01.000 --> 00:02.000\rHi\r"},
		{name: "CR CR LF line ends", content: "WEBVTT\r\r\n\r\r\n00:01.000 --> 00:02.000\r\r\nHi\r\r\n"},
		{name: "leading white space", content: " \n\t\nWEBVTT\n\n00:01.000 --> 00:02.000\nHi\n"},
		{name: "lowercase", content: "webvtt\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := webvtt.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, webvtt.ReadOptions{})
			if tt.wantErr {
				require.ErrorIs(t, err, model.ErrParse)
				return
			}
			require.NoError(t, err)
			s, warnings := parse(t, tt.content, false)
			assert.Empty(t, warnings)
			d, _ := s.FormatData("vtt")
			assert.Equal(t, tt.header, d)
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		content string
		want    bool
	}{
		{"WEBVTT", true},
		{"WEBVTT\n\n00:01.000 --> 00:02.000\nRain\n", true},
		{"WEBVTT - Weather report\n", true},
		{"WEBVTT\tTitle", true},
		{"WEBVTTX\n", false},
		{"WEBVT", false},
		{"", false},
		{"00:01.000 --> 00:02.000\nRain\n", false},
		{"\ufeffWEBVTT", false},
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, webvtt.Detect([]byte(tt.content)))
		})
	}
}

// Ports the time cases of tests/Parsers/WebVttParserTest.php and tests/Parsers/LooseTimestampsTest.php of subtitle-toolbox-php.
func TestReadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end string
		lenient    bool
		wantStart  time.Duration
		wantEnd    time.Duration
		wantErr    string
		warnings   []string
	}{
		{name: "hours", start: "01:02:03.004", end: "01:02:04.000", wantStart: 3723004 * ms, wantEnd: 3724 * sec},
		{name: "no hours", start: "02:03.004", end: "02:04.000", wantStart: 123004 * ms, wantEnd: 124 * sec},
		{name: "1 hour digit", start: "0:00:00.800", end: "0:00:02.933", wantStart: 800 * ms, wantEnd: 2933 * ms},
		{name: "4 hour digits", start: "0000:00:00.123", end: "1234:00:00.456", wantStart: 123 * ms, wantEnd: 4442400456 * ms},
		{name: "99999 hours", start: "00:00.000", end: "99999:59:59.999", wantEnd: 99999*time.Hour + 3599999*ms},
		{name: "100000 hours", start: "00:00.000", end: "100000:00:00.000", wantErr: `The time "100000:00:00.000" is not below 100000 hours.`},
		{name: "arrow without spaces", start: "00:01.000", end: "00:02.500", wantStart: sec, wantEnd: 2500 * ms},
		{name: "3 minute digits", start: "000:01.000", end: "00:02.000", wantErr: `The time "000:01.000" is not valid.`},
		{name: "1 minute digit", start: "0:01.000", end: "00:02.000", wantErr: `The time "0:01.000" is not valid.`},
		{name: "2 fraction digits strict", start: "00:00:01.25", end: "00:00:02.25", wantErr: `The time "00:00:02.25" is not valid.`},
		{name: "comma in the start strict", start: "00:00:01,500", end: "00:00:02.500 x", wantErr: `The time "00:00:01,500" is not valid.`},
		{
			name: "1 fraction digit lenient", start: "00:00:01.5", end: "00:00:02.5", lenient: true, wantStart: 1500 * ms, wantEnd: 2500 * ms,
			warnings: []string{
				`Block #1 has the time "00:00:01.5", which is not in the form hh:mm:ss.mmm. The parser read it as 1.5 s.`,
				`Block #1 has the time "00:00:02.5", which is not in the form hh:mm:ss.mmm. The parser read it as 2.5 s.`,
			},
		},
		{
			name: "2 fraction digits lenient", start: "00:00:01.25", end: "00:00:02.25", lenient: true, wantStart: 1250 * ms, wantEnd: 2250 * ms,
			warnings: []string{
				`Block #1 has the time "00:00:01.25", which is not in the form hh:mm:ss.mmm. The parser read it as 1.25 s.`,
				`Block #1 has the time "00:00:02.25", which is not in the form hh:mm:ss.mmm. The parser read it as 2.25 s.`,
			},
		},
		{
			name: "comma lenient", start: "00:00:01,500", end: "00:00:02,500", lenient: true, wantStart: 1500 * ms, wantEnd: 2500 * ms,
			warnings: []string{
				`Block #1 has the time "00:00:01,500", which is not in the form hh:mm:ss.mmm. The parser read it as 1.5 s.`,
				`Block #1 has the time "00:00:02,500", which is not in the form hh:mm:ss.mmm. The parser read it as 2.5 s.`,
			},
		},
		{
			name: "one warning per time string", start: "00:01,5", end: "00:01,5", lenient: true, wantStart: 1500 * ms, wantEnd: 1500 * ms,
			warnings: []string{`Block #1 has the time "00:01,5", which is not in the form hh:mm:ss.mmm. The parser read it as 1.5 s.`},
		},
		{
			name: "4th fraction digit lenient", start: "00:01.000", end: "00:01.0005", lenient: true, wantStart: sec, wantEnd: 1001 * ms,
			warnings: []string{`Block #1 has the time "00:01.0005", which is not in the form hh:mm:ss.mmm. The parser read it as 1.001 s.`},
		},
		{name: "cut-off time lenient", start: "00:00:01.000", end: "00:00:0", lenient: true, wantErr: `The time "00:00:0" is not valid.`},
		{name: "letter instead of the dot lenient", start: "00:00:01x000", end: "00:00:04.000", lenient: true, wantErr: `The time "00:00:01x000" is not valid.`},
		{name: "seconds of 75 lenient", start: "00:01.000", end: "00:75.000", lenient: true, wantErr: `The time "00:75.000" is not valid.`},
		{name: "minutes of 60 lenient", start: "00:00:01.000", end: "00:60:02,000", lenient: true, wantErr: `The time "00:60:02,000" is not valid.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "WEBVTT\n\n" + tt.start + " --> " + tt.end + "\nThe ferry leaves.\n\n02:00:00.000 --> 02:00:02.000\nIt is late.\n"
			if tt.wantErr != "" && !tt.lenient {
				perr := parseError(t, content, false)
				assert.Equal(t, tt.wantErr, perr.Msg)
				assert.Equal(t, 3, perr.Line)
				return
			}
			s, warnings := parse(t, content, tt.lenient)
			if tt.wantErr != "" {
				assert.Equal(t, []model.ParseWarning{{
					Message: tt.wantErr, Line: 3, Block: 1, Action: model.Skipped,
					Lines: []string{tt.start + " --> " + tt.end, "The ferry leaves."},
				}}, warnings)
				assert.Equal(t, []string{"It is late."}, cueTexts(s))
				return
			}
			c := firstCue(t, s)
			assert.Equal(t, tt.wantStart, c.Start)
			assert.Equal(t, tt.wantEnd, c.End)
			assert.Equal(t, 2, s.Len())
			assert.Equal(t, tt.warnings, nilIfEmpty(messages(warnings)))
			for _, w := range warnings {
				assert.Equal(t, model.Repaired, w.Action)
				assert.Equal(t, 3, w.Line)
				assert.Equal(t, 1, w.Block)
			}
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// Ports testCueSettingsKeepTheExactValues and testCueSettingsMapToAlignment of tests/Parsers/WebVttParserTest.php
// and testLenientModeKeepsTheSettingsAfterALooseWebVttEndTime of tests/Parsers/LooseTimestampsTest.php of subtitle-toolbox-php.
func TestReadCueSettings(t *testing.T) {
	t.Parallel()

	f := func(k string, v any) model.Field { return model.Field{Key: k, Value: v} }
	tests := []struct {
		name      string
		end       string
		lenient   bool
		alignment model.Alignment
		data      *model.Data
	}{
		{name: "no settings", end: "00:02.000", alignment: model.BottomCenter},
		{name: "line 0", end: "00:02.000 line:0", alignment: model.TopCenter, data: vttData(f("line", "0"), f("alignment", int64(8)))},
		{name: "line 0 percent left", end: "00:02.000 line:0% align:left", alignment: model.TopLeft, data: vttData(f("line", "0%"), f("align", "left"), f("alignment", int64(7)))},
		{name: "line 0 start right", end: "00:02.000 align:right line:0,start", alignment: model.TopRight, data: vttData(f("align", "right"), f("line", "0,start"), f("alignment", int64(9)))},
		{name: "middle", end: "00:02.000 line:50%,center", alignment: model.MiddleCenter, data: vttData(f("line", "50%,center"), f("alignment", int64(5)))},
		{name: "middle left", end: "00:02.000 line:50%,center align:left", alignment: model.MiddleLeft, data: vttData(f("line", "50%,center"), f("align", "left"), f("alignment", int64(4)))},
		{name: "bottom line", end: "00:02.000 line:-1", alignment: model.BottomCenter, data: vttData(f("line", "-1"), f("alignment", int64(2)))},
		{name: "bottom right", end: "00:02.000 line:100%,end align:right", alignment: model.BottomRight, data: vttData(f("line", "100%,end"), f("align", "right"), f("alignment", int64(3)))},
		{name: "align left", end: "00:02.000 align:left", alignment: model.BottomLeft, data: vttData(f("align", "left"), f("alignment", int64(1)))},
		{name: "align center", end: "00:02.000 align:center", alignment: model.BottomCenter, data: vttData(f("align", "center"), f("alignment", int64(2)))},
		{name: "region and align right", end: "00:02.000 region:bill align:right", alignment: model.BottomRight, data: vttData(f("region", "bill"), f("align", "right"), f("alignment", int64(3)))},
		{name: "align start is unclear", end: "00:02.000 align:start", alignment: model.BottomCenter, data: vttData(f("align", "start"))},
		{name: "align middle is unclear", end: "00:02.000 align:middle line:0", alignment: model.BottomCenter, data: vttData(f("align", "middle"), f("line", "0"))},
		{name: "line 85 percent is unclear", end: "00:02.000 line:85%", alignment: model.BottomCenter, data: vttData(f("line", "85%"))},
		{name: "vertical text is unclear", end: "00:02.000 vertical:rl line:0", alignment: model.BottomCenter, data: vttData(f("vertical", "rl"), f("line", "0"))},
		{name: "only position", end: "00:02.000 position:10%", alignment: model.BottomCenter, data: vttData(f("position", "10%"))},
		{
			name: "exact values, unknown names and bad tokens", end: "00:02.000 \tvertical:rl  size:50.5%  foo:bar position:10%,line-left noColon :x y:",
			alignment: model.BottomCenter, data: vttData(f("vertical", "rl"), f("size", "50.5%"), f("position", "10%,line-left")),
		},
		{name: "the last token wins and keeps its place", end: "00:02.000 line:0 size:10% line:-1", alignment: model.BottomCenter, data: vttData(f("line", "-1"), f("size", "10%"), f("alignment", int64(2)))},
		{name: "a value with a colon", end: "00:02.000 position:a:b", alignment: model.BottomCenter, data: vttData(f("position", "a:b"))},
		{name: "no space after the end", end: "00:02.000line:40%", alignment: model.BottomCenter, data: vttData(f("line", "40%"))},
		{name: "settings after a loose end", end: "00:00:02.25 align:start line:0", lenient: true, alignment: model.BottomCenter, data: vttData(f("align", "start"), f("line", "0"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := parse(t, "WEBVTT\n\n00:01.000 --> "+tt.end+"\nOne\n", tt.lenient)
			c := firstCue(t, s)
			assert.Equal(t, tt.alignment, c.Alignment)
			assert.Equal(t, tt.data, cueData(t, c))
		})
	}
}

// Ports the block cases of tests/Parsers/WebVttParserTest.php of subtitle-toolbox-php.
func TestReadBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		lenient  bool
		texts    []string
		ids      []string
		comments []model.Comment
		file     *model.Data
		warnings []model.ParseWarning
	}{
		{
			name:    "several empty lines between cues",
			content: "WEBVTT\n\n\n00:01.000 --> 00:02.000\nFirst\n\n \n\n00:03.000 --> 00:04.000\nSecond\n",
			texts:   []string{"First", "Second"}, ids: []string{"", ""},
		},
		{
			name:    "a timing line without an empty line starts a cue",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n00:03.000 --> 00:04.000\nTwo\n",
			texts:   []string{"One", "Two"}, ids: []string{"", ""},
		},
		{
			name:    "NOTE-1 is an identifier",
			content: "WEBVTT\n\nNOTE-1\n00:01.000 --> 00:02.000\nOne\n\nNotes1\n00:03.000 --> 00:04.000\nTwo\n",
			texts:   []string{"One", "Two"}, ids: []string{"NOTE-1", "Notes1"},
		},
		{
			name:    "lowercase note and style blocks are skipped",
			content: "WEBVTT\n\nnote a comment\n\nstyle\n::cue {}\n\n00:01.000 --> 00:02.000\nText\n",
			texts:   []string{"Text"}, ids: []string{""},
		},
		{
			name:    "STYLES is no style block",
			content: "WEBVTT\n\nSTYLES\n::cue {}\n\n00:01.000 --> 00:02.000\nOne\n",
			texts:   []string{"One"}, ids: []string{""},
		},
		{
			name:    "STYLE and REGION after a cue strict",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\nSTYLE\n::cue { color: lime }\n\nREGION\nid:late\n",
			texts:   []string{"One"}, ids: []string{""},
		},
		{
			name:    "STYLE and REGION after a cue lenient",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\nSTYLE\n::cue { color: lime }\n\nREGION\nid:late\n",
			lenient: true,
			texts:   []string{"One"}, ids: []string{""},
			warnings: []model.ParseWarning{
				{Message: "Block #2 is a STYLE block after a cue. The spec allows it only before the first cue, so the parser skipped it.", Line: 6, Block: 2, Lines: []string{"STYLE", "::cue { color: lime }"}, Action: model.Skipped},
				{Message: "Block #3 is a REGION block after a cue. The spec allows it only before the first cue, so the parser skipped it.", Line: 9, Block: 3, Lines: []string{"REGION", "id:late"}, Action: model.Skipped},
			},
		},
		{
			name:    "a STYLE block after a broken cue counts as before the first cue",
			content: "WEBVTT\n\n00:01.000 --> 00:0x.000\nOne\n\nSTYLE\n::cue {}\n\n00:03.000 --> 00:04.000\nTwo\n",
			lenient: true,
			texts:   []string{"Two"}, ids: []string{""},
			file: vttData(model.Field{Key: "styles", Value: []any{"::cue {}"}}),
			warnings: []model.ParseWarning{
				{Message: `The time "00:0x.000" is not valid.`, Line: 3, Block: 1, Lines: []string{"00:01.000 --> 00:0x.000", "One"}, Action: model.Skipped},
			},
		},
		{
			name:    "the exact CSS of a STYLE block",
			content: "WEBVTT\n\nSTYLE\n::cue {\n\tcolor: lime;\n}\n\n00:01.000 --> 00:02.000\nOne\n",
			texts:   []string{"One"}, ids: []string{""},
			file: vttData(model.Field{Key: "styles", Value: []any{"::cue {\n\tcolor: lime;\n}"}}),
		},
		{
			name:    "region settings without a value",
			content: "WEBVTT\n\nREGION\nid:fred width: :3 scroll:up foo:bar\n\n00:01.000 --> 00:02.000\nOne\n",
			texts:   []string{"One"}, ids: []string{""},
			file: vttData(model.Field{Key: "regions", Value: []any{vttData(model.Field{Key: "id", Value: "fred"}, model.Field{Key: "scroll", Value: "up"})}}),
		},
		{
			name: "header, region, note, identifier and settings",
			content: "WEBVTT Episode 1\nKind: captions\nLanguage: en\n\n" +
				"REGION\nid:fred\nwidth:40%\nlines:3\nregionanchor:0%,100%\nviewportanchor:10%,90%\nscroll:up\n\n" +
				"NOTE Translated by Jane Doe\n\n" +
				"intro\n00:00:01.000 --> 00:00:04.000 region:fred align:left line:85%\n" +
				"<v Fred>Hi, I am Fred &amp; this is <c.yellow>Bob</c></v>\n",
			texts:    []string{"<v Fred>Hi, I am Fred &amp; this is <c.yellow>Bob</c></v>"},
			ids:      []string{"intro"},
			comments: []model.Comment{{Text: "Translated by Jane Doe", BeforeCue: 0}},
			file: vttData(
				model.Field{Key: "header", Value: "Episode 1"},
				model.Field{Key: "headerLines", Value: []any{"Kind: captions", "Language: en"}},
				model.Field{Key: "regions", Value: []any{vttData(
					model.Field{Key: "id", Value: "fred"}, model.Field{Key: "width", Value: "40%"}, model.Field{Key: "lines", Value: "3"},
					model.Field{Key: "regionanchor", Value: "0%,100%"}, model.Field{Key: "viewportanchor", Value: "10%,90%"},
					model.Field{Key: "scroll", Value: "up"},
				)}},
			),
		},
		{
			name: "comments keep their place between cues",
			content: "WEBVTT\n\nNOTE\nfirst line\nsecond line\n\n00:01.000 --> 00:02.000\nOne\n\n" +
				"NOTE\tbetween\n\n00:03.000 --> 00:04.000\nTwo\n\nNOTE end of file\n\nNOTE\n",
			texts: []string{"One", "Two"}, ids: []string{"", ""},
			comments: []model.Comment{
				{Text: "first line\nsecond line", BeforeCue: 0}, {Text: "between", BeforeCue: 1},
				{Text: "end of file", BeforeCue: 2}, {Text: "", BeforeCue: 2},
			},
		},
		{
			name:    "a cue without text",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000",
			texts:   []string{""}, ids: []string{""},
		},
		{
			name:    "a line of white space inside a cue",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\n \t \nThe  ferry\t\tleaves.\n",
			texts:   []string{"The ferry leaves."}, ids: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := parse(t, tt.content, tt.lenient)
			assert.Equal(t, tt.texts, cueTexts(s))
			var ids []string
			for _, c := range s.All() {
				ids = append(ids, c.ID)
			}
			assert.Equal(t, tt.ids, ids)
			assert.Equal(t, tt.comments, nilIfNoComments(s.Comments()))
			d, _ := s.FormatData("vtt")
			assert.Equal(t, tt.file, d)
			assert.Equal(t, tt.warnings, nilIfNoWarnings(warnings))
		})
	}
}

func nilIfNoComments(c []model.Comment) []model.Comment {
	if len(c) == 0 {
		return nil
	}
	return c
}

func nilIfNoWarnings(w []model.ParseWarning) []model.ParseWarning {
	if len(w) == 0 {
		return nil
	}
	return w
}

// Ports the text cases of tests/Parsers/WebVttParserTest.php of subtitle-toolbox-php.
func TestReadText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines string
		want  []string
	}{
		{name: "entities for spaces and direction marks", lines: "A&nbsp;B &lrm;C&rlm; &amp; &lt;D&gt;", want: []string{"A\u00a0B \u200eC\u200f &amp; &lt;D&gt;"}},
		{name: "Rye & spelt", lines: "Rye & spelt", want: []string{"Rye &amp; spelt"}},
		{name: "unknown entity", lines: "&foo;", want: []string{"&amp;foo;"}},
		{name: "named entity", lines: "&eacute;", want: []string{"&eacute;"}},
		{name: "numeric entity", lines: "&#233; &#xE9;", want: []string{"&#233; &#xE9;"}},
		{name: "numeric entity that HTML5 does not allow", lines: "&#13;", want: []string{"&amp;#13;"}},
		{name: "entity without semicolon", lines: "&amp x", want: []string{"&amp;amp x"}},
		{
			name:  "ampersands between tags",
			lines: "<b>Rye</b> & <b>spelt</b> &foo; &#233; &eacute; &amp;",
			want:  []string{"<b>Rye</b> &amp; <b>spelt</b> &amp;foo; &#233; &eacute; &amp;"},
		},
		{
			name:  "unknown tag without its end tag",
			lines: "<b>Rye</b> &amp; <b>spelt</b> <today>\n<c>open<foo>a</foo> <lang en>x</lang>",
			want:  []string{"<b>Rye</b> &amp; <b>spelt</b> &lt;today&gt;", "<c>open<foo>a</foo> <lang en>x</lang>"},
		},
		{name: "raw > and < before white space stay", lines: "a < b > c", want: []string{"a < b > c"}},
		{name: "a quote in a voice name", lines: "<v O'Neil>Hi</v>", want: []string{"<v O'Neil>Hi</v>"}},
		{
			name:  "span tags stay as written",
			lines: "<v.loud Fred>Hi <c.yellow.bg_blue>you</c> <lang en>x</lang> <ruby>a<rt>b</rt></ruby> <b.loud>c</b>",
			want:  []string{"<v.loud Fred>Hi <c.yellow.bg_blue>you</c> <lang en>x</lang> <ruby>a<rt>b</rt></ruby> <b.loud>c</b>"},
		},
		{name: "word timestamps stay", lines: "One <00:00:02.500>two", want: []string{"One <00:00:02.500>two"}},
		{name: "short inline timestamps get hours", lines: "One <00:00:02.500>two <00:03.000>three", want: []string{"One <00:00:02.500>two <00:00:03.000>three"}},
		{name: "a 1-digit hour stays", lines: "<1:00:03.000>x", want: []string{"<1:00:03.000>x"}},
		{name: "NUL becomes U+FFFD", lines: "A\x00B", want: []string{"A\ufffdB"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, lenient := range []bool{false, true} {
				s, _ := parse(t, "WEBVTT\n\n00:01.000 --> 00:04.000\n"+tt.lines+"\n", lenient)
				assert.Equal(t, tt.want, firstCue(t, s).Lines)
			}
		})
	}
}

// Ports the decision of https://github.com/yama6a/subtitle-toolbox-go/issues/21 on short inline timestamps.
func TestShortInlineTimestampMovesWithItsCue(t *testing.T) {
	t.Parallel()

	s, _ := parse(t, "WEBVTT\n\n00:00.000 --> 00:04.000\n<00:03.000>Hi\n", false)
	assert.Equal(t, []string{"<00:00:03.000>Hi"}, firstCue(t, s).Lines)
	require.NoError(t, s.Shift(10*time.Second))
	assert.Equal(t, []string{"<00:00:13.000>Hi"}, firstCue(t, s).Lines)
}

func TestReadNoteBeforeAnEarlierCue(t *testing.T) {
	t.Parallel()

	s := readFile(t, filepath.Join(realDir, "own_note_before_earlier_cue.vtt"))
	assert.Equal(t, []string{"The door opens.", "The shop is open.", "The shop closes."}, cueTexts(s))
	assert.Equal(t, []model.Comment{
		{Text: "The door opens before the shop.", BeforeCue: 0},
		{Text: "The shop opens at nine.", BeforeCue: 1},
		{Text: "End of the shop scene.", BeforeCue: 3},
	}, s.Comments())
}

func TestReadBlankContent(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", " \n\t\r\n", "\ufeff"} {
		s, warnings := parse(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

func TestReadReaderError(t *testing.T) {
	t.Parallel()

	_, _, err := webvtt.Read(errReader{}, model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.ErrorIs(t, err, os.ErrClosed)
}

func TestReadNULInStrictMode(t *testing.T) {
	t.Parallel()

	s, warnings, err := webvtt.Read(bytes.NewReader([]byte("WEBVTT\n\nNOTE a\x00b\n")), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, "a\ufffdb", s.Comments()[0].Text)
}

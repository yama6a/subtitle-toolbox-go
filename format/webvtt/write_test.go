package webvtt_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const bom = "\ufeff"

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, webvtt.Write(&out, s, common, webvtt.WriteOptions{}))
	return out.String()
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

func withData(t *testing.T, c model.Cue, fields ...model.Field) model.Cue {
	t.Helper()

	require.NoError(t, c.SetFormatData("vtt", model.NewData(fields...)))
	return c
}

// Ports the fixture tests of tests/Formatters/WebVttFormatterTest.php and tests/Parsers/WebVttParserTest.php of subtitle-toolbox-php.
func TestWriteFixtures(t *testing.T) {
	t.Parallel()

	valid := fixture(t, filepath.Join(vttDir, "valid.vtt"))
	tests := []struct {
		name   string
		in     string
		common model.CommonWriteOptions
		want   string
	}{
		{name: "valid", in: "valid.vtt", want: valid},
		{name: "styles and notes", in: "with_styles_and_notes.vtt", want: fixture(t, filepath.Join(vttDir, "with_styles_and_notes_formatted.vtt"))},
		{name: "missing hours", in: "missing_hours.vtt", want: strings.Replace(valid, "WEBVTT\n", "WEBVTT - some title\n", 1)},
		{name: "missing cue number", in: "missing_cue_number.vtt", want: strings.Replace(valid, "WEBVTT\n", "WEBVTT - some title\n", 1)},
		{name: "unsupported tags go", in: "strip_xml.vtt", want: valid},
		{name: "StripTags", in: "strip_xml.vtt", common: model.CommonWriteOptions{StripTags: true}, want: fixture(t, filepath.Join(vttDir, "all_xml_tags_stripped.vtt"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, filepath.Join(vttDir, tt.in))
			assert.Equal(t, tt.want, write(t, s, tt.common))
		})
	}
}

// Ports testHeaderRegionAndNoteRoundTripByteForByte of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteHeaderRegionStyleAndNoteRoundTrip(t *testing.T) {
	t.Parallel()

	const content = "WEBVTT Episode 1\nKind: captions\nLanguage: en\n\n" +
		"REGION\nid:fred\nwidth:40%\nlines:3\nregionanchor:0%,100%\nviewportanchor:10%,90%\nscroll:up\n\n" +
		"STYLE\n::cue {\n  color: lime;\n}\n\n" +
		"NOTE Translated by Jane Doe\n\n" +
		"intro\n00:00:01.000 --> 00:00:04.000 region:fred align:left line:85%\n" +
		"<v Fred>Hi, I am Fred &amp; this is <c>Bob</c></v>\n\n" +
		"2\n00:00:05.000 --> 00:00:06.000\nSecond\n\n" +
		"NOTE\nend of\nfile\n"
	s, _ := parse(t, content, false)
	assert.Equal(t, bom+content, write(t, s, model.CommonWriteOptions{}))
}

func TestWriteDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        func(t *testing.T) *model.Subtitle
		common   model.CommonWriteOptions
		want     string
		comments []model.Comment
	}{
		{
			name: "empty subtitle",
			s:    func(*testing.T) *model.Subtitle { return &model.Subtitle{} },
			want: bom + "WEBVTT\n\n",
		},
		{
			name: "cue numbers for cues without a usable identifier",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				one := cue(sec, 2*sec, "One")
				one.ID = "intro"
				three := cue(5*sec, 6*sec, "Three")
				three.ID = "bad --> identifier"
				four := cue(7*sec, 8*sec, "Four")
				four.ID = " \t"
				five := cue(9*sec, 10*sec, "Five")
				five.ID = "a\nb"
				six := cue(11*sec, 12*sec, "Six")
				six.ID = "a\rb"
				return subtitle(t, one, cue(3*sec, 4*sec, "Two"), three, four, five, six)
			},
			want: bom + "WEBVTT\n\nintro\n00:00:01.000 --> 00:00:02.000\nOne\n\n" +
				"2\n00:00:03.000 --> 00:00:04.000\nTwo\n\n" +
				"3\n00:00:05.000 --> 00:00:06.000\nThree\n\n" +
				"4\n00:00:07.000 --> 00:00:08.000\nFour\n\n" +
				"5\n00:00:09.000 --> 00:00:10.000\nFive\n\n" +
				"6\n00:00:11.000 --> 00:00:12.000\nSix\n",
		},
		{
			name: "a cue without text",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				return subtitle(t, cue(sec, 2*sec))
			},
			want: bom + "WEBVTT\n\n1\n00:00:01.000 --> 00:00:02.000\n",
		},
		{
			name: "a negative start and hours of 3 digits",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				return subtitle(t, cue(-sec, 100*time.Hour, "x"))
			},
			want: bom + "WEBVTT\n\n1\n00:00:00.000 --> 100:00:00.000\nx\n",
		},
		{
			name: "CR LF and no BOM",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				return subtitle(t, cue(sec, 2*sec, "a", "b"))
			},
			common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove},
			want:   "WEBVTT\r\n\r\n1\r\n00:00:01.000 --> 00:00:02.000\r\na\r\nb\r\n",
		},
		{
			name: "comments past the last cue go after it",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				s := subtitle(t, cue(sec, 2*sec, "One"))
				require.NoError(t, s.AddComment("first", 0))
				require.NoError(t, s.AddComment("far", 7))
				require.NoError(t, s.AddComment("end", 1))
				return s
			},
			want: bom + "WEBVTT\n\nNOTE first\n\n1\n00:00:01.000 --> 00:00:02.000\nOne\n\nNOTE end\n\nNOTE far\n",
		},
		{
			name: "file data: header lines, regions in stored order, then styles",
			s: func(t *testing.T) *model.Subtitle {
				t.Helper()

				s := &model.Subtitle{}
				require.NoError(t, s.SetFormatData("vtt", model.NewData(
					model.Field{Key: "styles", Value: []any{"::cue { color: red }"}},
					model.Field{Key: "headerLines", Value: []any{"Kind: captions"}},
					model.Field{Key: "regions", Value: []any{
						model.NewData(model.Field{Key: "width", Value: "40%"}, model.Field{Key: "id", Value: "a"}),
						[]any{"x", "y"},
					}},
					model.Field{Key: "header", Value: ""},
				)))
				require.NoError(t, s.AddComment("note", 0))
				return s
			},
			want: bom + "WEBVTT\nKind: captions\n\nREGION\nwidth:40%\nid:a\n\nREGION\n0:x\n1:y\n\nSTYLE\n::cue { color: red }\n\nNOTE note\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s(t), tt.common))
		})
	}
}

// Ports testCommentsAreWrittenAsValidNoteBlocks of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "empty", text: "", want: "NOTE"},
		{name: "blank", text: " \t\n\n", want: "NOTE"},
		{name: "one line", text: "checked", want: "NOTE checked"},
		{name: "an arrow", text: "a --> b", want: "NOTE a -> b"},
		{name: "several lines", text: "end of\nfile", want: "NOTE\nend of\nfile"},
		{name: "spaces, tabs and empty lines", text: "  a \t b\r\n\r\n\rc  ", want: "NOTE\na b\nc"},
		{name: "an arrow that a single replace would leave", text: "a ---> b ----> c", want: "NOTE a -> b -> c"},
		{name: "a line of white space", text: "a\n \t \nb ", want: "NOTE\na\nb"},
		{name: "white space at the line ends", text: "a \n b", want: "NOTE\na\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(t, cue(sec, 2*sec, "One"))
			require.NoError(t, s.AddComment(tt.text, 1))
			out := write(t, s, model.CommonWriteOptions{})
			assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:01.000 --> 00:00:02.000\nOne\n\n"+tt.want+"\n", out)
			again, _ := parse(t, out, false)
			assert.Len(t, again.Comments(), 1)
		})
	}
}

// Ports testAlignmentBecomesCueSettings, testCueSettingsFromFormatDataStayWhileTheAlignmentIsUnchanged,
// testBottomCenterAlignmentDropsTheLineSetting and testSubRipCueSettingsStayWhileTheAlignmentIsUnchanged
// of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteSettings(t *testing.T) {
	t.Parallel()

	f := func(k string, v any) model.Field { return model.Field{Key: k, Value: v} }
	tests := []struct {
		name      string
		alignment model.Alignment
		data      []model.Field
		want      string
	}{
		{name: "bottom left", alignment: model.BottomLeft, want: " align:left"},
		{name: "bottom center", alignment: model.BottomCenter, want: ""},
		{name: "bottom right", alignment: model.BottomRight, want: " align:right"},
		{name: "middle left", alignment: model.MiddleLeft, want: " line:50%,center align:left"},
		{name: "middle center", alignment: model.MiddleCenter, want: " line:50%,center"},
		{name: "middle right", alignment: model.MiddleRight, want: " line:50%,center align:right"},
		{name: "top left", alignment: model.TopLeft, want: " line:0 align:left"},
		{name: "top center", alignment: model.TopCenter, want: " line:0"},
		{name: "top right", alignment: model.TopRight, want: " line:0 align:right"},
		{
			name: "unchanged settings stay in stored order, other names go", alignment: model.BottomCenter,
			data: []model.Field{f("size", "50%"), f("unknown", "x"), f("align", "start")}, want: " size:50% align:start",
		},
		{
			name: "unchanged top left keeps its exact settings", alignment: model.TopLeft,
			data: []model.Field{f("line", "0"), f("position", "30%"), f("align", "left"), f("alignment", int64(7))}, want: " line:0 position:30% align:left",
		},
		{
			name: "changed to bottom center keeps vertical, size and region", alignment: model.BottomCenter,
			data: []model.Field{f("region", "fred"), f("line", "0"), f("size", "35%"), f("vertical", "rl"), f("alignment", int64(8))}, want: " region:fred size:35% vertical:rl",
		},
		{
			name: "changed to top right adds the alignment after the kept settings", alignment: model.TopRight,
			data: []model.Field{f("position", "90%"), f("align", "right"), f("size", "35%"), f("region", "r")}, want: " size:35% region:r line:0 align:right",
		},
		{
			name: "SubRip settings stay while the alignment is unchanged", alignment: model.TopCenter,
			data: []model.Field{f("position", "10%"), f("alignment", int64(8))}, want: " position:10%",
		},
		{
			name: "SubRip settings go after a change", alignment: model.TopLeft,
			data: []model.Field{f("position", "10%"), f("alignment", int64(8))}, want: " line:0 align:left",
		},
		{
			name: "an alignment that is not an integer counts as missing", alignment: model.BottomCenter,
			data: []model.Field{f("line", "0"), f("alignment", "8")}, want: " line:0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := cue(sec, 2*sec, "One")
			c.Alignment = tt.alignment
			if tt.data != nil {
				c = withData(t, c, tt.data...)
			}
			assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:01.000 --> 00:00:02.000"+tt.want+"\nOne\n", write(t, subtitle(t, c), model.CommonWriteOptions{}))
		})
	}
}

// Ports testAlignmentRoundTripsThroughCueSettings of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteAlignmentRoundTrip(t *testing.T) {
	t.Parallel()

	var cues []model.Cue
	for n := 1; n <= 9; n++ {
		a, ok := model.AlignmentFromNumpad(n)
		require.True(t, ok)
		c := cue(time.Duration(n)*sec, time.Duration(n+1)*sec, "Cue")
		c.Alignment = a
		cues = append(cues, c)
	}
	s, _ := parse(t, write(t, subtitle(t, cues...), model.CommonWriteOptions{}), false)
	for i, c := range s.All() {
		assert.Equal(t, i+1, c.Alignment.Numpad())
	}
}

// Ports testChangedAlignmentReplacesTheStoredPosition of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteChangedAlignmentReplacesTheStoredPosition(t *testing.T) {
	t.Parallel()

	path := filepath.Join(realDir, "w3c_cue_settings.vtt")
	original := write(t, readFile(t, path), model.CommonWriteOptions{})
	s := readFile(t, path)
	for i, a := range []model.Alignment{model.TopCenter, model.BottomCenter} {
		c, _ := s.Cue(i)
		c.Alignment = a
		require.NoError(t, s.SetCue(i, c))
	}
	want := strings.NewReplacer(
		" position:10%,line-left align:left size:35%", " size:35% line:0",
		" position:90% align:right size:35%", " size:35%",
	).Replace(original)
	assert.Equal(t, want, write(t, s, model.CommonWriteOptions{}))
}

// Ports the tag tests of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		strip bool
		want  string
	}{
		{name: "inline timestamps stay", text: "<i>One</i> <00:00:02.500>two <00:03.000><foo>three</foo>", want: "<i>One</i> <00:00:02.500>two <00:03.000>three"},
		{name: "StripTags removes inline timestamps", text: "<i>One</i> <00:00:02.500>two <00:03.000><foo>three</foo>", strip: true, want: "One two three"},
		{name: "unknown tag with a class", text: "<foo.bar>hello</foo> world", want: "hello world"},
		{name: "a longer name than a span tag", text: "<bold.x>hello</bold> <cite.y>world</cite>", want: "hello world"},
		{name: "core tags without classes", text: `<font face="Arial">plain</font> <s>gone</s> <b>bold</b> <strong>x</strong>`, want: "plain gone <b>bold</b> <strong>x</strong>"},
		{name: "font color with a class", text: `<font color="#ff0000">red</font> <s>gone</s>`, want: "<c.red>red</c> gone"},
		{name: "an arrow in the text", text: "a --> b", want: "a --&gt; b"},
		{name: "an arrow when tags are stripped", text: "<i>a --> b</i>", strip: true, want: "a --&gt; b"},
		{name: "entities stay encoded", text: "Fish &amp; Chips &lt;3", want: "Fish &amp; Chips &lt;3"},
		{name: "ruby stays ruby", text: "<ruby>base<rt>kan</rt></ruby>", want: "<ruby>base<rt>kan</rt></ruby>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := write(t, subtitle(t, cue(sec, 4*sec, tt.text)), model.CommonWriteOptions{StripTags: tt.strip})
			assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:01.000 --> 00:00:04.000\n"+tt.want+"\n", got)
		})
	}
}

// Ports testSpansWithClassesAreKept of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteKeepsSpansWithClasses(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"<c.colorE5E5E5>hello</c> world",
		"<i.quiet>hello</i> world",
		"<b.loud>hello</b> world",
		"<u.link>hello</u> world",
		"<ruby.small>base <rt.top>text</rt></ruby> world",
		"<v.first.loud Fred>hello</v> world",
		"<lang.x en>hello</lang> world",
		"<c.yellow.bg_blue><b.loud>hello</b></c> world",
		"the<00:00:01.199><c> train</c><c.colorE5E5E5><00:00:01.379><c> leaves</c></c>",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			s, _ := parse(t, "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n"+text+"\n", false)
			assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:01.000 --> 00:00:02.000\n"+text+"\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

// Ports testTimingArrowInCueTextIsEscaped of tests/Formatters/WebVttFormatterTest.php of subtitle-toolbox-php.
func TestWriteArrowInText(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(sec, 2*sec, "a --> b", "", "c"))
	out := write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove})
	assert.Equal(t, fixture(t, filepath.Join(realDir, "own_arrow_in_text.vtt")), out)
	again, _ := parse(t, out, false)
	assert.Equal(t, []string{"a --&gt; b", "c"}, firstCue(t, again).Lines)
}

func TestWriteWithReport(t *testing.T) {
	t.Parallel()

	s := subtitle(t,
		cue(sec, 2*sec, `<font color="#FFFF00">yellow</font> <font color="#123456">dark</font>`),
		cue(4*sec, 5*sec, `<font color="rgb(1,2,3)">a</font> <font color="">b</font> <font color="white">c</font>`),
		cue(5500*ms, 6*sec, "plain"),
	)
	var out bytes.Buffer
	report, err := webvtt.WriteWithReport(&out, s, model.CommonWriteOptions{}, webvtt.WriteOptions{})
	require.NoError(t, err)
	assert.Equal(t, webvtt.WriteReport{DroppedColors: []webvtt.DroppedColor{
		{CueIndex: 0, Color: "#123456", Message: `Cue #0 at 1 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.`},
		{CueIndex: 1, Color: "rgb(1,2,3)", Message: `Cue #1 at 4 s: dropped the color "rgb(1,2,3)", because WebVTT has classes for 8 colors only.`},
	}}, report)
	assert.Contains(t, out.String(), "\n<c.yellow>yellow</c> dark\n")
	assert.Contains(t, out.String(), "\na b <c.white>c</c>\n")

	out.Reset()
	report, err = webvtt.WriteWithReport(&out, s, model.CommonWriteOptions{StripTags: true}, webvtt.WriteOptions{})
	require.NoError(t, err)
	assert.Empty(t, report.DroppedColors)
}

type failingWriter struct{}

var errDiskFull = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

func TestWriteError(t *testing.T) {
	t.Parallel()

	err := webvtt.Write(failingWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, webvtt.WriteOptions{})
	require.ErrorIs(t, err, errDiskFull)
	_, err = webvtt.WriteWithReport(failingWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, webvtt.WriteOptions{})
	require.ErrorIs(t, err, errDiskFull)
}

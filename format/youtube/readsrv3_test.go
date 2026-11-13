package youtube_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testSrv3WindowPositionsAndPens of tests/Parsers/YouTubeTimedTextParserTest.php of subtitle-toolbox-php.
func TestReadSrv3PensAndWindowPositions(t *testing.T) {
	t.Parallel()

	content := `<?xml version="1.0" encoding="utf-8" ?><timedtext format="3"><head>` +
		`<pen id="1" fc="#FF0000" b="1" et="3"/><pen id="2" i="1" u="1"/>` +
		`<wp id="0"/><wp id="1" ap="0" ah="0" av="0"/><wp id="2" ap="5" ah="100" av="50"/><wp id="3" ah="50" av="50"/>` +
		`</head><body>` +
		`<p t="0" d="1000" wp="1" p="1">red</p>` +
		`<p t="1000" d="1000" wp="2"><s p="2">styled</s> plain</p>` +
		`<p t="2000" d="1000" wp="3">no anchor</p>` +
		`</body></timedtext>`
	s := read(t, content, youtube.ReadOptions{})
	want := []struct {
		alignment model.Alignment
		line      string
	}{
		{model.TopLeft, `<font color="#ff0000"><b>red</b></font>`},
		{model.MiddleRight, "<i><u>styled</u></i> plain"},
		{model.BottomCenter, "no anchor"},
	}
	for i, c := range s.Cues() {
		assert.Equal(t, want[i].alignment, c.Position.Alignment)
		assert.Equal(t, []string{want[i].line}, c.Lines)
	}
	c, _ := s.Cue(1)
	data, ok := c.FormatData("youtube")
	require.True(t, ok)
	segments, _ := data.List("segments")
	span, _ := segments[0].(*model.Data)
	assert.Equal(t, []string{"p", "text"}, span.Keys())
	file, _ := s.FormatData("youtube")
	pens, _ := file.List("pen")
	pen, _ := pens[0].(*model.Data)
	assert.Equal(t, []string{"id", "fc", "b", "et"}, pen.Keys())
}

// Ports testAppendEventsAndDoubleEscapedTextStayInTheFormatData.
func TestReadSrv3AppendsAndDoubleEscapedText(t *testing.T) {
	t.Parallel()

	content := `<?xml version="1.0" encoding="utf-8" ?><timedtext format="3"><body>` +
		`<p t="0" d="3000" w="1">one</p><p t="900" d="2100" w="1" a="1">` + "\n" + `</p>` +
		`<p t="1000" d="3000" w="1">it&amp;#39;s two</p><p t="5000" w="1" a="1">` + "\n" + `</p>` +
		`</body></timedtext>`
	s := read(t, content, youtube.ReadOptions{})
	assert.Equal(t, [][2]int{{0, 1000}, {1000, 4000}}, times(s))
	c, _ := s.Cue(1)
	assert.Equal(t, []string{"it's two"}, c.Lines)
	data, _ := c.FormatData("youtube")
	appends, _ := data.List("appends")
	require.Len(t, appends, 1)
	assert.Equal(t, []string{"t", "d", "w", "a", "text"}, appends[0].(*model.Data).Keys())
	file, _ := s.FormatData("youtube")
	trailing, _ := file.List("appends")
	assert.Len(t, trailing, 1)
}

func TestReadSrv3WordTimes(t *testing.T) {
	t.Parallel()

	content := `<timedtext format="3"><body><p t="1200" d="2300">Hello<s t="400"> world</s></p></body></timedtext>`
	tests := []struct {
		name string
		opts youtube.ReadOptions
		want []model.Word
	}{
		{"without the option", youtube.ReadOptions{}, nil},
		{"with the option", withWords, []model.Word{{Text: "Hello", Start: ms(1200), End: ms(1600)}, {Text: "world", Start: ms(1600), End: ms(3500)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := read(t, content, tt.opts).Cue(0)
			assert.Equal(t, []string{"Hello world"}, c.Lines)
			assert.Equal(t, tt.want, c.Words)
		})
	}
}

func TestReadSrv3Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, want string
		line                int
	}{
		{"no t", `<timedtext format="3"><body><p d="1">a</p></body></timedtext>`, `The <p> element has no valid "t" attribute.`, 1},
		{"a bad t on line 3", "<timedtext format=\"3\"><body>\n<p t=\"0\" d=\"1\">a</p>\n<p t=\"soon\">b</p>\n</body></timedtext>", `The <p> element has no valid "t" attribute.`, 3},
		{"a bad span offset", `<timedtext format="3"><body><p t="0"><s t="-5">a</s></p></body></timedtext>`, `The <s> element has no valid "t" attribute.`, 1},
		{"a span offset over the limit", `<timedtext format="3"><body><p t="359999999999"><s t="2000000">a</s></p></body></timedtext>`, `The time "2000000" is not below 100000 hours.`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := youtube.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, youtube.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.want, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
		})
	}
}

func TestReadSrv3LenientReadsTextBeforeTheXML(t *testing.T) {
	t.Parallel()

	src := string(readTestdata(t, "lenient/credit_before_xml.srv3"))
	_, _, err := youtube.Read(strings.NewReader(src), model.CommonReadOptions{}, youtube.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, `The file has text before the XML: "Captions by the harbour film club".`, perr.Msg)
	assert.Equal(t, 1, perr.Line)

	s, warnings, err := youtube.Read(strings.NewReader(src), model.CommonReadOptions{Lenient: true}, youtube.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, [][2]int{{1000, 3000}, {4000, 6000}}, times(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.Equal(t, 1, warnings[0].Line)
}

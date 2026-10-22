package ttml_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// The tests in this file port tests/Parsers/LenientTtmlTest.php of subtitle-toolbox-php, then add rows of their own.

func TestMalformedXMLIsRepaired(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "lenient/malformed.ttml"))
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{
		{sec(1), sec(3), "Salt &amp; pepper are on the table."},
		{sec(4), sec(6), "Turn left\nat the bakery."},
		{sec(7), sec(9), "Tickets cost 5 &amp;cur; at the door."},
		{sec(10), sec(12), "The last bus leaves at ten."},
	}, cueRows(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, warningRow{
		5, model.NoBlock, model.Repaired,
		"The file is not well-formed XML. Invalid character entity & (no semicolon). The parser repaired it.",
	}, warningRows(warnings)[0])

	perr := readError(t, content, false)
	assert.Equal(t, "The file is not well-formed XML.", perr.Msg)
	assert.Equal(t, model.NoLine, perr.Line)
}

func TestHTMLEntitiesAndMalformedXMLGiveTwoWarnings(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, bare+"\n"+`<body><div><p begin="1s" end="2s">Caf&eacute;&nbsp;ok</p>`+"\n"+
		`<p begin="3s" end="4s">A & B</p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(1), sec(2), "Caf\u00e9\u00a0ok"}, {sec(3), sec(4), "A &amp; B"}}, cueRows(s))
	assert.Equal(t, []warningRow{
		{2, model.NoBlock, model.Repaired, `The file has HTML entities that XML does not define, such as "&eacute;". The parser read them as characters.`},
		{3, model.NoBlock, model.Repaired, "The file is not well-formed XML. Invalid character entity & (no semicolon). The parser repaired it."},
	}, warningRows(warnings))
}

func TestHTMLEntities(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, text, want string }{
		{"2 code points", "&nGt;", "\u226b\u20d2"},
		{"a name that PHP leaves out of its table", "&NonBreakingSpace;", "\u00a0"},
		{"XML entities stay", "&amp;&lt;&gt;&quot;&apos;&eacute;", "&amp;&lt;&gt;\"'\u00e9"},
		{"no semicolon", "&eacute &nbsp;", "&amp;eacute \u00a0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, bare+`<body><div><p end="1s">`+tt.text+`</p></div></body></tt>`, true)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Text())
		})
	}
}

func TestRecoverRepairs(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, p, want string }{
		{"open br with attributes", `<p end="1s">a<br class="x">b</p>`, "a\nb"},
		{"open BR closes but is no br", `<p end="1s">a<BR>b</p>`, "ab"},
		{"prefixed br", `<p end="1s">a<tt:br >b</p>`, "ab"},
		{"closed br stays", `<p end="1s">a<br />b<br/>c &</p>`, "a\nb\nc &amp;"},
		{"character references stay", `<p end="1s">&#233;&#xE9;&#XE9; &</p>`, "\u00e9\u00e9&amp;#XE9; &amp;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, bare+`<body><div>`+tt.p+`</div></body></tt>`, true)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Text())
		})
	}
}

func TestTTMLThatCannotBeRepairedFails(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"no markup", "Subtitles & captions", "The file is not well-formed XML."},
		{"an HTML root", "<html><body><p>Hello<br></body></html>", "The root element is not a TTML <tt> element."},
		{"no root element", "<?xml version=\"1.0\"?>\n<!-- empty -->", "The file is not well-formed XML."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, readError(t, tt.content, true).Msg)
		})
	}
}

func TestTextBeforeTheXML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content string
		wantLine      int
		wantLines     []string
		wantStart     float64
	}{
		{
			"text before a comment", "Credit\r\nline 2\r\n<!-- c -->\n" + bare + "\n<body><div><p begin=\"1s\" end=\"2s\">x</p></div></body></tt>",
			1,
			[]string{"Credit", "line 2"},
			1,
		},
		{
			"text after blank lines", "\n\nCredit\n" + bare + "<body><div>\n<p begin=\"0x\" end=\"2s\">x</p>\n<p begin=\"1s\" end=\"2s\">y</p></div></body></tt>",
			3,
			[]string{"Credit"},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, tt.content, false)
			assert.Equal(t, `The file has text before the XML: "`+tt.wantLines[0]+`".`, perr.Msg)
			assert.Equal(t, tt.wantLine, perr.Line)

			s, warnings := mustRead(t, tt.content, true)
			require.NotEmpty(t, warnings)
			assert.Equal(t, warningRow{tt.wantLine, model.NoBlock, model.Repaired, perr.Msg + " The parser skipped it."}, warningRows(warnings)[0])
			assert.Equal(t, tt.wantLines, warnings[0].Lines)
			c, _ := s.Cue(0)
			assert.Equal(t, sec(tt.wantStart), c.Start)
		})
	}

	_, warnings := mustRead(t, "Credit\n"+bare+"<body><div>\n\n<p begin=\"0x\" end=\"2s\">x</p></div></body></tt>", true)
	assert.Equal(t, 4, warnings[1].Line, "the skipped text keeps the line numbers")
}

func TestTextWithoutXMLStart(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "The file is not well-formed XML.", readError(t, "Credit <ttx>", false).Msg)
}

func TestParagraphAfterItsTimedDivWithoutOwnEnd(t *testing.T) {
	t.Parallel()

	content := bare + `<body><div begin="5s" end="6s">` + "\n" + `<p begin="2s">Late</p>` + "\n" + `<p>On time</p></div></body></tt>`
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{sec(5), sec(6), "On time"}}, cueRows(s))
	assert.Equal(t, []warningRow{{2, 0, model.Skipped, "The paragraph begins at 7s, but its parent ends at 6s."}}, warningRows(warnings))

	perr := readError(t, content, false)
	assert.Equal(t, "The paragraph begins at 7s, but its parent ends at 6s.", perr.Msg)
	assert.Equal(t, 2, perr.Line)
}

func TestLooseTimeExpressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, begin string
		want        float64
	}{
		{"one-digit seconds", "00:00:7.250", 7.25},
		{"one-digit hours", "0:01:02.500", 62.5},
		{"minutes and seconds", "02:15.500", 135.5},
		{"milliseconds without unit", "4200", 4.2},
		{"frames with three digits", "00:00:02:250", 2.25},
		{"one-digit fields", "1:2:3", 3723},
		{"minutes and seconds with a comma", "01:05,5", 65.5},
		{"bare milliseconds", "1500", 1.5},
		{"bare seconds", "12.5", 12.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := bare + "<body><div>\n<p begin=\"" + tt.begin + "\" end=\"02:00:00.000\">Text</p></div></body></tt>"
			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{{sec(tt.want), sec(7200), "Text"}}, cueRows(s))
			assert.Equal(t, []warningRow{{
				2, 0, model.Repaired,
				`The time expression "` + tt.begin + `" is not valid. The parser read it as ` + strconv.FormatFloat(tt.want, 'f', -1, 64) + "s.",
			}},
				warningRows(warnings))

			assert.Equal(t, `The time expression "`+tt.begin+`" is not valid.`, readError(t, content, false).Msg)
		})
	}
}

func TestLooseTimeThatIsNoTime(t *testing.T) {
	t.Parallel()

	_, warnings := mustRead(t, bare+"<body><div>\n<p begin=\"soon\" end=\"2s\">x</p>\n<p begin=\"1234567890123456\" end=\"2s\">y</p></div></body></tt>", true)
	assert.Equal(t, []warningRow{
		{2, 0, model.Skipped, `The time expression "soon" is not valid.`},
		{3, 1, model.Skipped, `The time expression "1234567890123456" is not valid.`},
	}, warningRows(warnings))
}

func TestBareSecondsOfAnAppleMusicFile(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "lenient/bare_seconds.ttml"))
	s, warnings := mustRead(t, content, true)
	// The spec adds the begin of the div to the begin of the paragraph. The second div holds absolute paragraph
	// times, which the reader detects.
	assert.Equal(t, []cueRow{{sec(39.528), sec(41.664), "Walking home"}, {sec(60.998), sec(62.794), "Again"}}, cueRows(s))
	assert.Contains(t, warningRows(warnings), warningRow{
		4, model.NoBlock, model.Repaired,
		`The time expression "19.764" is not valid. The parser read it as 19.764s.`,
	})

	assert.Contains(t, readError(t, content, false).Msg, "is not valid.")
}

func TestBareSecondsOnAParagraphWithADuration(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, bare+`<body><div><p begin="1.5" dur="2.5">Text</p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(1.5), sec(4), "Text"}}, cueRows(s))
}

func TestDivWithABadTime(t *testing.T) {
	t.Parallel()

	content := bare + "<body>\n" + `<div begin="soon"><p begin="1s" end="2s">Text</p></div></body></tt>`
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{sec(1), sec(2), "Text"}}, cueRows(s))
	assert.Equal(t, []warningRow{{
		2, model.NoBlock, model.Repaired,
		`The time expression "soon" is not valid. The parser ignored the timing of the div.`,
	}}, warningRows(warnings))

	assert.Equal(t, `The time expression "soon" is not valid.`, readError(t, content, false).Msg)

	_, warnings = mustRead(t, bare+"\n"+`<body end="later"><div><p begin="1s" end="2s">Text</p></div></body></tt>`, true)
	assert.Equal(t, []warningRow{{
		2, model.NoBlock, model.Repaired,
		`The time expression "later" is not valid. The parser ignored the timing of the body.`,
	}}, warningRows(warnings))
}

func TestLooseTimeOnADivHasNoParagraphIndex(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, bare+"<body>\n"+`<div begin="1000"><p begin="1s" end="2s">Text</p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(2), sec(3), "Text"}}, cueRows(s))
	assert.Equal(t, []warningRow{{2, model.NoBlock, model.Repaired, `The time expression "1000" is not valid. The parser read it as 1s.`}},
		warningRows(warnings))
}

func TestFrameRateGuess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, begin, end string
		start, stop      float64
		warning          string
	}{
		{"label 29 keeps 30 fps", "00:00:01:15", "00:00:02:29", 1.5, 2.967, ""},
		{"label 30 gives 50 fps", "00:00:01:30", "00:00:02:10", 1.6, 2.2, `The frame label 30 in "00:00:01:30" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the frames at 50 fps.`},
		{"label 45 gives 50 fps", "00:00:01:45", "00:00:02:10", 1.9, 2.2, `The frame label 45 in "00:00:01:45" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the frames at 50 fps.`},
		{"label 50 gives 60 fps", "00:00:01:50", "00:00:02:10", 1.833, 2.167, `The frame label 50 in "00:00:01:50" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the frames at 60 fps.`},
		{"label 55 in the end gives 60 fps", "00:00:01:30", "00:00:02:55", 1.5, 2.917, `The frame label 55 in "00:00:02:55" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the frames at 60 fps.`},
		{"label 60 gives hundredths", "00:00:01:60", "00:00:02:10", 1.6, 2.1, `The frame label 60 in "00:00:01:60" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the last field as hundredths of a second.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := bare + "\n<body><div><p begin=\"" + tt.begin + "\" end=\"" + tt.end + "\">Text</p></div></body></tt>"
			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{{sec(tt.start), sec(tt.stop), "Text"}}, cueRows(s))
			if tt.warning == "" {
				assert.Empty(t, warnings)
				_, _, err := read(t, content, false)
				require.NoError(t, err)
				return
			}
			assert.Equal(t, []warningRow{{2, model.NoBlock, model.Repaired, tt.warning}}, warningRows(warnings))
			perr := readError(t, content, false)
			strictMsg, _, _ := strings.Cut(tt.warning, " The parser")
			assert.Equal(t, strictMsg, perr.Msg)
			assert.Equal(t, 2, perr.Line)
		})
	}
}

func TestFrameLabelsOfSixtyOrMoreAsHundredths(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, bare+"<body><div>\n"+`<p begin="00:00:01:75" end="00:00:03:20">Text</p>`+"\n"+
		`<p begin="00:00:04:05" end="00:00:05.500">More</p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(1.75), sec(3.2), "Text"}, {sec(4.05), sec(5.5), "More"}}, cueRows(s))
	assert.Equal(t, []warningRow{{
		2, model.NoBlock, model.Repaired,
		`The frame label 75 in "00:00:01:75" is not below the default frame rate of 30, and the file has no ttp:frameRate. The parser read the last field as hundredths of a second.`,
	}},
		warningRows(warnings))
}

func TestFrameRateGuessWithAFrameRate(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, header+` ttp:frameRate="60"><body><div><p begin="00:00:01:45" end="2s">x</p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{sec(1.75), sec(2), "x"}}, cueRows(s))
	assert.Empty(t, warnings)
}

func TestOpenParagraphs(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, bare+"<body><div>\n"+`<p begin="1s">a</p>`+"\n"+`<p begin="1s" end="3s">b</p>`+"\n"+
		`<p begin="2s"><span begin="1s" dur="1s">c</span> <span begin="2s">d</span></p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(1), sec(3), "a"}, {sec(1), sec(3), "b"}, {sec(3), sec(4), "c d"}}, cueRows(s))
	assert.Equal(t, []warningRow{{
		2, 0, model.Repaired,
		"The paragraph that begins at 1s has no end time. The parser ended it at the next paragraph at 3s.",
	}}, warningRows(warnings))
}

func TestOpenParagraphKeepsItsLaterTimestamps(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, bare+"<body><div>\n"+`<p begin="1s">a <span begin="2s">b</span></p>`+"\n"+`<p begin="5s" end="6s">c</p></div></body></tt>`, true)
	assert.Equal(t, []cueRow{{sec(1), sec(5), "a <00:00:03.000>b"}, {sec(5), sec(6), "c"}}, cueRows(s))
}

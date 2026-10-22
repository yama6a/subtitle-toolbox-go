package sami_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const multiLanguage = "multi_language.smi"

func TestReadOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		class   string
		wantErr bool
	}{
		{"not set", "", false},
		{"a class", "ENCC", false},
		{"a space", " ", true},
		{"tab and LF", "\t\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := sami.ReadOptions{LanguageClass: tt.class}.Validate()
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			assert.Equal(t, model.InvalidArgumentError{Option: "ReadOptions.SAMI.LanguageClass", Msg: "The language class must not be empty."}, *aerr)
			_, _, err = read(t, "<SAMI>", model.CommonReadOptions{}, sami.ReadOptions{LanguageClass: tt.class})
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
}

// Ports testLanguageClassOptionPicksTheClass, testLanguageClassIsCaseInsensitive and testUnknownLanguageClassThrows of
// tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadLanguageClass(t *testing.T) {
	t.Parallel()

	english := readFile(t, multiLanguage, sami.ReadOptions{LanguageClass: "ENCC"})
	language, _ := english.Metadata(model.MetadataLanguage)
	assert.Equal(t, "en-US", language)
	data, _ := english.FormatData("sami")
	class, _ := data.String("class")
	assert.Equal(t, "ENCC", class)
	assert.Equal(t, []cueRow{
		{time.Second, 4200 * ms, []string{"The bakery opens at six in the morning."}},
		{5 * time.Second, 8 * time.Second, []string{`It is <font color="#ffff00">raining</font> today.`, "Take an umbrella."}},
		{8 * time.Second, 10500 * ms, []string{"<i>The train</i> leaves soon."}},
		{11 * time.Second, 13 * time.Second, []string{"Bread &amp; coffee cost less than 5 euros."}},
	}, cueRows(english))

	french := readFile(t, multiLanguage, sami.ReadOptions{LanguageClass: "frcc"})
	language, _ = french.Metadata(model.MetadataLanguage)
	assert.Equal(t, "fr-FR", language)
	data, _ = french.FormatData("sami")
	class, _ = data.String("class")
	assert.Equal(t, "FRCC", class)
	assert.Equal(t, []string{`Il <font color="#ffff00">pleut</font> aujourd'hui.`, "Prenez un parapluie."}, lines(french)[1])

	for _, lenient := range []bool{false, true} {
		_, _, err := read(t, string(testfiles.Read(t, realDir+"/"+multiLanguage)), model.CommonReadOptions{Lenient: lenient},
			sami.ReadOptions{LanguageClass: "DECC"})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, model.ParseError{Format: model.SAMI, Msg: "The SAMI file has no class DECC."}, *perr)
	}
}

// The class choice rules of https://github.com/yama6a/subtitle-toolbox-go/issues/25.
func TestReadChoosesTheClass(t *testing.T) {
	t.Parallel()

	const style = "<STYLE><!-- .FRCC { lang: fr-FR; } .ENCC { lang: en-US; } --></STYLE>"
	tests := []struct {
		name      string
		head      string
		body      string
		option    string
		wantClass string
		wantLines [][]string
	}{
		{"1 the option in STYLE", style, "<P Class=encc>one<P Class=frcc>un", "ENCC", "ENCC", [][]string{{"one"}}},
		{"1 the option in a <P> only", style, "<P Class=decc>eins<P Class=encc>one", "DeCC", "decc", [][]string{{"eins"}}},
		{"2 no STYLE classes", "", "<P Class=FRCC>un<P Class=ENCC>one", "", "FRCC", [][]string{{"un"}}},
		{"2 no classes at all", "", "<P>one", "", "", [][]string{{"one"}}},
		{"3 no <P> uses a class", style, "<P>one", "", "FRCC", [][]string{{"one"}}},
		{"4 the first STYLE class that a <P> uses", style, "<P Class=encc>one<P Class=DECC>eins", "", "ENCC", [][]string{{"one"}}},
		{"5 no <P> class in STYLE", style, "<P Class=DECC>eins<P Class=ITCC>uno", "", "DECC", [][]string{{"eins"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "<SAMI><HEAD>" + tt.head + "</HEAD><BODY>\n<SYNC Start=1000>" + tt.body + "\n</BODY></SAMI>"
			s, _, err := read(t, content, model.CommonReadOptions{}, sami.ReadOptions{LanguageClass: tt.option})
			require.NoError(t, err)
			data, _ := s.FormatData("sami")
			class, ok := data.String("class")
			assert.Equal(t, tt.wantClass != "", ok)
			assert.Equal(t, tt.wantClass, class)
			assert.Equal(t, tt.wantLines, lines(s))
		})
	}
}

// Ports testStyleBlockAndHeaderGoToFormatData of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadHead(t *testing.T) {
	t.Parallel()

	s := readFile(t, "mantas_smi.smi", sami.ReadOptions{})
	title, _ := s.Metadata(model.MetadataTitle)
	assert.Equal(t, "file", title)
	data, _ := s.FormatData("sami")
	class, _ := data.String("class")
	param, _ := data.String("samiParam")
	style, _ := data.String("style")
	assert.Equal(t, "ENUSCC", class)
	assert.Equal(t, "\n  Metrics {time:ms;}\n  Spec {MSFT:1.0;}\n", param)
	assert.True(t, strings.HasPrefix(style, "\n<!--\n  P { font-family: Arial;"))
	assert.True(t, strings.HasSuffix(style, ".ENUSCC { name: English; lang: en-US ; SAMIType: CC ; }\n-->\n"))

	s, _ = mustRead(t, "<SAMI><HEAD><title> Rain &amp; sun </title></HEAD></SAMI>", false)
	title, _ = s.Metadata(model.MetadataTitle)
	assert.Equal(t, "Rain & sun", title)
	s, _ = mustRead(t, "<SAMI><HEAD><TITLE> </TITLE></HEAD></SAMI>", false)
	_, ok := s.Metadata(model.MetadataTitle)
	assert.False(t, ok)
}

// Ports testSourceIdParagraphIsKeptInCueFormatData of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadKeepsTheParagraphsInCueData(t *testing.T) {
	t.Parallel()

	s := readFile(t, "pysubs2_source_id.smi", sami.ReadOptions{})
	c, _ := s.Cue(s.Len() - 1)
	data, ok := c.FormatData("sami")
	require.True(t, ok)
	paragraphs, _ := data.List("paragraphs")
	require.Len(t, paragraphs, 2)
	first, _ := paragraphs[0].(*model.Data)
	attributes, _ := first.Object("attributes")
	id, _ := attributes.String("id")
	html, _ := first.String("html")
	assert.Equal(t, []string{"id"}, attributes.Keys())
	assert.Equal(t, "Source", id)
	assert.Equal(t, "End of:", html)
	second, _ := paragraphs[1].(*model.Data)
	empty, _ := second.List("attributes")
	html, _ = second.String("html")
	assert.Equal(t, []any{}, empty)
	assert.Equal(t, "Weather Report for Tuesday", html)
	stored, _ := data.Strings("lines")
	assert.Equal(t, []string{"End of:", "Weather Report for Tuesday"}, stored)
}

// Ports testInlineTagsBecomeCoreMarkup and testBreakStartsALineAndSourceLineBreaksAreSpaces of
// tests/Parsers/SamiParserTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestReadParagraphMarkup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"style tags", "The <B>train <I>leaves <U>at <S>nine</S></U></I></B>", []string{"The <b>train <i>leaves <u>at <s>nine</s></u></i></b>"}},
		{
			"strike and colors", `<STRIKE>old</STRIKE> <font color=red>red</font> <font color="00FF00">green</font>`,
			[]string{`<s>old</s> <font color="#ff0000">red</font> <font color="#00ff00">green</font>`},
		},
		{"no color", "<font face=Arial color=#ABC>short</font> <font face=Arial>face</font> <span>span</span>", []string{"short face span"}},
		{"a color name of TTML only", "<font color=magenta>a</font> <font color=' Navy '>b</font>", []string{`a <font color="#000080">b</font>`}},
		{
			"breaks and source line ends", "\n  one\n  two<BR>three<br/>&nbsp;<br>&lt;four&gt; &amp; &#039;five&#039;",
			[]string{"one two", "three", "&lt;four&gt; &amp; 'five'"},
		},
		{"&nbsp without ;", "a&nbspb &nbsp;", []string{"a\u00A0b \u00A0"}},
		{"a comment", "a<!-- b -->c", []string{"ac"}},
		{"unknown tags", "<ruby>a<rt>b</rt></ruby>", []string{"ab"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readBody(t, "<SYNC Start=0><P Class=ENCC>"+tt.in+"\n<SYNC Start=1500><P Class=ENCC>&nbsp;\n")
			require.Equal(t, 1, s.Len())
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Lines)
			assert.Equal(t, 1500*ms, c.End)
		})
	}
}

func TestReadNbspBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{"&nbsp;", "&nbsp;"},
		{"&nbsp", "&nbsp"},
		{"2 &nbsp; and spaces", " &nbsp; &nbsp "},
		{"&nbsp; between breaks", "<br>&nbsp;<br>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readBody(t, "<SYNC Start=0><P Class=ENCC>one\n<SYNC Start=1000><P Class=ENCC>"+tt.in+"\n")
			assert.Equal(t, []cueRow{{0, time.Second, []string{"one"}}}, cueRows(s))
		})
	}
}

// Ports testSyncWithoutParagraphEndsTheCueOfEveryClass of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadBlockWithoutParagraphEndsEveryClass(t *testing.T) {
	t.Parallel()

	content := "<SAMI><BODY>\n" +
		"<SYNC Start=1000><P Class=ENCC>one<P Class=FRCC>un\n" +
		"<SYNC Start=2000>&nbsp;\n" +
		"<SYNC Start=3000><P Class=ENCC>two\n" +
		"<SYNC Start=3500><P Class=FRCC>deux\n" +
		"<SYNC Start=4000><P Class=ENCC>&nbsp;\n" +
		"</BODY></SAMI>"
	tests := []struct {
		class string
		want  [][2]time.Duration
	}{
		{"ENCC", [][2]time.Duration{{time.Second, 2 * time.Second}, {3 * time.Second, 4 * time.Second}}},
		{"FRCC", [][2]time.Duration{{time.Second, 2 * time.Second}, {3500 * ms, 4500 * ms}}},
	}
	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(t, content, model.CommonReadOptions{LastCueDuration: new(time.Second)}, sami.ReadOptions{LanguageClass: tt.class})
			require.NoError(t, err)
			var got [][2]time.Duration
			for _, c := range s.All() {
				got = append(got, [2]time.Duration{c.Start, c.End})
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ports testFileWithoutClassesReadsEveryParagraph and testDefaultClassIsTheFirstParagraphClassWithoutStyleBlock of
// tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadWithoutStyle(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<sami><body><sync start=500><p>one<sync start=900><p>two<sync start=1200><p>&nbsp;</body></sami>", false)
	_, ok := s.Metadata(model.MetadataLanguage)
	assert.False(t, ok)
	assert.Empty(t, s.FormatDataKeys())
	assert.Equal(t, []cueRow{{500 * ms, 900 * ms, []string{"one"}}, {900 * ms, 1200 * ms, []string{"two"}}}, cueRows(s))

	s, _ = mustRead(t, "<SAMI><BODY><SYNC Start=0><P Class=FRCC>un<P Class=ENCC>one\n<SYNC Start=900><P Class=ENCC>two</BODY></SAMI>", false)
	data, _ := s.FormatData("sami")
	assert.Equal(t, []string{"class"}, data.Keys())
	class, _ := data.String("class")
	assert.Equal(t, "FRCC", class)
	assert.Equal(t, [][]string{{"un"}}, lines(s))
}

// Ports testParagraphClassThatNoStyleRuleDefinesIsRead of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadClassThatNoStyleRuleDefines(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, realDir+"/undefined_class.smi"))
	want := []cueRow{{time.Second, 3500 * ms, []string{"Hello"}}, {4 * time.Second, 6 * time.Second, []string{"The ferry leaves at noon."}}}

	strict, warnings := mustRead(t, content, false)
	assert.Equal(t, want, cueRows(strict))
	_, ok := strict.Metadata(model.MetadataLanguage)
	assert.False(t, ok)
	assert.Empty(t, warnings)

	lenient, warnings := mustRead(t, content, true)
	assert.Equal(t, want, cueRows(lenient))
	assert.Equal(t, []warningRow{{
		"No <P> class matches a class of the STYLE block. The parser read the class ENCC.", 12, model.NoBlock,
		[]string{"<SYNC Start=1000><P Class=ENCC>Hello"},
		model.Repaired,
	}}, warningRows(warnings))

	_, warnings = mustRead(t, "<SAMI><HEAD><STYLE>.ENUSCC {}</STYLE></HEAD><BODY><SYNC Start=0><P Class=ENCC>Hello</BODY></SAMI>", true)
	assert.Equal(t, []warningRow{{
		"No <P> class matches a class of the STYLE block. The parser read the class ENCC.", model.NoLine, model.NoBlock, nil, model.Repaired,
	}}, warningRows(warnings))
}

// Ports testFirstStyleClassThatAParagraphUsesIsRead of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadFirstStyleClassThatAParagraphUses(t *testing.T) {
	t.Parallel()

	s := readFile(t, "unused_first_class.smi", sami.ReadOptions{})
	assert.Equal(t, []cueRow{
		{time.Second, 3500 * ms, []string{"The museum opens at nine."}},
		{4 * time.Second, 6 * time.Second, []string{"Tickets are sold at the door."}},
	}, cueRows(s))
	language, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "en-US", language)
}

// Ports testNegativeStartIsReadAsZeroAndBareNbspEndsTheCue of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadNegativeStart(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, realDir+"/negative_start_bare_nbsp.smi"))
	want := []cueRow{{0, time.Second, []string{"The tide is out."}}, {2 * time.Second, 4500 * ms, []string{"Boats wait on the sand."}}}

	strict, warnings := mustRead(t, content, false)
	assert.Equal(t, want, cueRows(strict))
	assert.Empty(t, warnings)

	lenient, warnings := mustRead(t, content, true)
	assert.Equal(t, want, cueRows(lenient))
	assert.Equal(t, []warningRow{{
		"SYNC tag 1 has a negative Start. The parser read it as 0.", 10, 0,
		[]string{"<SYNC Start=-200><P Class=ENUSCC>The tide is out."},
		model.Repaired,
	}}, warningRows(warnings))

	_, warnings = mustRead(t, "<SAMI><BODY><SYNC Start=-99999999999999999999><P>a<SYNC Start=-0><P>b</BODY></SAMI>", true)
	assert.Equal(t, []string{"SYNC tag 1 has a negative Start. The parser read it as 0."}, []string{warnings[0].Message})
	assert.Len(t, warnings, 1)
}

// Ports testSyncsAreSortedByStart and testLastCueDurationOption of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadStarts(t *testing.T) {
	t.Parallel()

	s := readBody(t, "<SYNC Start=3000><P Class=ENCC>two\n<SYNC Start=1000><P Class=ENCC>one\n<SYNC Start=4000><P Class=ENCC>&nbsp;\n")
	assert.Equal(t, []cueRow{{time.Second, 3 * time.Second, []string{"one"}}, {3 * time.Second, 4 * time.Second, []string{"two"}}}, cueRows(s))

	s = readBody(t, "<SYNC Start=\"7300\"><P Class=ENCC>quoted\n<SYNC start = ' 0009000 '><P Class=ENCC>&nbsp;\n")
	assert.Equal(t, []cueRow{{7300 * ms, 9 * time.Second, []string{"quoted"}}}, cueRows(s))

	content := "<SAMI><BODY><SYNC Start=1000><P Class=ENCC>one</BODY></SAMI>"
	for _, tt := range []struct {
		last *time.Duration
		want time.Duration
	}{{nil, 6 * time.Second}, {new(2500 * ms), 3500 * ms}, {new(time.Duration(0)), time.Second}} {
		s, _, err := read(t, content, model.CommonReadOptions{LastCueDuration: tt.last}, sami.ReadOptions{})
		require.NoError(t, err)
		c, _ := s.Cue(0)
		assert.Equal(t, tt.want, c.End)
	}
}

// Ports testSyncWithoutStartThrows of tests/Parsers/SamiParserTest.php and the SAMI case of
// tests/Parsers/ParseErrorLineNumbersTest.php of subtitle-toolbox-php.
func TestReadSyncWithoutStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, body string
		want       model.ParseError
	}{
		{"End only", "<SYNC Start=0><P Class=ENCC>one\n<SYNC End=1000><P Class=ENCC>two\n", model.ParseError{
			Format: model.SAMI, Line: 4, Msg: "SYNC tag 2 has no valid Start attribute.",
		}},
		{"no attribute", "<SYNC>The ferry leaves.\n", model.ParseError{Format: model.SAMI, Line: 3, Msg: "SYNC tag 1 has no valid Start attribute."}},
		{"no >", "<SYNC Start=1000", model.ParseError{Format: model.SAMI, Line: 3, Msg: "SYNC tag 1 has no valid Start attribute."}},
		{"Start after >", "<SYNC><P Start=1000>x", model.ParseError{Format: model.SAMI, Line: 3, Msg: "SYNC tag 1 has no valid Start attribute."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, *readError(t, "<SAMI>\n<BODY>\n"+tt.body+"</BODY>\n</SAMI>\n"))
		})
	}
}

// Ports the SAMI case of tests/Parsers/TimesPastTheLimitTest.php of subtitle-toolbox-php.
func TestReadStartPastTheLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		start, digits string
	}{
		{"900000000000000", "900000000000000"},
		{"360000000000", "360000000000"},
		{"99999999999999999999999", "99999999999999999999999"},
	}
	for _, tt := range tests {
		t.Run(tt.start, func(t *testing.T) {
			t.Parallel()

			content := "<SAMI>\n<BODY>\n<SYNC Start=1000><P>The pool opens at seven.\n<SYNC Start=3000><P>&nbsp;\n" +
				"<SYNC Start=" + tt.start + "><P>Towels are at the desk.\n<SYNC Start=7000><P>No running, please.\n" +
				"<SYNC Start=9000><P>&nbsp;\n</BODY>\n</SAMI>\n"
			msg := `The time "` + tt.digits + `" is not below 100000 hours.`
			assert.Equal(t, model.ParseError{Format: model.SAMI, Line: 5, Msg: msg}, *readError(t, content))

			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{
				{time.Second, 3 * time.Second, []string{"The pool opens at seven."}},
				{7 * time.Second, 9 * time.Second, []string{"No running, please."}},
			}, cueRows(s))
			require.Len(t, warnings, 1)
			assert.Equal(t, warningRow{msg, 5, 2, []string{"<SYNC Start=" + tt.start + "><P>Towels are at the desk."}, model.Skipped},
				warningRows(warnings)[0])
		})
	}

	s, _ := mustRead(t, "<SAMI><BODY><SYNC Start=359999999999><P>last</BODY></SAMI>", false)
	c, _ := s.Cue(0)
	assert.Equal(t, 359999999999*ms, c.Start)
}

// Ports testInputThatIsNotUtf8Throws of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadInvalidUTF8(t *testing.T) {
	t.Parallel()

	content := "<SAMI>\n<BODY>\n<SYNC Start=0><P Class=KRCC>\xbf\xc0\xb4\xc3\n</BODY>\n</SAMI>\n"
	const msg = "The content is not valid UTF-8. The first bad byte is at offset 42. Set the source encoding of the file."
	assert.Equal(t, model.ParseError{Format: model.SAMI, Msg: msg}, *readError(t, content))

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, [][]string{{"\uFFFD\uFFFD\uFFFD\uFFFD"}}, lines(s))
	assert.Equal(t, []warningRow{{msg + " The parser read the bad bytes as U+FFFD.", model.NoLine, model.NoBlock, nil, model.Repaired}},
		warningRows(warnings))
}

// Ports testBrokenHtmlRaisesNoWarningAndKeepsTheLibxmlSetting of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadBrokenHTML(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "<SAMI>\n<BODY>\n<SYNC Start=0><P Class=ENCC><b>open <i>tags</p></div></font>\n<SYNC Start=900><P Class=ENCC>&nbsp;\n</BODY>\n</SAMI>\n", true)
	assert.Equal(t, [][]string{{"<b>open <i>tags</i></b>"}}, lines(s))
	assert.Empty(t, warnings)
}

// Ports testReadsTheBodyAfterAHeadOfMoreThanOneMegabyte of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php.
func TestReadLargeHead(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, realDir+"/subsrt_sample.smi"))
	large := strings.Replace(content, "</HEAD>", "<!-- "+strings.Repeat("Padding before the body. ", 50000)+"-->\n</HEAD>", 1)
	require.Greater(t, len(large), 1_000_000)
	want, _ := mustRead(t, content, false)
	got, _ := mustRead(t, large, false)
	assert.Equal(t, cueRows(want), cueRows(got))
}

// Ports the SAMI cases of tests/Parsers/NoCuesTest.php of subtitle-toolbox-php.
func TestReadNoCues(t *testing.T) {
	t.Parallel()

	content := "<SAMI>\n<HEAD><TITLE>Walk</TITLE></HEAD>\n<BODY>\n<P>The ferry leaves.</P>\n</BODY>\n</SAMI>\n"
	assert.Equal(t, model.ParseError{Format: model.SAMI, Line: 4, Msg: "The file has text but no cues."}, *readError(t, content))
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, 0, s.Len())
	assert.Equal(t, []warningRow{{"The file has text but no cues.", 4, model.NoBlock, nil, model.Skipped}}, warningRows(warnings))

	for _, content := range []string{
		"<SAMI>\n<BODY>\n<SYNC Start=1000><P>&nbsp;</P>\n</BODY>\n</SAMI>\n",
		"<SAMI>\n<HEAD><TITLE>Walk</TITLE></HEAD>\n<BODY><!-- x\ny -->&nbsp;</BODY>\n</SAMI>\n",
		"<SAMI><BODY><syncx></BODY></SAMI>",
		"",
	} {
		s, warnings := mustRead(t, content, false)
		assert.Equal(t, 0, s.Len())
		assert.Empty(t, warnings)
	}
}

func TestReadLineNumbers(t *testing.T) {
	t.Parallel()

	content := "<SAMI>\r\n<HEAD>\r\n</HEAD>\r\n<BODY><SYNC Start=0><P>a</SYNC\n>\r\r\n<SYNC Start=x>\r\n\r\n<SYNC Start=1>\n<sync>"
	_, warnings := mustRead(t, content, true)
	assert.Equal(t, []warningRow{
		{"SYNC tag 2 has no valid Start attribute.", 5, 1, []string{"<SYNC Start=x>"}, model.Skipped},
		{"SYNC tag 4 has no valid Start attribute.", 8, 3, []string{"<SYNC>"}, model.Skipped},
	}, warningRows(warnings))
}

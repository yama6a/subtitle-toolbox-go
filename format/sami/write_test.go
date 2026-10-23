package sami_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileRoundTrips of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file, class string
	}{
		{"mantas_smi.smi", ""},
		{"mantas_smi_formatted.smi", ""},
		{multiLanguage, ""},
		{multiLanguage, "ENCC"},
		{multiLanguage, "FRCC"},
		{"pysubs2_source_id.smi", ""},
		{"subsrt_sample.smi", ""},
		{"own_windows_media.smi", ""},
		{"own_subtitle_edit_modern.smi", ""},
		{"undefined_class.smi", ""},
	}
	for _, tt := range tests {
		t.Run(tt.file+" "+tt.class, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, tt.file, sami.ReadOptions{LanguageClass: tt.class})
			out := write(t, s, model.CommonWriteOptions{})
			back, _ := mustRead(t, out, false)
			assert.Equal(t, cueRows(s), cueRows(back))
			assert.Equal(t, s.AllMetadata(), back.AllMetadata())
			assert.Equal(t, out, write(t, back, model.CommonWriteOptions{}))
		})
	}
}

// Ports testWritesTheChosenClassOnly of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteTheChosenClassOnly(t *testing.T) {
	t.Parallel()

	out := write(t, readFile(t, multiLanguage, sami.ReadOptions{LanguageClass: "FRCC"}), model.CommonWriteOptions{})
	assert.Contains(t, out, "\n.FRCC { Name:Fran\u00E7ais; lang:fr-FR; SAMIType:CC; }\n-->\n</STYLE>\n")
	assert.NotContains(t, out, "KRCC")
	assert.NotContains(t, out, "ENCC")
	assert.True(t, strings.HasSuffix(out, "<SYNC Start=1000><P Class=FRCC>La boulangerie ouvre \u00E0 six heures du matin.\n"+
		"<SYNC Start=4200><P Class=FRCC>&nbsp;\n"+
		"<SYNC Start=5000><P Class=FRCC>Il <font color=\"yellow\">pleut</font> aujourd'hui.<br>Prenez un parapluie.\n"+
		"<SYNC Start=8000><P Class=FRCC><u>Le train</u> part bient\u00F4t.\n"+
		"<SYNC Start=10500><P Class=FRCC>&nbsp;\n"+
		"</BODY>\n</SAMI>\n"))
}

// Ports testCuesFromAnotherFormat of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteCuesFromAnotherFormat(t *testing.T) {
	t.Parallel()

	s := subtitle(
		model.Cue{Start: time.Second, End: 2500 * ms, Lines: []string{
			"<v Anna>The <b>bakery</b> &amp; caf\u00E9", `<font color="#ff0000">opens</font> <00:00:02.000>at six`,
		}},
		model.Cue{Start: 2500 * ms, End: 4 * time.Second, Lines: []string{"Line\u00A0one"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second},
	)
	assert.Equal(t, "<SAMI>\n<HEAD>\n<STYLE TYPE=\"text/css\"><!--\nP { font-family: Arial; text-align: center; }\n"+
		".SUBTTL { Name: Subtitles; }\n--></STYLE>\n</HEAD>\n<BODY>\n"+
		"<SYNC Start=1000><P Class=SUBTTL>The <b>bakery</b> &amp; caf\u00E9<br><font color=\"#ff0000\">opens</font> at six\n"+
		"<SYNC Start=2500><P Class=SUBTTL>Line&nbsp;one\n"+
		"<SYNC Start=4000><P Class=SUBTTL>&nbsp;\n"+
		"<SYNC Start=5000><P Class=SUBTTL>&nbsp;\n"+
		"<SYNC Start=6000><P Class=SUBTTL>&nbsp;\n"+
		"</BODY>\n</SAMI>\n", write(t, s, model.CommonWriteOptions{}))
}

func TestWriteClearLines(t *testing.T) {
	t.Parallel()

	s := subtitle(
		model.Cue{Start: -time.Second, End: time.Second, Lines: []string{"a"}},
		model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"b"}},
		model.Cue{Start: 2 * time.Second, End: 4 * time.Second, Lines: []string{"c"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"<c.yellow>d</c>", "<ruby>e<rt>f</rt></ruby>"}},
	)
	out := write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF})
	assert.True(t, strings.HasSuffix(out, "<BODY>\r\n"+
		"<SYNC Start=0><P Class=SUBTTL>a\r\n"+
		"<SYNC Start=1000><P Class=SUBTTL>b\r\n"+
		"<SYNC Start=2000><P Class=SUBTTL>c\r\n"+
		"<SYNC Start=4000><P Class=SUBTTL>&nbsp;\r\n"+
		"<SYNC Start=5000><P Class=SUBTTL><font color=\"#ffff00\">d</font><br>e (f)\r\n"+
		"<SYNC Start=6000><P Class=SUBTTL>&nbsp;\r\n"+
		"</BODY>\r\n</SAMI>\r\n"), out)
}

// Ports testTagsThatTheCueLeavesOpenAreClosedAtItsEnd of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteClosesTagsThatTheCueLeavesOpen(t *testing.T) {
	t.Parallel()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "srt/real/own_styled.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	s.AddCue(model.Cue{Start: 900 * time.Second, End: 901 * time.Second, Lines: []string{`<b><font color="#ff0000">Red`, "<u>and bold</b>"}})
	out := write(t, s, model.CommonWriteOptions{})
	assert.Contains(t, out, "<SYNC Start=540000><P Class=SUBTTL><i>Italics that never close</i>\n")
	assert.Contains(t, out, "<SYNC Start=900000><P Class=SUBTTL><b><font color=\"#ff0000\">Red<br><u>and bold</b></u></font>\n")
}

// Ports testLanguageAndTitleMetadata of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php, with ASCII text.
func TestWriteLanguageAndTitle(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{Start: 0, End: time.Second, Lines: []string{"It rains"}})
	s.SetMetadata(model.MetadataLanguage, "ko-KR")
	s.SetMetadata(model.MetadataTitle, "Rain & <Sun>")
	out := write(t, s, model.CommonWriteOptions{})
	assert.Contains(t, out, "<TITLE>Rain &amp; &lt;Sun&gt;</TITLE>\n")
	assert.Contains(t, out, "\n.KOKRCC { Name: ko-KR; lang: ko-KR; }\n")
	assert.Contains(t, out, "<SYNC Start=0><P Class=KOKRCC>It rains\n")

	back, _ := mustRead(t, out, false)
	assert.Equal(t, map[string]string{"title": "Rain & <Sun>", "language": "ko-KR"}, back.AllMetadata())

	s.SetMetadata(model.MetadataLanguage, "")
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "\n.SUBTTL { Name: ; lang: ; }\n")
}

// Ports testChangedCueIsWrittenFromCoreMarkup of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteChangedCue(t *testing.T) {
	t.Parallel()

	s := readFile(t, "pysubs2_source_id.smi", sami.ReadOptions{})
	last := s.Len() - 1
	c, _ := s.Cue(last)
	c.Lines = []string{"<i>End</i> of the report"}
	require.NoError(t, s.SetCue(last, c))
	out := write(t, s, model.CommonWriteOptions{})
	assert.Contains(t, out, "<SYNC Start=73000><P Class=ENUSCC><i>End</i> of the report\n<SYNC Start=78000><P Class=ENUSCC>&nbsp;\n")
	assert.Contains(t, out, "<SYNC Start=0><P Class=ENUSCC ID=Source>Weather Desk\n")
}

// Ports testStripAllXmlTagsOption of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteStripTags(t *testing.T) {
	t.Parallel()

	s := readFile(t, "own_windows_media.smi", sami.ReadOptions{})
	out := write(t, s, model.CommonWriteOptions{StripTags: true})
	assert.Contains(t, out, "<SYNC Start=8000><P Class=ENUSCC>Bus 7 runs every ten minutes.\n")
	assert.Contains(t, out, "<SYNC Start=0><P Class=ENUSCC>Announcer<br>Good evening. Here is the station news.\n")
}

// Ports testFileWithoutClassesWritesParagraphsWithoutClass of tests/Formatters/SamiFormatterTest.php of subtitle-toolbox-php.
func TestWriteFileWithoutClasses(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<SAMI><HEAD><STYLE><!-- P { color: white; } --></STYLE></HEAD><BODY><SYNC Start=0><P>one<SYNC Start=900><P>&nbsp;</BODY></SAMI>", false)
	assert.Equal(t, "<SAMI>\n<HEAD>\n<STYLE TYPE=\"text/css\"><!-- P { color: white; } --></STYLE>\n</HEAD>\n<BODY>\n"+
		"<SYNC Start=0><P>one\n<SYNC Start=900><P>&nbsp;\n</BODY>\n</SAMI>\n", write(t, s, model.CommonWriteOptions{}))
}

func TestWriteStoredParagraphs(t *testing.T) {
	t.Parallel()

	attributes := model.NewData(
		model.Field{Key: "id", Value: "a.b-c_1"},
		model.Field{Key: "title", Value: "it's \"x\" & <y>"},
		model.Field{Key: "lang", Value: "en\n"},
	)
	data := model.NewData(
		model.Field{Key: "paragraphs", Value: []any{
			model.NewData(model.Field{Key: "attributes", Value: attributes}, model.Field{Key: "html", Value: "a\u00A0<b>b</b>"}),
			model.NewData(model.Field{Key: "attributes", Value: []any{}}, model.Field{Key: "html", Value: "c"}),
		}},
		model.Field{Key: "lines", Value: []any{"x"}},
	)
	c := model.Cue{Start: 0, End: time.Second, Lines: []string{"x"}}
	require.NoError(t, c.SetFormatData("sami", data))
	s := subtitle(c)
	require.NoError(t, s.SetFormatData("sami", model.NewData(model.Field{Key: "class", Value: "ENCC"})))
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}),
		"<SYNC Start=0><P Class=ENCC ID=a.b-c_1 TITLE=\"it&#039;s &quot;x&quot; &amp; &lt;y&gt;\" LANG=\"en\n\">a&nbsp;<b>b</b><P Class=ENCC>c\n")
	assert.Contains(t, write(t, s, model.CommonWriteOptions{StripTags: true}), "<SYNC Start=0><P Class=ENCC>x\n")

	c.Lines = []string{"y"}
	require.NoError(t, s.SetCue(0, c))
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "<SYNC Start=0><P Class=ENCC>y\n")
}

func TestWriteStoredStyle(t *testing.T) {
	t.Parallel()

	const style = "\n<!--\nP { color: white; }\n  .KRCC { lang: ko-KR; }  \n.encc { lang: en-US; }\n.FRCC{}\n-->\n"
	tests := []struct {
		name string
		data *model.Data
		want string
	}{
		{
			"the stored class", model.NewData(model.Field{Key: "style", Value: style}, model.Field{Key: "class", Value: "ENCC"}),
			"<STYLE TYPE=\"text/css\">\n<!--\nP { color: white; }\n.encc { lang: en-US; }\n-->\n</STYLE>\n",
		},
		{
			"a style without class keeps every rule", model.NewData(model.Field{Key: "style", Value: style}),
			"<STYLE TYPE=\"text/css\">" + style + "</STYLE>\n",
		},
		{
			"a class without style", model.NewData(model.Field{Key: "class", Value: "XX"}, model.Field{Key: "samiParam", Value: " Spec {MSFT:1.0;} "}),
			"<SAMIParam> Spec {MSFT:1.0;} </SAMIParam>\n<STYLE TYPE=\"text/css\"><!--\nP { font-family: Arial; text-align: center; }\n.XX { Name: Subtitles; }\n--></STYLE>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(model.Cue{Start: 0, End: time.Second, Lines: []string{"a"}})
			require.NoError(t, s.SetFormatData("sami", tt.data))
			assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "<HEAD>\n"+tt.want+"</HEAD>\n")
		})
	}
}

func TestWriteInvalidUTF8(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    func() *model.Subtitle
		cue  int
	}{
		{"cue text", func() *model.Subtitle {
			return subtitle(model.Cue{Start: 0, End: time.Second, Lines: []string{"a"}}, model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"caf\xe9"}})
		}, 1},
		{"title", func() *model.Subtitle {
			s := subtitle(model.Cue{Start: 0, End: time.Second, Lines: []string{"a"}})
			s.SetMetadata(model.MetadataTitle, "caf\xe9")
			return s
		}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			err := sami.Write(&out, tt.s(), model.CommonWriteOptions{}, sami.WriteOptions{})
			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			require.ErrorIs(t, err, model.ErrInvalidUTF8)
			require.ErrorIs(t, err, model.ErrUnwritable)
			assert.Equal(t, model.UnwritableError{
				Format: model.SAMI, CueIndex: tt.cue, Err: model.ErrInvalidUTF8,
				Reason: "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.",
			}, *uerr)
			assert.Zero(t, out.Len())
		})
	}
}

func TestWriteBOM(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{Start: 0, End: time.Second, Lines: []string{"a"}})
	assert.True(t, strings.HasPrefix(write(t, s, model.CommonWriteOptions{}), "<SAMI>\n"))
	assert.True(t, strings.HasPrefix(write(t, s, model.CommonWriteOptions{BOM: model.BOMAdd}), "\uFEFF<SAMI>\n"))
}

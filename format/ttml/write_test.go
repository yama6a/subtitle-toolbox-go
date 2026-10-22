package ttml_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	xmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	// ttmlRoot is the root start tag that the writer writes for a subtitle of another format, without its closing >.
	ttmlRoot = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang=""`
	// twoParagraphs is the TTML of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
	twoParagraphs = "<tt xmlns=\"http://www.w3.org/ns/ttml\" xmlns:tts=\"http://www.w3.org/ns/ttml#styling\" xml:lang=\"en\">\n" +
		"  <body>\n    <div>\n" +
		"      <p begin=\"00:00:01.500\" end=\"00:00:04.000\">Hello<br/><span tts:fontStyle=\"italic\">world</span></p>\n" +
		"      <p begin=\"5s\" dur=\"2500ms\">Second cue</p>\n" +
		"    </div>\n  </body>\n</tt>"
)

func writeTTML(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, ttml.Write(&out, s, common, ttml.WriteOptions{}))
	return out.String()
}

func writeDefault(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	return writeTTML(t, s, model.CommonWriteOptions{})
}

func writeError(t *testing.T, s *model.Subtitle) *model.UnwritableError {
	t.Helper()

	err := ttml.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, ttml.WriteOptions{})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	require.ErrorIs(t, err, model.ErrUnwritable)
	assert.Equal(t, model.TTML, uerr.Format)
	return uerr
}

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

func subtitleOf(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func readSRT(t *testing.T, content string) *model.Subtitle {
	t.Helper()

	s, _, err := srt.Read(strings.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

func readVTT(t *testing.T, content string) *model.Subtitle {
	t.Helper()

	s, _, err := webvtt.Read(strings.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	return s
}

func setCue(t *testing.T, s *model.Subtitle, i int, change func(*model.Cue)) {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok)
	change(&c)
	require.NoError(t, s.SetCue(i, c))
}

// paragraphs returns the <p> lines of a TTML output, trimmed.
func paragraphs(out string) []string {
	var ps []string
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "<p") {
			ps = append(ps, line)
		}
	}
	return ps
}

var xmlIDRegexp = regexp.MustCompile(`xml:id="([^"]*)"`)

// ids returns the xml:id values of a TTML output in document order.
func ids(out string) []string {
	var found []string
	for _, m := range xmlIDRegexp.FindAllStringSubmatch(out, -1) {
		found = append(found, m[1])
	}
	return found
}

// Ports testTwoParagraphsAreWrittenBack of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestTwoParagraphsAreWrittenBack(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, twoParagraphs, false)
	assert.Equal(t, xmlDeclaration+
		`<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en">`+"\n"+
		"  <head/>\n"+
		"  <body>\n    <div>\n"+
		"      <p begin=\"00:00:01.500\" end=\"00:00:04.000\">Hello<br/><span tts:fontStyle=\"italic\">world</span></p>\n"+
		"      <p begin=\"00:00:05.000\" end=\"00:00:07.500\">Second cue</p>\n"+
		"    </div>\n  </body>\n</tt>\n",
		writeDefault(t, s))
}

// Ports testSingleQuotedColourBecomesAColourSpan of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestSingleQuotedColorBecomesAColorSpan(t *testing.T) {
	t.Parallel()

	out := writeDefault(t, subtitleOf(cue(time.Second, 2*time.Second, "<font color='#ff0000'>red</font>")))
	assert.Contains(t, out, `<p begin="00:00:01.000" end="00:00:02.000" region="bottomCenter"><span tts:color="#ff0000">red</span></p>`)
}

// Ports testSubtitleFromAnotherFormatGetsRegionsAgentsAndSpans of tests/Formatters/TtmlFormatterTest.php of
// subtitle-toolbox-php.
func TestSubtitleFromAnotherFormatGetsRegionsAgentsAndSpans(t *testing.T) {
	t.Parallel()

	s := readSRT(t, "1\n00:00:01,000 --> 00:00:02,000\n{\\an8}<v Fred>Hi & <b>bold <i>both</i></b>\n<font color=\"#ff0000\">red</font> <u>u</u> <s>s</s>\n\n"+
		"2\n00:00:03,000 --> 00:00:04,000\nplain\n")
	s.SetMetadata(model.MetadataTitle, "Trains & buses")
	s.SetMetadata(model.MetadataLanguage, "en")

	assert.Equal(t, xmlDeclaration+
		`<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en">`+"\n"+
		"  <head>\n"+
		"    <ttm:title>Trains &amp; buses</ttm:title>\n"+
		"    <ttm:agent xml:id=\"agent1\" type=\"person\"><ttm:name type=\"full\">Fred</ttm:name></ttm:agent>\n"+
		"    <layout>\n"+
		"      <region xml:id=\"topCenter\" tts:origin=\"10% 10%\" tts:extent=\"80% 80%\" tts:displayAlign=\"before\" tts:textAlign=\"center\"/>\n"+
		"      <region xml:id=\"bottomCenter\" tts:origin=\"10% 10%\" tts:extent=\"80% 80%\" tts:displayAlign=\"after\" tts:textAlign=\"center\"/>\n"+
		"    </layout>\n"+
		"  </head>\n"+
		"  <body>\n    <div>\n"+
		"      <p begin=\"00:00:01.000\" end=\"00:00:02.000\" region=\"topCenter\" ttm:agent=\"agent1\">"+
		"Hi &amp; <span tts:fontWeight=\"bold\">bold <span tts:fontStyle=\"italic\">both</span></span><br/>"+
		"<span tts:color=\"#ff0000\">red</span> <span tts:textDecoration=\"underline\">u</span>"+
		" <span tts:textDecoration=\"lineThrough\">s</span></p>\n"+
		"      <p begin=\"00:00:03.000\" end=\"00:00:04.000\" region=\"bottomCenter\">plain</p>\n"+
		"    </div>\n  </body>\n</tt>\n",
		writeDefault(t, s))
}

// Ports testChangedAlignmentReplacesTheStoredRegion of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestChangedAlignmentReplacesTheStoredRegion(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/pysubs2_regions.ttml", false)
	unchanged := writeDefault(t, s)
	setCue(t, s, 0, func(c *model.Cue) { c.Alignment = model.BottomCenter })
	setCue(t, s, 1, func(c *model.Cue) { c.Alignment = model.TopRight })
	out := writeDefault(t, s)

	assert.Contains(t, out, `<p xml:id="subtitle1" begin="00:00:01.375" end="00:00:05.750" style="s1" region="bottomCenter">`)
	assert.Contains(t, out, `<p xml:id="subtitle2" begin="00:00:05.916" end="00:00:10.216" style="s1" tts:textAlign="right" region="topRight">`)
	tail := func(s string) []string { lines := strings.Split(s, "\n"); return lines[len(lines)-11:] }
	assert.Equal(t, tail(unchanged), tail(out))

	back, _ := mustRead(t, out, false)
	var alignments []model.Alignment
	for _, c := range back.Cues()[:4] {
		alignments = append(alignments, c.Alignment)
	}
	assert.Equal(t, []model.Alignment{model.BottomCenter, model.TopRight, model.TopCenter, model.BottomCenter}, alignments)
}

// Ports testOutputParsesBackToTheSameCues of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestOutputParsesBackToTheSameCues(t *testing.T) {
	t.Parallel()

	first := cue(time.Second, 2*time.Second, "<v Ann>one</v> <v Ben>two", "<b>bold <i>open")
	first.Alignment = model.MiddleLeft
	s := subtitleOf(first, cue(3*time.Second, 4*time.Second, "<i>mis<b>nested</i>tags</b> &lt;3 <c.yellow>class</c> <00:00:03.500>word"))

	back, _ := mustRead(t, writeDefault(t, s), false)
	cues := back.Cues()
	require.Len(t, cues, 2)
	assert.Equal(t, "<v Ann>one </v><v Ben>two\n<b>bold <i>open</i></b>", cues[0].Text())
	assert.Equal(t, model.MiddleLeft, cues[0].Alignment)
	assert.Equal(t, "<i>mis</i><b><i>nested</i>tags</b> &lt;3 <font color=\"#ffff00\">class</font> word", cues[1].Text())
	assert.Equal(t, model.BottomCenter, cues[1].Alignment)
}

// Ports testAllXmlTagsAreStrippedAwayIfOptionIsSet of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestStripTagsWritesPlainText(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, twoParagraphs, false)
	assert.Contains(t, writeTTML(t, s, model.CommonWriteOptions{StripTags: true}), `<p begin="00:00:01.500" end="00:00:04.000">Hello<br/>world</p>`)
}

// Ports testKeepsTextThatLooksLikeATag of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestKeepsTextThatLooksLikeATag(t *testing.T) {
	t.Parallel()

	out := writeDefault(t, subtitleOf(cue(time.Second, 2*time.Second, "a < b > c", "<i>a</i>< b > c")))
	assert.Contains(t, out, `<p begin="00:00:01.000" end="00:00:02.000" region="bottomCenter">a &lt; b &gt; c<br/>`+
		`<span tts:fontStyle="italic">a</span>&lt; b &gt; c</p>`)
}

// Ports testDfxpFileKeepsItsNamespaceAndAgents of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestDFXPFileKeepsItsNamespaceAndAgents(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/2006/10/ttaf1" xmlns:m="http://www.w3.org/2006/10/ttaf1#metadata"`+
		` xmlns:ttp="http://www.w3.org/2006/10/ttaf1#parameter" ttp:timeBase="smpte" ttp:frameRate="25">`+"\n"+
		"  <head>\n    <m:agent xml:id=\"x\"><m:name>Ann</m:name></m:agent>\n  </head>\n"+
		"  <body><div><p begin=\"00:00:01:00\" end=\"00:00:02:00\" m:agent=\"x\">Hi</p></div></body>\n</tt>", false)
	out := writeDefault(t, s)

	assert.Contains(t, out, `<tt xmlns="http://www.w3.org/2006/10/ttaf1" xmlns:m="http://www.w3.org/2006/10/ttaf1#metadata"`+
		` xmlns:ttp="http://www.w3.org/2006/10/ttaf1#parameter" xmlns:tts="http://www.w3.org/2006/10/ttaf1#style"`+
		` xml:lang="" ttp:frameRate="25">`)
	assert.Contains(t, out, `<p begin="00:00:01.000" end="00:00:02.000" m:agent="x">Hi</p>`)
	assert.Equal(t, 1, strings.Count(out, "<m:agent "))
}

// Ports testChangedTitleReplacesTheStoredTitle of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestChangedTitleReplacesTheStoredTitle(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<tt xmlns=\"http://www.w3.org/ns/ttml\" xmlns:ttm=\"http://www.w3.org/ns/ttml#metadata\">\n"+
		"  <head>\n    <metadata>\n      <ttm:title>Old</ttm:title>\n    </metadata>\n  </head>\n"+
		"  <body><div><p begin=\"0s\" end=\"1s\">x</p></div></body>\n</tt>", false)
	s.SetMetadata(model.MetadataTitle, "New")
	out := writeDefault(t, s)

	assert.Contains(t, out, "<head>\n    <ttm:title>New</ttm:title>\n    <metadata>\n    </metadata>\n  </head>")
	assert.NotContains(t, out, "Old")
}

// Ports testEmptySubtitle of tests/Formatters/TtmlFormatterTest.php of subtitle-toolbox-php.
func TestEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, xmlDeclaration+ttmlRoot+">\n  <head/>\n  <body>\n    <div/>\n  </body>\n</tt>\n", writeDefault(t, &model.Subtitle{}))
}

// Ports testDuplicateCueIdsGetASuffix of tests/Formatters/TtmlUniqueIdsTest.php of subtitle-toolbox-php. The iTT
// rows come with the iTT writer.
func TestDuplicateCueIDsGetASuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file string
		want       []string
	}{
		{"astisub", "astisub_merging_style.ttml", []string{"sub_1", "sub_2", "sub_3", "sub_2_2"}},
		{"mantas", "mantas_duplicated_ids.ttml", []string{"c1", "c1_2", "c3"}},
		{"w3c", "w3c_imsc11_paragraphs.ttml", []string{"subtitle1", "subtitle1_2", "subtitle1_3", "subtitle1_4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, realDir+"/"+tt.file, false)
			got := ids(writeDefault(t, s))
			assert.Equal(t, tt.want, got[len(got)-len(tt.want):])
			assertUnique(t, got)
		})
	}
}

func assertUnique(t *testing.T, values []string) {
	t.Helper()

	seen := map[string]bool{}
	for _, v := range values {
		assert.False(t, seen[v], "the ID %q comes twice", v)
		seen[v] = true
	}
}

// Ports testCueIdEqualToANewRegionIdGetsASuffix of tests/Formatters/TtmlUniqueIdsTest.php of subtitle-toolbox-php.
func TestCueIDEqualToANewRegionIDGetsASuffix(t *testing.T) {
	t.Parallel()

	s := readVTT(t, "WEBVTT\n\nbottomCenter\n00:00:01.000 --> 00:00:02.000\nOne\n\nagent1\n00:00:03.000 --> 00:00:04.000\n<v Fred>Two\n")
	out := writeDefault(t, s)

	assert.Equal(t, []string{"agent1", "bottomCenter", "bottomCenter_2", "agent1_2"}, ids(out))
	assert.Contains(t, out, `<p xml:id="bottomCenter_2" begin="00:00:01.000" end="00:00:02.000" region="bottomCenter">One</p>`)
}

// Ports testStoredBodyAndDivIdsShareTheIdsWithTheHeadAndTheCues of tests/Formatters/TtmlUniqueIdsTest.php of
// subtitle-toolbox-php.
func TestStoredBodyAndDivIDsShareTheIDsWithTheHeadAndTheCues(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<tt xmlns=\"http://www.w3.org/ns/ttml\" xml:lang=\"en\">\n"+
		"  <head><styling><style xml:id=\"s1\"/></styling></head>\n"+
		"  <body xml:id=\"s1\"><div>\n"+
		"    <p xml:id=\"d1\" begin=\"1s\" end=\"2s\">One</p>\n"+
		"    <p xml:id=\"s1\" begin=\"3s\" end=\"4s\">Two</p>\n"+
		"  </div></body>\n</tt>", false)
	for i, space := range []string{"default", "preserve"} {
		setCue(t, s, i, func(c *model.Cue) {
			require.NoError(t, c.SetFormatData("ttml", model.NewData(model.Field{Key: "div", Value: attrs("xml:id", "d1", "xml:space", space)})))
		})
	}

	assert.Equal(t, []string{"s1", "s1_2", "d1", "d1_2", "d1_3", "s1_3"}, ids(writeDefault(t, s)))
}

// Ports testCharactersThatXmlForbidsAreRemoved of tests/Formatters/XmlOutputTest.php of subtitle-toolbox-php,
// with the text in Go in place of a YouTube file.
func TestCharactersThatXMLForbidsAreRemoved(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cue(time.Second, 2*time.Second, "Wel\x01come to\x0bthe bakery tour.\uffff"))
	s.SetMetadata(model.MetadataTitle, "Bakery\x0ctour")
	s.SetMetadata(model.MetadataLanguage, "en\x1f")
	out := writeDefault(t, s)

	assert.Contains(t, out, "Bakerytour")
	assert.Contains(t, out, `xml:lang="en"`)
	back, _ := mustRead(t, out, false)
	c, _ := back.Cue(0)
	assert.Equal(t, "Welcome tothe bakery tour.", c.Text())
}

// Ports testTextThatIsNotUtf8Throws and testTextReadWithItsEncodingIsWritten of tests/Formatters/XmlOutputTest.php
// of subtitle-toolbox-php.
func TestTextThatIsNotUTF8IsUnwritable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        func() *model.Subtitle
		cueIndex int
	}{
		{"text", func() *model.Subtitle {
			return subtitleOf(cue(0, time.Second, "ok"), cue(time.Second, 2*time.Second, "Caf\xe9"))
		}, 1},
		{"speaker", func() *model.Subtitle { return subtitleOf(cue(0, time.Second, "<v Jos\xe9>Hi")) }, 0},
		{"title", func() *model.Subtitle {
			s := subtitleOf(cue(0, time.Second, "Hi"))
			s.SetMetadata(model.MetadataTitle, "Caf\xe9")
			return s
		}, -1},
		{"language", func() *model.Subtitle {
			s := subtitleOf(cue(0, time.Second, "Hi"))
			s.SetMetadata(model.MetadataLanguage, "\xe9")
			return s
		}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uerr := writeError(t, tt.s())
			require.ErrorIs(t, uerr, model.ErrInvalidUTF8)
			assert.Equal(t, "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.", uerr.Reason)
			assert.Equal(t, tt.cueIndex, uerr.CueIndex)
		})
	}
}

// Ports testRealFileRoundTripKeepsCues of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileRoundTripKeepsCues(t *testing.T) {
	t.Parallel()

	for _, file := range realFiles(t) {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, realDir+"/"+file, false)
			out := writeDefault(t, s)
			back, _ := mustRead(t, out, false)

			want := make([]realCue, 0, s.Len())
			for _, c := range s.All() {
				r := describe(c)
				r.Text = wordTimestampRegexp.ReplaceAllString(r.Text, "")
				want = append(want, r)
			}
			got := make([]realCue, 0, back.Len())
			for _, c := range back.All() {
				got = append(got, describe(c))
			}
			assert.Equal(t, want, got)
			assert.Equal(t, s.AllMetadata(), back.AllMetadata())
			assert.Equal(t, out, writeDefault(t, back))
		})
	}
}

var wordTimestampRegexp = regexp.MustCompile(`<\d{2,}:[0-5]\d:[0-5]\d\.\d{3}>`)

// Ports testTtmlFormatterKeepsTheFileWhenTheFlagComesFromTheRegion of tests/ForcedCuesTest.php of
// subtitle-toolbox-php.
func TestForcedFlagFromTheRegionKeepsTheFile(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, "forced/w3c_imsc11_forced.ttml", false)
	out := writeDefault(t, s)
	assert.Equal(t, 1, strings.Count(out, "itts:forcedDisplay"))
	assert.Contains(t, out, "<p begin=\"00:00:01.000\" end=\"00:00:06.000\" region=\"r1\">Lyc\u00e9e</p>")
}

// srtWithThreeCues is the SubRip subtitle of tests/ForcedCuesTest.php of subtitle-toolbox-php.
const srtWithThreeCues = "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nEXIT\n\n3\n00:00:05,000 --> 00:00:06,000\nBye\n"

// Ports testTtmlFormatterWritesTheFlagOnTheParagraph of tests/ForcedCuesTest.php of subtitle-toolbox-php.
func TestForcedFlagOnTheParagraph(t *testing.T) {
	t.Parallel()

	s := readSRT(t, srtWithThreeCues)
	setCue(t, s, 1, func(c *model.Cue) { c.Forced = true })
	out := writeDefault(t, s)

	assert.Contains(t, out, ` xmlns:itts="http://www.w3.org/ns/ttml/profile/imsc1#styling"`)
	assert.Contains(t, out, `<p begin="00:00:03.000" end="00:00:04.000" region="bottomCenter" itts:forcedDisplay="true">EXIT</p>`)
	assert.Equal(t, 1, strings.Count(out, "itts:forcedDisplay"))
	assert.NotContains(t, writeDefault(t, readSRT(t, srtWithThreeCues)), "itts")
}

// Ports testTtmlFormatterWritesAClearedFlag of tests/ForcedCuesTest.php of subtitle-toolbox-php.
func TestForcedFlagCleared(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, "forced/forced_inheritance.ttml", false)
	setCue(t, s, 3, func(c *model.Cue) { c.Forced = false })
	setCue(t, s, 4, func(c *model.Cue) { c.Forced = true })
	out := writeDefault(t, s)

	assert.Contains(t, out, `<p begin="00:00:08.000" end="00:00:09.500" region="bottom" itts:forcedDisplay="false">BAHNHOF</p>`)
	assert.Contains(t, out, "<p begin=\"00:00:10.000\" end=\"00:00:12.000\" itts:forcedDisplay=\"true\" region=\"bottom\">Der Zug f\u00e4hrt um acht.</p>")
	back, _ := mustRead(t, out, false)
	assert.Equal(t, []bool{false, true, true, false, true}, forcedFlags(back))
}

// Ports the TTML rows of testRealFileRoundTripKeepsTheFlags of tests/ForcedCuesTest.php of subtitle-toolbox-php.
func TestForcedRealFileRoundTripKeepsTheFlags(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"w3c_imsc11_forced.ttml", "forced_inheritance.ttml"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, "forced/"+file, false)
			out := writeDefault(t, s)
			back, _ := mustRead(t, out, false)
			assert.Equal(t, forcedFlags(s), forcedFlags(back))
			var texts, backTexts []string
			for _, c := range s.All() {
				texts = append(texts, c.Text())
			}
			for _, c := range back.All() {
				backTexts = append(backTexts, c.Text())
			}
			assert.Equal(t, texts, backTexts)
			assert.Equal(t, out, writeDefault(t, back))
		})
	}
}

// Ports the TTML row of testJsonFrom3xKeepsTheStoredPositions of tests/Parsers/JsonRealFilesTest.php of
// subtitle-toolbox-php.
func TestJSONFrom3xKeepsTheStoredPositions(t *testing.T) {
	t.Parallel()

	fromJSON, _, err := stboxjson.Read(bytes.NewReader(testfiles.Read(t, "json/real/pysubs2_regions_3x.json")), model.CommonReadOptions{}, stboxjson.ReadOptions{})
	require.NoError(t, err)
	source, _ := readFile(t, realDir+"/pysubs2_regions.ttml", false)
	assert.Equal(t, writeDefault(t, source), writeDefault(t, fromJSON))
}

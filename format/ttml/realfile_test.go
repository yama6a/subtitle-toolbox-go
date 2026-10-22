package ttml_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type realCue struct {
	Start, End float64
	Text       string
	Alignment  model.Alignment
}

func describe(c model.Cue) realCue {
	return realCue{c.Start.Seconds(), c.End.Seconds(), c.Text(), c.Alignment}
}

// Ports testRealFileParses of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileParses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last realCue
	}{
		{"astisub_breaklines.ttml", 4, realCue{0, 1, "First line\nSecond line", model.BottomCenter}, realCue{3, 4, "Seventh line\nEighth middle line", model.BottomCenter}},
		{"astisub_merging_style.ttml", 4, realCue{0, 60, `<font color="#ffff00">text1.0 </font>text1.1`, model.TopCenter}, realCue{120, 180, "text3", model.TopCenter}},
		{"astisub_smpte.ttml", 6, realCue{99, 101.04, "(light rain)", model.TopCenter}, realCue{151.4, 153.44, "<font color=\"#ffff00\"><i>(music for the</i></font>\ntraffic news)", model.BottomCenter}},
		{"bbc_ebu_tt_d.ttml", 20, realCue{10, 13, "<v Anna>Good morning from the harbour.", model.BottomCenter}, realCue{76.12, 79.52, "<i>(music)</i>", model.TopCenter}},
		{"blank_lines_before_xml.ttml", 2, realCue{1, 3, "The market opens at eight.", model.BottomCenter}, realCue{3.5, 6, "<i>Fresh bread</i> is sold first.", model.BottomCenter}},
		{"flash_ttaf1_2006_04.xml", 4, realCue{0.5, 3, "Welcome to the second lesson.", model.BottomCenter}, realCue{8.4, 11, "<b>Save</b> your work before you go on.", model.BottomCenter}},
		{"mantas_dfxp_br.dfxp", 1, realCue{0, 1, "one\ntwo\nthree", model.BottomCenter}, realCue{0, 1, "one\ntwo\nthree", model.BottomCenter}},
		{"mantas_duplicated_ids.ttml", 3, realCue{0, 1, "First line.", model.BottomCenter}, realCue{2, 3, "Third line.", model.BottomCenter}},
		{"mantas_fps_multiplier.ttml", 1, realCue{15.015, 17.684, "First line.", model.BottomCenter}, realCue{15.015, 17.684, "First line.", model.BottomCenter}},
		{"mantas_netflix_ticks.dfxp", 2, realCue{137.4, 140.4, "The bakery's first bread\nis ready at six o'clock.", model.BottomCenter}, realCue{3740.5, 3742.5, "The last train leaves at midnight.", model.BottomCenter}},
		{"mantas_ttml2.ttml", 5, realCue{0, 2, "Hello I am your first line.", model.BottomCenter}, realCue{8, 10, `<font color="#ff0000">I am the last caption displayed in red and centered.</font>`, model.TopCenter}},
		{"pysubs2_regions.ttml", 10, realCue{1.375, 5.75, "TOP SAMPLE TEXT", model.TopCenter}, realCue{45.325, 50.041, "for the weekend market.", model.BottomCenter}},
		{"w3c_dfxp_timing.dfxp", 4, realCue{0, 2, `<font color="#ff0000"><b>Text 1</b></font>`, model.TopCenter}, realCue{1, 3, `<font color="#ff0000"><b>Text 4</b></font>`, model.TopCenter}},
		{"w3c_imsc11_frames.ttml", 3, realCue{1.01, 3, "This should appear on frame 25.", model.BottomCenter}, realCue{7.33, 9, "This should appear on frame 176.", model.BottomCenter}},
		{"w3c_imsc11_line_gaps.ttml", 1, realCue{0, 30, "##Line gaps##\nThe quick brown fox\njumps over the <font color=\"#000000\">lazy </font>dog\n##Line gaps##", model.BottomCenter}, realCue{0, 30, "##Line gaps##\nThe quick brown fox\njumps over the <font color=\"#000000\">lazy </font>dog\n##Line gaps##", model.BottomCenter}},
		{"w3c_imsc11_paragraphs.ttml", 4, realCue{0, 30, "Paragraph 1", model.BottomCenter}, realCue{0, 30, "Paragraph 2", model.TopCenter}},
		{"w3c_imsc11_regions.ttml", 3, realCue{0, 6, "This region is within the editorial area.", model.TopCenter}, realCue{0, 6, `<font color="#ffff00">This region is not.</font>`, model.BottomCenter}},
		{"w3c_ttml1_cells.ttml", 5, realCue{0, 8, "Lorem ipsum dolor sit", model.BottomCenter}, realCue{18, 29, "Ut enim ad minim veniam quis, nostrud", model.BottomCenter}},
		{"w3c_ttml1_timed_spans.ttml", 5, realCue{0, 25, "Lorem <00:00:01.000>ipsum <00:00:02.000>dolor <00:00:03.000>sit", model.BottomCenter}, realCue{0, 25, "<00:00:18.000>Ut <00:00:19.000>enim <00:00:20.000>ad <00:00:21.000>minim <00:00:22.000>veniam <00:00:23.000>quis, <00:00:24.000>nostrud", model.BottomCenter}},
		{"w3c_ttml1_timing.ttml", 4, realCue{0, 2, `<font color="#ff0000"><b>Text 1</b></font>`, model.TopCenter}, realCue{1, 3, `<font color="#ff0000"><b>Text 4</b></font>`, model.TopCenter}},
		{"style_inheritance.ttml", 5, realCue{1, 2, "<i>The ferry leaves at noon.</i>", model.BottomCenter}, realCue{9, 10, `<font color="#00ffff"><i>Next stop </i></font>harbour`, model.BottomCenter}},
		{"word_timed_spans.ttml", 3, realCue{1, 4.5, "The <00:00:02.000>tide <00:00:03.000>turns", model.BottomCenter}, realCue{10, 13, "Boats <00:00:11.000>return <00:00:12.000>home", model.BottomCenter}},
		{"smpte_drop_ntsc.ttml", 5, realCue{57.391, 60.027, "The morning train leaves platform two.", model.BottomCenter}, realCue{3599.996, 3602.999, "The evening train runs on time.", model.BottomCenter}},
		{"own_netflix_dfxp.dfxp", 5, realCue{2.002, 4.505, "The winter timetable\nstarts on Monday.", model.BottomCenter}, realCue{3600, 3602.503, "- Is the last train still at 11?\n- Yes, at 11:15.", model.BottomCenter}},
		{"own_youtube_ttml.ttml", 5, realCue{0.24, 3.12, "welcome back to the station channel", model.BottomCenter}, realCue{12, 15.36, "and the bus waits ten minutes &amp; then leaves", model.BottomCenter}},
		{"own_ttconv_imsc11.ttml", 10, realCue{17.985, 20.521, `<font color="#00ff00"><b>[train horn]</b></font>`, model.BottomCenter}, realCue{720, 780, "<i>x</i>^3 * <i>x</i> = 100", model.BottomCenter}},
		{"ttconv_referential_styling.ttml", 2, realCue{0, 2, `<font color="#008000">one</font>`, model.BottomCenter}, realCue{2, 4, `<font color="#000000">two</font>`, model.BottomCenter}},
		{"ttconv_lwsp_default.ttml", 2, realCue{0, 2, "hello my name is Mathilda", model.BottomCenter}, realCue{2, 4, "bonjour mon nom\nest", model.BottomCenter}},
		{"ttconv_lwsp_preserve.ttml", 1, realCue{0, 2, "hello my\nname is Mathilda", model.BottomCenter}, realCue{0, 2, "hello my\nname is Mathilda", model.BottomCenter}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, realDir+"/"+tt.file, false)
			require.Equal(t, tt.cues, s.Len())
			first, _ := s.Cue(0)
			last, _ := s.Cue(s.Len() - 1)
			assert.Equal(t, tt.first, describe(first))
			assert.Equal(t, tt.last, describe(last))
		})
	}
}

// Ports testRealFileSmpteDropFrameTimes of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileSMPTEDropFrameTimes(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/smpte_drop_ntsc.ttml", false)
	var times [][2]float64
	for _, c := range s.All() {
		times = append(times, [2]float64{c.Start.Seconds(), c.End.Seconds()})
	}
	assert.Equal(t, [][2]float64{{57.391, 60.027}, {60.06, 63.497}, {597.997, 599.999}, {599.999, 602.669}, {3599.996, 3602.999}}, times)
}

// Ports testRealFileEbuTtDAgentsTitleAndNestedSpans of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileEBUTTDAgentsTitleAndNestedSpans(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/bbc_ebu_tt_d.ttml", false)
	title, _ := s.Metadata(model.MetadataTitle)
	lang, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "Harbour weather", title)
	assert.Equal(t, "en-GB", lang)
	c2, _ := s.Cue(2)
	c10, _ := s.Cue(10)
	assert.Equal(t, "sub3", c2.ID)
	assert.Equal(t, "<v Ben>The first ferry left\nat seven o'clock.", c2.Text())
	assert.Equal(t, "<v Ben>Tide tables are <b>on the <u>board</u></b>", c10.Text())
	head := stringOf(t, fileData(t, s), "head")
	assert.NotContains(t, head, "ttm:title")
	assert.Contains(t, head, "ebuttm:documentMetadata")
}

// Ports testRealFileInheritsStylesFromRegionBodyAndDiv of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileInheritsStylesFromRegionBodyAndDiv(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/style_inheritance.ttml", false)
	var texts []string
	for _, c := range s.All() {
		texts = append(texts, c.Text())
	}
	assert.Equal(t, []string{
		"<i>The ferry leaves at noon.</i>",
		"Tickets are sold on board.",
		"<b><u>Mind the gap.</u></b>",
		`<font color="#ffff00">Platform </font>two`,
		`<font color="#00ffff"><i>Next stop </i></font>harbour`,
	}, texts)
}

// Ports testRealFileTakesTheCueTimesFromTimedSpansInStrictAndLenientMode of tests/Parsers/TtmlRealFileTest.php of
// subtitle-toolbox-php.
func TestRealFileTakesTheCueTimesFromTimedSpans(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		s, warnings := readFile(t, realDir+"/word_timed_spans.ttml", lenient)
		var cues []realCue
		for _, c := range s.All() {
			cues = append(cues, describe(c))
		}
		assert.Equal(t, []realCue{
			{1, 4.5, "The <00:00:02.000>tide <00:00:03.000>turns", model.BottomCenter},
			{5.5, 8, "at <00:00:06.500><i>dusk</i>", model.BottomCenter},
			{10, 13, "Boats <00:00:11.000>return <00:00:12.000>home", model.BottomCenter},
		}, cues)
		assert.Empty(t, warnings)
	}
}

// Ports testRealFileTicksAndPreservedSpace of tests/Parsers/TtmlRealFileTest.php of subtitle-toolbox-php.
func TestRealFileTicksAndPreservedSpace(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/mantas_netflix_ticks.dfxp", false)
	c, _ := s.Cue(0)
	d := cueData(t, c)
	assert.Equal(t, attrs("region", "bottomCenter"), objectOf(t, d, "attributes"))
	assert.Equal(t, attrs("xml:space", "preserve"), objectOf(t, d, "div"))
}

// Ports testRealFileWithoutNamespaceWritesNoInvalidIdentifier of tests/Parsers/TtmlRealFileTest.php of
// subtitle-toolbox-php, the read part.
func TestRealFileWithoutNamespace(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, realDir+"/astisub_breaklines.ttml", false)
	c, _ := s.Cue(0)
	assert.Equal(t, "1", c.ID)
	assert.Empty(t, stringOf(t, fileData(t, s), "namespace"))
}

// Ports testRealFileParses of tests/ForcedCuesTest.php of subtitle-toolbox-php, the TTML rows.
func TestRealFileForcedCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		first, last string
		flags       []bool
	}{
		{"w3c_imsc11_forced.ttml", "1s 6s Lycée", "4s 6s Nous étions inscrits au même lycée.", []bool{true, false}},
		{"forced_inheritance.ttml", "1s 3s Wir gehen zum Hafen.", "10s 12s Der Zug fährt um acht.", []bool{false, true, true, true, false}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, "forced/"+tt.file, false)
			first, _ := s.Cue(0)
			last, _ := s.Cue(s.Len() - 1)
			assert.Equal(t, tt.first, first.Start.String()+" "+first.End.String()+" "+first.Text())
			assert.Equal(t, tt.last, last.Start.String()+" "+last.End.String()+" "+last.Text())
			assert.Equal(t, tt.flags, forcedFlags(s))
		})
	}
}

func TestForcedCues(t *testing.T) {
	t.Parallel()

	const itts = ` xmlns:itts="http://www.w3.org/ns/ttml/profile/imsc1#styling"`
	tests := []struct {
		name, root, head, body string
		want                   []bool
	}{
		{"paragraph", itts, "", `<div><p end="1s" itts:forcedDisplay=" true ">a</p><p end="1s" itts:forcedDisplay="True">b</p></div>`, []bool{true, false}},
		{"div", itts, "", `<div itts:forcedDisplay="true"><p end="1s">a</p><p end="1s" itts:forcedDisplay="false">b</p></div>`, []bool{true, false}},
		{"nearest container", itts, "", `<div itts:forcedDisplay="true"><div itts:forcedDisplay="false"><p end="1s">a</p></div></div>`, []bool{false}},
		{
			"region", itts, `<layout><region xml:id="r" itts:forcedDisplay="true"/><region xml:id="q"><style itts:forcedDisplay="false"/><style itts:forcedDisplay="true"/></region></layout>`,
			`<div><p end="1s" region="r">a</p><p end="1s" region="q">b</p><p end="1s" region="r" itts:forcedDisplay="false">c</p></div>`,
			[]bool{true, false, false},
		},
		{
			"container wins over the region", itts, `<layout><region xml:id="r" itts:forcedDisplay="true"/></layout>`,
			`<div itts:forcedDisplay="false"><p end="1s" region="r">a</p></div>`,
			[]bool{false},
		},
		{
			"style reference, the last that sets it wins", itts,
			`<styling><style xml:id="f" itts:forcedDisplay="true"/><style xml:id="n" itts:forcedDisplay="false"/><style xml:id="x"/></styling>`,
			`<div><p end="1s" style="n f x">a</p><p end="1s" style="f n">b</p><p end="1s" style="f" itts:forcedDisplay="false">c</p></div>`,
			[]bool{true, false, false},
		},
		{
			"span", itts, "",
			`<div><p end="1s">a <span itts:forcedDisplay="true">b</span></p><p end="1s">a <span><span itts:forcedDisplay="true">b</span></span></p>` +
				`<p end="1s" itts:forcedDisplay="true"><span itts:forcedDisplay="false">b</span></p></div>`,
			[]bool{true, true, true},
		},
		{"undeclared prefix", "", "", `<div><p end="1s" itts:forcedDisplay="true">a</p></div>`, []bool{true}},
		{"other namespace", ` xmlns:itts="urn:x"`, "", `<div><p end="1s" itts:forcedDisplay="true">a</p></div>`, []bool{false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, document(tt.body, tt.root, tt.head), false)
			assert.Equal(t, tt.want, forcedFlags(s))
		})
	}
}

func forcedFlags(s *model.Subtitle) []bool {
	var flags []bool
	for _, c := range s.All() {
		flags = append(flags, c.Forced)
	}
	return flags
}

func TestRealFilesReadInLenientModeWithoutWarnings(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		_, warnings := readFile(t, realDir+"/"+name, true)
		assert.Empty(t, warnings, name)
	}
}

func realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, realDir))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	return names
}

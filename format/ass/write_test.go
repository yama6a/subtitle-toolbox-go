package ass_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func writeDefault(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	return write(t, s, model.CommonWriteOptions{}, ass.WriteOptions{})
}

func writeStyle(t *testing.T, s *model.Subtitle, style string) string {
	t.Helper()

	return write(t, s, model.CommonWriteOptions{}, ass.WriteOptions{Style: style})
}

// Ports testRealFileSurvivesARoundTrip of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestRealFileSurvivesARoundTrip(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"own_aegisub.ass", "own_ffmpeg.ass", "own_signs_crlf.ass", "own_ssa_v4.ssa", "own_style_flags.ass"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+file)
			written := writeDefault(t, s)
			back, warnings := mustRead(t, written, false)
			assert.Empty(t, warnings)

			assert.Equal(t, s.Cues(), back.Cues())
			assert.Equal(t, s.Comments(), back.Comments())
			assert.Equal(t, s.AllMetadata(), back.AllMetadata())
			assert.Equal(t, fileData(t, s), fileData(t, back))
			assert.Equal(t, written, writeDefault(t, back))
		})
	}
}

// Ports testSortedRealFileIsWrittenBackWithBomAndLineFeeds of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestSortedRealFileIsWrittenBackWithBOMAndLineFeeds(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"own_aegisub.ass", "own_ffmpeg.ass", "own_ssa_v4.ssa", "own_style_flags.ass"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			raw := string(testfiles.Read(t, realDir+"/"+file))
			want := "\xEF\xBB\xBF" + strings.ReplaceAll(strings.TrimPrefix(raw, "\xEF\xBB\xBF"), "\r\n", "\n")
			assert.Equal(t, want, writeDefault(t, readFile(t, realDir+"/"+file)))
		})
	}
}

// Ports testUnsortedEventsAreWrittenInTimeOrder of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestUnsortedEventsAreWrittenInTimeOrder(t *testing.T) {
	t.Parallel()

	got := writeDefault(t, readFile(t, realDir+"/own_signs_crlf.ass"))
	assert.Contains(t, got, `Dialogue: 0,0:00:04.50,0:00:07.50,Default,Reporter,0,0,0,,Clouds move in from the west\nduring the night.`+"\n"+
		`Dialogue: 1,0:00:05.00,0:00:09.00,Sign,,0,0,0,,{\pos(640,90)\fad(200,200)\t(0,500,\1c&H00FFFF&)}SUNNY{\fs24}\N{\fs}25 `+"\xc2\xb0C\n")
	assert.Contains(t, got, "[Fonts]\nfontname: WeatherSans_0.ttf\n")
}

// Ports testBottomCenterFromAnotherFormatGetsNoAlignmentTag of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestBottomCenterFromAnotherFormatGetsNoAlignmentTag(t *testing.T) {
	t.Parallel()

	s, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(t, "vtt/real/w3c_cue_settings.vtt")), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, model.BottomCenter, cueAt(t, s, 2).Alignment)
	assert.True(t, strings.HasSuffix(writeDefault(t, s), "Dialogue: 0,0:00:04.00,0:00:06.50,Default,,0,0,0,,What are you waiting for?\n"))
}

// Ports testCuesFromOtherFormatsGetAMinimalHeader of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestCuesFromOtherFormatsGetTheDefaultHeader(t *testing.T) {
	t.Parallel()

	top := cue(3004*ms, 4996*ms, "Top")
	top.Alignment = model.TopCenter
	s := subtitle(t, cue(time.Second, 2500*ms, "<i>Hello</i>", "world"), top)
	assert.Equal(t, defaultHeader+
		`Dialogue: 0,0:00:01.00,0:00:02.50,Default,,0,0,0,,{\i1}Hello{\i0}\Nworld`+"\n"+
		`Dialogue: 0,0:00:03.00,0:00:05.00,Default,,0,0,0,,{\an8}Top`+"\n",
		writeDefault(t, s))
}

// Ports testStyleOptionChangesTheFieldsOfTheDefaultStyle of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestStyleOptionChangesTheFieldsOfTheDefaultStyle(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(time.Second, 2500*ms, "Hello"))
	header := strings.Replace(strings.Replace(defaultHeader, ",1,1,0,2,", ",1,2,0,2,", 1), ",Arial,16,", ",Roboto,48,", 1)
	assert.Equal(t, header+"Dialogue: 0,0:00:01.00,0:00:02.50,Default,,0,0,0,,Hello\n",
		writeStyle(t, s, "Fontname=Roboto, fontsize = 48,Outline=2"))
}

// Ports testStyleOptionSetsTheLookOfCuesWithoutWritingResetTags of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestStyleOptionSetsTheLookOfCuesWithoutResetTags(t *testing.T) {
	t.Parallel()

	got := writeStyle(t, subtitle(t, cue(time.Second, 2*time.Second, "Hello"), cue(3*time.Second, 4*time.Second, "<i>Hello</i> world")), "Italic=-1,Alignment=8")
	assert.Contains(t, got, "Style: Default,Arial,16,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,-1,0,0,100,100,0,0,1,1,0,8,")
	assert.Contains(t, got, "Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello\n")
	assert.Contains(t, got, `Dialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,{\i1}Hello{\i0} world`+"\n")
}

// Ports testStyleOptionChangesOnlyTheDefaultStyleOfAnAssFile of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestStyleOptionChangesOnlyTheDefaultStyleOfAnASSFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_aegisub.ass")
	original := writeDefault(t, s)
	want := strings.Replace(strings.Replace(original, ",1,3.5,1.5,2,60,60,50,1", ",1,3.5,1.5,2,60,60,80,1", 1),
		"Style: Default,Arial,72,", "Style: Default,Arial,60,", 1)
	assert.Equal(t, want, writeStyle(t, s, "Fontsize=60,MarginV=80"))
}

// Ports testStyleOptionAddsADefaultStyleWhenTheFileHasNone and testStyleOptionAddsAStylesSectionWhenTheFileHasNone
// of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestStyleOptionAddsWhatTheFileLacks(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[V4+ Styles]\nFormat: Name, Fontname, Italic\nStyle: Sign,Arial,0\n\n"+eventsHeader+
		"Dialogue: 0,0:00:01.00,0:00:02.00,Sign,,0,0,0,,Hello\n", false)
	assert.Contains(t, writeStyle(t, s, "Italic=-1"), "Format: Name, Fontname, Italic\nStyle: Default,Arial,-1\nStyle: Sign,Arial,0\n")

	s, _ = mustRead(t, "[Events]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello\n", false)
	got := writeStyle(t, s, "Fontsize=30")
	assert.Contains(t, got, "[Script Info]\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize,")
	assert.Contains(t, got, "Style: Default,Arial,30,")

	s, _ = mustRead(t, "[Script Info]\nScriptType: V4.00\n\n[Events]\nDialogue: Marked=0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello\n", false)
	got = writeStyle(t, s, "Fontsize=30")
	assert.Contains(t, got, "\n\n[V4 Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour,")
	assert.Contains(t, got, "Style: Default,Arial,30,&H00FFFFFF,&H00FFFFFF,0,&H00000000,0,0,1,1,0,2,10,10,10,0,1\n\n[Events]\n")
}

// Ports testStyleOptionRejectsAnUnknownField and testStyleOptionThrowsForAFieldThatTheSsaStylesDoNotHave of
// tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestStyleOptionErrors(t *testing.T) {
	t.Parallel()

	fields := "Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, " +
		"StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding"
	tests := []struct {
		name  string
		style string
		msg   string
	}{
		{"unknown field", "Fontsize=48,Bogus=1", `The ASS style field "Bogus" does not exist. Use one of: ` + fields + "."},
		{"the Name field", "Name=Other", `The ASS style field "Name" does not exist. Use one of: ` + fields + "."},
		{"pair without =", "Fontsize=48,Bold", `The ASS style "Fontsize=48,Bold" must hold Field=Value pairs such as Fontsize=48, got "Bold".`},
		{"empty pair", "Fontsize=48,", `The ASS style "Fontsize=48," must hold Field=Value pairs such as Fontsize=48, got "".`},
		{"line break in a value", "Fontname=A\nB", `The ASS style "Fontname=A` + "\n" + `B" must hold Field=Value pairs such as Fontsize=48, got "Fontname=A` + "\n" + `B".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ass.WriteOptions{Style: tt.style}.Validate()
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, "WriteOptions.ASS.Style", aerr.Option)
			assert.Equal(t, tt.msg, aerr.Msg)
			require.ErrorIs(t, ass.Write(&bytes.Buffer{}, &model.Subtitle{}, model.CommonWriteOptions{}, ass.WriteOptions{Style: tt.style}), model.ErrInvalidArgument)
		})
	}

	err := ass.Write(&bytes.Buffer{}, readFile(t, realDir+"/own_ssa_v4.ssa"), model.CommonWriteOptions{}, ass.WriteOptions{Style: "underline=-1"})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	require.ErrorIs(t, err, model.ErrUnwritable)
	assert.Equal(t, "The Format line of the styles has no field Underline. It holds: Name, Fontname, Fontsize, PrimaryColour, "+
		"SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, "+
		"MarginV, AlphaLevel, Encoding.", uerr.Reason)
	assert.Equal(t, model.ASS, uerr.Format)
}

func TestStyleOptionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, style, want string
	}{
		{"spaces around the pair", "  Fontname =  Open Sans  ", "Style: Default,Open Sans,16,"},
		{"a later pair wins", "Fontsize=20,FONTSIZE=30", "Style: Default,Arial,30,"},
		{"empty value", "Fontname=", "Style: Default,,16,"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, ass.WriteOptions{Style: tt.style}.Validate())
			assert.Contains(t, writeStyle(t, subtitle(t, cue(time.Second, 2*time.Second, "a")), tt.style), tt.want)
		})
	}
}

func TestStyleOptionChangesEveryDefaultStyle(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[V4+ Styles]\nFormat: Name, Fontsize\nStyle: Default,10\nStyle: *default,11\nStyle: Sign,12\n\n"+eventsHeader, false)
	assert.Contains(t, writeStyle(t, s, "Fontsize=40"), "Style: Default,40\nStyle: *default,40\nStyle: Sign,12\n")
}

func TestKaraokeTagValidate(t *testing.T) {
	t.Parallel()

	for _, k := range []ass.KaraokeTag{"", ass.KaraokeInstant, ass.KaraokeFill, ass.KaraokeOutline} {
		require.NoError(t, ass.WriteOptions{KaraokeTag: k}.Validate())
	}
	err := ass.WriteOptions{KaraokeTag: "kx", Style: "Bogus=1"}.Validate()
	var aerr *model.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "WriteOptions.ASS.KaraokeTag", aerr.Option)
	assert.Equal(t, `The ASS karaoke tag "kx" is unknown. Use one of: k, kf, ko.`, aerr.Msg)

	assert.Equal(t, "k", ass.KaraokeTag("").String())
	assert.Equal(t, "kf", ass.KaraokeFill.String())
}

// Ports testCoreMarkupBecomesOverrideTags of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestCoreMarkupBecomesOverrideTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		text  string
		who   string
	}{
		{"underline and strike", []string{"<u>a</u> <s>b</s>"}, `{\u1}a{\u0} {\s1}b{\s0}`, ""},
		{"colour as BGR", []string{`<font color="#FF8000">a</font>`}, `{\c&H0080FF&}a{\c}`, ""},
		{"single quoted colour", []string{`<font color='#FF8000'>a</font>`}, `{\c&H0080FF&}a{\c}`, ""},
		{"inner colour restores", []string{`<font color="#ff0000">a<font color="#00ff00">b</font>c</font>`}, `{\c&H0000FF&}a{\c&H00FF00&}b{\c&H0000FF&}c{\c}`, ""},
		{"font without colour", []string{`<font face="Arial">a</font>`}, "a", ""},
		{"colour inside a font without colour", []string{`<font color="#ff0000"><font face="Arial">a</font>b</font>`}, `{\c&H0000FF&}ab{\c}`, ""},
		{"upper case font tag", []string{`<FONT COLOR="#FF8000">a</FONT>`}, `{\c&H0080FF&}a{\c}`, ""},
		{"spaces around equals", []string{`<font color = "#ff8000" >a</font>`}, `{\c&H0080FF&}a{\c}`, ""},
		{"last tag of a kind wins", []string{"<b>a</b><b>b</b>"}, `{\b1}a{\b1}b{\b0}`, ""},
		{"other tags are stripped", []string{"<c.loud>a</c> <lang en>b</lang>"}, "a b", ""},
		{"webvtt color class", []string{"<c.loud.yellow>a</c> b"}, `{\c&H00FFFF&}a{\c} b`, ""},
		{"entities are decoded", []string{"1 &lt; 2 &amp;&amp; 3&nbsp;&gt; 2"}, `1 < 2 && 3\h> 2`, ""},
		{"text like a tag", []string{"a < b > c"}, "a < b > c", ""},
		{"text like a tag after", []string{"<i>a</i>< b > c"}, `{\i1}a{\i0}< b > c`, ""},
		{"speaker", []string{"<v.loud Fred, Jr.>Hi", "there"}, `Hi\Nthere`, "Fred Jr."},
		{"ruby as text", []string{"<ruby>K<rt>kan</rt></ruby>"}, "K (kan)", ""},
		{"braces and backslashes stay raw", []string{`{a} \N`}, `{a} \N`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(t, cue(time.Second, 2*time.Second, tt.lines...))
			assert.Equal(t, defaultHeader+"Dialogue: 0,0:00:01.00,0:00:02.00,Default,"+tt.who+",0,0,0,,"+tt.text+"\n", writeDefault(t, s))
		})
	}
}

// Ports testWordTimestampsBecomeKaraokeTags of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestWordTimestampsBecomeKaraokeTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		text string
	}{
		{"timestamp at the start", "<00:00:10.000>Ka<00:00:10.500>ra<00:00:10.754>o", `{\k50}Ka{\k25}ra{\k125}o`},
		{"text before the first", "Ka<00:00:10.300>ra", `{\k30}Ka{\k170}ra`},
		{"timestamp before start", "<00:00:09.000>Ka<00:00:11.000>ra", `{\k100}Ka{\k100}ra`},
		{"timestamp after end", "<00:00:10.000>Ka<00:00:13.000>ra", `{\k300}Ka{\k0}ra`},
		{"text before a timestamp at the start gets a gap of 0", "Ka<00:00:10.000>ra", `{\k0}Ka{\k200}ra`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(t, cue(10*time.Second, 12*time.Second, tt.line))
			assert.Equal(t, defaultHeader+"Dialogue: 0,0:00:10.00,0:00:12.00,Default,,0,0,0,,"+tt.text+"\n", writeDefault(t, s))
		})
	}
}

func TestKaraokeTagOption(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(10*time.Second, 12*time.Second, "Ka<00:00:10.300>ra"))
	for _, tt := range []struct {
		tag  ass.KaraokeTag
		want string
	}{
		{ass.KaraokeFill, `{\kf30}Ka{\kf170}ra`},
		{ass.KaraokeOutline, `{\ko30}Ka{\ko170}ra`},
	} {
		assert.Contains(t, write(t, s, model.CommonWriteOptions{}, ass.WriteOptions{KaraokeTag: tt.tag}), ",,"+tt.want+"\n")
	}
}

// Ports testUnchangedCueKeepsItsOriginalText of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestUnchangedCueKeepsItsOriginalText(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_signs_crlf.ass")
	require.NoError(t, s.Shift(time.Second))
	assert.Contains(t, writeDefault(t, s), `Dialogue: 0,0:00:06.00,0:00:10.00,Sign,,0,0,0,,{\an7\pos(40,40)\p1\bord0\c&HFFFFFF&}m 0 0 l 200 0 200 60 0 60{\p0}`+"\n")
}

// Ports testChangedCueIsWrittenFromItsLines of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestChangedCueIsWrittenFromItsLines(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_aegisub.ass")
	setCue(t, s, 2, func(c *model.Cue) { c.Lines = []string{"<v Guard>Is it <i>late</i> today?"} })
	setCue(t, s, 3, func(c *model.Cue) { c.Alignment = model.TopLeft })

	got := writeDefault(t, s)
	assert.Contains(t, got, `Dialogue: 0,0:00:06.30,0:00:08.00,Default,Guard,0,0,0,,Is it {\i1}late{\i0} today?`+"\n")
	assert.Contains(t, got, `Dialogue: 0,0:00:08.10,0:00:10.90,Top,,0,0,0,,{\an7}Platform 4: {\c&H00D7FF&}Coast Express{\c}`+"\n")
}

// Ports testChangedCueOmitsTheTagsThatItsStyleImplies of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestChangedCueOmitsTheTagsThatItsStyleImplies(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_style_flags.ass")
	setCue(t, s, 1, func(c *model.Cue) { c.Lines = []string{"<i>I should have taken the night bus.</i>"} })
	setCue(t, s, 2, func(c *model.Cue) { c.Lines = []string{"Upright now"} })
	setCue(t, s, 3, func(c *model.Cue) { c.Alignment = model.BottomCenter })
	setCue(t, s, 5, func(c *model.Cue) { c.Lines = []string{"<i>Left</i> again"} })

	got := writeDefault(t, s)
	assert.Contains(t, got, ",Thoughts,,0,0,0,,I should have taken the night bus.\n")
	assert.Contains(t, got, `,Thoughts,,0,0,0,,{\an2\i0}Upright now`+"\n")
	assert.Contains(t, got, `,Sign,,0,0,0,,{\an2}Depot closed for repairs`+"\n")
	assert.Contains(t, got, `,Thoughts,,0,0,0,,{\i1}Left{\i0} again`+"\n")

	back, _ := mustRead(t, got, false)
	lines := make([][]string, 0, 4)
	for _, i := range []int{1, 2, 3, 5} {
		lines = append(lines, cueAt(t, back, i).Lines)
	}
	assert.Equal(t, [][]string{{"<i>I should have taken the night bus.</i>"}, {"Upright now"}, {"<b><u><s>Depot closed for repairs</s></u></b>"}, {"<i>Left</i> again"}}, lines)
}

// Ports testSsaUsesLegacyAlignmentAndKeepsMarkedColumn of tests/Formatters/AssFormatterTest.php of
// subtitle-toolbox-php.
func TestSSAUsesLegacyAlignmentAndKeepsTheMarkedColumn(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_ssa_v4.ssa")
	setCue(t, s, 1, func(c *model.Cue) { c.Alignment = model.MiddleLeft })
	assert.Contains(t, writeDefault(t, s), `Dialogue: Marked=0,0:00:05.50,0:00:08.00,Notice,,0000,0000,0000,,{\a9}Next ferry: 9:00`+"\n")
}

// Ports testStripAllTagsKeepsSpeaker of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestStripTagsKeepsTheSpeaker(t *testing.T) {
	t.Parallel()

	got := write(t, readFile(t, realDir+"/own_aegisub.ass"), model.CommonWriteOptions{StripTags: true}, ass.WriteOptions{})
	assert.Contains(t, got, "Dialogue: 0,0:00:06.30,0:00:08.00,Default,Passenger,0,0,0,,Is it on time today?\n")
	assert.Contains(t, got, "Dialogue: 0,0:00:08.10,0:00:10.90,Top,,0,0,0,,Platform 4: Coast Express\n")
}

// Ports testTitleAndNewCommentsAreWritten of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestTitleAndNewCommentsAreWritten(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(time.Second, 2*time.Second, "a"))
	s.SetMetadata(model.MetadataTitle, "Bakery\nnews")
	require.NoError(t, s.AddComment("first", 0))
	require.NoError(t, s.AddComment("last\nline", 1))

	got := writeDefault(t, s)
	assert.True(t, strings.HasPrefix(got, "\xEF\xBB\xBF[Script Info]\nTitle: Bakery news\nScriptType: v4.00+\n"))
	assert.True(t, strings.HasSuffix(got, "Comment: 0,0:00:01.00,0:00:01.00,Default,,0,0,0,,first\n"+
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,a\n"+
		`Comment: 0,0:00:02.00,0:00:02.00,Default,,0,0,0,,last\Nline`+"\n"))

	empty := &model.Subtitle{}
	require.NoError(t, empty.AddComment("alone", 0))
	assert.True(t, strings.HasSuffix(writeDefault(t, empty), "Comment: 0,0:00:00.00,0:00:00.00,Default,,0,0,0,,alone\n"))
}

func TestStoredCommentEventIsUsedOnce(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, eventsHeader+"Comment: 1,0:00:00.50,0:00:09.00,Note,,0,0,0,,x\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,a\n", false)
	require.NoError(t, s.AddComment("x", 1))
	assert.True(t, strings.HasSuffix(writeDefault(t, s), "Comment: 1,0:00:00.50,0:00:09.00,Note,,0,0,0,,x\n"+
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,a\n"+
		"Comment: 0,0:00:02.00,0:00:02.00,Default,,0,0,0,,x\n"))
}

// Ports testTimesAreRoundedToCentiseconds of tests/Formatters/AssFormatterTest.php of subtitle-toolbox-php.
func TestTimesAreRoundedToCentiseconds(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(3599996*ms, 36000004*ms, "a"), cue(-time.Second, 5*ms, "b"), cue(1005*ms, 1015*ms, "c"))
	got := writeDefault(t, s)
	assert.Contains(t, got, "Dialogue: 0,0:00:01.01,0:00:01.02,Default,,0,0,0,,c\n", "half a centisecond rounds away from zero")
	assert.Contains(t, got, "Dialogue: 0,0:00:00.00,0:00:00.01,Default,,0,0,0,,b\n")
	assert.Contains(t, got, "Dialogue: 0,1:00:00.00,10:00:00.00,Default,,0,0,0,,a\n")
}

func TestDefaultFieldValues(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[Events]\nFormat: Marked, Start, End, Text\nDialogue: Marked=1,0:00:01.00,0:00:02.00,a\n", false)
	s.AddCue(cue(3*time.Second, 4*time.Second, "b"))
	data := fileData(t, s).Clone()
	data.Set("eventFormat", []any{"Marked", "Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"})
	require.NoError(t, s.SetFormatData("ass", data))
	got := writeDefault(t, s)
	assert.Contains(t, got, "Dialogue: Marked=1,0,0:00:01.00,0:00:02.00,Default,,0,0,0,,a\n")
	assert.Contains(t, got, "Dialogue: Marked=0,0,0:00:03.00,0:00:04.00,Default,,0,0,0,,b\n")
}

// The unchanged rule of https://github.com/yama6a/subtitle-toolbox-go/issues/22.
func TestUnchangedRule(t *testing.T) {
	t.Parallel()

	content := eventsHeader + `Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\an8\fs40}Hi` + "\n" +
		`Dialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,{\fs40}Bye` + "\n"
	tests := []struct {
		name   string
		change func(*model.Cue)
		strip  bool
		want   string
	}{
		{"no change", func(*model.Cue) {}, false, `{\an8\fs40}Hi`},
		{"changed lines", func(c *model.Cue) { c.Lines = []string{"Hello"} }, false, `{\an8}Hello`},
		{"changed alignment", func(c *model.Cue) { c.Alignment = model.TopLeft }, false, `{\an7}Hi`},
		{"strip tags keeps the alignment tag", func(*model.Cue) {}, true, `{\an8}Hi`},
		{"no stored text", func(c *model.Cue) {
			d := cueData(t, *c).Clone()
			d.Delete("text")
			require.NoError(t, c.SetFormatData("ass", d))
		}, false, `{\an8}Hi`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, content, false)
			setCue(t, s, 0, tt.change)
			got := write(t, s, model.CommonWriteOptions{StripTags: tt.strip}, ass.WriteOptions{})
			assert.Contains(t, got, "Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,"+tt.want+"\n")
		})
	}

	s, _ := mustRead(t, content, false)
	assert.Contains(t, writeDefault(t, s), `0:00:03.00,0:00:04.00,Default,,0,0,0,,{\fs40}Bye`+"\n", "a null alignment counts as 2")
}

func TestSectionOrder(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(time.Second, 2*time.Second, "a"))
	require.NoError(t, s.SetFormatData("ass", model.NewData(
		model.Field{Key: "sectionOrder", Value: []any{"Fonts", "Fonts", "Graphics"}},
		model.Field{Key: "scriptInfo", Value: []any{}},
		model.Field{Key: "sections", Value: model.NewData(model.Field{Key: "Fonts", Value: []any{"fontname: a.ttf"}})},
	)))
	assert.Equal(t, "\xEF\xBB\xBF[Script Info]\n\n[Fonts]\nfontname: a.ttf\n\n[Graphics]\n\n[Events]\n"+
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,a\n", writeDefault(t, s))
}

func TestWriteOptionsOfOutput(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(time.Second, 2*time.Second, "a"))
	got := write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, ass.WriteOptions{})
	assert.True(t, strings.HasPrefix(got, "[Script Info]\r\nScriptType: v4.00+\r\n"))
	assert.True(t, strings.HasSuffix(got, ",a\r\n"))
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, ass.Write(errWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, ass.WriteOptions{}), errRead)
}

// Ports the ASS row of tests/Formatters/RubyFallbackTest.php of subtitle-toolbox-php.
func TestRubyFixture(t *testing.T) {
	t.Parallel()

	s, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(t, "ruby/own_ruby.vtt")), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(testfiles.Read(t, "ruby/own_ruby_expected.ass")), write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}, ass.WriteOptions{}))
}

// Ports the ASS rows of tests/QuotesInTagsTest.php of subtitle-toolbox-php.
func TestNamesWithQuotes(t *testing.T) {
	t.Parallel()

	s := readFile(t, "quotes-in-tags/own_names_with_quotes.ass")
	assert.Equal(t, [][]string{
		{"<v O'Neil>Hi."},
		{"<v O'Neil>We're out of rye bread today."},
		{`<v Sam "Ace" Reed>Then I'll take the "seeded" loaf.`},
		{`<v Mo "Baker>Two <i>warm</i> rolls.`},
		{`<v D'Arcy><font color="#ff0000">Red</font> jam, please.`},
	}, linesOf(s))
	written := writeDefault(t, s)
	assert.Contains(t, written, `,Default,Sam "Ace" Reed,0,0,0,,Then I'll take the "seeded" loaf.`)

	for _, strip := range []bool{false, true} {
		out := write(t, subtitle(t, cue(time.Second, 2*time.Second, "<v O'Neil>We're out of rye."), cue(3*time.Second, 4*time.Second, `<v Mo "Baker>Two rolls.`)),
			model.CommonWriteOptions{StripTags: strip}, ass.WriteOptions{})
		assert.Contains(t, out, ",O'Neil,0,0,0,,We're out of rye.\n")
		assert.Contains(t, out, `,Mo "Baker,0,0,0,,Two rolls.`+"\n")
	}
}

// Ports the ASS rows of testEveryFontColorSyntaxKeepsTheColor and testEveryWriterReadsAColorWithTheSameRule of
// tests/FontColorTest.php of subtitle-toolbox-php.
func TestFontColorsSurviveARoundTrip(t *testing.T) {
	t.Parallel()

	type row struct{ line, want string }
	tests := make([]row, 0, 11)
	tests = append(tests, []row{
		{"<font color='#FF0000'>Red</font>", `<font color="#ff0000">Red</font>`},
		{"<font color=#ff0000>Red</font>", `<font color="#ff0000">Red</font>`},
		{`<FONT COLOR="#FF0000">Red</FONT>`, `<font color="#ff0000">Red</font>`},
		{`<font color = "#ff0000" >Red</font>`, `<font color="#ff0000">Red</font>`},
		{`<font face="Arial" color="#ff0000">Red</font>`, `<font color="#ff0000">Red</font>`},
	}...)
	for _, color := range []string{"#ffff00", "#FF0", "yellow", "Yellow", "rgb(255, 255, 0)", "&#35;ff0"} {
		tests = append(tests, row{`<font color="` + color + `">Hi</font>`, `<font color="#ffff00">Hi</font>`})
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()

			back, _ := mustRead(t, writeDefault(t, subtitle(t, cue(time.Second, 2*time.Second, tt.line))), false)
			assert.Equal(t, tt.want, cueAt(t, back, 0).Text())
		})
	}
}

// Ports the ASS row of testAWebVttColorClassBecomesTheTextColor of tests/Formatters/TextColorTest.php of
// subtitle-toolbox-php.
func TestAWebVTTColorClassBecomesTheTextColor(t *testing.T) {
	t.Parallel()

	s, _, err := webvtt.Read(strings.NewReader("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n<c.yellow>Hi</c>\n"), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	assert.Contains(t, writeDefault(t, s), `,,{\c&H00FFFF&}Hi{\c}`+"\n")
}

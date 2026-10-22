package microdvd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func writeAt(t *testing.T, s *model.Subtitle, fps float64) string {
	t.Helper()

	return write(t, s, model.CommonWriteOptions{}, microdvd.WriteOptions{FrameRate: fps})
}

// Ports testRealFileRoundTrips of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestRealFileRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		fps  float64
	}{
		{"mantas-done plain", "sub_microdvd.sub", 23.976},
		{"mantas-done styles", "sub_microdvd_with_styles.sub", 23.976},
		{"subsrt sample", "subsrt_sample.sub", 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := string(testfiles.Read(t, realDir+"/"+tt.file))
			s, _ := mustRead(t, raw, false, tt.fps)
			assert.Equal(t, strings.TrimRight(raw, "\n")+"\n", writeAt(t, s, tt.fps))
		})
	}
}

// Ports testValidFileRoundTripsWithFrameRateLine and testNullFrameRateTakesTheStoredFrameRate of
// tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestValidFileRoundTrips(t *testing.T) {
	t.Parallel()

	raw := string(testfiles.Read(t, "microdvd/valid.sub"))
	s, _ := mustRead(t, raw, false, 0)
	assert.Equal(t, raw, write(t, s, model.CommonWriteOptions{}, microdvd.WriteOptions{FrameRate: 23.976, WriteFrameRateLine: true}))
	assert.Equal(t, raw, write(t, s, model.CommonWriteOptions{}, microdvd.WriteOptions{WriteFrameRateLine: true}))
	assert.Equal(t, raw, write(t, s, model.CommonWriteOptions{}, microdvd.WriteOptions{}))
}

// Ports testFrameRateLineOfTheInputIsKept of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestFrameRateLineOfTheInputIsKept(t *testing.T) {
	t.Parallel()

	raw := string(testfiles.Read(t, realDir+"/own_max_frames.sub"))
	s, _ := mustRead(t, raw, false, 0)
	written := write(t, s, model.CommonWriteOptions{}, microdvd.WriteOptions{})
	assert.Equal(t, strings.TrimRight(raw, "\n")+"\n", written)
	back, _ := mustRead(t, written, false, 0)
	assert.Equal(t, s.Cues(), back.Cues())
}

func TestFrameRateOfTheWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		edit  func(t *testing.T, s *model.Subtitle)
		opts  microdvd.WriteOptions
		want  string
	}{
		// Ports testRemovingTheFrameRateLineFieldDropsTheLine of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{
			name: "removing frameRateLine drops the line", input: "{1}{1}25\n{25}{50}Hello",
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				require.NoError(t, s.SetFormatData("microdvd", model.NewData(model.Field{Key: "frameRate", Value: 25.0})))
			},
			want: "{25}{50}Hello\n",
		},
		// Ports testFrameRateOptionWinsOverTheStoredFrameRate of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{
			name: "option wins over the stored frame rate", input: "{1}{1}25\n{25}{50}Hello\n",
			opts: microdvd.WriteOptions{FrameRate: 50, WriteFrameRateLine: true}, want: "{1}{1}50\n{50}{100}Hello\n",
		},
		{name: "frame rate line of 25", input: "{25}{50}Hello", opts: microdvd.WriteOptions{FrameRate: 25, WriteFrameRateLine: true}, want: "{1}{1}25\n{25}{50}Hello\n"},
		{name: "frame rate line of 23.976", input: "{25}{50}Hello", opts: microdvd.WriteOptions{FrameRate: 23.976, WriteFrameRateLine: true}, want: "{1}{1}23.976\n{24}{48}Hello\n"},
		{name: "frame rate line of 30000/1001", input: "{25}{50}Hello", opts: microdvd.WriteOptions{FrameRate: 30000.0 / 1001, WriteFrameRateLine: true}, want: "{1}{1}29.97002997003\n{30}{60}Hello\n"},
		{name: "stored integer frame rate", input: "{25}{50}Hello", edit: func(t *testing.T, s *model.Subtitle) {
			t.Helper()
			require.NoError(t, s.SetFormatData("microdvd", model.NewData(model.Field{Key: "frameRate", Value: int64(50)})))
		}, want: "{50}{100}Hello\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.input, false, 25)
			if tt.edit != nil {
				tt.edit(t, s)
			}
			assert.Equal(t, tt.want, write(t, s, model.CommonWriteOptions{}, tt.opts))
		})
	}
}

// Ports testNullFrameRateWithoutAStoredFrameRateThrows and testFrameRateOptionIsRequired of
// tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestWriteNeedsAFrameRate(t *testing.T) {
	t.Parallel()

	for _, opts := range []microdvd.WriteOptions{{}, {WriteFrameRateLine: true}} {
		err := microdvd.Write(&bytes.Buffer{}, subtitle([]string{"Hello"}), model.CommonWriteOptions{}, opts)
		assert.Equal(t, &model.InvalidArgumentError{Option: "WriteOptions.MicroDVD.FrameRate", Msg: "MicroDVD output needs the frame rate of the video."}, err)
	}

	s := subtitle([]string{"Hello"})
	require.NoError(t, s.SetFormatData("microdvd", model.NewData(model.Field{Key: "frameRate", Value: 0.0})))
	err := microdvd.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, microdvd.WriteOptions{})
	assert.Equal(t, &model.InvalidArgumentError{Msg: "The frame rate must be greater than 0, got 0."}, err)

	err = microdvd.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, microdvd.WriteOptions{FrameRate: -1})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
}

func TestControlCodesOfTheWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines [][]string
		want  string
	}{
		// Ports testCoreMarkupBecomesControlCodes of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{"core markup", [][]string{{`<font color="#ff0000"><b><i>Red</i></b></font>`, "<u><s>Under &amp; strike</s></u>", "Partly <i>italic</i> &lt;3"}},
			"{25}{50}{c:$0000FF}{y:b}{y:i}Red|{y:u}{y:s}Under & strike|Partly italic <3\n"},
		// Ports testSingleQuotedAndUnquotedColoursBecomeColourCodes of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{"single-quoted and unquoted colors", [][]string{{"<font color='#ff0000'>Red</font>"}, {"<font color=#00ff00>Green</font>"}},
			"{25}{50}{c:$0000FF}Red\n{75}{100}{c:$00FF00}Green\n"},
		// Ports testColourWithOtherCaseSpacingOrAttributesBecomesColourCode of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{"colors with other case, spacing or attributes", [][]string{{`<FONT COLOR="#FF0000">Red</FONT>`}, {"<font color = '#00ff00' >Green</font>"}, {`<font face="Arial" color="#0000ff">Blue</font>`}},
			"{25}{50}{c:$0000FF}Red\n{75}{100}{c:$00FF00}Green\n{125}{150}{c:$FF0000}Blue\n"},
		// Ports testAColorAndAStyleThatSpanLinesStyleEachLine of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{"color and style over lines", [][]string{{`<font color="#ff0000"><u>Red`, "still red</u></font>", "plain"}},
			"{25}{50}{c:$0000FF}{y:u}Red|{c:$0000FF}{y:u}still red|plain\n"},
		// Ports testASpeakerTagDoesNotHideTheStyleOfItsLines of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
		{"speaker tag", [][]string{{"<v Ben><i>The harbour is closed</i>", "<i>until Monday.</i>"}},
			"{25}{50}{y:i}The harbour is closed|{y:i}until Monday.\n"},
		{"lines over 2 lines", [][]string{{"<i>One", "Two</i>"}}, "{25}{50}{y:i}One|{y:i}Two\n"},
		{"cue without text", [][]string{nil}, "{25}{50}\n"},
		{"tag inside the line", [][]string{{"<b>One</b> and <b>two</b>"}}, "{25}{50}One and two\n"},
		{"same tag inside", [][]string{{"<b>One <b>two</b></b>"}}, "{25}{50}One two\n"},
		{"font without a color", [][]string{{`<font face="Arial">One</font>`}}, "{25}{50}One\n"},
		{"font with a bad color", [][]string{{`<font color="nope">One</font>`}}, "{25}{50}One\n"},
		{"fontx is no font", [][]string{{`<fontx color="#ff0000">One</fontx>`}}, "{25}{50}One\n"},
		{"two fonts", [][]string{{`<font color="#ff0000"><font color="#00ff00">One</font></font>`}}, "{25}{50}One\n"},
		{"color name", [][]string{{`<font color="red">One</font>`}}, "{25}{50}{c:$0000FF}One\n"},
		{"font inside b", [][]string{{`<b><font color="#ff0000">One</font></b>`}}, "{25}{50}{c:$0000FF}{y:b}One\n"},
		{"font that ends before the line", [][]string{{`<font color="#ff0000"><b>One</b></font> x`}}, "{25}{50}One x\n"},
		{"ruby as text", [][]string{{"<i><ruby>Kan<rt>ka</rt></ruby></i>"}}, "{25}{50}{y:i}Kan (ka)\n"},
		{"upper-case b is no style", [][]string{{"<B>One</B>"}}, "{25}{50}One\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, writeAt(t, subtitle(tt.lines...), 25))
		})
	}
}

// Ports testStripAllTagsOptionDropsStyleCodes and testStripAllTagsOptionDropsTheFontAndSizeCodesOfARealFile of
// tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestStripTags(t *testing.T) {
	t.Parallel()

	strip := model.CommonWriteOptions{StripTags: true}
	assert.Equal(t, "{25}{50}Hello\n", write(t, subtitle([]string{"<i>Hello</i>"}), strip, microdvd.WriteOptions{FrameRate: 25}))

	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/sub_microdvd_with_styles.sub")), false, 23.976)
	assert.Equal(t, "{3294}{3366}Passengers, the train is now|arriving at the central station.\n{89682}{89730}Thank you, conductor.\n",
		write(t, s, strip, microdvd.WriteOptions{}))
}

// Ports testChangedStyleWritesNewCodesAndKeepsOtherCodes of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestChangedStyleWritesNewCodesAndKeepsOtherCodes(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{1}{1}25\n{25}{50}{Y:i}{f:Arial}One|Two", false, 0)
	c, _ := s.Cue(0)
	c.Lines = []string{"<b>One</b>", "<i>Two</i>"}
	require.NoError(t, s.SetCue(0, c))
	assert.Equal(t, "{1}{1}25\n{25}{50}{y:b}{f:Arial}One|{y:i}Two\n", writeAt(t, s, 25))

	c.Lines = []string{"<i>One</i>", "<i>Two</i>"}
	require.NoError(t, s.SetCue(0, c))
	assert.Equal(t, "{1}{1}25\n{25}{50}{Y:i}{f:Arial}One|Two\n", writeAt(t, s, 25), "the stored codes when the styles did not change")

	c.Lines = []string{"<i>One</i>"}
	require.NoError(t, s.SetCue(0, c))
	assert.Equal(t, "{1}{1}25\n{25}{50}{y:i}{f:Arial}One\n", writeAt(t, s, 25), "new codes for another line count")
}

// Ports testStylesThatSpanLinesStyleEachLine of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestStylesThatSpanLinesStyleEachLine(t *testing.T) {
	t.Parallel()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "srt/real/own_styled.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	out := writeAt(t, s, 25)
	assert.Contains(t, out, "{12000}{13500}{y:i}Italics over two lines|{y:i}that close on the second line\n")
	assert.Contains(t, out, "{13500}{15000}{y:i}Italics that never close\n")
	back, _ := mustRead(t, out, false, 25)
	assert.Equal(t, []string{"<i>Italics over two lines</i>", "<i>that close on the second line</i>"}, cueLines(t, back, 6))
}

// Ports testSubRipConvertsToMicroDvd of tests/Formatters/MicroDvdFormatterTest.php of subtitle-toolbox-php.
func TestSubRipConvertsToMicroDVD(t *testing.T) {
	t.Parallel()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "srt/valid.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	back, _ := mustRead(t, writeAt(t, s, 25), false, 25)
	require.Equal(t, s.Len(), back.Len())
	for i, c := range s.All() {
		got, _ := back.Cue(i)
		assert.InDelta(t, c.Start.Seconds(), got.Start.Seconds(), 0.02)
		assert.InDelta(t, c.End.Seconds(), got.End.Seconds(), 0.02)
	}
}

func TestWriteClampsNegativeTimes(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: -2 * 1000 * ms, End: 1000 * ms, Lines: []string{"Hello"}})
	assert.Equal(t, "{0}{25}Hello\n", writeAt(t, s, 25))
}

func TestWriteLineEndingAndBOM(t *testing.T) {
	t.Parallel()

	s := subtitle([]string{"Hello"})
	assert.Equal(t, "\xEF\xBB\xBF{25}{50}Hello\r\n", write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}, microdvd.WriteOptions{FrameRate: 25}))
}

func TestWriteFrameBeyondTheIntRange(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 359999999 * 1000 * ms, End: 359999999 * 1000 * ms, Lines: []string{"x"}})
	assert.Equal(t, "{359999999000000004096}{359999999000000004096}x\n", writeAt(t, s, 1e12))
}

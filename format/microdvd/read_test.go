package microdvd_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const unknownFrameRate = "The frame rate is unknown. Start the file with {1}{1}<fps>, set the frame rate, or read the file in lenient mode for 23.976 fps."

// Ports testRealFileParses of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
func TestRealFileParses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		fps         float64
		cues        int
		first, last cueRow
	}{
		{
			"mantas-done plain", "sub_microdvd.sub", 23.976, 2,
			cueRow{137387 * ms, 140390 * ms, []string{"Passengers, the train is now", "arriving at the central station."}},
			cueRow{3740490 * ms, 3742492 * ms, []string{"Thank you, conductor."}},
		},
		{
			"mantas-done styles", "sub_microdvd_with_styles.sub", 23.976, 2,
			cueRow{137387 * ms, 140390 * ms, []string{"Passengers, the train is now", "<i>arriving at the central station.</i>"}},
			cueRow{3740490 * ms, 3742492 * ms, []string{`<font color="#ff0000"><b><u>Thank you, conductor.</u></b></font>`}},
		},
		{
			"subsrt sample", "subsrt_sample.sub", 25, 5,
			cueRow{599000 * ms, 4160000 * ms, []string{"Hi, my name is Alice Miller and this is John Brown"}},
			cueRow{16700000 * ms, 21480000 * ms, []string{"Okay, so we have all the ingredients laid out here"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/"+tt.file)), false, tt.fps)
			rows := cueRows(s)
			require.Len(t, rows, tt.cues)
			assert.Equal(t, tt.first, rows[0])
			assert.Equal(t, tt.last, rows[len(rows)-1])
		})
	}
}

func fileData(t *testing.T, s *model.Subtitle) map[string]any {
	t.Helper()

	d, ok := s.FormatData("microdvd")
	require.True(t, ok)
	out := map[string]any{}
	for _, k := range d.Keys() {
		out[k], _ = d.Get(k)
	}
	return out
}

func lineData(t *testing.T, s *model.Subtitle, cue, line int) map[string]any {
	t.Helper()

	c, ok := s.Cue(cue)
	require.True(t, ok)
	d, ok := c.FormatData("microdvd")
	require.True(t, ok)
	lines, ok := d.List("lines")
	require.True(t, ok)
	require.Greater(t, len(lines), line)
	item, ok := lines[line].(*model.Data)
	require.True(t, ok)
	out := map[string]any{}
	for _, k := range item.Keys() {
		out[k], _ = item.Get(k)
	}
	return out
}

// Ports testFrameRateLineSetsTheFrameRate, testControlCodesBecomeCoreMarkup and
// testOtherControlCodesGoToTheFormatData of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
func TestValidFile(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, string(testfiles.Read(t, "microdvd/valid.sub")), false, 0)
	assert.Empty(t, warnings)
	require.Equal(t, 4, s.Len())
	first, _ := s.Cue(0)
	assert.Equal(t, 1001*ms, first.Start)
	assert.Equal(t, 3003*ms, first.End)
	assert.Equal(t, map[string]any{"frameRate": 23.976, "frameRateLine": true}, fileData(t, s))

	assert.Equal(t, []string{"Hello", "<i>world</i>"}, cueLines(t, s, 0))
	assert.Equal(t, []string{`<font color="#ff0000">Red text</font>`}, cueLines(t, s, 1))
	assert.Equal(t, []string{"<b>Bold</b>", "<b>on two lines</b>"}, cueLines(t, s, 2))
	assert.Equal(t, []string{"<u><s>Underline &lt; strike &amp; more</s></u>"}, cueLines(t, s, 3))

	assert.Equal(t, map[string]any{"codes": "{Y:b}{f:Arial}{s:20}", "otherCodes": "{f:Arial}{s:20}", "color": nil, "tags": []any{"b"}}, lineData(t, s, 2, 0))
	assert.Equal(t, map[string]any{"codes": "", "otherCodes": "", "color": nil, "tags": []any{"b"}}, lineData(t, s, 2, 1))
	assert.Equal(t, "{P:0}", lineData(t, s, 3, 0)["otherCodes"])
	assert.Equal(t, map[string]any{"codes": "{c:$0000FF}", "otherCodes": "", "color": "#ff0000", "tags": []any{}}, lineData(t, s, 1, 0))
}

func TestCueText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		// Ports testLowerCaseColorOverridesUpperCaseColorOnItsLine of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"line color wins over cue color", `{C:$0000FF}{Y:i}One|{c:$00FF00}{y:b}Two`, []string{`<font color="#ff0000"><i>One</i></font>`, `<font color="#00ff00"><b><i>Two</i></b></font>`}},
		// Ports testCodesInsideTheLineStayText of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"codes inside the line stay text", "Hello, {y:i}world", []string{"Hello, {y:i}world"}},
		// Ports testLatin1TextKeepsItsBytes of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"Latin-1 keeps its bytes", "{y:i}caf\xE9 & bread|<3 jam", []string{"<i>caf\xE9 &amp; bread</i>", "&lt;3 jam"}},
		{"upper-case code on line 2 styles line 1", "One|{Y:u}Two", []string{"<u>One</u>", "<u>Two</u>"}},
		{"codes of a dropped line still style the cue", "One|{Y:b}", []string{"<b>One</b>"}},
		{"tags add up without repeats", "{Y:i,b}{y:I}{y:s b}One", []string{"<b><i><s>One</s></i></b>"}},
		{"unknown style letter is no style", "{y:x}One", []string{"One"}},
		{"color name is no color", "{c:red}One", []string{"One"}},
		{"last color wins", "{c:$0000FF}{c:$00FF00}One", []string{`<font color="#00ff00">One</font>`}},
		{"empty y code is no style", "{y:}One", []string{"One"}},
		{"unclosed code is text", "{y:iOne", []string{"{y:iOne"}},
		{"line of only codes", "{y:i}", nil},
		{"spaces around the text", "{y:i}  One  |  Two", []string{"<i>One</i>", "Two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, "{0}{25}"+tt.text, false, 25)
			assert.Equal(t, tt.want, cueLines(t, s, 0))
		})
	}
}

func TestOtherCodes(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{0}{25}{y:x}{f:Arial}{c:red}{Y:i}One", false, 25)
	assert.Equal(t, map[string]any{"codes": "{y:x}{f:Arial}{c:red}{Y:i}", "otherCodes": "{y:x}{f:Arial}{c:red}", "color": nil, "tags": []any{"i"}}, lineData(t, s, 0, 0))
}

func TestFrameRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		lenient  bool
		fps      float64
		wantEnd  int64
		wantData map[string]any
		warnings []warningRow
	}{
		// Ports testConstructorFrameRateWinsOverFrameRateLine of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"option wins over the line", "{1}{1}23.976\n{25}{50}Hello", false, 25, 2000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, nil},
		{"line without option", "{1}{1}25\n{25}{50}Hello", false, 0, 2000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, nil},
		{"option without line", "{25}{50}Hello", false, 25, 2000, map[string]any{"frameRate": 25.0}, nil},
		// Ports testLenientModeWithoutFrameRateUses23976FpsAndWarns of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"lenient default", "{0}{25}Hello", true, 0, 1043, map[string]any{"frameRate": 23.976}, []warningRow{
			{"The file has no {1}{1}<fps> line. The parser used 23.976 fps.", model.NoLine, model.NoBlock, model.Repaired},
		}},
		// Ports testLenientModeKeepsTheFrameRateOfTheOptions of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"lenient keeps the option", "{0}{25}Hello", true, 25, 1000, map[string]any{"frameRate": 25.0}, nil},
		// Ports testLenientModeKeepsTheFrameRateLine of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"lenient keeps the line", "{1}{1}25\n{0}{25}Hello", true, 0, 1000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, nil},
		{"lenient finds the line after junk", "Movie.Name\n{1}{1}25\n{0}{25}Hello", true, 0, 1000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, []warningRow{
			{`The line "Movie.Name" is not a MicroDVD cue.`, 1, 0, model.Skipped},
		}},
		{"option wins silently over a line of 0", "{1}{1}0\n{25}{50}Hello", false, 25, 2000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, nil},
		{"30000/1001 in the option", "{0}{30}Hello", false, 30000.0 / 1001, 1001, map[string]any{"frameRate": 30000.0 / 1001}, nil},
		// Ports testWindowsLineEndingsBomAndEmptyLinesAreAccepted of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"CR LF, BOM and empty lines", "\xEF\xBB\xBF\r\n{1}{1}25\r\n\r\n{25}{50}Hello\r\n", false, 0, 2000, map[string]any{"frameRate": 25.0, "frameRateLine": true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, tt.lenient, tt.fps)
			require.Equal(t, 1, s.Len())
			c, _ := s.Cue(0)
			assert.Equal(t, tt.wantEnd, c.End.Milliseconds())
			assert.Equal(t, tt.wantData, fileData(t, s))
			assert.Equal(t, []string{"Hello"}, c.Lines)
			assert.Equal(t, tt.warnings, append([]warningRow(nil), warningRows(warnings)...))
		})
	}
}

func TestFrameRateLineIsACueWhenItHasNoNumber(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{1}{1}Hello\n{25}{50}World", false, 25)
	assert.Equal(t, []cueRow{{40 * ms, 40 * ms, []string{"Hello"}}, {1000 * ms, 2000 * ms, []string{"World"}}}, cueRows(s))
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		lenient bool
		fps     float64
		msg     string
		line    int
		option  string
	}{
		// Ports testMissingFrameRateThrowsException and testStrictModeWithoutFrameRateNamesLenientMode of
		// tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"no frame rate", "{25}{50}Hello", false, 0, unknownFrameRate, 0, "ReadOptions.MicroDVD.FrameRate"},
		{"frame rate line after junk in strict mode", "Movie.Name\n{1}{1}25\n{25}{50}Hello", false, 0, unknownFrameRate, 0, "ReadOptions.MicroDVD.FrameRate"},
		// Ports testZeroFrameRateThrowsException of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"frame rate line of 0", "{1}{1}0\n{25}{50}Hello", false, 0, "The frame rate must be greater than 0, got 0.", 0, ""},
		{"frame rate line of 0 in lenient mode", "{1}{1}0\n{25}{50}Hello", true, 0, "The frame rate must be greater than 0, got 0.", 0, ""},
		// Ports testLineWithoutFramesThrowsException of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
		{"line without frames", "{1}{1}25\n{25}{50}Hello\n00:00:01,000 --> 00:00:02,000", false, 0, `The line "00:00:01,000 --> 00:00:02,000" is not a MicroDVD cue.`, 3, ""},
		{"empty end frame", "{24}{}Hello", false, 25, `The line "{24}{}Hello" is not a MicroDVD cue.`, 1, ""},
		{"20-digit frame number", "{24}{99999999999999999999}Hello", false, 25, `The time "{99999999999999999999}" is not below 100000 hours.`, 1, ""},
		{"13-digit frame number", "{1}{1}25\n{9000000000000}{9000000000050}Hello", false, 0, `The time "{9000000000000}" is not below 100000 hours.`, 2, ""},
		{"end before start", "{50}{25}Hello", false, 25, "The cue ends at 1 s, before it starts at 2 s.", 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := read(tt.content, tt.lenient, tt.fps)
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.MicroDVD, perr.Format)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
			assert.Equal(t, tt.option, perr.Option)
		})
	}
}

func TestFrameRateLineBeyondTheFloatRange(t *testing.T) {
	t.Parallel()

	digits := "1"
	for range 400 {
		digits += "0"
	}
	_, _, err := read("{1}{1}"+digits+"\n{25}{50}Hello", false, 0)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The frame rate must be greater than 0, got INF.", perr.Msg)
}

func TestLenientSkipsBrokenLines(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "{1}{1}25\n{25}{75}One\n\n{x}{1}Two\n{9000000000000}{1}Three\n{100}{50}Four\n{2000}{100}Five", true, 0)
	assert.Equal(t, []cueRow{{1000 * ms, 3000 * ms, []string{"One"}}, {2000 * ms, 4000 * ms, []string{"Four"}}}, cueRows(s))
	assert.Equal(t, []warningRow{
		{`The line "{x}{1}Two" is not a MicroDVD cue.`, 4, 2, model.Skipped},
		{`The time "{9000000000000}" is not below 100000 hours.`, 5, 3, model.Skipped},
		{"The cue ends at 2 s, before it starts at 4 s. The parser swapped the times.", 6, 4, model.Repaired},
		{"The cue ends at 4 s, before it starts at 80 s.", 7, 5, model.Skipped},
	}, warningRows(warnings))
}

func TestMaxFrames(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/own_max_frames.sub")), false, 0)
	c, _ := s.Cue(1)
	assert.Equal(t, 359999999960*ms, c.End)

	// 359999999.99965 s is below 100000 h, but it rounds to 100000 h. PHP checks the time before the rounding.
	_, _, err := read("{359999999999650}{359999999999650}x", false, 1e6)
	require.ErrorIs(t, err, model.ErrParse)
}

// Ports testRealFileWithoutFrameRateParsesInLenientMode of tests/Parsers/MicroDvdParserTest.php of subtitle-toolbox-php.
func TestRealFileWithoutFrameRateParsesInLenientMode(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"sub_microdvd.sub", "sub_microdvd_with_styles.sub", "subsrt_sample.sub"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			content := string(testfiles.Read(t, realDir+"/"+file))
			lenient, warnings := mustRead(t, content, true, 0)
			at23976, _ := mustRead(t, content, false, 23.976)
			assert.Equal(t, at23976.Cues(), lenient.Cues())
			assert.Len(t, warnings, 1)
		})
	}
}

func TestBlankInput(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, " \n\r\n", false, 0)
	assert.Equal(t, 0, s.Len())
	assert.Empty(t, warnings)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fps  float64
		want string
	}{
		{"negative", -1, "-1"},
		{"NaN", math.NaN(), "NAN"},
		{"infinite", math.Inf(1), "INF"},
		{"minus infinite", math.Inf(-1), "-INF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := microdvd.ReadOptions{FrameRate: tt.fps}.Validate()
			assert.Equal(t, &model.InvalidArgumentError{Option: "ReadOptions.MicroDVD.FrameRate", Msg: "The frame rate must be greater than 0, got " + tt.want + "."}, err)
			require.ErrorIs(t, err, model.ErrInvalidArgument)

			err = microdvd.WriteOptions{FrameRate: tt.fps}.Validate()
			assert.Equal(t, &model.InvalidArgumentError{Option: "WriteOptions.MicroDVD.FrameRate", Msg: "The MicroDVD frame rate must be greater than 0, got " + tt.want + "."}, err)

			_, _, err = read("{0}{25}Hello", false, tt.fps)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
	require.NoError(t, microdvd.ReadOptions{}.Validate())
	require.NoError(t, microdvd.WriteOptions{FrameRate: 23.976}.Validate())
}

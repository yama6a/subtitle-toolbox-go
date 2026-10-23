package sbv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testValidSbvFileParses and testTwoDigitHoursParse of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want []cueRow
	}{
		{"sbv/valid.sbv", []cueRow{
			{0, 456 * ms, []string{"Only milliseconds"}},
			{5222 * ms, 46444 * ms, []string{"Seconds and milliseconds"}},
			{15*time.Minute + 20555*ms, 58*time.Minute + 1888*ms, []string{"Minutes and seconds and milliseconds"}},
			{time.Hour + time.Minute + time.Second, 2*time.Hour + 800*ms, []string{"Full timestamp", "on two lines"}},
			{10 * time.Hour, 99*time.Hour + 59*time.Minute + 59999*ms, []string{"Two-digit hours"}},
		}},
		{"sbv/two_digit_hours.sbv", []cueRow{
			{1500 * ms, 4 * time.Second, []string{"Hello world"}},
			{time.Hour + 5*time.Second, time.Hour + 7250*ms, []string{"Second cue", "on two lines"}},
		}},
		{"sbv/three_digit_hours.sbv", []cueRow{{100*time.Hour + 1500*ms, 100*time.Hour + 4*time.Second, []string{"Hello world"}}}},
		{"sbv/missing_text.sbv", []cueRow{
			{1500 * ms, 4 * time.Second, []string{"Hello world"}},
			{5 * time.Second, 7250 * ms, nil},
		}},
		{"lenient/missing_empty_line.sbv", []cueRow{
			{time.Second, 2500 * ms, []string{"The wind is cold today."}},
			{3 * time.Second, 5 * time.Second, []string{"Snow falls in the hills."}},
			{5500 * ms, 7 * time.Second, []string{"The roads are closed."}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := sbv.Read(strings.NewReader(string(testfiles.Read(t, tt.file))), model.CommonReadOptions{}, sbv.ReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

// Ports testUtf8BomAndWindowsLineEndingsAreAccepted and testSeveralEmptyLinesBetweenCuesStillSeparateCues
// of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadLineEndingsAndEmptyLines(t *testing.T) {
	t.Parallel()

	valid := string(testfiles.Read(t, "sbv/valid.sbv"))
	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{"BOM and CR LF", "\xEF\xBB\xBF" + strings.ReplaceAll(valid, "\n", "\r\n"), nil},
		{"several empty lines", "\n\n0:00:01.000,0:00:02.000\nFirst\n\n \n\n0:00:03.000,0:00:04.000\nSecond\n\n\n", []cueRow{
			{time.Second, 2 * time.Second, []string{"First"}},
			{3 * time.Second, 4 * time.Second, []string{"Second"}},
		}},
		{"tabs and runs of spaces", "0:00:01.000\t,  0:00:02.000\n\tSome \t text  \n", []cueRow{
			{time.Second, 2 * time.Second, []string{"Some text"}},
		}},
		{"timing line without text", "0:00:01.000,0:00:02.000\n", []cueRow{{time.Second, 2 * time.Second, nil}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, false)
			assert.Empty(t, warnings)
			if tt.want == nil {
				assert.Equal(t, 5, s.Len())
				return
			}
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

// Ports testTextIsStoredWithMarkupCharactersEscaped and testLatin1TextKeepsItsBytes of tests/Parsers/SbvParserTest.php
// of subtitle-toolbox-php.
func TestReadText(t *testing.T) {
	t.Parallel()

	s := readFile(t, "sbv/special_characters.sbv")
	assert.Equal(t, []cueRow{
		{time.Second, 2 * time.Second, []string{"I &lt;3 bread &amp; jam"}},
		{3 * time.Second, 5500 * ms, []string{"&lt;b&gt;is not bold&lt;/b&gt; in SBV", "The bakery &amp;amp; the station"}},
	}, cueRows(s))

	s, warnings := mustRead(t, "0:00:01.000,0:00:02.000\ncaf\xE9 & bread\n", false)
	assert.Equal(t, []string{"caf\xE9 &amp; bread"}, cueRows(s)[0].Lines)
	assert.Empty(t, warnings)

	_, warnings = mustRead(t, "0:00:01.000,0:00:02.000\ncaf\xE9\n", true)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.Equal(t, "The content is not valid UTF-8. The first bad byte is at offset 27. Set the source encoding of the file.", warnings[0].Message)
}

func TestReadNoFormatData(t *testing.T) {
	t.Parallel()

	s := readFile(t, "sbv/valid.sbv")
	for _, c := range s.All() {
		assert.Zero(t, c.Alignment)
		assert.Empty(t, c.ID)
	}
	assert.Empty(t, s.Comments())
}

// Ports testAFileWithoutCuesReadsAsZeroCues of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadInputsWithoutCues(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content string }{
		{"empty", ""},
		{"BOM only", "\xEF\xBB\xBF"},
		{"white space only", " \n\t\r\n\n  "},
		{"BOM and empty lines", "\xEF\xBB\xBF\r\n\r\n"},
	}
	for _, tt := range tests {
		for _, lenient := range []bool{false, true} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, warnings := mustRead(t, tt.content, lenient)
				assert.Zero(t, s.Len())
				assert.Empty(t, warnings)
			})
		}
	}
}

// Ports testStrictModeReadsOnlyTheSbvTimingLine and testLenientModeReadsOtherSeparatorsAndFractionsAndWarns
// of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadTimingLineForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		start, end time.Duration
		strict     bool
	}{
		{"SBV form", "0:00:07.980,0:00:11.300", 7980 * ms, 11300 * ms, true},
		{"spaces around the comma", "0:00:07.980 , 0:00:11.300", 7980 * ms, 11300 * ms, true},
		{"dot between the times", "0:00:07.980.0:00:11.300", 7980 * ms, 11300 * ms, false},
		{"commas everywhere", "0:00:07,980,0:00:11,300", 7980 * ms, 11300 * ms, false},
		{"short fractions", "0:00:07.98,0:00:11.3", 7980 * ms, 11300 * ms, false},
		{"colon before the fraction", "0:00:07:98,0:00:11:3", 7980 * ms, 11300 * ms, false},
		{"fractions of 4 digits", "0:00:07.9995,0:00:11.0004", 8 * time.Second, 11 * time.Second, false},
		{"one digit minutes and seconds", "0:0:7.5,0:1:2.5", 7500 * ms, 62500 * ms, false},
		{"spaces around a dot", "0:00:07.980 . 0:00:11.300", 7980 * ms, 11300 * ms, false},
	}
	for _, tt := range tests {
		content := tt.line + "\nText\n\n0:00:20.000,0:00:21.000\nMore\n"
		t.Run(tt.name+", strict", func(t *testing.T) {
			t.Parallel()

			if !tt.strict {
				perr := readError(t, content)
				assert.Equal(t, 1, perr.Line)
				return
			}
			s, warnings := mustRead(t, content, false)
			assert.Empty(t, warnings)
			assert.Equal(t, cueRow{tt.start, tt.end, []string{"Text"}}, cueRows(s)[0])
		})
		t.Run(tt.name+", lenient", func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, content, true)
			require.Equal(t, 2, s.Len())
			assert.Equal(t, cueRow{tt.start, tt.end, []string{"Text"}}, cueRows(s)[0])
			if tt.strict {
				assert.Empty(t, warnings)
				return
			}
			assert.Equal(t, []warningRow{{
				"Block #0 has a timing line with other separators or fraction digits than SBV uses. The parser read it.",
				1, 0, model.Repaired,
			}}, warningRows(warnings))
			assert.Equal(t, []string{tt.line, "Text"}, warnings[0].Lines)
		})
	}
}

// Ports testLenientModeRejectsLooseTimesWithMinutesOrSecondsAbove59 of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadLooseTimeAbove59(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "0:00:61.5,0:01:02.0\nText\n", true)
	assert.Zero(t, s.Len())
	assert.Equal(t, []warningRow{{`The time "0:00:61.5" is not valid.`, 1, 0, model.Skipped}}, warningRows(warnings))
}

// Ports testExceededMinutesThrowsException, testExceededSecondsThrowsException, testMissingMilliDigitsThrowsException
// and testSubRipTimestampsThrowException of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 70)
	tests := []struct {
		name    string
		content string
		msg     string
		line    int
	}{
		{"minutes of 60", string(testfiles.Read(t, "sbv/exceeded_minutes.sbv")), `The time "0:60:04.000" is not valid.`, 1},
		{"seconds of 60", string(testfiles.Read(t, "sbv/exceeded_seconds.sbv")), `The time "0:00:60.000" is not valid.`, 1},
		{"1 fraction digit", string(testfiles.Read(t, "sbv/missing_milli_digits.sbv")), `The time "0:00:01.5" is not valid.`, 1},
		{
			"SubRip timing line", string(testfiles.Read(t, "sbv/srt_timestamps.sbv")),
			`Block #0 has no timing line on its first line. The line is "0:00:01,500 --> 0:00:04,000".`, 1,
		},
		{"note line with one comma", "Auto-generated, may contain errors\n\n0:00:01.000,0:00:02.000\nA\n", `The time "Auto-generated" is not valid.`, 1},
		{"2 commas", "0:00:01.000,0:00:02.000,0:00:03.000\nA\n", `Block #0 has no timing line on its first line. The line is "0:00:01.000,0:00:02.000,0:00:03.000".`, 1},
		{"no comma on line 4", "0:00:01.000,0:00:02.000\nA\n\n1:2\nB\n\n0:00:00.000 0:00:01.000\n", `Block #1 has no timing line on its first line. The line is "1:2".`, 4},
		{"long line cut to 60 characters", "1:00:00.000 " + long + "\n", `Block #0 has no timing line on its first line. The line is "1:00:00.000 ` + strings.Repeat("x", 45) + `...".`, 1},
		{"end before start", "0:00:05.000,0:00:04.000\nA\n", "The cue ends at 4 s, before it starts at 5 s.", 1},
		{"20 hour digits", "99999999999999999999:00:04.000,99999999999999999999:00:06.000\nA\n", `The time "99999999999999999999:00:04.000" is not below 100000 hours.`, 1},
		{"100000 hours", "100000:00:00.000,100000:00:01.000\nA\n", `The time "100000:00:00.000" is not below 100000 hours.`, 1},
		{"hour digits with leading zeros", "000000000000000100000:00:00.000,1:00:00.000\nA\n", `The time "000000000000000100000:00:00.000" is not below 100000 hours.`, 1},
		{"text before the first cue", "Hello\n0:00:01.000,0:00:02.000\n", `Block #0 has no timing line on its first line. The line is "Hello".`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, tt.content)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
		})
	}
}

func TestReadHours(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "99999:59:59.999,000000000000099999:59:59.999\nA\n", false)
	assert.Equal(t, []cueRow{{99999*time.Hour + 59*time.Minute + 59999*ms, 99999*time.Hour + 59*time.Minute + 59999*ms, []string{"A"}}}, cueRows(s))
}

func TestReadLooseBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, msg string
	}{
		{"20 hour digits", "99999999999999999999:00:04.0,99999999999999999999:00:06.0\nA\n", `The time "99999999999999999999:00:04.0" is not below 100000 hours.`},
		{"rounding up to 100000 hours", "99999:59:59.9995,100000:0:0.0\nA\n", `The time "99999:59:59.9995" is not below 100000 hours.`},
		{"digits as written", "0000100000:0:4,5,1:00:00.000\nA\n", `The time "0000100000:0:4.5" is not below 100000 hours.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Zero(t, s.Len())
			assert.Equal(t, []warningRow{{tt.msg, 1, 0, model.Skipped}}, warningRows(warnings))
		})
	}
}

// Ports testTextWithoutTimingLineAfterAnEmptyLineStaysInTheCueBefore of tests/Parsers/SbvParserTest.php
// of subtitle-toolbox-php.
func TestReadTextBlockJoinsTheCueBefore(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		s, warnings := mustRead(t, string(testfiles.Read(t, "sbv/missing_timestamps.sbv")), lenient)
		assert.Equal(t, []cueRow{{1500 * ms, 4 * time.Second, []string{"Hello world", "Second cue", "on two lines"}}}, cueRows(s))
		if lenient {
			assert.Equal(t, []warningRow{{
				"Block #0 has an empty line before line 4 inside the cue. The parser kept the text after it in the cue.", 4, 0, model.Repaired,
			}}, warningRows(warnings))
		} else {
			assert.Empty(t, warnings)
		}
	}
}

func TestReadTimingLineInsideABlock(t *testing.T) {
	t.Parallel()

	content := "0:00:01.000,0:00:02.000\nOne\n0:00:03.000,0:00:04.000\nTwo\n"
	s, warnings := mustRead(t, content, false)
	assert.Equal(t, 2, s.Len())
	assert.Empty(t, warnings)

	_, warnings = mustRead(t, content, true)
	assert.Equal(t, []warningRow{{"Block #0 has no empty line before line 3. The parser split the block there.", 3, 0, model.Repaired}}, warningRows(warnings))
}

func TestReadSwapsTimesInLenientMode(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "0:00:05.000,0:00:04.000\nA\n", true)
	assert.Equal(t, []cueRow{{4 * time.Second, 5 * time.Second, []string{"A"}}}, cueRows(s))
	assert.Equal(t, []warningRow{{"The cue ends at 4 s, before it starts at 5 s. The parser swapped the times.", 1, 0, model.Repaired}}, warningRows(warnings))

	s, warnings = mustRead(t, "0:01:05.000,0:00:04.000\nA\n", true)
	assert.Zero(t, s.Len())
	assert.Equal(t, []warningRow{{"The cue ends at 4 s, before it starts at 65 s.", 1, 0, model.Skipped}}, warningRows(warnings))
}

func TestReadFailsOnReaderError(t *testing.T) {
	t.Parallel()

	_, _, err := sbv.Read(errReader{}, model.CommonReadOptions{}, sbv.ReadOptions{})
	require.ErrorIs(t, err, errRead)
}

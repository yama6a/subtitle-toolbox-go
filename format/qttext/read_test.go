package qttext_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports the time rows of docs/formats.md and testReadsOtherTimeScalesAndDecimalFractionsWithoutATimeScale,
// testAddsRelativeTimestampsToThePreviousOne and testStyleDescriptorsLastUntilPlain of
// tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"a time scale of 600",
			"{QTtext}{timeScale:600}\n[00:00:07.300]\nRain\n[00:00:09.000]\n",
			[]cueRow{{7500 * ms, 9 * time.Second, "Rain"}},
		},
		{
			"a time scale of 10",
			"{QTtext}{timeScale:10}\n[00:00:07.5]\nRain\n[00:00:09.0]\n",
			[]cueRow{{7500 * ms, 9 * time.Second, "Rain"}},
		},
		{
			"a decimal fraction without a time scale",
			"{QTtext}{font:Geneva}\n[00:00:07.25]\nRain\n[00:00:09.5]\n",
			[]cueRow{{7250 * ms, 9500 * ms, "Rain"}},
		},
		{
			"a time scale of 30 rounds 1.13 to 1433 ms",
			"{QTtext}{timeScale:30}\n[00:00:01.13]\nRain\n[00:00:04.00]\n",
			[]cueRow{{1433 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a time scale of 30 rounds half up",
			"{QTtext}{timeScale:30}\n[00:00:00.15]\nRain\n[00:00:04.00]\n",
			[]cueRow{{500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a time scale of 2000 rounds a half millisecond up",
			"{QTtext}{timeScale:2000}\n[00:00:00.0001]\nRain\n[00:00:04.0000]\n",
			[]cueRow{{ms, 4 * time.Second, "Rain"}},
		},
		{
			"a time scale beyond the int64 range counts as the largest int64",
			"{QTtext}{timeScale:99999999999999999999}\n[00:00:00.4611686018427387903]\nRain\n[00:00:04.0]\n",
			[]cueRow{{500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a decimal fraction of 4 digits rounds half up",
			"{QTtext}\n[00:00:01.0005]\nRain\n[00:00:04.0004]\n",
			[]cueRow{{1001 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a decimal fraction of 1 digit",
			"{QTtext}\n[00:00:01.5]\nRain\n[00:00:04.9]\n",
			[]cueRow{{1500 * ms, 4900 * ms, "Rain"}},
		},
		{
			"a timestamp without a fraction",
			"{QTtext}\n[00:00:01]\nRain\n[00:00:04]\n",
			[]cueRow{{time.Second, 4 * time.Second, "Rain"}},
		},
		{
			"a time scale of 0 is no time scale",
			"{QTtext}{timeScale:0}\n[00:00:01.5]\nRain\n[00:00:04.0]\n",
			[]cueRow{{1500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a time scale that is not a number is no time scale",
			"{QTtext}{timeScale:fast}\n[00:00:01.5]\nRain\n[00:00:04.0]\n",
			[]cueRow{{1500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"a descriptor in lower case with spaces around the colon",
			"{QTtext}{ timescale : 10 }\n[00:00:01.5]\nRain\n[00:00:04.0]\n",
			[]cueRow{{1500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"the first time scale descriptor counts",
			"{QTtext}{timeScale:10}\n{timeScale:600}\n[00:00:01.5]\nRain\n[00:00:04.0]\n",
			[]cueRow{{1500 * ms, 4 * time.Second, "Rain"}},
		},
		{
			"hours and minutes",
			"{QTtext}\n[12:34:56.789]\nRain\n[12:34:57.000]\n",
			[]cueRow{{(12*3600+34*60+56)*time.Second + 789*ms, (12*3600 + 34*60 + 57) * time.Second, "Rain"}},
		},
		{
			"a sample ends at the next sample",
			"{QTtext}\n[00:00:01.00]\nOne\n[00:00:04.00]\n[00:00:05.00]\nTwo\n",
			[]cueRow{{time.Second, 4 * time.Second, "One"}, {5 * time.Second, 10 * time.Second, "Two"}},
		},
		{
			"a sample with the next sample at once",
			"{QTtext}\n[00:00:01.00]\nOne\n[00:00:02.00]\nTwo\n[00:00:03.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "One"}, {2 * time.Second, 3 * time.Second, "Two"}},
		},
		{
			"text on the line of the timestamp",
			"{QTtext}\n[00:00:01.00] One\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "One"}},
		},
		{
			"several text lines",
			"{QTtext}\n[00:00:01.00]\nOne\n  Two  \n\nThree\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "One\nTwo\nThree"}},
		},
		{
			"a timestamp line with spaces around it",
			"{QTtext}\n  [00:00:01.00]  \nOne\n\t[00:00:02.00]\t\n",
			[]cueRow{{time.Second, 2 * time.Second, "One"}},
		},
		{
			"text with a tag is escaped",
			"{QTtext}\n[00:00:01.00]\na < b & <i>c</i>\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "a &lt; b &amp; &lt;i&gt;c&lt;/i&gt;"}},
		},
		{
			"CR LF and a lone CR",
			"{QTtext}\r\n[00:00:01.00]\rOne\r\n[00:00:02.00]\r\n",
			[]cueRow{{time.Second, 2 * time.Second, "One"}},
		},
		{
			"relative timestamps add to the one before",
			"{QTtext}{timeScale:600}{timeStamps:relative}\n[00:00:01.300]\nOne\n[00:00:02.000]\n[00:00:01.000]\nTwo\n[00:00:02.000]\n",
			[]cueRow{{1500 * ms, 3500 * ms, "One"}, {4500 * ms, 6500 * ms, "Two"}},
		},
		{
			"relative in any case",
			"{QTtext}{timeStamps:RELATIVE}\n[00:00:01.5]\nOne\n[00:00:02.0]\n",
			[]cueRow{{1500 * ms, 3500 * ms, "One"}},
		},
		{
			"absolute is the default",
			"{QTtext}{timeStamps:absolute}\n[00:00:01.5]\nOne\n[00:00:02.0]\n",
			[]cueRow{{1500 * ms, 2 * time.Second, "One"}},
		},
		{
			"styles last until plain",
			"{QTtext}{timeScale:30}{plain}\n[00:00:01.00]\n{italic}Sung\n[00:00:02.00]\nstill sung {underline}loud\n[00:00:03.00]\n{plain}Spoken\n[00:00:04.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "<i>Sung</i>"}, {2 * time.Second, 3 * time.Second, "<i>still sung <u>loud</u></i>"}, {3 * time.Second, 4 * time.Second, "Spoken"}},
		},
		{
			"the header sets the starting style",
			"{QTtext}{bold}\n[00:00:01.00]\nLoud\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "<b>Loud</b>"}},
		},
		{
			"descriptors in any case, in order bold italic underline",
			"{QTtext}\n[00:00:01.00]\n{BOLD}{Underline}{iTaLiC}x\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "<b><i><u>x</u></i></b>"}},
		},
		{
			"other descriptors are dropped",
			"{QTtext}\n[00:00:01.00]\n{font:Geneva}Hello{bold:1} {x}world\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "Hello world"}},
		},
		{
			"white space around a style stays outside its tags",
			"{QTtext}\n[00:00:01.00]\nA {bold} loud {plain} word\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "A <b>loud</b> word"}},
		},
		{
			"a sample with only descriptors gives no cue and sets the style",
			"{QTtext}\n[00:00:01.00]\n{italic}\n[00:00:02.00]\nSung\n[00:00:03.00]\n",
			[]cueRow{{2 * time.Second, 3 * time.Second, "<i>Sung</i>"}},
		},
		{
			"a lone brace is text",
			"{QTtext}\n[00:00:01.00]\na { b } c {\n[00:00:02.00]\n",
			[]cueRow{{time.Second, 2 * time.Second, "a { b } c {"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, lenient := range []bool{false, true} {
				s, warnings := mustRead(t, tt.content, lenient)
				assert.Empty(t, warnings)
				assert.Equal(t, tt.want, cueRows(s))
			}
		})
	}
}

// Adapted from testConvertingToInternalFormat of mantas-done/subtitles, tests/formats/TxtQuickTimeTest.php
// (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TxtQuickTimeTest.php), MIT.
// Ports testTheFractionCountsUnitsOfTheTimeScale of tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php.
func TestReadSubtitleEditShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"mantas-done Subtitle Edit shape",
			subtitleEditHeader + "[00:02:17.11]\nThe ferry leaves at nine,\nfrom the north pier.\n[00:02:20.09]\n\n[01:02:20.11]\nTickets at the kiosk.\n[01:02:22.12]\n\n",
			[]cueRow{
				{137367 * ms, 140300 * ms, "The ferry leaves at nine,\nfrom the north pier."},
				{3740367 * ms, 3742400 * ms, "Tickets at the kiosk."},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, false)
			assert.Empty(t, warnings)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

// Adapted from testParsesWithNoEndTimestamp of mantas-done/subtitles, tests/formats/TxtQuickTimeTest.php
// (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TxtQuickTimeTest.php), MIT.
// Ports testASampleEndsAtTheNextTimestampAndTheLastLastsTheLastCueDuration of
// tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php.
func TestReadLastCueDuration(t *testing.T) {
	t.Parallel()

	content := subtitleEditHeader + "[00:00:00.00]\na\n\n[00:00:12.00]\nb\n"
	tests := []struct {
		name string
		last *time.Duration
		want []cueRow
	}{
		{"mantas-done with 1 s", new(time.Second), []cueRow{{0, 12 * time.Second, "a"}, {12 * time.Second, 13 * time.Second, "b"}}},
		{"the default of 5 s", nil, []cueRow{{0, 12 * time.Second, "a"}, {12 * time.Second, 17 * time.Second, "b"}}},
		{"0", new(time.Duration(0)), []cueRow{{0, 12 * time.Second, "a"}, {12 * time.Second, 12 * time.Second, "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(content, model.CommonReadOptions{LastCueDuration: tt.last})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

func TestReadLastCueEndStaysBelowTheLimit(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}\n[99999:59:58.000]\nlast\n", false)
	assert.Equal(t, []cueRow{{99999*time.Hour + 59*time.Minute + 58*time.Second, timecode.MaxValidTime, "last"}}, cueRows(s))
}

func TestReadBlankContentGivesNoCues(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", " \n\t\r\n", "\xEF\xBB\xBF\n"} {
		s, warnings := mustRead(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
		assert.Nil(t, fileData(s))
	}
}

func TestReadFileData(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "\xEF\xBB\xBF{QTtext} {font:Tahoma}\n  {plain} \n\n[00:00:01.00]\nOne\n[00:00:02.00]\n", false)
	assert.Equal(t, map[string]any{"header": []any{"{QTtext} {font:Tahoma}", "  {plain} ", ""}}, fileData(s))

	s, _ = mustRead(t, "{QTtext}\n", false)
	assert.Zero(t, s.Len())
	assert.Equal(t, map[string]any{"header": []any{"{QTtext}", ""}}, fileData(s))
}

func TestReadStoresTheSourceOfSamplesWithDescriptors(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}\n[00:00:01.00] {font:Geneva}One\n  Two\n[00:00:02.00]\nPlain\n[00:00:03.00]\n", false)
	first, _ := s.Cue(0)
	data, ok := first.FormatData("qttext")
	require.True(t, ok)
	source, _ := data.Strings("source")
	assert.Equal(t, []string{" {font:Geneva}One", "  Two"}, source)
	second, _ := s.Cue(1)
	_, ok = second.FormatData("qttext")
	assert.False(t, ok)
}

// Ports the paired detection of docs/formats.md: an empty line after an end timestamp, and another timestamp after it.
func TestReadDetectsThePairedLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		paired  bool
	}{
		{"Subtitle Edit layout", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\n\n[00:00:03.00]\nB\n[00:00:04.00]\n\n", true},
		{"2 samples before the empty line", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\n\n[00:00:03.00]\nB\n", true},
		{"1 sample before the empty line", "{QTtext}\n[00:00:01.00]\nA\n\n[00:00:03.00]\nB\n", false},
		{"an empty line after text", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\nB\n\n[00:00:03.00]\nC\n", false},
		{"no timestamp after the empty line", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\n\n", false},
		{"QuickTime layout", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\nB\n[00:00:03.00]\n", false},
		{"an empty line with spaces", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\n \t\n[00:00:03.00]\nB\n", true},
		{"2 empty lines", "{QTtext}\n[00:00:01.00]\nA\n[00:00:02.00]\n\n\n[00:00:03.00]\nB\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, false)
			paired, _ := fileData(s)["pairedTimestamps"].(bool)
			assert.Equal(t, tt.paired, paired)
		})
	}
}

// Ports testStrictModeNeedsQtTextFirst and testLenientModeReadsAFileWithoutQtText of
// tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php.
func TestReadNeedsQtTextFirst(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"{timeScale:30}\n[00:00:01.00]\nOne\n[00:00:02.00]\n",
		"[00:00:01.00]\nOne\n[00:00:02.00]\n",
		"\nNote\n{QTtext}\n[00:00:01.00]\nOne\n[00:00:02.00]\n",
	} {
		_, _, err := read(content, model.CommonReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, &model.ParseError{Format: model.QuickTimeText, Line: 1, Msg: "The file does not start with {QTtext}."}, perr)

		s, warnings := mustRead(t, content, true)
		assert.Equal(t, []warningRow{{
			"The file does not start with {QTtext}. The parser read it as QuickTime text.", 1, model.NoBlock, nil, model.Repaired,
		}}, warningRows(warnings))
		assert.Equal(t, 1, s.Len())
	}
}

func TestReadAcceptsQtTextInAnyCaseAfterWhiteSpace(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"{qttext}", "  {QTTEXT} {timeScale:30}", "\t{QtText}"} {
		s, warnings := mustRead(t, content+"\n[00:00:01.00]\nOne\n[00:00:02.00]\n", false)
		assert.Empty(t, warnings)
		assert.Equal(t, 1, s.Len())
	}
}

// Ports testAFractionAtTheTimeScaleThrowsInStrictModeAndCountsInLenientMode of
// tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php.
func TestReadFractionAtTheTimeScale(t *testing.T) {
	t.Parallel()

	content := "{QTtext}{timeScale:30}\n[00:00:01.45]\nOne\n[00:00:04.00]\n"
	_, _, err := read(content, model.CommonReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, &model.ParseError{
		Format: model.QuickTimeText, Line: 2, Msg: `The timestamp "[00:00:01.45]" counts 45 units, but the time scale is 30 per second.`,
	}, perr)

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{2500 * ms, 4 * time.Second, "One"}}, cueRows(s))
	assert.Equal(t, []warningRow{{
		`The timestamp "[00:00:01.45]" counts 45 units, but the time scale is 30 per second. The parser added them to the seconds.`,
		2, model.NoBlock,
		[]string{"[00:00:01.45]"},
		model.Repaired,
	}}, warningRows(warnings))
}

func TestReadFractionBeyondTheTimeScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
		message string
	}{
		{
			"exactly the time scale",
			"{QTtext}{timeScale:30}\n[00:00:01.30]\nOne\n[00:00:04.00]\n",
			[]cueRow{{2 * time.Second, 4 * time.Second, "One"}},
			`The timestamp "[00:00:01.30]" counts 30 units, but the time scale is 30 per second.`,
		},
		{
			"leading zeros show in the message",
			"{QTtext}{timeScale:30}\n[00:00:01.0045]\nOne\n[00:00:04.00]\n",
			[]cueRow{{2500 * ms, 4 * time.Second, "One"}},
			`The timestamp "[00:00:01.0045]" counts 0045 units, but the time scale is 30 per second.`,
		},
		{
			"a fraction beyond the int64 range",
			"{QTtext}{timeScale:1000}\n[00:00:01.99999999999999999999]\nOne\n[00:00:04.00]\n",
			nil,
			`The timestamp "[00:00:01.99999999999999999999]" counts 99999999999999999999 units, but the time scale is 1000 per second.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := read(tt.content, model.CommonReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.message, perr.Msg)

			s, warnings := mustRead(t, tt.content, true)
			require.NotEmpty(t, warnings)
			assert.Equal(t, tt.message+" The parser added them to the seconds.", warnings[0].Message)
			if tt.want != nil {
				assert.Equal(t, tt.want, cueRows(s))
			}
		})
	}
}

func TestReadOrderedTimes(t *testing.T) {
	t.Parallel()

	t.Run("strict fails at the line of the sample", func(t *testing.T) {
		t.Parallel()

		_, _, err := read("{QTtext}\n[00:00:05.00]\nOne\n[00:00:01.00]\nTwo\n", model.CommonReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, &model.ParseError{Format: model.QuickTimeText, Line: 2, Msg: "The cue ends at 1 s, before it starts at 5 s."}, perr)
	})
	t.Run("lenient swaps up to 30 s", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, "{QTtext}\n[00:00:05.00]\nOne\n[00:00:01.00]\nTwo\n", true)
		assert.Equal(t, []cueRow{{time.Second, 5 * time.Second, "One"}, {time.Second, 6 * time.Second, "Two"}}, cueRows(s))
		assert.Equal(t, []warningRow{{
			"The cue ends at 1 s, before it starts at 5 s. The parser swapped the times.", 2, model.NoBlock, []string{"", "One"}[1:], model.Repaired,
		}}, warningRows(warnings))
	})
	t.Run("lenient skips a swap over 30 s and keeps the style", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, "{QTtext}\n[00:01:00.00]\n{italic}One\n[00:00:01.00]\nTwo\n", true)
		assert.Equal(t, []cueRow{{time.Second, 6 * time.Second, "Two"}}, cueRows(s))
		assert.Equal(t, []warningRow{{
			"The cue ends at 1 s, before it starts at 60 s.", 2, model.NoBlock, []string{"{italic}One"}, model.Skipped,
		}}, warningRows(warnings))
	})
	t.Run("relative timestamps cannot go back", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, "{QTtext}{timeStamps:relative}\n[00:00:05.0]\nOne\n[00:00:00.0]\nTwo\n", false)
		assert.Empty(t, warnings)
		assert.Equal(t, []cueRow{{5 * time.Second, 5 * time.Second, "One"}, {5 * time.Second, 10 * time.Second, "Two"}}, cueRows(s))
	})
}

func TestReadTimestampLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		ok      bool
		line    int
		message string
	}{
		{"the largest time", "{QTtext}\n[99999:59:59.999]\nx\n[99999:59:59.999]\n", true, 0, ""},
		{"100000 hours", "{QTtext}\n[100000:00:00.00]\nx\n", false, 2, `The time "100000:00:00.00" is not below 100000 hours.`},
		{"hours beyond the int64 range", "{QTtext}\n[99999999999999999999:00:00]\nx\n", false, 2, `The time "99999999999999999999:00:00" is not below 100000 hours.`},
		{"20 hour digits with zeros", "{QTtext}\n[00000000000000000001:00:00]\nx\n[00000000000000000002:00:00]\n", true, 0, ""},
		{"rounds up to 100000 hours", "{QTtext}\n[99999:59:59.9995]\nx\n", false, 2, `The time "99999:59:59.9995" is not below 100000 hours.`},
		{"rounds down below 100000 hours", "{QTtext}\n[99999:59:59.9994]\nx\n[99999:59:59.9994]\n", true, 0, ""},
		{
			"a time scale rounds up to 100000 hours", "{QTtext}{timeScale:2000}\n[99999:59:59.1999]\nx\n", false, 2,
			`The time "99999:59:59.1999" is not below 100000 hours.`,
		},
		{
			"relative times add up to the limit", "{QTtext}{timeStamps:relative}\n[99999:00:00]\nx\n[00:59:59.999]\n[00:00:00.001]\n", false, 5,
			`The time "00:00:00.001" is not below 100000 hours.`,
		},
		{"relative times below the limit", "{QTtext}{timeStamps:relative}\n[99999:00:00]\nx\n[00:59:59.998]\n[00:00:00.001]\n", true, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(tt.content, model.CommonReadOptions{})
			if tt.ok {
				require.NoError(t, err)
				for _, c := range s.All() {
					assert.Less(t, c.End, timecode.MaxTime)
				}
				return
			}
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.message, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
			assert.Equal(t, model.QuickTimeText, perr.Format)

			_, warnings := mustRead(t, tt.content, true)
			require.NotEmpty(t, warnings)
			last := warnings[len(warnings)-1]
			assert.Equal(t, tt.message, last.Message)
			assert.Equal(t, tt.line, last.Line)
			assert.Equal(t, model.NoBlock, last.Block)
			assert.Equal(t, model.Skipped, last.Action)
		})
	}
}

func TestReadFractionBeyondTheTimeScaleAndTheLimit(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "{QTtext}{timeScale:1}\n[00:00:00.4000000000]\nx\n", true)
	assert.Zero(t, s.Len())
	assert.Equal(t, []warningRow{
		{
			`The timestamp "[00:00:00.4000000000]" counts 4000000000 units, but the time scale is 1 per second. The parser added them to the seconds.`,
			2, model.NoBlock,
			[]string{"[00:00:00.4000000000]"},
			model.Repaired,
		},
		{`The time "00:00:00.4000000000" is not below 100000 hours.`, 2, model.NoBlock, []string{"[00:00:00.4000000000]"}, model.Skipped},
	}, warningRows(warnings))
}

func TestReadSkipsATimestampOfTheLimitWithItsTextInLenientMode(t *testing.T) {
	t.Parallel()

	content := "{QTtext}\n[00:00:01.00]\nOne\n[100000:00:00.00]  \nStray\n[00:00:03.00]\n"
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []warningRow{{
		`The time "100000:00:00.00" is not below 100000 hours.`, 4, model.NoBlock, []string{"[100000:00:00.00]  "}, model.Skipped,
	}}, warningRows(warnings))
	assert.Equal(t, []cueRow{{time.Second, 3 * time.Second, "One"}}, cueRows(s))
}

func TestReadKeepsInvalidUTF8(t *testing.T) {
	t.Parallel()

	content := "{QTtext}\n[00:00:01.00]\ncaf\xE9\n[00:00:02.00]\n"
	s, warnings := mustRead(t, content, false)
	assert.Empty(t, warnings)
	assert.Equal(t, "caf\xE9", cueRows(s)[0].Text)

	_, warnings = mustRead(t, content, true)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.True(t, strings.HasPrefix(warnings[0].Message, "The content is not valid UTF-8."), warnings[0].Message)
}

func TestReadReturnsTheReaderError(t *testing.T) {
	t.Parallel()

	_, _, err := qttext.Read(errReader{}, model.CommonReadOptions{}, qttext.ReadOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errRead)
}

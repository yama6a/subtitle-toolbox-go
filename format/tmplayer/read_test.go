package tmplayer_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testReadsTheVariants, testCueEndsAtTheNextCue, testLineNumberOneStartsANewCueAtTheSameTime and
// testKeepsColonsAndEscapesTagCharacters of tests/Parsers/TmPlayerParserTest.php of subtitle-toolbox-php,
// then adds rows of its own.
func TestRead(t *testing.T) {
	t.Parallel()

	twoLines := []string{"Where are you?", "Home."}
	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{"colon", "00:00:01:Where are you?|Home.\n", []cueRow{{time.Second, 6 * time.Second, twoLines}}},
		{"equals sign", "0:00:01=Where are you?|Home.\n", []cueRow{{time.Second, 6 * time.Second, twoLines}}},
		{"line numbers", "00:00:01,1=Where are you?\n00:00:01,2=Home.\n", []cueRow{{time.Second, 6 * time.Second, twoLines}}},
		{"line numbers and colon", "00:00:01,1:Where are you?\n00:00:01,2:Home.\n", []cueRow{{time.Second, 6 * time.Second, twoLines}}},
		{
			"a cue ends at the next entry",
			"00:00:01:One\n00:00:03:Two\n00:00:09:\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One"}}, {3 * time.Second, 9 * time.Second, []string{"Two"}}},
		},
		{
			"line number 1 at the same time starts a new cue",
			"00:00:01,1=One\n00:00:01,1=Two\n",
			[]cueRow{{time.Second, time.Second, []string{"One"}}, {time.Second, 6 * time.Second, []string{"Two"}}},
		},
		{
			"equal times without line numbers",
			"00:00:05:b\n00:00:05:a\n",
			[]cueRow{{5 * time.Second, 5 * time.Second, []string{"b"}}, {5 * time.Second, 10 * time.Second, []string{"a"}}},
		},
		{
			"line number 0 or 00 does not append",
			"00:00:01,0=One\n00:00:01,00=Two\n",
			[]cueRow{{time.Second, time.Second, []string{"One"}}, {time.Second, 6 * time.Second, []string{"Two"}}},
		},
		{
			"a line number above 1 at another time starts a new cue",
			"00:00:01,1=One\n00:00:02,2=Two\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}, {2 * time.Second, 7 * time.Second, []string{"Two"}}},
		},
		{
			"a line number above 1 appends only to the last entry of the file",
			"00:00:01,1=A\n00:00:02,1=B\n00:00:01,2=C\n",
			[]cueRow{
				{time.Second, time.Second, []string{"A"}},
				{time.Second, 2 * time.Second, []string{"C"}},
				{2 * time.Second, 7 * time.Second, []string{"B"}},
			},
		},
		{
			"a line number of 20 digits appends",
			"00:00:01,1=One\n00:00:01,99999999999999999999=Two\n",
			[]cueRow{{time.Second, 6 * time.Second, []string{"One", "Two"}}},
		},
		{
			"lines appended to an entry without text",
			"00:00:04,1=\n00:00:04,2=Text\n",
			[]cueRow{{4 * time.Second, 9 * time.Second, []string{"Text"}}},
		},
		{
			"out of time order",
			"00:00:07:B\n00:00:04:A\n",
			[]cueRow{{4 * time.Second, 7 * time.Second, []string{"A"}}, {7 * time.Second, 12 * time.Second, []string{"B"}}},
		},
		{
			"colons stay in the text and tag characters are escaped",
			"00:00:01:Time: 10:30 <now> & then",
			[]cueRow{{time.Second, 6 * time.Second, []string{"Time: 10:30 &lt;now&gt; &amp; then"}}},
		},
		{
			"empty parts of a break are dropped",
			"00:00:01: | A ||B |\n",
			[]cueRow{{time.Second, 6 * time.Second, []string{"A", "B"}}},
		},
		{"hours with 3 digits", "100:00:00:x\n", []cueRow{{100 * time.Hour, 100*time.Hour + 5*time.Second, []string{"x"}}}},
		{"leading zeros", "0000000000000000000001:02:03:x\n", []cueRow{{time.Hour + 2*time.Minute + 3*time.Second, time.Hour + 2*time.Minute + 8*time.Second, []string{"x"}}}},
		{
			"CR LF, a lone CR, blank lines and a BOM",
			"\xEF\xBB\xBF00:00:01:a\r\n\r\n00:00:03:b\r00:00:05:\n\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"a"}}, {3 * time.Second, 5 * time.Second, []string{"b"}}},
		},
		{"only an end entry", "00:00:01:\n", nil},
		{
			"the last cue ends at the time limit",
			"99999:59:59:x\n",
			[]cueRow{{99999*time.Hour + 59*time.Minute + 59*time.Second, timecode.MaxValidTime, []string{"x"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, false)
			assert.Equal(t, tt.want, cueRows(s))
			assert.Empty(t, warnings)
		})
	}
}

// Ports testLastCueDurationIsAConstructorArgument of tests/Parsers/TmPlayerParserTest.php of subtitle-toolbox-php,
// and the last cue rules of https://github.com/yama6a/subtitle-toolbox-go/issues/14.
func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		last time.Duration
		want time.Duration
	}{
		{"2.5 s", 2500 * ms, 62500 * ms},
		{"0 keeps the cue", 0, time.Minute},
		{"a duration beyond int64 with the start", time.Duration(math.MaxInt64), timecode.MaxValidTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read("00:01:00:Bye", model.CommonReadOptions{LastCueDuration: &tt.last})
			require.NoError(t, err)
			assert.Equal(t, []cueRow{{time.Minute, tt.want, []string{"Bye"}}}, cueRows(s))
		})
	}
}

func TestReadBlankContentGivesNoCues(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", " \n\r\n", "\xEF\xBB\xBF"} {
		s, warnings := mustRead(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
		msg     string
	}{
		{"a line without a time", "00:00:02:Fine\nbroken line\n", 2, `The line "broken line" is not a TMPlayer line.`},
		{"a blank line counts", "00:00:02:Fine\n\n\nbroken line\n", 4, `The line "broken line" is not a TMPlayer line.`},
		{"60 minutes", "00:60:00:x", 1, `The line "00:60:00:x" is not a TMPlayer line.`},
		{"60 seconds", "00:00:60:x", 1, `The line "00:00:60:x" is not a TMPlayer line.`},
		{"1 minute digit", "00:0:00:x", 1, `The line "00:0:00:x" is not a TMPlayer line.`},
		{"no separator", "00:00:01", 1, `The line "00:00:01" is not a TMPlayer line.`},
		{"a comma without digits", "00:00:01,=x", 1, `The line "00:00:01,=x" is not a TMPlayer line.`},
		{"a dot", "00:00:01.5:x", 1, `The line "00:00:01.5:x" is not a TMPlayer line.`},
		{"digits that are not ASCII", "00:00:0\u0661:x", 1, "The line \"00:00:0\u0661:x\" is not a TMPlayer line."},
		{"100000 hours", "100000:00:00:x", 1, `The time "100000:00:00" is not below 100000 hours.`},
		{"20 digits in the hours", "99999999999999999999:00:04:x", 1, `The time "99999999999999999999:00:04" is not below 100000 hours.`},
		{"digits as written", "000000000000000100000:59:59:x", 1, `The time "000000000000000100000:59:59" is not below 100000 hours.`},
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

// Ports testLenientModeSkipsLinesWithoutATime of tests/Parsers/TmPlayerParserTest.php of subtitle-toolbox-php.
func TestBrokenFile(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, string(testfiles.Read(t, "tmplayer/lenient/broken.txt")), true)
	assert.Equal(t, []cueRow{
		{2 * time.Second, 9 * time.Second, []string{"The museum opens at ten."}},
		{9 * time.Second, 14 * time.Second, []string{"The cafe is on the top floor."}},
	}, cueRows(s))
	assert.Equal(t, []warningRow{
		{`The line "Movie.Name.2004.DVDRip" is not a TMPlayer line.`, 1, 0, []string{"Movie.Name.2004.DVDRip"}, model.Skipped},
		{`The line "00:0x:06:Entry is free on Mondays." is not a TMPlayer line.`, 3, 2, []string{"00:0x:06:Entry is free on Mondays."}, model.Skipped},
	}, warningRows(warnings))
}

func TestLenientReadCountsBlocksFromTheNonEmptyLines(t *testing.T) {
	t.Parallel()

	_, warnings := mustRead(t, "00:00:01:a\n\n100000:00:00:b\nx\n", true)
	assert.Equal(t, []warningRow{
		{`The time "100000:00:00" is not below 100000 hours.`, 3, 1, []string{"100000:00:00:b"}, model.Skipped},
		{`The line "x" is not a TMPlayer line.`, 4, 2, []string{"x"}, model.Skipped},
	}, warningRows(warnings))
}

func TestInvalidUTF8StaysInTheText(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "00:00:01:caf\xe9\n", false)
	assert.Equal(t, []cueRow{{time.Second, 6 * time.Second, []string{"caf\xe9"}}}, cueRows(s))
	assert.Empty(t, warnings)

	s, warnings = mustRead(t, "00:00:01:caf\xe9\n", true)
	assert.Equal(t, []cueRow{{time.Second, 6 * time.Second, []string{"caf\xe9"}}}, cueRows(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
}

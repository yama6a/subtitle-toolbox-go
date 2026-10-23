package mpl2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testReadsItalicsAndLineBreaks, testKeepsASlashInsideALineAsText and testSortsCuesByStart of
// tests/Parsers/Mpl2ParserTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"italic line and line break",
			"[12][45]Where are you?|/Home.\n",
			[]cueRow{{1200 * ms, 4500 * ms, []string{"Where are you?", "<i>Home.</i>"}}},
		},
		{
			"spaces around the slash and the break",
			"[50][60]/ Both| / lines\n",
			[]cueRow{{5 * time.Second, 6 * time.Second, []string{"<i>Both</i>", "<i>lines</i>"}}},
		},
		{
			"a slash inside a line is text",
			"[0][10]Either/or <b>",
			[]cueRow{{0, time.Second, []string{"Either/or &lt;b&gt;"}}},
		},
		{
			"ampersand and angle brackets",
			"[0][10]Prices: 2 < 3 & 4 > 1.",
			[]cueRow{{0, time.Second, []string{"Prices: 2 &lt; 3 &amp; 4 &gt; 1."}}},
		},
		{
			"a slash alone and an empty part are dropped",
			"[0][10]/|  |/ |Text\n",
			[]cueRow{{0, time.Second, []string{"Text"}}},
		},
		{
			"two slashes make the second one text",
			"[0][10]//Text\n",
			[]cueRow{{0, time.Second, []string{"<i>/Text</i>"}}},
		},
		{"a cue without text", "[10][20]\n", []cueRow{{time.Second, 2 * time.Second, nil}}},
		{
			"sorted by start",
			"[30][40]Second\n[10][20]First\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"First"}}, {3 * time.Second, 4 * time.Second, []string{"Second"}}},
		},
		{
			"CR LF, a lone CR, blank lines and a BOM",
			"\xEF\xBB\xBF[1][2]a\r\n\r\n[3][4]b\r[5][6]c\n\n\n",
			[]cueRow{{100 * ms, 200 * ms, []string{"a"}}, {300 * ms, 400 * ms, []string{"b"}}, {500 * ms, 600 * ms, []string{"c"}}},
		},
		{
			"leading zeros",
			"[0000000000000000000005][010]x",
			[]cueRow{{500 * ms, time.Second, []string{"x"}}},
		},
		{
			"the largest time",
			"[3599999999][3599999999]x",
			[]cueRow{{timecode.MaxTime - 100*ms, timecode.MaxTime - 100*ms, []string{"x"}}},
		},
		{"a line trimmed of spaces and tabs", " \t[1][2]x \t", []cueRow{{100 * ms, 200 * ms, []string{"x"}}}},
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
		{"a line without times", "[1][2]ok\n\nhello", 3, `The line "hello" is not an MPL2 cue.`},
		{"one time only", "[1]hello", 1, `The line "[1]hello" is not an MPL2 cue.`},
		{"a letter in a time", "[1a][2]hello", 1, `The line "[1a][2]hello" is not an MPL2 cue.`},
		{"a space between the brackets", "[1] [2]hello", 1, `The line "[1] [2]hello" is not an MPL2 cue.`},
		{"digits that are not ASCII", "[\u0661][\u0662]hello", 1, "The line \"[\u0661][\u0662]hello\" is not an MPL2 cue."},
		{"a start at 100000 hours", "[3600000000][3600000001]x", 1, `The time "[3600000000]" is not below 100000 hours.`},
		{"an end at 100000 hours", "[1][3600000000]x", 1, `The time "[3600000000]" is not below 100000 hours.`},
		{
			"20 digits in the start", "[99999999999999999999][1]x", 1,
			`The time "[99999999999999999999]" is not below 100000 hours.`,
		},
		{
			"20 digits in the end", "[1][99999999999999999999]x", 1,
			`The time "[99999999999999999999]" is not below 100000 hours.`,
		},
		{"the start is checked before the end", "[3600000000][99999999999999999999]x", 1, `The time "[3600000000]" is not below 100000 hours.`},
		{"an end before the start", "[20][10]x", 1, "The cue ends at 1 s, before it starts at 2 s."},
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

func TestLenientReadSkipsOrSwaps(t *testing.T) {
	t.Parallel()

	content := "[1][2]a\n[3x][4]b\n\n[200][100]swapped\n[3000][1]too far\n[3600000000][1]c\n[5][6]d\n"
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{
		{100 * ms, 200 * ms, []string{"a"}},
		{500 * ms, 600 * ms, []string{"d"}},
		{10 * time.Second, 20 * time.Second, []string{"swapped"}},
	}, cueRows(s))
	assert.Equal(t, []warningRow{
		{`The line "[3x][4]b" is not an MPL2 cue.`, 2, 1, []string{"[3x][4]b"}, model.Skipped},
		{"The cue ends at 10 s, before it starts at 20 s. The parser swapped the times.", 4, 2, []string{"[200][100]swapped"}, model.Repaired},
		{"The cue ends at 0.1 s, before it starts at 300 s.", 5, 3, []string{"[3000][1]too far"}, model.Skipped},
		{`The time "[3600000000]" is not below 100000 hours.`, 6, 4, []string{"[3600000000][1]c"}, model.Skipped},
	}, warningRows(warnings))
}

// Ports testStrictModeThrowsWithTheLineNumber and testLenientModeSkipsLinesWithoutTimes of
// tests/Parsers/Mpl2ParserTest.php of subtitle-toolbox-php.
func TestBrokenFile(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "mpl2/lenient/broken.txt"))
	perr := readError(t, content)
	assert.Equal(t, `The line "Downloaded from a subtitle site" is not an MPL2 cue.`, perr.Msg)
	assert.Equal(t, 1, perr.Line)

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{
		{time.Second, 3 * time.Second, []string{"The market opens at ten."}},
		{7 * time.Second, 9500 * ms, []string{"<i>Bring a basket.</i>"}},
	}, cueRows(s))
	assert.Equal(t, []warningRow{
		{`The line "Downloaded from a subtitle site" is not an MPL2 cue.`, 1, 0, []string{"Downloaded from a subtitle site"}, model.Skipped},
		{`The line "[4x][60]The stalls sell fish." is not an MPL2 cue.`, 3, 2, []string{"[4x][60]The stalls sell fish."}, model.Skipped},
	}, warningRows(warnings))
}

func TestInvalidUTF8StaysInTheText(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "[1][2]caf\xe9\n", false)
	assert.Equal(t, []cueRow{{100 * ms, 200 * ms, []string{"caf\xe9"}}}, cueRows(s))
	assert.Empty(t, warnings)

	s, warnings = mustRead(t, "[1][2]caf\xe9\n", true)
	assert.Equal(t, []cueRow{{100 * ms, 200 * ms, []string{"caf\xe9"}}}, cueRows(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
}

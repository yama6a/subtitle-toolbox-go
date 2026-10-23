package mpsub_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testFrameRateUsesOnlyTheLeadingInteger, testWindowsLineEndingsAndMissingTrailingNewlineAreAccepted,
// testPlainTextIsEscapedAndRoundTrips and testLatin1TextKeepsItsBytes of tests/Parsers/MpSubParserTest.php
// of subtitle-toolbox-php, then adds a row for each line state of docs/formats.md.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"seconds",
			"TITLE=Harbour walk\nFORMAT=TIME\n\n1 2.5\nThe boat leaves.\n\n0.5 2\nMind the gap.\n",
			[]cueRow{{time.Second, 3500 * ms, []string{"The boat leaves."}}, {4 * time.Second, 6 * time.Second, []string{"Mind the gap."}}},
		},
		{
			"frames at 25 fps",
			"FORMAT=25\n\n25 50\nHello\n\n0 25\nWorld\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}, {3 * time.Second, 4 * time.Second, []string{"World"}}},
		},
		{
			"only the leading integer of the frame rate counts",
			"FORMAT=29.97\n\n29 58\nHello\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
		},
		{"a frame rate with text after the digits", "FORMAT=25fps\n\n25 25\nHello\n", []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}},
		{"a frame rate with leading zeros", "FORMAT=025\n\n25 25\nHello\n", []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}},
		{
			"FORMAT between cues changes the unit for the later cues",
			"FORMAT=TIME\n\n1 1\nA\n\nFORMAT=10\n\n10 10\nB\n\nFORMAT=TIME\n\n1 1\nC\n",
			[]cueRow{
				{time.Second, 2 * time.Second, []string{"A"}},
				{3 * time.Second, 4 * time.Second, []string{"B"}},
				{5 * time.Second, 6 * time.Second, []string{"C"}},
			},
		},
		{"fractional frames", "FORMAT=25\n\n12.5 12.5\nHalf\n", []cueRow{{500 * ms, time.Second, []string{"Half"}}}},
		{"a negative wait at the file start", "FORMAT=TIME\n\n-2 3\nHello\n", []cueRow{{-2 * time.Second, time.Second, []string{"Hello"}}}},
		{
			"a negative wait makes the cue overlap the cue before it",
			"FORMAT=TIME\n\n1 3\nFirst\n\n-1.5 2.5\nSecond\n",
			[]cueRow{{time.Second, 4 * time.Second, []string{"First"}}, {2500 * ms, 5 * time.Second, []string{"Second"}}},
		},
		{
			"the cues sort by start",
			"FORMAT=TIME\n\n5 1\nLate\n\n-4 1\nEarly\n",
			[]cueRow{{2 * time.Second, 3 * time.Second, []string{"Early"}}, {5 * time.Second, 6 * time.Second, []string{"Late"}}},
		},
		{"a negative zero", "FORMAT=TIME\n\n-0 -0\nHello\n", []cueRow{{0, 0, []string{"Hello"}}}},
		{
			"no FORMAT line reads seconds without a warning",
			"1 2\nHello\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
		},
		{
			"CR LF, blank lines and no final line end",
			"FORMAT=TIME\r\n\r\n1 2\r\nHello\r\n\r\n\r\n0.5 1\r\nWorld",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}, {3500 * ms, 4500 * ms, []string{"World"}}},
		},
		{
			"a lone CR ends a line",
			"FORMAT=TIME\r\r1 2\rHello\r\r0.5 1\rWorld",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}, {3500 * ms, 4500 * ms, []string{"World"}}},
		},
		{"a BOM", "\xEF\xBB\xBFFORMAT=TIME\n\n1 2\nHello\n", []cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}}},
		{
			"text lines are trimmed and escaped",
			"FORMAT=TIME\n\n1 1\n  I <3 bread & jam  \n\t<b>x</b>\t\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"I &lt;3 bread &amp; jam", "&lt;b&gt;x&lt;/b&gt;"}}},
		},
		{
			"a timing line with a tab and extra spaces",
			"FORMAT=TIME\n\n  1.50 \t\t 2.25  \nHello\n",
			[]cueRow{{1500 * ms, 3750 * ms, []string{"Hello"}}},
		},
		{
			"# lines outside a cue are comments, inside a cue they are text",
			"# top\nFORMAT=TIME # unit\n   # indented\n\n1 1\n# text\nHello # not a comment\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"# text", "Hello # not a comment"}}},
		},
		{
			"a header and a timing line inside a cue are text",
			"FORMAT=TIME\n\n1 1\nNOTE=x\n2 3\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"NOTE=x", "2 3"}}},
		},
		{
			"a header between cues",
			"FORMAT=TIME\n\n1 1\nA\n\nTITLE=Late\n\n1 1\nB\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"A"}}, {3 * time.Second, 4 * time.Second, []string{"B"}}},
		},
		{
			"many blank lines",
			"\n\nFORMAT=TIME\n\n\n\n1 1\nA\n\n\n\n\n0 1\nB\n\n\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"A"}}, {2 * time.Second, 3 * time.Second, []string{"B"}}},
		},
		{"invalid UTF-8 stays", "FORMAT=TIME\n\n1 1\ncaf\xE9 & tea\n", []cueRow{{time.Second, 2 * time.Second, []string{"caf\xE9 &amp; tea"}}}},
		{"a non-breaking space in the text", "FORMAT=TIME\n\n1 1\n Hi \n", []cueRow{{time.Second, 2 * time.Second, []string{" Hi "}}}},
		{"a zero duration", "FORMAT=TIME\n\n1 0\nHello\n", []cueRow{{time.Second, time.Second, []string{"Hello"}}}},
		{
			"digits beyond the float precision",
			"FORMAT=TIME\n\n0.1234567891234 0.0001\nHello\n",
			[]cueRow{{123 * ms, 124 * ms, []string{"Hello"}}},
		},
		{
			"times round half away from zero",
			"FORMAT=TIME\n\n1.0005 0.0005\nHello\n",
			[]cueRow{{1001 * ms, 1001 * ms, []string{"Hello"}}},
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

func TestReadBlankContentGivesNoCues(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", " \n\r\n", "\xEF\xBB\xBF", "FORMAT=TIME\n", "# comment\n\n"} {
		s, warnings := mustRead(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
	}
}

func TestReadHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		metadata map[string]string
		data     []string
	}{
		{
			"title, author and other keys",
			"TITLE=A\nAUTHOR=B\nTYPE=VIDEO\nFILE=1,abc\nNOTE=n\nFORMAT=TIME\n\n1 1\nx\n",
			map[string]string{"title": "A", "author": "B"},
			[]string{"TYPE=VIDEO", "FILE=1,abc", "NOTE=n"},
		},
		{"empty values are dropped", "EMPTY=\nTYPE=  \nNOTE=x\n", map[string]string{}, []string{"NOTE=x"}},
		{"an empty title deletes the key", "TITLE=A\nTITLE=\nAUTHOR=B\nAUTHOR= # none\n", map[string]string{}, nil},
		{"a later title replaces the earlier one", "TITLE=A\nTITLE=B\n", map[string]string{"title": "B"}, nil},
		{"a repeated key keeps its place and takes the last value", "NOTE=a\nTYPE=x\nNOTE=b\n", map[string]string{}, []string{"NOTE=b", "TYPE=x"}},
		{
			"the value is trimmed and a trailing comment goes",
			"TITLE=  Bakery  # the shop\nNOTE=a \t# b # c\nTYPE=a#b\nFILE=#x\n",
			map[string]string{"title": "Bakery"},
			[]string{"NOTE=a", "TYPE=a#b", "FILE=#x"},
		},
		{"an equals sign in the value", "NOTE=a=b\n", map[string]string{}, []string{"NOTE=a=b"}},
		{"FORMAT is never stored", "FORMAT=25\n", map[string]string{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, false)
			assert.Empty(t, warnings)
			assert.Equal(t, tt.metadata, s.AllMetadata())
			d, ok := s.FormatData("mpsub")
			assert.Equal(t, len(tt.data) > 0, ok)
			var got []string
			for _, k := range d.Keys() {
				v, _ := d.String(k)
				got = append(got, k+"="+v)
			}
			assert.Equal(t, tt.data, got)
		})
	}
}

// Ports testUnknownLineThrowsException, testNegativeDurationThrowsException, testCueWithoutTextThrowsException,
// testInvalidFormatThrowsException and testZeroFrameRateThrowsException of tests/Parsers/MpSubParserTest.php of
// subtitle-toolbox-php, and testMpSubSkipsABadFormatLineAndReadsTheTimesAsSeconds of
// tests/Parsers/LenientOtherFormatsTest.php, then adds rows for each error of docs/formats.md.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	const limit = `" is not below 100000 hours.`
	tests := []struct {
		name    string
		content string
		line    int
		msg     string
		// lenient is the result of a lenient read.
		cues     []cueRow
		warnings []warningRow
	}{
		{
			"unknown FORMAT value",
			"FORMAT=PAL\n\n1 2\nHello\n", 1, `The FORMAT value "PAL" is not known.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The FORMAT value "PAL" is not known.`, 1, 0, []string{"FORMAT=PAL"}, model.Skipped}},
		},
		{
			"FORMAT in lower case",
			"FORMAT=time\n\n1 2\nHello\n", 1, `The FORMAT value "time" is not known.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The FORMAT value "time" is not known.`, 1, 0, []string{"FORMAT=time"}, model.Skipped}},
		},
		{
			"empty FORMAT value",
			"FORMAT=\n\n1 2\nHello\n", 1, `The FORMAT value "" is not known.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The FORMAT value "" is not known.`, 1, 0, []string{"FORMAT="}, model.Skipped}},
		},
		{
			"a frame rate of 0",
			"FORMAT=0\n\n1 2\nHello\n", 1, `The frame rate "0" is not valid.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The frame rate "0" is not valid.`, 1, 0, []string{"FORMAT=0"}, model.Skipped}},
		},
		{
			"a frame rate that starts with 0",
			"FORMAT=0.5\n\n1 2\nHello\n", 1, `The frame rate "0.5" is not valid.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The frame rate "0.5" is not valid.`, 1, 0, []string{"FORMAT=0.5"}, model.Skipped}},
		},
		{
			"a bad FORMAT keeps the unit before it",
			"FORMAT=25\nFORMAT=PAL\n\n25 25\nHello\n", 2, `The FORMAT value "PAL" is not known.`,
			[]cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The FORMAT value "PAL" is not known.`, 2, 0, []string{"FORMAT=PAL"}, model.Skipped}},
		},
		{
			"a bad FORMAT after a cue has the block count of the timing lines so far",
			"FORMAT=TIME\n\n1 1\nA\n\nFORMAT=x\n", 6, `The FORMAT value "x" is not known.`,
			[]cueRow{{time.Second, 2 * time.Second, []string{"A"}}},
			[]warningRow{{`The FORMAT value "x" is not known.`, 6, 1, []string{"FORMAT=x"}, model.Skipped}},
		},
		{
			"a line that is no header, comment or timing line",
			"FORMAT=TIME\n\n00:00:01,000\nHello\nmore\n\n1 2\nOK\n", 3, `The line "00:00:01,000" is not a header, a comment or a timing line.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"OK"}}},
			[]warningRow{{`The line "00:00:01,000" is not a header, a comment or a timing line.`, 3, 0, []string{"00:00:01,000"}, model.Skipped}},
		},
		{
			"a header in lower case",
			"format=25\n\n1 2\nHello\n", 1, `The line "format=25" is not a header, a comment or a timing line.`,
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{{`The line "format=25" is not a header, a comment or a timing line.`, 1, 0, []string{"format=25"}, model.Skipped}},
		},
		{
			"a plus sign",
			"FORMAT=TIME\n\n+1 2\nHello\n", 3, `The line "+1 2" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "+1 2" is not a header, a comment or a timing line.`, 3, 0, []string{"+1 2"}, model.Skipped}},
		},
		{
			"an exponent",
			"FORMAT=TIME\n\n1e3 2\nHello\n", 3, `The line "1e3 2" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "1e3 2" is not a header, a comment or a timing line.`, 3, 0, []string{"1e3 2"}, model.Skipped}},
		},
		{
			"a number without digits after the dot",
			"FORMAT=TIME\n\n1. 2\nHello\n", 3, `The line "1. 2" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "1. 2" is not a header, a comment or a timing line.`, 3, 0, []string{"1. 2"}, model.Skipped}},
		},
		{
			"a comma as decimal separator",
			"FORMAT=TIME\n\n1,5 2\nHello\n", 3, `The line "1,5 2" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "1,5 2" is not a header, a comment or a timing line.`, 3, 0, []string{"1,5 2"}, model.Skipped}},
		},
		{
			"three numbers",
			"FORMAT=TIME\n\n1 2 3\nHello\n", 3, `The line "1 2 3" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "1 2 3" is not a header, a comment or a timing line.`, 3, 0, []string{"1 2 3"}, model.Skipped}},
		},
		{
			"one number",
			"FORMAT=TIME\n\n1\nHello\n", 3, `The line "1" is not a header, a comment or a timing line.`,
			nil,
			[]warningRow{{`The line "1" is not a header, a comment or a timing line.`, 3, 0, []string{"1"}, model.Skipped}},
		},
		{
			"a non-breaking space between the numbers",
			"FORMAT=TIME\n\n1 2\nHello\n", 3, "The line \"1 2\" is not a header, a comment or a timing line.",
			nil,
			[]warningRow{{"The line \"1 2\" is not a header, a comment or a timing line.", 3, 0, []string{"1 2"}, model.Skipped}},
		},
		{
			"a negative duration",
			"FORMAT=TIME\n\n1 -2\nHello\n\n1 1\nOK\n", 3, `The timing line "1 -2" has a negative duration.`,
			[]cueRow{{time.Second, 2 * time.Second, []string{"OK"}}},
			[]warningRow{{`The timing line "1 -2" has a negative duration.`, 3, 0, []string{"1 -2"}, model.Skipped}},
		},
		{
			"a cue without text lines before another cue",
			"FORMAT=TIME\n\n1 2\n\n3 4\nHello\n", 3, "The cue has no text lines.",
			[]cueRow{{6 * time.Second, 10 * time.Second, []string{"Hello"}}},
			[]warningRow{{"The cue has no text lines.", 3, 0, []string{"1 2"}, model.Skipped}},
		},
		{
			"a cue without text lines at the file end",
			"FORMAT=TIME\n\n1 2", 3, "The cue has no text lines.",
			nil,
			[]warningRow{{"The cue has no text lines.", 3, 0, []string{"1 2"}, model.Skipped}},
		},
		{
			"a cue without text lines at the file end with a line end",
			"FORMAT=TIME\n\n1 2\n", 3, "The cue has no text lines.",
			nil,
			[]warningRow{{"The cue has no text lines.", 3, 0, []string{"1 2"}, model.Skipped}},
		},
		{
			"a start at 100000 hours",
			"FORMAT=TIME\n\n360000000 1\nx\n\n1 1\nOK\n", 3, `The time "360000000 1` + limit,
			[]cueRow{{time.Second, 2 * time.Second, []string{"OK"}}},
			[]warningRow{{`The time "360000000 1` + limit, 3, 0, []string{"360000000 1"}, model.Skipped}},
		},
		{
			"an end at 100000 hours",
			"FORMAT=TIME\n\n1 359999999\nx\n\n1 1\nOK\n", 3, `The time "1 359999999` + limit,
			[]cueRow{{time.Second, 2 * time.Second, []string{"OK"}}},
			[]warningRow{{`The time "1 359999999` + limit, 3, 0, []string{"1 359999999"}, model.Skipped}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, tt.content)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.cues, cueRows(s))
			assert.Equal(t, tt.warnings, lenientWarnings(warnings))
		})
	}
}

// lenientWarnings drops the Repaired warning about the missing FORMAT line, which the error rows do not test.
func lenientWarnings(warnings []model.ParseWarning) []warningRow {
	rows := make([]warningRow, 0, len(warnings))
	for _, w := range warningRows(warnings) {
		if !strings.HasPrefix(w.Message, "The file has no FORMAT line") {
			rows = append(rows, w)
		}
	}
	return rows
}

func TestLenientWarnsOnceAboutAMissingFormatLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		cues     []cueRow
		warnings []warningRow
	}{
		{
			"at the first timing line",
			"1 2\nHello\n\n3 4\nWorld\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}, {6 * time.Second, 10 * time.Second, []string{"World"}}},
			[]warningRow{{"The file has no FORMAT line before line 1. The parser read the times as seconds.", 1, 0, []string{"1 2"}, model.Repaired}},
		},
		{
			"also when the first timing line fails",
			"x\nskipped\n\n1 2\nHello\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}},
			[]warningRow{
				{"The file has no FORMAT line before line 1. The parser read the times as seconds.", 1, 0, []string{"x"}, model.Repaired},
				{`The line "x" is not a header, a comment or a timing line.`, 1, 0, []string{"x"}, model.Skipped},
			},
		},
		{
			"a FORMAT line after the first timing line adds no warning",
			"1 1\nA\n\nFORMAT=10\n\n10 10\nB\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"A"}}, {3 * time.Second, 4 * time.Second, []string{"B"}}},
			[]warningRow{{"The file has no FORMAT line before line 1. The parser read the times as seconds.", 1, 0, []string{"1 1"}, model.Repaired}},
		},
		{
			"a bad FORMAT line counts as a FORMAT line",
			"FORMAT=PAL\n\n1 1\nA\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"A"}}},
			[]warningRow{{`The FORMAT value "PAL" is not known.`, 1, 0, []string{"FORMAT=PAL"}, model.Skipped}},
		},
		{"a file without timing lines", "TITLE=x\n", nil, []warningRow{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.cues, cueRows(s))
			assert.Equal(t, tt.warnings, warningRows(warnings))
		})
	}
}

func TestLenientSkipsUpToTheNextEmptyLine(t *testing.T) {
	t.Parallel()

	content := "FORMAT=TIME\n\nhello\nTITLE=skipped\n1 2\n\n1 2\nA\n"
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{time.Second, 3 * time.Second, []string{"A"}}}, cueRows(s))
	assert.Len(t, warnings, 1)
	assert.Empty(t, s.AllMetadata())
}

func TestFailedTimingLinesKeepThePosition(t *testing.T) {
	t.Parallel()

	content := "FORMAT=TIME\n\n1 1\nA\n\n0 999999999\nB\n\n-9 -1\nC\n\n0 1\nD\n"
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"A"}}, {2 * time.Second, 3 * time.Second, []string{"D"}}}, cueRows(s))
	assert.Len(t, warnings, 2)
}

func TestACueWithoutTextMovesThePosition(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "FORMAT=TIME\n\n1 2\n\n1 1\nA\n", true)
	assert.Equal(t, []cueRow{{4 * time.Second, 5 * time.Second, []string{"A"}}}, cueRows(s))
	assert.Len(t, warnings, 1)
}

// Ports the MPSub rows of testParsingExceptionsCarryTheLineNumber of tests/Exceptions/ParsingExceptionTest.php of
// subtitle-toolbox-php. The file there has no blank line before some timing lines.
func TestReadErrorLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
	}{
		{"unknown line", "FORMAT=TIME\n\n0 1\nfirst\n\nsecond\n", 6},
		{"negative duration", "FORMAT=TIME\n0 -1\ntext\n", 2},
		{"cue without text", "FORMAT=TIME\n0 1\n\n", 2},
		{"unknown FORMAT", "TITLE=x\nFORMAT=FAST\n", 2},
		{"frame rate 0", "TITLE=x\nFORMAT=0\n", 2},
		{"no timing line", "FORMAT=TIME\ntext\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.line, readError(t, tt.content).Line)
		})
	}
}

// Ports the MPSub row of timesPastTheLimit of tests/Parsers/TimesPastTheLimitTest.php of subtitle-toolbox-php.
func TestAWaitOf100000HoursFailsOrSkipsTheCue(t *testing.T) {
	t.Parallel()

	content := "FORMAT=TIME\n\n1 2\nThe pool opens at seven.\n\n360000000 2\nTowels are at the desk.\n\n4 2\nNo running, please.\n"
	perr := readError(t, content)
	assert.Equal(t, `The time "360000000 2" is not below 100000 hours.`, perr.Msg)
	assert.Equal(t, 6, perr.Line)

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{
		{time.Second, 3 * time.Second, []string{"The pool opens at seven."}},
		{7 * time.Second, 9 * time.Second, []string{"No running, please."}},
	}, cueRows(s))
	assert.Equal(t, []warningRow{{`The time "360000000 2" is not below 100000 hours.`, 6, 1, []string{"360000000 2"}, model.Skipped}}, warningRows(warnings))
}

// Rows of bug 159 of https://github.com/yama6a/subtitle-toolbox-go/issues/159: the limit applies to the time after the
// rounding to milliseconds, and to the sum of the relative times.
func TestReadTimeLimit(t *testing.T) {
	t.Parallel()

	maxStart := timecode.MaxTime - ms
	tests := []struct {
		name    string
		content string
		start   time.Duration
		end     time.Duration
		// errLine is the text of the error message, "" when the read works.
		errLine string
	}{
		{"a wait that rounds up to the limit", "FORMAT=TIME\n\n359999999.9996 0\nx\n", 0, 0, "359999999.9996 0"},
		{"a wait that rounds down", "FORMAT=TIME\n\n359999999.9994 0\nx\n", maxStart, maxStart, ""},
		{"a duration that rounds up to the limit", "FORMAT=TIME\n\n359999999 0.9996\nx\n", 0, 0, "359999999 0.9996"},
		{"each part below the limit, the sum at it", "FORMAT=TIME\n\n359999999.6 0.3996\nx\n", 0, 0, "359999999.6 0.3996"},
		{"each part below the limit, the sum below it", "FORMAT=TIME\n\n359999999.6 0.3994\nx\n", 359999999600 * ms, maxStart, ""},
		{"the end at the largest time", "FORMAT=TIME\n\n0 359999999.999\nx\n", 0, maxStart, ""},
		{
			"the position and the wait of the next cue", "FORMAT=TIME\n\n359999999 0\nx\n\n0.9996 0\ny\n",
			0, 0, "0.9996 0",
		},
		{"the position and the duration of the next cue", "FORMAT=TIME\n\n359999999 0\nx\n\n0 0.9996\ny\n", 0, 0, "0 0.9996"},
		{"frames that round up to the limit", "FORMAT=25\n\n8999999999.99 0\nx\n", 0, 0, "8999999999.99 0"},
		{"frames below the limit", "FORMAT=25\n\n8999999999.9 0\nx\n", 359999999996 * ms, 359999999996 * ms, ""},
		{"a negative wait at the limit", "FORMAT=TIME\n\n-359999999.9996 1\nx\n", 0, 0, "-359999999.9996 1"},
		{"a negative wait above the limit", "FORMAT=TIME\n\n-359999999.9994 1\nx\n", -maxStart, -maxStart + time.Second, ""},
		{"a negative wait of 23 digits", "FORMAT=TIME\n\n-99999999999999999999999 1\nx\n", 0, 0, "-99999999999999999999999 1"},
		{"a wait of 400 digits", "FORMAT=TIME\n\n1" + strings.Repeat("0", 400) + " 1\nx\n", 0, 0, "1" + strings.Repeat("0", 400) + " 1"},
		{"a negative wait of 400 digits", "FORMAT=TIME\n\n-1" + strings.Repeat("0", 400) + " 1\nx\n", 0, 0, "-1" + strings.Repeat("0", 400) + " 1"},
		{"a duration of 400 digits", "FORMAT=TIME\n\n1 1" + strings.Repeat("0", 400) + "\nx\n", 0, 0, "1 1" + strings.Repeat("0", 400)},
		{"a wait beyond the int64 range in nanoseconds", "FORMAT=TIME\n\n9999999999 1\nx\n", 0, 0, "9999999999 1"},
		{"a negative wait beyond the int64 range in nanoseconds", "FORMAT=TIME\n\n-9999999999 1\nx\n", 0, 0, "-9999999999 1"},
		{"a frame rate beyond the int64 range", "FORMAT=99999999999999999999\n\n1 1\nx\n", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.errLine == "" {
				s, _ := mustRead(t, tt.content, false)
				require.Equal(t, 1, s.Len())
				c := s.Cues()[0]
				assert.Equal(t, [2]time.Duration{tt.start, tt.end}, [2]time.Duration{c.Start, c.End})
				return
			}
			perr := readError(t, tt.content)
			assert.Equal(t, `The time "`+tt.errLine+`" is not below 100000 hours.`, perr.Msg)
			before, _, found := strings.Cut(tt.content, tt.errLine)
			require.True(t, found)
			assert.Equal(t, strings.Count(before, "\n")+1, perr.Line)
		})
	}
}

// Fractional frames: the reader keeps the position in float64 seconds and rounds only the cue times, as PHP does.
// Rounding the position of each cue to milliseconds would drift from the expected times.
func TestReadFractionalFramesKeepTheUnroundedPosition(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	content.WriteString("FORMAT=30\n")
	want := make([]cueRow, 0, 100)
	position := 0.0
	for i := range 100 {
		content.WriteString("\n1.1 2.3\ncue " + string(rune('A'+i%26)) + "\n")
		start := position + 1.1/30
		position = start + 2.3/30
		want = append(want, cueRow{phpRound(start), phpRound(position), []string{"cue " + string(rune('A'+i%26))}})
	}
	s, warnings := mustRead(t, content.String(), false)
	assert.Empty(t, warnings)
	assert.Equal(t, want, cueRows(s))

	// The same file with a position rounded after each cue ends 100 cues later by more than a millisecond.
	rounded := 0.0
	for range 100 {
		rounded = float64(phpRound(rounded+1.1/30)/ms) / 1000
		rounded = float64(phpRound(rounded+2.3/30)/ms) / 1000
	}
	assert.Greater(t, math.Abs(rounded-position), 0.01)
}

// phpRound rounds seconds as PHP round($seconds, 3) does for the times of the read goldens.
func phpRound(secs float64) time.Duration {
	return time.Duration(math.Round(secs * 1e9)).Round(ms)
}

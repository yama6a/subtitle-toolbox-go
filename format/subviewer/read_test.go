package subviewer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const script = "******** START SCRIPT ********\n"

// Ports the SubViewer 2 tests of tests/Parsers/SubViewerParserTest.php of subtitle-toolbox-php, then adds a row
// for each rule of docs/formats.md.
func TestReadVersion2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"text lines and [br] become lines",
			"00:00:01.50,00:00:04.00\nFresh bread[br]every day\nand cakes\n",
			[]cueRow{{1500 * ms, 4 * time.Second, []string{"Fresh bread", "every day", "and cakes"}}},
		},
		{
			"1 to 3 fraction digits are a decimal fraction",
			"00:00:01.5,00:00:02.250\nOne\n\n00:00:03.05,00:00:04.00\nTwo\n",
			[]cueRow{{1500 * ms, 2250 * ms, []string{"One"}}, {3050 * ms, 4 * time.Second, []string{"Two"}}},
		},
		{
			"[DELAY] is kept and not applied",
			"[INFORMATION]\n[DELAY]5\n[END INFORMATION]\n[SUBTITLE]\n00:00:01.00,00:00:02.00\nHello\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}},
		},
		{
			"a style line between cues is not text",
			"[SUBTITLE]\n[COLF]&HFFFFFF,[SIZE]18\n00:00:01.00,00:00:02.00\nOne\n\n[COLF]&H00FF00,[SIZE]20\n00:00:03.00,00:00:04.00\nTwo\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}, {3 * time.Second, 4 * time.Second, []string{"Two"}}},
		},
		{
			"a timing line without text gives no cue",
			"00:00:01.00,00:00:02.00\n\n00:00:03.00,00:00:04.00\nHello\n",
			[]cueRow{{3 * time.Second, 4 * time.Second, []string{"Hello"}}},
		},
		{
			"[BR] in upper case stays text",
			"00:00:01.00,00:00:02.00\nOne[BR]two\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One[BR]two"}}},
		},
		{
			"parts of [br] are trimmed and empty parts dropped",
			"00:00:01.00,00:00:02.00\n  One [br] [br]two  [br]\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One", "two"}}},
		},
		{
			"a text line with a comma and a time",
			"00:00:01.00,00:00:03.00\nWe meet at 10:30, at the gate.\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"We meet at 10:30, at the gate."}}},
		},
		{
			"a text line that looks like a header",
			"00:00:01.00,00:00:03.00\n[Music] starts\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"[Music] starts"}}},
		},
		{
			"text is escaped",
			"00:00:01.00,00:00:02.00\nFlour & water <and> salt\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"Flour &amp; water &lt;and&gt; salt"}}},
		},
		{
			"the largest time",
			"99999:59:59.999,99999:59:59.999\nHello\n",
			[]cueRow{{99999*time.Hour + 59*time.Minute + 59*time.Second + 999*ms, 99999*time.Hour + 59*time.Minute + 59*time.Second + 999*ms, []string{"Hello"}}},
		},
		{
			"hours with leading zeros",
			"000000000001:00:00.00,0000000000002:00:00.00\nHello\n",
			[]cueRow{{time.Hour, 2 * time.Hour, []string{"Hello"}}},
		},
		{
			"CR LF, a lone CR and no final line end",
			"00:00:01.00,00:00:02.00\r\nOne\r\r00:00:03.00,00:00:04.00\rTwo",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}, {3 * time.Second, 4 * time.Second, []string{"Two"}}},
		},
		{
			"the cues sort by start",
			"00:00:05.00,00:00:06.00\nLate\n\n00:00:01.00,00:00:02.00\nEarly\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"Early"}}, {5 * time.Second, 6 * time.Second, []string{"Late"}}},
		},
		{"a header line only", "[INFORMATION]\n[TITLE]Bakery\n[END INFORMATION]\n[SUBTITLE]\n", nil},
		{"a start script line in the middle of a line is no script", "00:00:01.00,00:00:02.00\nx ******** START SCRIPT ********\n", []cueRow{{time.Second, 2 * time.Second, []string{"x ******** START SCRIPT ********"}}}},
		{"empty", "", nil},
		{"blank", " \n\t\n", nil},
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

func TestVersion2Header(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[INFORMATION]\n[TITLE] Weather report\n[AUTHOR]\n[source]  Dubbed \n[CD TRACK]0\n[Comment]\n[END INFORMATION]\n[subtitle]\n"+
		"[COLF]&HFFFFFF,[STYLE]bd\n[COLF]&H00FF00\n00:00:01.00,00:00:02.00\nHello\n\n[SIZE]3\n", false)
	assert.Equal(t, map[string]string{"title": "Weather report"}, s.AllMetadata())
	assert.Equal(t, map[string]string{"SOURCE": "Dubbed", "CD TRACK": "0", "COMMENT": ""}, header(s))
	d, _ := s.FormatData("subviewer")
	assert.Equal(t, []string{"version", "header", "style"}, d.Keys())
	assert.Equal(t, "[COLF]&HFFFFFF,[STYLE]bd", fileData(s)["style"])
	assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}, cueRows(s))
}

func TestVersion2HeaderEmptyValueDeletesMetadata(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[TITLE]Bakery\n[TITLE]\n[AUTHOR]Jane\n00:00:01.00,00:00:02.00\nHello\n", false)
	assert.Equal(t, map[string]string{"author": "Jane"}, s.AllMetadata())
	assert.Empty(t, header(s))
}

// PHP stores the tags 0, 1 and so on as a list, so library JSON writes an array.
func TestHeaderOfNumberedTagsIsAList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content string
		want          any
	}{
		{"0 and 1", "[0]a\n[1]b\n" + script, []any{"a", "b"}},
		{"0, 1 and 0 again", "[0]a\n[1]b\n[0]c\n" + script, []any{"c", "b"}},
		{"0 and 2", "[0]a\n[2]b\n" + script, map[string]any{"0": "a", "2": "b"}},
		{"1 and 0", "[1]a\n[0]b\n" + script, map[string]any{"1": "a", "0": "b"}},
		{"01", "[01]a\n" + script, map[string]any{"01": "a"}},
		{"0 and a name", "[0]a\n[B]b\n" + script, map[string]any{"0": "a", "B": "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustReadAll(t, tt.content)
			assert.Equal(t, tt.want, fileData(s)["header"])
			var out bytes.Buffer
			require.NoError(t, subviewer.Write(&out, s, model.CommonWriteOptions{}, subviewer.WriteOptions{}))
			back := mustReadAll(t, out.String())
			assert.Equal(t, tt.want, fileData(back)["header"])
		})
	}
}

func TestVersion2WithoutHeaderStoresAnEmptyList(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "00:00:01.00,00:00:02.00\nHello\n", false)
	assert.Equal(t, map[string]any{"version": int64(2), "header": []any{}}, fileData(s))
}

// Ports testSubViewer2TextBeforeTheFirstTimingLineThrows and the strict rows of tests/Parsers/LenientSubViewerTest.php.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
		msg     string
	}{
		{
			"text before the first timing line", "[SUBTITLE]\nHello\n00:00:01.00,00:00:02.00\nWorld\n", 2,
			`The line "Hello" is neither a header tag nor a timing line.`,
		},
		{
			"a bad time", "00:00:01.00,00:00:03.00\nOne\n\n00:00:04.00,00:00:0x.00\nTwo\n", 4,
			`The timing line "00:00:04.00,00:00:0x.00" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`,
		},
		{
			"minutes of 60 or more", "00:00:01.00,00:60:03.00\nOne\n", 1,
			`The timing line "00:00:01.00,00:60:03.00" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`,
		},
		{
			"4 fraction digits", "00:00:01.5000,00:00:03.0000\nOne\n", 1,
			`The timing line "00:00:01.5000,00:00:03.0000" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`,
		},
		{
			"6 hour digits", "100000:00:01.00,00:00:03.00\nOne\n", 1,
			`The timing line "100000:00:01.00,00:00:03.00" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`,
		},
		{
			"one field is a time", "Hello,00:00:03.00\nOne\n", 1,
			`The timing line "Hello,00:00:03.00" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`,
		},
		{
			"the cue ends before it starts", "00:00:05.00,00:00:03.00\nOne\n", 1,
			"The cue ends at 3 s, before it starts at 5 s.",
		},
		{
			"a header line without a bracket in version 1", "Ferry times\n" + script + "[00:00:02]\nHello\n", 1,
			`The line "Ferry times" is not a SubViewer 1 header tag.`,
		},
		{
			"a header line with an empty tag in version 1", "[]x\n" + script, 1,
			`The line "[]x" is not a SubViewer 1 header tag.`,
		},
		{
			"a time of 60 minutes in version 1", script + "[00:60:02]\nHello\n", 2,
			`The time "[00:60:02]" is not valid.`,
		},
		{
			"a time of 60 seconds in version 1", script + "[00:00:60]\nHello\n", 2,
			`The time "[00:00:60]" is not valid.`,
		},
		{
			"the cue ends before it starts in version 1", script + "[00:00:05]\nHello\n[00:00:03]\n", 2,
			"The cue ends at 3 s, before it starts at 5 s.",
		},
		{
			"a delay beyond the limit", "[DELAY]\n359999990\n" + script + "[00:00:12]\nHello\n", 4,
			`The time "[00:00:12] with DELAY 359999990" is not below 100000 hours.`,
		},
		{
			"a delay beyond the int64 range", "[DELAY]99999999999999999999\n" + script + "[00:00:12]\nHello\n", 3,
			`The time "[00:00:12] with DELAY 9223372036854775807" is not below 100000 hours.`,
		},
		{
			"a negative delay beyond the limit", "[DELAY]-99999999999999999999\n" + script + "[00:00:12]\nHello\n", 3,
			`The time "[00:00:12] with DELAY -9223372036854775808" is not below 100000 hours.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := readError(t, tt.content)
			assert.Equal(t, tt.line, err.Line)
			assert.Equal(t, tt.msg, err.Msg)
		})
	}
}

// Ports the SubViewer 1 tests of tests/Parsers/SubViewerParserTest.php.
func TestReadVersion1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			"the pipe is a line break",
			script + "[00:00:01]\nRain|then sun\n[00:00:04]\n\n",
			[]cueRow{{time.Second, 4 * time.Second, []string{"Rain", "then sun"}}},
		},
		{
			"a cue without an end line ends at the next cue",
			script + "[00:00:01]\nOne\n[00:00:03]\nTwo\n[00:00:05]\n\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One"}}, {3 * time.Second, 5 * time.Second, []string{"Two"}}},
		},
		{
			"the last cue without an end line lasts 5 seconds",
			script + "[00:00:01]\nOne\n",
			[]cueRow{{time.Second, 6 * time.Second, []string{"One"}}},
		},
		{
			"the end of the script ends a pending time",
			script + "[00:00:01]\nOne\n[00:00:03]",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One"}}},
		},
		{
			"only the first line after a time line is read",
			script + "[00:00:01]\nOne\nTwo\n[00:00:03]\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One"}}},
		},
		{
			"an empty part between pipes is dropped by the cue",
			script + "[00:00:01]\nOne||Two|\n[00:00:03]\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One", "Two"}}},
		},
		{
			"a time line after a time line ends the cue",
			script + "[00:00:01]\nOne\n[00:00:03]\n[00:00:04]\nTwo\n[00:00:06]\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"One"}}, {4 * time.Second, 6 * time.Second, []string{"Two"}}},
		},
		{
			"a cue without an end line at the time of the next cue",
			script + "[00:00:01]\nOne\n[00:00:01]\nTwo\n[00:00:04]\n",
			[]cueRow{{time.Second, time.Second, []string{"One"}}, {time.Second, 4 * time.Second, []string{"Two"}}},
		},
		{
			"a cue without an end line before an earlier cue lasts the last cue duration",
			script + "[00:00:05]\nOne\n[00:00:03]\nTwo\n[00:00:09]\n",
			[]cueRow{{3 * time.Second, 9 * time.Second, []string{"Two"}}, {5 * time.Second, 10 * time.Second, []string{"One"}}},
		},
		{
			"the script marker with white space and the text of a second marker",
			"  ******** START SCRIPT ********  \r\n[00:00:01]\r\n" + strings.TrimSuffix(script, "\n") + "\r\n[00:00:03]\r\n",
			[]cueRow{{time.Second, 3 * time.Second, []string{"******** START SCRIPT ********"}}},
		},
		{
			"hours with 5 digits and leading zeros",
			script + "[0099999:59:58]\nOne\n[99999:59:59]\n",
			[]cueRow{{99999*time.Hour + 59*time.Minute + 58*time.Second, 99999*time.Hour + 59*time.Minute + 59*time.Second, []string{"One"}}},
		},
		{"no cues", script + "[end]\n******** END SCRIPT ********\n", nil},
		{"text is escaped", script + "[00:00:01]\nTea & <cake>\n[00:00:02]\n", []cueRow{{time.Second, 2 * time.Second, []string{"Tea &amp; &lt;cake&gt;"}}}},
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

// Ports testSubViewer1LastCueWithoutEndLineLastsTheGivenDuration of tests/Parsers/SubViewerParserTest.php.
func TestVersion1LastCueDuration(t *testing.T) {
	t.Parallel()

	content := script + "[00:00:01]\nOne\n"
	tests := []struct {
		name string
		last *time.Duration
		want time.Duration
	}{
		{"default", nil, 6 * time.Second},
		{"2.5 seconds", new(2500 * ms), 3500 * ms},
		{"0", new(time.Duration(0)), time.Second},
		{"rounded to milliseconds", new(1234567 * time.Microsecond), 2235 * ms},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(content, model.CommonReadOptions{LastCueDuration: tt.last})
			require.NoError(t, err)
			assert.Equal(t, []cueRow{{time.Second, tt.want, []string{"One"}}}, cueRows(s))
		})
	}
}

func TestVersion1LastCueEndStopsBelowTheLimit(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, script+"[99999:59:58]\nOne\n", false)
	assert.Equal(t, 100000*time.Hour-ms, s.Cues()[0].End)
}

func TestVersion1Header(t *testing.T) {
	t.Parallel()

	content := "[TITLE]Ferry times\n[Author]\nTom Berg\n[DELAY]\n-1\n[CD TRACK]\n[BEGIN]\n[MIXED]  a b  \n[ALONE]\n\n[ODD]\n" +
		script + "[00:00:02]\nHello\n[00:00:03]\n"
	s, _ := mustRead(t, content, false)
	assert.Equal(t, map[string]string{"title": "Ferry times", "author": "Tom Berg"}, s.AllMetadata())
	assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}, cueRows(s))
	assert.Equal(t, map[string]string{"DELAY": "0", "CD TRACK": "", "BEGIN": "", "MIXED": "a b", "ALONE": "", "ODD": ""}, header(s))
	d, _ := s.FormatData("subviewer")
	version, _ := d.Int("version")
	assert.Equal(t, int64(1), version)
	h, _ := d.Object("header")
	assert.Equal(t, []string{"DELAY", "CD TRACK", "BEGIN", "MIXED", "ALONE", "ODD"}, h.Keys())
}

func TestVersion1HeaderValueOnTheNextLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, header string
		want         map[string]string
	}{
		{"the next line is the value", "[TITLE]\nMarket\n", map[string]string{"title": "Market"}},
		{"a value on the tag line", "[TITLE]Fair\n", map[string]string{"title": "Fair"}},
		{"an empty next line is no value", "[TITLE]\n\n", nil},
		{"a next line with [ is no value", "[TITLE]\n[SOURCE]\nMarket\n", nil},
		{"the next line is trimmed", "[TITLE]\n  Market \t\n", map[string]string{"title": "Market"}},
		{"the tag is trimmed and in upper case", "[ title ]\nMarket\n", map[string]string{"title": "Market"}},
		{"an empty title deletes the key", "[TITLE]Fair\n[TITLE]\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustReadAll(t, tt.header+script)
			assert.Equal(t, orEmpty(tt.want), s.AllMetadata())
		})
	}
}

func TestVersion1TagLineAfterAnEmptyTagIsATag(t *testing.T) {
	t.Parallel()

	s := mustReadAll(t, "[SOURCE]\n[x]y\n"+script)
	assert.Equal(t, map[string]string{"SOURCE": "", "X": "y"}, header(s))
}

func TestVersion1Delay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		delay string
		start time.Duration
	}{
		{"2", 3 * time.Second},
		{"1e999", time.Second},
		{"-1e999", time.Second},
		{"1.8e308", time.Second},
		{"2.5", 3 * time.Second},
		{"12abc", 13 * time.Second},
		{"abc", time.Second},
		{"", time.Second},
		{"-1", 0},
		{"+4", 5 * time.Second},
		{" 3", 4 * time.Second},
		{"1e3", 1001 * time.Second},
		{"2.5e1", 26 * time.Second},
		{".5e1", 6 * time.Second},
		{"1e", 2 * time.Second},
		{"0x10", time.Second},
		{"-.5", time.Second},
		{"-0", time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.delay, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, "[DELAY]"+tt.delay+"\n"+script+"[00:00:01]\nHello\n[00:00:02]\n", false)
			assert.Equal(t, tt.start, s.Cues()[0].Start)
			assert.Equal(t, "0", header(s)["DELAY"])
		})
	}
}

// Ports testSubViewer1HeaderValueOnTheTagLineIsRead of tests/Parsers/SubViewerParserTest.php, with a negative delay.
func TestVersion1NegativeDelay(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[TITLE]Ferry times\n[DELAY]\n-1\n"+script+"[00:00:02]\nHello\n[00:00:03]\n", false)
	assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}, cueRows(s))

	s, _ = mustRead(t, "[DELAY]-5\n"+script+"[00:00:02]\nHello\n[00:00:03]\n", false)
	assert.Equal(t, []cueRow{{-3 * time.Second, -2 * time.Second, []string{"Hello"}}}, cueRows(s))
}

// The example of the issue: a delay of 2 gives Sunny and cloudy from 3 to 6 s, Light wind from 8 to 11 s, and Snow
// tonight from 11 s for the last cue duration.
func TestVersion1ExampleOfTheIssue(t *testing.T) {
	t.Parallel()

	content := "[DELAY]\n2\n" + script + "[00:00:01]\nSunny|cloudy\n[00:00:04]\n\n[00:00:06]\nLight wind\n[00:00:09]\nSnow tonight\n"
	s, _ := mustRead(t, content, false)
	assert.Equal(t, []cueRow{
		{3 * time.Second, 6 * time.Second, []string{"Sunny", "cloudy"}},
		{8 * time.Second, 11 * time.Second, []string{"Light wind"}},
		{11 * time.Second, 16 * time.Second, []string{"Snow tonight"}},
	}, cueRows(s))
}

func TestVersion1DelayUpToTheLimit(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[DELAY]359999990\n"+script+"[00:00:09]\nHello\n[00:00:09]\n", false)
	assert.Equal(t, 100000*time.Hour-time.Second, s.Cues()[0].Start)
}

func TestVersionSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		version int64
	}{
		{"the marker", script + "[00:00:01]\nHello\n[00:00:02]\n", 1},
		{"the marker in the middle", "[TITLE]x\n" + script, 1},
		{"the marker with spaces", " \t******** START SCRIPT ******** \n", 1},
		{"a marker with other stars", "*** START SCRIPT ***\n00:00:01.00,00:00:02.00\nHello\n", 2},
		{"the marker in lower case", "******** start script ********\n00:00:01.00,00:00:02.00\nHello\n", 2},
		{"no marker", "00:00:01.00,00:00:02.00\nHello\n", 2},
		{"a V2 file with the marker as text is a V1 file", "00:00:01.00,00:00:02.00\nHello\n" + script, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := tt.content
			s, _, err := read(content, model.CommonReadOptions{Lenient: true})
			require.NoError(t, err)
			d, _ := s.FormatData("subviewer")
			version, _ := d.Int("version")
			assert.Equal(t, tt.version, version)
		})
	}
}

func TestInvalidUTF8StaysAsItIs(t *testing.T) {
	t.Parallel()

	content := "00:00:01.00,00:00:02.00\nCaf\xe9\n"
	s, warnings := mustRead(t, content, false)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"Caf\xe9"}, s.Cues()[0].Lines)

	s, warnings = mustRead(t, content, true)
	assert.Equal(t, []string{"Caf\xe9"}, s.Cues()[0].Lines)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.Contains(t, warnings[0].Message, "first bad byte is at offset 27")
}

func TestControlCharactersAreRemoved(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "00:00:01.00,00:00:02.00\nHe\x01llo\n", true)
	assert.Equal(t, []string{"Hello"}, s.Cues()[0].Lines)
	require.Len(t, warnings, 1)
	assert.Equal(t, "The file has control characters, the first on line 2. The parser removed them.", warnings[0].Message)
}

func mustReadAll(t *testing.T, content string) *model.Subtitle {
	t.Helper()

	s, _ := mustRead(t, content, false)
	return s
}

func orEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

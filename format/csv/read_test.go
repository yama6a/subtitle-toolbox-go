package csv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var (
	strict  = model.CommonReadOptions{}
	lenient = model.CommonReadOptions{Lenient: true}
	at25    = csv.ReadOptions{FrameRate: 25}
)

// Ports the cue tests of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		common  model.CommonReadOptions
		opts    csv.ReadOptions
		want    []cueRow
	}{
		{
			name: "speakers and a quoted line break",
			content: "start,end,speaker,text\n00:00:01.000,00:00:04.000,Anna,\"Where are you going?\"\n" +
				"00:00:04.500,00:00:06.000,Ben,\"Home.\nNow.\"\n",
			want: []cueRow{{time.Second, 4 * time.Second, []string{"<v Anna>Where are you going?"}}, {4500 * ms, 6 * time.Second, []string{"<v Ben>Home.", "Now."}}},
		},
		{
			name: "delimiter option wins over detection", content: "start;end;text\n1;2;a,b\n",
			opts: csv.ReadOptions{Delimiter: csv.Semicolon},
			want: []cueRow{{time.Second, 2 * time.Second, []string{"a,b"}}},
		},
		{
			name: "comma option with a mapped header that holds a semicolon", content: "start,end,text;more\n1,2,x;y\n",
			opts: csv.ReadOptions{Columns: csv.Columns{Text: csv.ColumnNamed("text;more")}, Delimiter: csv.Comma},
			want: []cueRow{{time.Second, 2 * time.Second, []string{"x;y"}}},
		},
		{
			name: "RFC 4180 quoting", content: "start,end,text\r\n1,2,\"She said \"\"hi\"\",\r\nthen left.\"\r\n3,4,plain \"quote\"\r\n",
			want: []cueRow{{time.Second, 2 * time.Second, []string{`She said "hi",`, "then left."}}, {3 * time.Second, 4 * time.Second, []string{`plain "quote"`}}},
		},
		{
			name: "markup characters are text", content: "start,end,text\n1,2,<i>Tom & Ann</i>\n",
			want: []cueRow{{time.Second, 2 * time.Second, []string{"&lt;i&gt;Tom &amp; Ann&lt;/i&gt;"}}},
		},
		{
			name: "without end a cue ends at the next start", content: "text,start\nb,1\na,1\nc,4\n",
			common: model.CommonReadOptions{LastCueDuration: new(2 * time.Second)},
			want:   []cueRow{{time.Second, 4 * time.Second, []string{"b"}}, {time.Second, 4 * time.Second, []string{"a"}}, {4 * time.Second, 6 * time.Second, []string{"c"}}},
		},
		{
			name: "an empty end cell ends at the next start", content: "start,end,text\n1,,a\n3,5,b\n",
			want: []cueRow{{time.Second, 3 * time.Second, []string{"a"}}, {3 * time.Second, 5 * time.Second, []string{"b"}}},
		},
		{
			name: "duration column", content: "Start,Duration,Text\n1.5,2,a\n",
			want: []cueRow{{1500 * ms, 3500 * ms, []string{"a"}}},
		},
		{
			name: "whole number duration column with long cues reads milliseconds", content: "start,duration,text\n1500,2000,a\n3500,500,b\n4000,2250,c\n",
			want: []cueRow{{1500 * ms, 3500 * ms, []string{"a"}}, {3500 * ms, 4 * time.Second, []string{"b"}}, {4 * time.Second, 6250 * ms, []string{"c"}}},
		},
		{
			name: "whole second times with short cues read seconds", content: "start,end,text\n1,4,a\n5,99,b\n100,104,c\n",
			want: []cueRow{{time.Second, 4 * time.Second, []string{"a"}}, {5 * time.Second, 99 * time.Second, []string{"b"}}, {100 * time.Second, 104 * time.Second, []string{"c"}}},
		},
		{
			name: "long cues read seconds when one time has a fraction", content: "start,end,text\n0,2340,a\n2340,5100.5,b\n",
			want: []cueRow{{0, 2340 * time.Second, []string{"a"}}, {2340 * time.Second, 5100500 * ms, []string{"b"}}},
		},
		{
			name: "columns by index without a header", content: "a,1,2\nb,3,4\n",
			opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(1), End: csv.ColumnAt(2), Text: csv.ColumnAt(0), NoHeader: true}},
			want: []cueRow{{time.Second, 2 * time.Second, []string{"a"}}, {3 * time.Second, 4 * time.Second, []string{"b"}}},
		},
		{
			name: "empty rows and a UTF-8 BOM", content: "\ufeffstart,end,text\n\n,,\n1,2,a",
			want: []cueRow{{time.Second, 2 * time.Second, []string{"a"}}},
		},
		{
			name: "a lenient read skips a bad row", content: "start,end,text\nsoon,2,a\n3,4,b\n", common: lenient,
			want: []cueRow{{3 * time.Second, 4 * time.Second, []string{"b"}}},
		},
		{
			name: "speaker after leading blank lines", content: "start,end,speaker,text\n1,2,Anna,\"\n  Hi\"\n",
			want: []cueRow{{time.Second, 2 * time.Second, []string{"<v Anna>Hi"}}},
		},
		{
			name: "start plus duration stops at the limit", content: "start,duration,text\n99999:59:59,2,a\n",
			want: []cueRow{{359999999 * time.Second, 359999999999 * ms, []string{"a"}}},
		},
		{
			name: "the last open end stops at the limit", content: "start,text\n99999:59:59,a\n",
			want: []cueRow{{359999999 * time.Second, 359999999999 * ms, []string{"a"}}},
		},
		{
			name: "a frame number is not checked against the rate", content: "start,end,text\n00:01:02:12,00:01:02:99,a\n", opts: at25,
			want: []cueRow{{62480 * ms, 65960 * ms, []string{"a"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, tt.common, tt.opts)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

// Ports testDetectsTheDelimiterFromTheHeaderLine of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadDetectsTheDelimiter(t *testing.T) {
	t.Parallel()

	for _, d := range []csv.Delimiter{csv.Comma, csv.Semicolon, csv.Tab} {
		t.Run(string(d), func(t *testing.T) {
			t.Parallel()

			content := strings.Join([]string{"Start", "End", "Text"}, string(d)) + "\n" + strings.Join([]string{"1", "2", `"a, b; c"`}, string(d)) + "\n"
			s, _ := mustRead(t, content, strict, csv.ReadOptions{})
			delimiter, _ := fileData(t, s).String("delimiter")
			assert.Equal(t, string(d), delimiter)
			assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"a, b; c"}}}, cueRows(s))
		})
	}
}

// Ports testReadsTimeFormats of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php, with more shapes.
func TestReadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		time string
		want time.Duration
	}{
		{"62.5", 62500 * ms},
		{"62", 62 * time.Second},
		{"00:01:02.500", 62500 * ms},
		{`"00:01:02,500"`, 62500 * ms},
		{"1:00:00.25", 3600250 * ms},
		{"00:01:02:12", 62480 * ms},
		{"0:06", 6 * time.Second},
		{"00:00:1", time.Second},
		{"01:02.500", 62500 * ms},
		{`"01:02,500"`, 62500 * ms},
		{"00:00:01.500", 1500 * ms},
		{"61:02", 3662 * time.Second},
		{"0:00:5.5", 5500 * ms},
		{"1:2.5", 62500 * ms},
		{" 1.5 ", 1500 * ms},
		{"1.1235", 1124 * ms},
		{"00:00:01.1235", 1124 * ms},
		{"00:00:01.12349999999999999", 1123 * ms},
		{"99999:59:59.999", 359999999999 * ms},
		{"359999999.999", 359999999999 * ms},
	}
	for _, tt := range tests {
		t.Run(tt.time, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, "start,text\n"+tt.time+",a\n", strict, at25)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Start)
		})
	}
}

func TestReadTimeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		opts    csv.ReadOptions
		want    *model.ParseError
	}{
		{
			// Ports testFramesNeedAFrameRate of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "frames need a frame rate", content: "start,end,text\n00:00:01:12,00:00:02:00,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "00:00:01:12" counts frames and needs a frame rate.`, Option: "ReadOptions.CSV.FrameRate"},
		},
		{
			// Ports testABadTimeNamesTheLine of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "a bad time names the line after a quoted line break", content: "start,end,text\n1,2,\"a\nb\"\nsoon,3,c\n",
			want: &model.ParseError{Format: model.CSV, Line: 4, Msg: `The time "soon" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`},
		},
		{
			// Ports testATimeOf100000HoursNamesTheLimit of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "100000 hours", content: "start,end,text\n100000:00:00.000,100000:00:01.000,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "100000:00:00.000" is not below 100000 hours.`},
		},
		{
			name: "a clock time that rounds to 100000 hours", content: "start,end,text\n99999:59:59.9996,1,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "99999:59:59.9996" is not below 100000 hours.`},
		},
		{
			name: "seconds that round to 100000 hours", content: "start,end,text\n359999999.9996,1,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "359999999.9996" is not below 100000 hours.`},
		},
		{
			name: "minutes that reach 100000 hours", content: "start,end,text\n6000000:00,1,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "6000000:00" is not below 100000 hours.`},
		},
		{
			name: "20 hour digits of frames", content: "start,end,text\n99999999999999999999:00:00:00,1,a\n", opts: at25,
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "99999999999999999999:00:00:00" is not below 100000 hours.`},
		},
		{
			name: "20 digits of seconds", content: "start,end,text\n1,99999999999999999999,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "99999999999999999999" is not below 100000 hours.`},
		},
		{
			name: "a negative time", content: "start,end,text\n-1,4,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "-1" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`},
		},
		{
			name: "one-digit minutes", content: "start,end,text\n00:00:1,00:0:2,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "00:0:2" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`},
		},
		{
			name: "an empty start", content: "start,end,text\n,2,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: `The time "" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`},
		},
		{
			name: "an end before the start", content: "start,end,text\n1:2.5,00:00:5,a\n",
			want: &model.ParseError{Format: model.CSV, Line: 2, Msg: "The cue ends at 5 s, before it starts at 62.5 s."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, readError(t, tt.content, strict, tt.opts))
		})
	}
}

// Ports tests/Parsers/StartAfterEndTest.php of subtitle-toolbox-php for CSV: lenient mode drops the row, it never swaps.
func TestReadDropsAnEndBeforeTheStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content string
		line          int
	}{
		{"cut-off row", "start,end,text\n00:00:10.000,00:00:12.000,World\n00:00:20.000,00\n", 3},
		{"commas in times", "start,end,text\n00:00:01,000,00:00:02,000,Hello\n00:00:10.000,00:00:12.000,World\n", 2},
		{"short swap", "start,end,text\n5,4,a\n10,12,World\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, lenient, csv.ReadOptions{})
			assert.Equal(t, []cueRow{{10 * time.Second, 12 * time.Second, []string{"World"}}}, cueRows(s))
			require.Len(t, warnings, 1)
			assert.Equal(t, tt.line, warnings[0].Line)
			assert.Equal(t, model.Skipped, warnings[0].Action)
		})
	}
}

// Ports testLenientModeSkipsABadRow of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadLenientSkipsABadRow(t *testing.T) {
	t.Parallel()

	_, warnings := mustRead(t, "start,end,text\nsoon,2,a\n3,4,b\n", lenient, csv.ReadOptions{})
	assert.Equal(t, []warningRow{{
		`The time "soon" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`, 2, 0, []string{"soon,2,a"}, model.Skipped,
	}}, warningRows(warnings))
}

func TestReadFrameTimes(t *testing.T) {
	t.Parallel()

	// Ports testFramesUseTheFrameRateOfTheReadOptions of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, _ := mustRead(t, string(testfiles.Read(t, "csv/own_frame_times.csv")), strict, at25)
	c, _ := s.Cue(0)
	assert.Equal(t, 1480*ms, c.Start)
	tf, _ := fileData(t, s).String("timeFormat")
	assert.Equal(t, "hh:mm:ss:ff", tf)
	fps, _ := fileData(t, s).Number("frameRate")
	assert.InDelta(t, 25, fps, 0)
}

// Ports testLenientModeEndsAnUnclosedQuoteAtTheLineEnd and testStrictModeThrowsForAnUnclosedQuoteInAFixture of
// tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadUnclosedQuote(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "csv/own_unclosed_quote.csv"))
	s, warnings := mustRead(t, content, lenient, csv.ReadOptions{})
	assert.Equal(t, []cueRow{
		{time.Second, 2500 * ms, []string{"The lamp is lit."}},
		{3 * time.Second, 4500 * ms, []string{"Close the shutters."}},
		{5 * time.Second, 6 * time.Second, []string{"Good night."}},
	}, cueRows(s))
	assert.Equal(t, []warningRow{{
		"A quoted CSV cell has no closing quote. The cell ends at the end of the line.", 2, model.NoBlock,
		[]string{"1.0,2.5,\"The lamp is lit."},
		model.Repaired,
	}}, warningRows(warnings))

	assert.Equal(t, &model.ParseError{Format: model.CSV, Line: 2, Msg: "A quoted CSV cell has no closing quote."}, readError(t, content, strict, csv.ReadOptions{}))
	// Ports testAnOpenQuoteThrows of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	assert.Equal(t, 2, readError(t, "start,end,text\n1,2,\"open\n", strict, csv.ReadOptions{}).Line)
}

// Ports testLenientModeSkipsRowsBeforeTheHeader and testStrictModeThrowsForRowsBeforeTheHeader of
// tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadRowsBeforeTheHeader(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "csv/own_junk_before_header.csv"))
	s, warnings := mustRead(t, content, lenient, csv.ReadOptions{})
	assert.Equal(t, []cueRow{
		{time.Second, 3 * time.Second, []string{"The ferry leaves at noon."}},
		{4 * time.Second, 6500 * ms, []string{"Then we wait, Ana; I bring the map."}},
	}, cueRows(s))
	delimiter, _ := fileData(t, s).String("delimiter")
	assert.Equal(t, ";", delimiter)
	header, _ := fileData(t, s).Strings("header")
	assert.Equal(t, []string{"start", "end", "text"}, header)
	assert.Equal(t, []warningRow{{
		"The table has 2 rows before the header row.", 1, model.NoBlock,
		[]string{"Exported by Subtitle Desk 4.2", "Project;Harbor Lights;Reel 2;"},
		model.Skipped,
	}}, warningRows(warnings))

	assert.Equal(t, &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "start" for start.`}, readError(t, content, strict, csv.ReadOptions{}))

	_, warnings = mustRead(t, "note\nstart,end,text\n1,2,a\n", lenient, csv.ReadOptions{})
	assert.Equal(t, []string{"The table has 1 row before the header row."}, messages(warnings))
}

func TestReadColumnErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		common  model.CommonReadOptions
		opts    csv.ReadOptions
		want    *model.ParseError
	}{
		{
			// Ports testLenientModeKeepsTheMissingColumnErrorWithoutAHeaderRow of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "lenient mode keeps the missing column error", content: "note\nstart,end\n1,2\n", common: lenient,
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "start" for start.`},
		},
		{
			// Ports testColumnOptionsTurnOffHeaderSynonyms of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "a layout turns the synonyms off", content: "Start Time,Stop,Subtitle\n1,2,a\n",
			opts: csv.ReadOptions{Columns: csv.Columns{End: csv.ColumnNamed("Stop")}},
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "start" for start.`},
		},
		{
			// Ports testAMissingColumnThrows of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "a missing mapped column", content: "start,end,text\n1,2,a\n",
			opts: csv.ReadOptions{Columns: csv.Columns{Speaker: csv.ColumnNamed("Character")}},
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "Character" for speaker.`},
		},
		{
			// Ports testATableWithoutTextThrows of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "a table without text", content: "start,end\n1,2\n",
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "text" for text.`},
		},
		{
			name: "an index beyond the header", content: "start,end,text\n1,2,a\n",
			opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(7)}},
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "7" for start.`},
		},
		{
			name: "an index beyond the column limit", content: "a,1,2\n", common: lenient,
			opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(5000), Text: csv.ColumnAt(0), NoHeader: true}},
			want: &model.ParseError{Format: model.CSV, Line: 1, Msg: `The table has no column "5000" for start.`},
		},
		{
			name: "the header line after blank rows", content: "\n\nstart,end\n1,2\n",
			want: &model.ParseError{Format: model.CSV, Line: 3, Msg: `The table has no column "text" for text.`},
		},
		{
			// Ports testATableWithMoreThan1000ColumnsThrowsAParsingException of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name:    "1001 columns",
			content: "start,end,text," + strings.Repeat("c,", 996) + "c997\n1,2,a\n3,4,b," + strings.Repeat("c,", 996) + "c997,extra\n",
			common:  lenient,
			want:    &model.ParseError{Format: model.CSV, Line: 3, Msg: "The table has 1001 columns. The limit is 1000."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, readError(t, tt.content, tt.common, tt.opts))
		})
	}
}

// Ports testATableWith1000ColumnsParses of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestRead1000Columns(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "start,end,text"+strings.Repeat(",", 997)+"\n1,2,a\n", strict, csv.ReadOptions{})
	width, _ := fileData(t, s).Int("width")
	assert.Equal(t, int64(1000), width)
}

// Ports testReadsWholeNumberTimesWithLongCuesAsMilliseconds of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadMillisecondTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content string
		want          []cueRow
		warning       string
	}{
		{
			name: "even count", content: "start\tend\ttext\n0\t2340\tFirst line\n2340\t5100\tSecond line\n",
			want:    []cueRow{{0, 2340 * ms, []string{"First line"}}, {2340 * ms, 5100 * ms, []string{"Second line"}}},
			warning: "The times are whole numbers, and the median cue lasts 2550 seconds. The parser reads the times as milliseconds.",
		},
		{
			name: "half median", content: "start,end,text\n0,2340,a\n2340,5101,b\n",
			want:    []cueRow{{0, 2340 * ms, []string{"a"}}, {2340 * ms, 5101 * ms, []string{"b"}}},
			warning: "The times are whole numbers, and the median cue lasts 2550.5 seconds. The parser reads the times as milliseconds.",
		},
		{
			name: "odd count", content: "start,end,text\n0,100,a\n100,300,b\n300,350,c\n",
			want:    []cueRow{{0, 100 * ms, []string{"a"}}, {100 * ms, 300 * ms, []string{"b"}}, {300 * ms, 350 * ms, []string{"c"}}},
			warning: "The times are whole numbers, and the median cue lasts 100 seconds. The parser reads the times as milliseconds.",
		},
		{
			name: "an open end", content: "start,end,text\n0,2000,a\n1000,,b\n",
			want:    []cueRow{{0, 2 * time.Second, []string{"a"}}, {time.Second, 6 * time.Second, []string{"b"}}},
			warning: "The times are whole numbers, and the median cue lasts 2000 seconds. The parser reads the times as milliseconds.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, strict, csv.ReadOptions{})
			assert.Equal(t, tt.want, cueRows(s))
			require.Len(t, warnings, 1)
			assert.Equal(t, warningRow{tt.warning, 2, model.NoBlock, []string{strings.SplitN(tt.content, "\n", 3)[1]}, model.Repaired}, warningRows(warnings)[0])
			tf, _ := fileData(t, s).String("timeFormat")
			assert.Equal(t, "milliseconds", tf)
		})
	}
}

func TestReadTimeFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		content string
		want    string
	}{
		// Ports testTimeFormatOfTheNewShapesIsDot of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
		{"start,text\n0:06,a\n", "hh:mm:ss.mmm"},
		{"start,text\n1.5,a\n", "seconds"},
		{"start,text\n\"00:00:01,500\",a\n", "hh:mm:ss,mmm"},
		{"start,text\n00:00:01:12,a\n", "hh:mm:ss:ff"},
		{"start,text\nsoon,a\n1.5,b\n", "seconds"},
		{"start,text\n", "hh:mm:ss.mmm"},
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, lenient, at25)
			tf, _ := fileData(t, s).String("timeFormat")
			assert.Equal(t, tt.want, tf)
		})
	}
}

func TestReadQuotedColumns(t *testing.T) {
	t.Parallel()

	// Ports testStoresTheColumnsThatEveryRowQuotes of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, _ := mustRead(t, "\"start\",end,text,notes\n1,2,\"a\",\"x\"\n\"3\",4,\"b\",y\n", strict, csv.ReadOptions{})
	quoted, _ := fileData(t, s).List("quotedColumns")
	assert.Equal(t, []any{int64(2)}, quoted)

	// Ports testStoresNoQuotedColumnsWhenNoColumnIsQuotedInEveryRow of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, _ = mustRead(t, "start,end,text\n1,2,\"a, b\"\n3,4,c\n", strict, csv.ReadOptions{})
	_, ok := fileData(t, s).Get("quotedColumns")
	assert.False(t, ok)
}

func TestReadSynonyms(t *testing.T) {
	t.Parallel()

	// Ports testFindsColumnsByHeaderSynonyms of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, warnings := mustRead(t, "Start Time,End Time,Subtitle\n1,2.5,a\n", strict, csv.ReadOptions{})
	assert.Equal(t, []cueRow{{time.Second, 2500 * ms, []string{"a"}}}, cueRows(s))
	assert.Equal(t, []warningRow{{
		`The parser reads the columns "Start Time" as start, "End Time" as end and "Subtitle" as text.`, 1, model.NoBlock,
		[]string{"Start Time,End Time,Subtitle"},
		model.Repaired,
	}}, warningRows(warnings))

	// Ports testFindsTheDubbingScriptColumnsWithoutColumnOptions of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, warnings = mustRead(t, string(testfiles.Read(t, "csv/real/dubbing_script.csv")), strict, at25)
	c, _ := s.Cue(0)
	assert.Equal(t, 36001480*ms, c.Start)
	assert.Equal(t, []string{"<v NARRATOR>The weather turns cold tonight."}, c.Lines)
	roles, _ := fileData(t, s).Object("roles")
	assert.Equal(t, []string{"start", "speaker", "text"}, roles.Keys())
	assert.Equal(t, []string{`The parser reads the columns "Start TC" as start and "Character" as speaker.`}, messages(warnings))

	// Ports testAnExactHeaderNameWinsOverASynonym of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
	s, warnings = mustRead(t, "Begin,Start,Text\n9,1,a\n", strict, csv.ReadOptions{})
	c, _ = s.Cue(0)
	assert.Equal(t, time.Second, c.Start)
	assert.Empty(t, warnings)

	_, warnings = mustRead(t, "Begin,End,Text,In\n1,2,a,3\n", strict, csv.ReadOptions{})
	assert.Equal(t, []string{`The parser reads the column "Begin" as start.`}, messages(warnings))

	_, warnings = mustRead(t, "tc-in,TC_OUT,dialogue\n1,2,a\n", strict, csv.ReadOptions{})
	assert.Equal(t, []string{`The parser reads the columns "tc-in" as start, "TC_OUT" as end and "dialogue" as text.`}, messages(warnings))
}

// Ports the role name tests of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadRoleNameColumns(t *testing.T) {
	t.Parallel()

	// testAStartAndTextTableKeepsAnOutColumnAsData
	s, warnings := mustRead(t, string(testfiles.Read(t, "csv/own_role_names_out_column.csv")), strict, csv.ReadOptions{})
	assert.Equal(t, []cueRow{{time.Second, 3 * time.Second, []string{"Hello"}}, {3 * time.Second, 8 * time.Second, []string{"World"}}}, cueRows(s))
	var columns []any
	for _, c := range s.All() {
		d, _ := c.FormatData("csv")
		v, _ := d.Object("columns")
		columns = append(columns, v.Keys(), must(v.String("out")))
	}
	assert.Equal(t, []any{[]string{"out"}, "yes", []string{"out"}, "no"}, columns)
	assert.Empty(t, warnings)

	// testAStartAndTextTableReadsNoSpeakerFromANameColumn
	s, warnings = mustRead(t, string(testfiles.Read(t, "csv/own_role_names_name_column.csv")), strict, csv.ReadOptions{})
	c, _ := s.Cue(0)
	assert.Equal(t, []string{"Hello"}, c.Lines)
	roles, _ := fileData(t, s).Object("roles")
	assert.Equal(t, []string{"start", "end", "text"}, roles.Keys())
	assert.Empty(t, warnings)
}

func must[T any](v T, _ bool) T { return v }

// Ports testKeepsOtherColumnsAndIdentifiers and testIdentifierHeaderMapsToTheCueIdentifier of
// tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
func TestReadOtherColumnsAndIdentifiers(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "ID,Start,End,Text,Take\nshot-1,1,2,a,3\n\n", strict, csv.ReadOptions{Columns: csv.Columns{ID: csv.ColumnNamed("id")}})
	c, _ := s.Cue(0)
	assert.Equal(t, "shot-1", c.ID)
	cueData, _ := c.FormatData("csv")
	assert.Equal(t, model.NewData(model.Field{Key: "columns", Value: model.NewData(model.Field{Key: "Take", Value: "3"})}), cueData)
	assert.Equal(t, model.NewData(
		model.Field{Key: "delimiter", Value: ","},
		model.Field{Key: "header", Value: []any{"ID", "Start", "End", "Text", "Take"}},
		model.Field{Key: "roles", Value: model.NewData(
			model.Field{Key: "identifier", Value: int64(0)}, model.Field{Key: "start", Value: int64(1)},
			model.Field{Key: "end", Value: int64(2)}, model.Field{Key: "text", Value: int64(3)},
		)},
		model.Field{Key: "width", Value: int64(5)},
		model.Field{Key: "timeFormat", Value: "seconds"},
		model.Field{Key: "frameRate", Value: nil},
	), fileData(t, s))

	s, _ = mustRead(t, "identifier,start,text\nintro,1,a\n", strict, csv.ReadOptions{})
	c, _ = s.Cue(0)
	assert.Equal(t, "intro", c.ID)
}

func TestReadOtherColumnsShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		opts    csv.ReadOptions
		want    any
	}{
		{"names that are list indexes", "start,end,text,0,1\n1,2,a,x,y\n", csv.ReadOptions{}, []any{"x", "y"}},
		{"a repeated name keeps the last cell", "start,end,text,n,n\n1,2,a,x,y\n", csv.ReadOptions{}, model.NewData(model.Field{Key: "n", Value: "y"})},
		{"a repeated role name", "start,start,text\n1,2,a\n", csv.ReadOptions{}, model.NewData(model.Field{Key: "start", Value: "2"})},
		{"a short row", "start,end,text,Notes\n1,2,a\n", csv.ReadOptions{}, model.NewData(model.Field{Key: "Notes", Value: ""})},
		{"a long row", "start,end,text\n1,2,a,x\n", csv.ReadOptions{}, model.NewData(model.Field{Key: "3", Value: "x"})},
		{
			"indexes without a header", "1,2,a,x\n",
			csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(0), End: csv.ColumnAt(1), Text: csv.ColumnAt(2), NoHeader: true}},
			model.NewData(model.Field{Key: "3", Value: "x"}),
		},
		{
			"index 0 without a header is a list", "x,1,2,a\n",
			csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(1), End: csv.ColumnAt(2), Text: csv.ColumnAt(3), NoHeader: true}},
			[]any{"x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, strict, tt.opts)
			c, _ := s.Cue(0)
			d, _ := c.FormatData("csv")
			got, _ := d.Get("columns")
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ports the CSV rows of tests/Parsers/NoCuesTest.php of subtitle-toolbox-php.
func TestReadNoCues(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "start,end,text\n", strict, csv.ReadOptions{})
	assert.Zero(t, s.Len())
	assert.Empty(t, warnings)

	assert.Equal(t, 2, readError(t, "start,end,text\nsoon,later,The ferry leaves at nine.\n", strict, csv.ReadOptions{}).Line)
}

func TestReadInvalidUTF8StaysAsItIs(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "start,end,text\n1,2,caf\xe9\n", lenient, csv.ReadOptions{})
	c, _ := s.Cue(0)
	assert.Equal(t, []string{"caf\xe9"}, c.Lines)
	assert.Len(t, warnings, 1)
}

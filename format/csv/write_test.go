package csv_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const bom = "\ufeff"

func english() *model.Subtitle {
	return subtitle(
		model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<v Anna>Where are you going?"}},
		model.Cue{Start: 4500 * ms, End: 6 * time.Second, Lines: []string{"<v Ben><i>Home.</i>", "Now."}},
	)
}

func setCue(t *testing.T, s *model.Subtitle, i int, change func(c *model.Cue)) {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok)
	change(&c)
	require.NoError(t, s.SetCue(i, c))
}

// Ports tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      *model.Subtitle
		common model.CommonWriteOptions
		opts   csv.WriteOptions
		want   string
	}{
		{
			name: "speakers and a quoted line break with a BOM", s: english(),
			want: bom + "start,end,speaker,text\n00:00:01.000,00:00:04.000,Anna,Where are you going?\n" +
				"00:00:04.500,00:00:06.000,Ben,\"Home.\nNow.\"\n",
		},
		{
			name: "quotes cells and decodes entities", common: noBOM,
			s:    subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{`Tom &amp; "Ann", <b>both</b>`}}),
			want: "start,end,text\n00:00:01.000,00:00:02.000,\"Tom & \"\"Ann\"\", both\"\n",
		},
		{
			name: "the delimiter and the line ending", s: english(),
			common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, opts: csv.WriteOptions{Delimiter: csv.Semicolon},
			want: "start;end;speaker;text\r\n00:00:01.000;00:00:04.000;Anna;Where are you going?\r\n" +
				"00:00:04.500;00:00:06.000;Ben;\"Home.\nNow.\"\r\n",
		},
		{
			name: "a second cue over 2 rows fills the first row it overlaps most", common: noBOM,
			s: subtitle(model.Cue{End: 2 * time.Second, Lines: []string{"a"}}, model.Cue{Start: 2 * time.Second, End: 6 * time.Second, Lines: []string{"b"}}),
			opts: csv.WriteOptions{
				SecondText: subtitle(model.Cue{Start: time.Second, End: 6 * time.Second, Lines: []string{"x"}}), TimeFormat: csv.TimeSeconds,
			},
			want: "start,end,text,text2\n0,2,a,x\n2,6,b,\n",
		},
		{
			name: "no overlap gives an empty second cell", common: noBOM,
			s: subtitle(model.Cue{End: time.Second, Lines: []string{"a"}}),
			opts: csv.WriteOptions{
				SecondText: subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"x"}}), TimeFormat: csv.TimeSeconds,
			},
			want: "start,end,text,text2\n0,1,a,\n",
		},
		{
			name: "negative times write as 0", common: noBOM, opts: csv.WriteOptions{TimeFormat: csv.TimeFrames, FrameRate: 25},
			s:    subtitle(model.Cue{Start: -2 * time.Second, End: -time.Second, Lines: []string{"a"}}),
			want: "start,end,text\n00:00:00:00,00:00:00:00,a\n",
		},
		{
			name: "ruby as text and a speaker after it", common: noBOM,
			s:    subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<v Ann><ruby>Kan<rt>kan</rt></ruby>ji", "<v Bob>Yes"}}),
			want: "start,end,speaker,text\n00:00:01.000,00:00:02.000,Ann,\"Kan (kan)ji\nYes\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, tt.common, tt.opts))
		})
	}
}

// Ports testWritesTimeFormats of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteTimeFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format     csv.TimeFormat
		start, end string
	}{
		{csv.TimeSeconds, "62.48", "3599.99"},
		{csv.TimeDot, "00:01:02.480", "00:59:59.990"},
		{csv.TimeComma, `"00:01:02,480"`, `"00:59:59,990"`},
		{csv.TimeFrames, "00:01:02:12", "01:00:00:00"},
		{csv.TimeMilliseconds, "62480", "3599990"},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			t.Parallel()

			s := subtitle(model.Cue{Start: 62480 * ms, End: 3599990 * ms, Lines: []string{"a"}})
			assert.Equal(t, "start,end,text\n"+tt.start+","+tt.end+",a\n", write(t, s, noBOM, csv.WriteOptions{TimeFormat: tt.format, FrameRate: 25}))
		})
	}
}

func TestWriteTimeCells(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		d      time.Duration
		format csv.TimeFormat
		fps    float64
		want   string
	}{
		{"whole seconds", 4 * time.Second, csv.TimeSeconds, 0, "4"},
		{"zero seconds", 0, csv.TimeSeconds, 0, "0"},
		{"ten seconds", 10 * time.Second, csv.TimeSeconds, 0, "10"},
		{"half a second", 62500 * ms, csv.TimeSeconds, 0, "62.5"},
		{"frames at 25 fps", 62500 * ms, csv.TimeFrames, 25, "00:01:02:13"},
		{"a frame that rounds to the next second", 59990 * ms, csv.TimeFrames, 25, "00:01:00:00"},
		{"23.976 fps", 1980 * ms, csv.TimeFrames, 23.976, "00:00:01:23"},
		{"hours of 3 digits", 360000 * time.Second, csv.TimeComma, 0, `"100:00:00,000"`},
		{"99999:59:59.999 clamps", timecode.MaxValidTime, csv.TimeFrames, 25, "99999:59:59:24"},
		{"99999:59:59.995 clamps", timecode.MaxTime - 5*ms, csv.TimeFrames, 25, "99999:59:59:24"},
		{"99999:59:59.900 stays", timecode.MaxTime - 100*ms, csv.TimeFrames, 25, "99999:59:59:23"},
		{"99999:59:59.999 at 23.976 fps clamps", timecode.MaxValidTime, csv.TimeFrames, 23.976, "99999:59:59:23"},
		{"99999:59:59.999 at 1000000 fps", timecode.MaxValidTime, csv.TimeFrames, 1e6, "99999:59:59:999000"},
		{"a time beyond the limit clamps", timecode.MaxTime + time.Hour, csv.TimeFrames, 25, "99999:59:59:24"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(model.Cue{Start: tt.d, End: tt.d, Lines: []string{"a"}})
			want := "start,end,text\n" + tt.want + "," + tt.want + ",a\n"
			assert.Equal(t, want, write(t, s, noBOM, csv.WriteOptions{TimeFormat: tt.format, FrameRate: tt.fps}))
		})
	}
}

// Ports testSetFormatDataRejectsAnUnknownStoredValue of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteRejectedStoredValueKeepsTheTable(t *testing.T) {
	t.Parallel()

	for _, field := range []struct{ key, value, msg string }{
		{"timeFormat", "hh:mm:ss;fff", `The field formatData.csv.timeFormat must be one of "seconds", "hh:mm:ss.mmm", "hh:mm:ss,mmm", "hh:mm:ss:ff", "milliseconds".`},
		{"delimiter", "|", `The field formatData.csv.delimiter must be one of ",", ";", "\t".`},
	} {
		t.Run(field.key, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, "start,end,text\n1.5,2,a\n", strict, csv.ReadOptions{})
			d := fileData(t, s)
			d.Set(field.key, field.value)
			err := s.SetFormatData("csv", d)
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			assert.Equal(t, field.msg, aerr.Msg)
			assert.Equal(t, "start,end,text\n1.5,2,a\n", write(t, s, noBOM, csv.WriteOptions{}))
		})
	}
}

// Ports testFrameRateOptionWinsOverTheParsedFrameRate of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteFrameRateOptionWins(t *testing.T) {
	t.Parallel()

	opts := csv.ReadOptions{
		Columns:   csv.Columns{Start: csv.ColumnNamed("Start TC"), Text: csv.ColumnNamed("Text"), Speaker: csv.ColumnNamed("Character")},
		FrameRate: 25,
	}
	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/dubbing_script.csv")), strict, opts)
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}, csv.WriteOptions{FrameRate: 24}), "\n10:00:05:19,BEN,- I have one.,\n")
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}, csv.WriteOptions{}), "\n10:00:05:20,BEN,- I have one.,\n")
}

// Ports testExplicitCsvFrameRateAndTimeFormatWinOverTheParsedTable of tests/Formatters/OutputOptionsTest.php of
// subtitle-toolbox-php.
func TestWriteTimeFormatOptionWins(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/excel_de_semicolon.csv")), strict, csv.ReadOptions{})
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}, csv.WriteOptions{TimeFormat: csv.TimeFrames, FrameRate: 25}), "\n00:00:02:00;")
}

// Ports testWritesIdentifiersAndOtherColumnsFromAnotherFormat of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteIdentifiersAndOtherColumnsFromAnotherFormat(t *testing.T) {
	t.Parallel()

	s, _, err := webvtt.Read(strings.NewReader("WEBVTT\n\nintro\n00:00:01.000 --> 00:00:02.000\nHi\n"), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	setCue(t, s, 0, func(c *model.Cue) {
		require.NoError(t, c.SetFormatData("csv", model.NewData(model.Field{Key: "columns", Value: model.NewData(model.Field{Key: "Take", Value: "2"})})))
	})
	assert.Equal(t, "identifier,start,end,text,Take\nintro,00:00:01.000,00:00:02.000,Hi,2\n", write(t, s, noBOM, csv.WriteOptions{}))
}

func TestWriteOtherColumnsOfOtherInput(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{End: time.Second, Lines: []string{"a"}}, model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"b"}})
	setCue(t, s, 0, func(c *model.Cue) {
		require.NoError(t, c.SetFormatData("csv", model.NewData(model.Field{Key: "columns", Value: []any{"x", int64(7)}})))
	})
	setCue(t, s, 1, func(c *model.Cue) {
		require.NoError(t, c.SetFormatData("csv", model.NewData(model.Field{Key: "columns", Value: model.NewData(
			model.Field{Key: "1", Value: true}, model.Field{Key: "n", Value: 1.5}, model.Field{Key: "0", Value: nil},
		)})))
	})
	assert.Equal(t, "start,end,text,0,1,n\n0,1,a,x,7,\n1,2,b,,1,1.5\n", write(t, s, noBOM, csv.WriteOptions{TimeFormat: csv.TimeSeconds}))
}

// Ports testAddsASpeakerColumnToAParsedTableWithoutOne of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteAddsASpeakerColumn(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "Start;Text\n1;a\n", strict, csv.ReadOptions{})
	setCue(t, s, 0, func(c *model.Cue) { c.Lines = []string{"<v Lena>a"} })
	assert.Equal(t, "Start;speaker;Text\n1;Lena;a\n", write(t, s, noBOM, csv.WriteOptions{}))
}

// Ports testWritesATableWithoutAHeaderRowBack and testKeepsAHeaderColumnThatNoRowFills of
// tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteKeepsTheTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, want string
		opts                csv.ReadOptions
	}{
		{
			name: "no header row", content: "a\t1\t2\textra\nb\t3\t4\t\n", want: "a\t1\t2\textra\nb\t3\t4\t\n",
			opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(1), End: csv.ColumnAt(2), Text: csv.ColumnAt(0), NoHeader: true}},
		},
		{name: "a header column that no row fills", content: "start,end,text,Notes\n1,2,a\n", want: "start,end,text,Notes\n1,2,a,\n"},
		{name: "a length column", content: string(testfiles.Read(t, "csv/own_role_names_length_column.csv")), want: string(testfiles.Read(t, "csv/own_role_names_length_column.csv"))},
		{name: "a duration column", content: "start,duration,text\n1.5,2,a\n", want: "start,duration,text\n1.5,2,a\n"},
		{name: "repeated names", content: "start,end,text,n,n\n1,2,a,x,y\n", want: "start,end,text,n,n\n1,2,a,y,y\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, strict, tt.opts)
			assert.Empty(t, warnings)
			assert.Equal(t, tt.want, write(t, s, noBOM, csv.WriteOptions{}))
		})
	}
}

// Ports testWritesABilingualTable of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteBilingualTable(t *testing.T) {
	t.Parallel()

	german, _, err := srt.Read(strings.NewReader("1\n00:00:01,200 --> 00:00:03,900\nWohin gehst du?\n\n2\n00:00:04,400 --> 00:00:09,000\nNach Hause. Jetzt.\n"),
		model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	en := english()
	en.AddCue(model.Cue{Start: 6500 * ms, End: 8 * time.Second, Lines: []string{"Right now."}})
	assert.Equal(t, "start,end,speaker,text,text (de)\n"+
		"00:00:01.000,00:00:04.000,Anna,Where are you going?,Wohin gehst du?\n"+
		"00:00:04.500,00:00:06.000,Ben,\"Home.\nNow.\",Nach Hause. Jetzt.\n"+
		"00:00:06.500,00:00:08.000,,Right now.,\n",
		write(t, en, noBOM, csv.WriteOptions{SecondText: german, SecondTextHeader: "text (de)"}))
}

// Ports testEscapesFormulasOnlyWhenAsked of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteEscapesFormulasOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	s := subtitle(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"- Hi.", "- Hello."}},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"=SUM(A1)"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"+1 @home"}},
	)
	assert.Equal(t, "start,end,text\n1,2,\"- Hi.\n- Hello.\"\n3,4,=SUM(A1)\n5,6,+1 @home\n",
		write(t, s, noBOM, csv.WriteOptions{TimeFormat: csv.TimeSeconds}))
	assert.Equal(t, "start,end,text\n1,2,\"'- Hi.\n- Hello.\"\n3,4,'=SUM(A1)\n5,6,'+1 @home\n",
		write(t, s, noBOM, csv.WriteOptions{TimeFormat: csv.TimeSeconds, EscapeFormulas: true}))
}

// Ports testWritesFileDataThatStoresTheRolesByTheirNames of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteStoredRoles(t *testing.T) {
	t.Parallel()

	first := model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<v Anna>Where are you going?"}, ID: "a1"}
	require.NoError(t, first.SetFormatData("csv", model.NewData(model.Field{Key: "columns", Value: model.NewData(model.Field{Key: "Notes", Value: "calm"})})))
	second := model.Cue{Start: 4500 * ms, End: 6 * time.Second, Lines: []string{"Home.", "Now."}}
	require.NoError(t, second.SetFormatData("csv", model.NewData(model.Field{Key: "columns", Value: model.NewData(model.Field{Key: "Notes", Value: "=1+1"})})))
	s := subtitle(first, second)
	require.NoError(t, s.SetFormatData("csv", model.NewData(
		model.Field{Key: "delimiter", Value: ";"},
		model.Field{Key: "header", Value: []any{"ID", "In", "Length", "Who", "Line", "Notes"}},
		model.Field{Key: "roles", Value: model.NewData(
			model.Field{Key: "identifier", Value: int64(0)}, model.Field{Key: "start", Value: int64(1)},
			model.Field{Key: "duration", Value: int64(2)}, model.Field{Key: "speaker", Value: int64(3)}, model.Field{Key: "text", Value: int64(4)},
		)},
		model.Field{Key: "width", Value: int64(6)},
		model.Field{Key: "timeFormat", Value: "seconds"},
		model.Field{Key: "frameRate", Value: nil},
	)))
	assert.Equal(t, "ID;In;Length;Who;Line;Second;Notes\n"+
		"a1;1;3;Anna;Where are you going?;Wohin?;calm\n"+
		";4.5;1.5;;\"Home.\nNow.\";;'=1+1\n",
		write(t, s, noBOM, csv.WriteOptions{
			SecondText:       subtitle(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"Wohin?"}}),
			SecondTextHeader: "Second",
			EscapeFormulas:   true,
		}))
}

// Ports testQuotesTheStoredQuotedColumnsInEveryRow of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteQuotesStoredQuotedColumns(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "start,end,text,notes\n1,2,\"Hello\",x\n", strict, csv.ReadOptions{})
	setCue(t, s, 0, func(c *model.Cue) { c.Lines = []string{"<v Ann>Bye"} })
	second := subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Tsch\u00fcss"}})
	assert.Equal(t, "start,end,speaker,text,Second,notes\n1,2,Ann,\"Bye\",Tsch\u00fcss,x\n",
		write(t, s, noBOM, csv.WriteOptions{SecondText: second, SecondTextHeader: "Second"}))
}

// Ports testIgnoresAStoredQuotedColumnOutsideTheTable of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteIgnoresAStoredQuotedColumnOutsideTheTable(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "start,end,text\n1,2,a\n", strict, csv.ReadOptions{})
	d := fileData(t, s)
	d.Set("quotedColumns", []any{int64(7)})
	require.NoError(t, s.SetFormatData("csv", d))
	assert.Equal(t, "start,end,text\n1,2,a\n", write(t, s, noBOM, csv.WriteOptions{}))
}

// Ports testRoundTripsThroughTheParser of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
func TestWriteRoundTripsThroughTheReader(t *testing.T) {
	t.Parallel()

	out := write(t, english(), model.CommonWriteOptions{}, csv.WriteOptions{})
	s, _ := mustRead(t, out, strict, csv.ReadOptions{})
	assert.Equal(t, out, write(t, s, model.CommonWriteOptions{}, csv.WriteOptions{}))
}

func TestWriteFrameRateErrors(t *testing.T) {
	t.Parallel()

	// Ports testFramesWithoutAFrameRateThrow of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
	err := csv.Write(&bytes.Buffer{}, english(), model.CommonWriteOptions{}, csv.WriteOptions{TimeFormat: csv.TimeFrames})
	assert.Equal(t, &model.InvalidArgumentError{Option: "WriteOptions.CSV.FrameRate", Msg: "The time format hh:mm:ss:ff needs a frame rate."}, err)

	s, _ := mustRead(t, "start,end,text\n1,2,a\n", strict, csv.ReadOptions{})
	d := fileData(t, s)
	d.Set("frameRate", int64(0))
	require.NoError(t, s.SetFormatData("csv", d))
	err = csv.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, csv.WriteOptions{})
	assert.Equal(t, &model.InvalidArgumentError{Option: "WriteOptions.CSV.FrameRate", Msg: "The frame rate must be greater than 0, got 0."}, err)
	require.NoError(t, csv.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, csv.WriteOptions{FrameRate: 25}))
}

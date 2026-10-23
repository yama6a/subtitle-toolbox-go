package csv_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestReadOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opts   csv.ReadOptions
		option string
		msg    string
	}{
		{name: "zero", opts: csv.ReadOptions{}},
		{name: "every delimiter", opts: csv.ReadOptions{Delimiter: csv.Tab, FrameRate: 23.976}},
		{
			name: "indexes without a header",
			opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(0), Text: csv.ColumnAt(1), Speaker: csv.ColumnAt(2), NoHeader: true}},
		},
		{
			name: "negative index", opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(-1)}},
			option: "ReadOptions.CSV.Columns.Start", msg: "The column index of start must be 0 or more, got -1.",
		},
		{
			name: "negative ID index", opts: csv.ReadOptions{Columns: csv.Columns{ID: csv.ColumnAt(-2), Start: csv.ColumnAt(-1)}},
			option: "ReadOptions.CSV.Columns.ID", msg: "The column index of identifier must be 0 or more, got -2.",
		},
		{
			// Ports testColumnsWithoutAHeaderNeedIndexes of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "no header without a text index", opts: csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(0), NoHeader: true}},
			option: "ReadOptions.CSV.Columns.Text", msg: "A table without a header row needs a column index for text.",
		},
		{
			name:   "no header with a name",
			opts:   csv.ReadOptions{Columns: csv.Columns{Start: csv.ColumnAt(0), Text: csv.ColumnAt(1), Duration: csv.ColumnNamed("Length"), NoHeader: true}},
			option: "ReadOptions.CSV.Columns.Duration", msg: "A table without a header row needs a column index for duration.",
		},
		{
			// Ports testRejectsAnUnknownDelimiter of tests/Parsers/CsvParserTest.php of subtitle-toolbox-php.
			name: "pipe delimiter", opts: csv.ReadOptions{Delimiter: "|"},
			option: "ReadOptions.CSV.Delimiter", msg: `The CSV delimiter must be ",", ";" or a tab.`,
		},
		{name: "negative frame rate", opts: csv.ReadOptions{FrameRate: -1}, option: "ReadOptions.CSV.FrameRate", msg: "The frame rate must be greater than 0, got -1."},
		{name: "NaN frame rate", opts: csv.ReadOptions{FrameRate: math.NaN()}, option: "ReadOptions.CSV.FrameRate", msg: "The frame rate must be greater than 0, got NAN."},
		{name: "infinite frame rate", opts: csv.ReadOptions{FrameRate: math.Inf(1)}, option: "ReadOptions.CSV.FrameRate", msg: "The frame rate must be greater than 0, got INF."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.msg == "" {
				require.NoError(t, err)
				return
			}
			assert.Equal(t, &model.InvalidArgumentError{Option: tt.option, Msg: tt.msg}, err)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			_, _, err = csv.Read(strings.NewReader("start,text\n1,a\n"), model.CommonReadOptions{}, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
}

func TestWriteOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opts   csv.WriteOptions
		option string
		msg    string
	}{
		{name: "zero", opts: csv.WriteOptions{}},
		{name: "every field", opts: csv.WriteOptions{Delimiter: csv.Semicolon, TimeFormat: csv.TimeFrames, FrameRate: 25, SecondTextHeader: "de", EscapeFormulas: true}},
		{
			// Ports testRejectsAnInvalidDelimiterWhenTheOptionsAreCreated of tests/Formatters/CsvFormatterTest.php of subtitle-toolbox-php.
			name: "pipe delimiter", opts: csv.WriteOptions{Delimiter: "|"},
			option: "WriteOptions.CSV.Delimiter", msg: `The CSV delimiter must be ",", ";" or a tab.`,
		},
		{
			name: "unknown time format", opts: csv.WriteOptions{TimeFormat: "hh:mm:ss;fff"}, option: "WriteOptions.CSV.TimeFormat",
			msg: `The CSV time format must be seconds, hh:mm:ss.mmm, hh:mm:ss,mmm, hh:mm:ss:ff or milliseconds, got "hh:mm:ss;fff".`,
		},
		{name: "negative frame rate", opts: csv.WriteOptions{FrameRate: -1}, option: "WriteOptions.CSV.FrameRate", msg: "The CSV frame rate must be greater than 0, got -1."},
		{name: "NaN frame rate", opts: csv.WriteOptions{FrameRate: math.NaN()}, option: "WriteOptions.CSV.FrameRate", msg: "The CSV frame rate must be greater than 0, got NAN."},
		{name: "infinite frame rate", opts: csv.WriteOptions{FrameRate: math.Inf(1)}, option: "WriteOptions.CSV.FrameRate", msg: "The CSV frame rate must be greater than 0, got INF."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.msg == "" {
				require.NoError(t, err)
				return
			}
			assert.Equal(t, &model.InvalidArgumentError{Option: tt.option, Msg: tt.msg}, err)
			require.ErrorIs(t, csv.Write(&strings.Builder{}, &model.Subtitle{}, model.CommonWriteOptions{}, tt.opts), model.ErrInvalidArgument)
		})
	}
}

func TestColumn(t *testing.T) {
	t.Parallel()

	name, isName := csv.ColumnNamed("Start TC").Name()
	assert.Equal(t, "Start TC", name)
	assert.True(t, isName)
	_, isIndex := csv.ColumnNamed("Start TC").Index()
	assert.False(t, isIndex)

	index, isIndex := csv.ColumnAt(0).Index()
	assert.Equal(t, 0, index)
	assert.True(t, isIndex)
	_, isName = csv.ColumnAt(0).Name()
	assert.False(t, isName)

	assert.True(t, csv.Column{}.IsZero())
	assert.False(t, csv.ColumnAt(0).IsZero())
	assert.False(t, csv.ColumnNamed("").IsZero())
}

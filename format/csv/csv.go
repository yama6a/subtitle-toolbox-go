// Package csv reads and writes subtitles as CSV and TSV tables with RFC 4180 quoting.
//
// The reader finds the columns by their header names, or by a column layout that the caller passes. It detects
// the delimiter, so one reader serves CSV and TSV. A table read and written unchanged comes out byte for byte,
// because the reader stores its columns, header, delimiter, time format and quoting in the csv format data.
package csv

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.CSV)

// maxColumns is the most cells that a row of a table may have.
const maxColumns = 1000

// Delimiter is the cell separator of a table.
type Delimiter string

// The delimiters.
const (
	DelimiterDefault Delimiter = ""   // read: detect it. Write: the stored delimiter, else Comma
	Comma            Delimiter = ","  // RFC 4180
	Semicolon        Delimiter = ";"  // Excel in locales with a decimal comma
	Tab              Delimiter = "\t" // TSV
)

// TimeFormat is the format of the time cells that Write writes. The values are the stored format data values.
type TimeFormat string

// The time formats.
const (
	TimeFormatDefault TimeFormat = ""             // the stored format, else TimeDot
	TimeSeconds       TimeFormat = "seconds"      // 62.5
	TimeDot           TimeFormat = "hh:mm:ss.mmm" // 00:01:02.500
	TimeComma         TimeFormat = "hh:mm:ss,mmm" // 00:01:02,500
	TimeFrames        TimeFormat = "hh:mm:ss:ff"  // 00:01:02:13 at 25 fps
	TimeMilliseconds  TimeFormat = "milliseconds" // 62500
)

// Column maps a role to a table column. The zero value means the header named like the role.
type Column struct {
	name    string
	index   int
	isName  bool
	isIndex bool
}

// ColumnNamed returns the column of the header with this name, compared without case and after a trim.
func ColumnNamed(name string) Column {
	return Column{name: name, isName: true}
}

// ColumnAt returns the column at the 0-based index.
func ColumnAt(index int) Column {
	return Column{index: index, isIndex: true}
}

// Name returns the header name of a column of ColumnNamed.
func (c Column) Name() (string, bool) {
	return c.name, c.isName
}

// Index returns the index of a column of ColumnAt.
func (c Column) Index() (int, bool) {
	return c.index, c.isIndex
}

// IsZero reports whether c is the zero value, the header named like the role.
func (c Column) IsZero() bool {
	return c == Column{}
}

// Columns is the column layout of a table.
type Columns struct {
	Start    Column // the start time
	End      Column // the end time
	Text     Column // the cue text
	Speaker  Column // the speaker name
	ID       Column // the cue identifier, the header named "identifier" by default
	Duration Column // the duration. An end cell wins
	// NoHeader says that the first row is data. Then Start and Text need a column index.
	NoHeader bool
}

// isZero reports whether c is the zero value, which reads the columns by their header names, with synonyms.
func (c Columns) isZero() bool {
	return c == Columns{}
}

type role struct {
	name, field string
}

// roles holds the column roles in the order in which the reader resolves them.
var roles = [...]role{ //nolint:gochecknoglobals // a fixed list
	{"identifier", "ID"},
	{"start", "Start"},
	{"end", "End"},
	{"duration", "Duration"},
	{"speaker", "Speaker"},
	{"text", "Text"},
}

func (c Columns) column(r string) Column {
	switch r {
	case "identifier":
		return c.ID
	case "start":
		return c.Start
	case "end":
		return c.End
	case "duration":
		return c.Duration
	case "speaker":
		return c.Speaker
	default:
		return c.Text
	}
}

func (c Columns) validate() error {
	for _, r := range roles {
		col := c.column(r.name)
		option := "ReadOptions.CSV.Columns." + r.field
		if col.isIndex && col.index < 0 {
			return invalid(option, "The column index of "+r.name+" must be 0 or more, got "+strconv.Itoa(col.index)+".")
		}
		if c.NoHeader && !col.isIndex && (col.isName || r.name == "start" || r.name == "text") {
			return invalid(option, "A table without a header row needs a column index for "+r.name+".")
		}
	}
	return nil
}

// ReadOptions holds the settings of the CSV and TSV reader.
type ReadOptions struct {
	// Columns is the column layout. The zero value reads the columns by their header names, with synonyms.
	Columns Columns
	// Delimiter is the cell separator. The zero value detects it from the first line.
	Delimiter Delimiter
	// FrameRate is the frames per second of times in hh:mm:ss:ff. Zero means none.
	FrameRate float64
}

// Validate returns an error that matches model.ErrInvalidArgument for a negative column index, a column name
// without a header row, an unknown delimiter, or a frame rate that is negative, NaN or infinite.
func (o ReadOptions) Validate() error {
	if err := o.Columns.validate(); err != nil {
		return err
	}
	if err := checkDelimiter(o.Delimiter, "ReadOptions.CSV.Delimiter"); err != nil {
		return err
	}
	if o.FrameRate == 0 {
		return nil
	}
	return parsing.CheckFrameRate(o.FrameRate, "ReadOptions.CSV.FrameRate", "The frame rate")
}

// WriteOptions holds the settings of the CSV and TSV writer.
type WriteOptions struct {
	// Delimiter is the cell separator. The zero value takes the stored delimiter, else Comma.
	Delimiter Delimiter
	// TimeFormat is the format of the time cells. The zero value takes the stored format, else TimeDot.
	TimeFormat TimeFormat
	// FrameRate is the frames per second of TimeFrames. Zero takes the stored frame rate.
	FrameRate float64
	// SecondText holds the cues whose text goes into a column after the text. Nil writes no such column.
	SecondText *model.Subtitle
	// SecondTextHeader is the header of the SecondText column. The zero value is "text2".
	SecondTextHeader string
	// EscapeFormulas puts ' before a cell that starts with =, +, - or @, so a spreadsheet shows it as text.
	EscapeFormulas bool
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown delimiter or time format, or a
// frame rate that is negative, NaN or infinite.
func (o WriteOptions) Validate() error {
	if err := checkDelimiter(o.Delimiter, "WriteOptions.CSV.Delimiter"); err != nil {
		return err
	}
	switch o.TimeFormat {
	case TimeFormatDefault, TimeSeconds, TimeDot, TimeComma, TimeFrames, TimeMilliseconds:
	default:
		return invalid("WriteOptions.CSV.TimeFormat", "The CSV time format must be seconds, hh:mm:ss.mmm, hh:mm:ss,mmm, "+
			"hh:mm:ss:ff or milliseconds, got "+strconv.Quote(string(o.TimeFormat))+".")
	}
	if o.FrameRate == 0 {
		return nil
	}
	return parsing.CheckFrameRate(o.FrameRate, "WriteOptions.CSV.FrameRate", "The CSV frame rate")
}

func checkDelimiter(d Delimiter, option string) error {
	switch d {
	case DelimiterDefault, Comma, Semicolon, Tab:
		return nil
	default:
		return invalid(option, `The CSV delimiter must be ",", ";" or a tab.`)
	}
}

func invalid(option, msg string) error {
	return &model.InvalidArgumentError{Option: option, Msg: msg}
}

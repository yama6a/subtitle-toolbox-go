package model

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
)

// Position is where a cue shows on the screen.
type Position struct {
	// Alignment is the 9-cell alignment. The zero value is BottomCenter.
	Alignment Alignment
	// Row is the 1-based row of the first line on Grid. 0 means the cue has no row.
	Row int
	// Column is the 1-based column of the first character on Grid. 0 means the cue has no column.
	Column int
	// Grid is the caption grid of Row and Column. It is zero when Row and Column are 0.
	Grid Grid
	// Offset is the point of the cue that Alignment names, in percent of the video. Nil means the cue has no offset.
	Offset *Offset
}

// Grid is a caption grid of rows and columns.
type Grid struct {
	// Rows is the number of rows.
	Rows int
	// Columns is the number of columns.
	Columns int
}

// Offset is a point in percent of the video width and height.
type Offset struct {
	// X is the distance from the left edge. 0 is the left edge and 100 is the right edge.
	X float64
	// Y is the distance from the top edge. 0 is the top edge and 100 is the bottom edge.
	Y float64
}

// RowOn returns the row on grid g of the first of lines lines of p, and false when p has no row or g or p.Grid has no rows.
// It splits the rows of p.Grid into thirds by the first row. A top row keeps its number, and a bottom row keeps
// the distance of the last line from the bottom row. A middle row scales as (row - 1) * (g.Rows - 1) / (p.Grid.Rows - 1) + 1,
// rounded. The result is clamped so that all lines fit on g. Example: with 1 line, rows 2, 15 and 8 of
// a 15-row grid give rows 2, 23 and 12 of a 23-row grid.
func (p Position) RowOn(g Grid, lines int) (int, bool) {
	rows := p.Grid.Rows
	if p.Row < 1 || rows < 1 || g.Rows < 1 {
		return 0, false
	}
	lines = max(lines, 1)
	var row int
	switch above := 3 * (p.Row - 1); {
	case above < rows:
		row = p.Row
	case above >= 2*rows:
		row = g.Rows - (rows - (p.Row + lines - 1)) - lines + 1
	default:
		den := rows - 1
		row = ((p.Row-1)*(g.Rows-1)*2+den)/(2*den) + 1
	}
	return min(max(row, 1), max(g.Rows-lines+1, 1)), true
}

// Equal reports whether p and o have the same fields, with Offset compared by value.
func (p Position) Equal(o Position) bool {
	if p.Offset == nil || o.Offset == nil {
		return p.Offset == o.Offset && p.withoutOffset() == o.withoutOffset()
	}
	return *p.Offset == *o.Offset && p.withoutOffset() == o.withoutOffset()
}

func (p Position) clone() Position {
	if p.Offset != nil {
		offset := *p.Offset
		p.Offset = &offset
	}
	return p
}

func (p Position) withoutOffset() Position {
	p.Offset = nil
	return p
}

// Validate returns an error that matches ErrInvalidArgument unless Row is from 0 to Grid.Rows, Column is
// from 0 to Grid.Columns, and Offset is from 0 to 100 on both axes. A zero Grid therefore allows no Row and no Column.
// The Option of the error names the field, for example "Position.Offset.X".
func (p Position) Validate() error {
	switch {
	case p.Row < 0 || p.Row > p.Grid.Rows:
		return positionError("Row", "The row must be from 0 to the grid rows "+strconv.Itoa(p.Grid.Rows)+", got "+strconv.Itoa(p.Row)+".")
	case p.Column < 0 || p.Column > p.Grid.Columns:
		return positionError("Column", "The column must be from 0 to the grid columns "+strconv.Itoa(p.Grid.Columns)+", got "+strconv.Itoa(p.Column)+".")
	case p.Offset == nil:
		return nil
	case !(p.Offset.X >= 0 && p.Offset.X <= 100):
		return positionError("Offset.X", "The offset must be from 0 to 100, got "+timecode.Number(p.Offset.X)+".")
	case !(p.Offset.Y >= 0 && p.Offset.Y <= 100):
		return positionError("Offset.Y", "The offset must be from 0 to 100, got "+timecode.Number(p.Offset.Y)+".")
	}
	return nil
}

func positionError(field, msg string) error {
	return &InvalidArgumentError{Option: "Position." + field, Msg: msg}
}


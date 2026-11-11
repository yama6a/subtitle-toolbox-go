// Package position holds the rules for cue positions: an explicit cue alignment wins over an exact position that a reader stored,
// and an offset counts only inside the screen cell of the alignment.
package position

import (
	"math"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// AlignmentUnchanged reports whether a equals the numpad alignment in the field "alignment" of the cue format data d.
func AlignmentUnchanged(d *model.Data, a model.Alignment) bool {
	n, ok := d.Int("alignment")
	if !ok {
		n = int64(model.BottomCenter.Numpad())
	}
	return n == int64(a.Numpad())
}

// OffsetFits reports whether o lies in the screen cell of a. X below 100/3 is the left column, above 200/3 the
// right column, else the center column. Y below 100/3 is the top row, above 200/3 the bottom row, else the middle
// row. A nil offset, or a value outside 0 to 100 or NaN, does not fit.
func OffsetFits(a model.Alignment, o *model.Offset) bool {
	if o == nil || !(o.X >= 0 && o.X <= 100 && o.Y >= 0 && o.Y <= 100) {
		return false
	}
	return Column(o.X) == a.Column() && Row(o.Y) == a.Row()
}

// Column returns the column of the screen cell that holds x percent: 1 left, 2 center, 3 right.
func Column(x float64) int {
	switch {
	case x < 100.0/3:
		return 1
	case x > 200.0/3:
		return 3
	default:
		return 2
	}
}

// Row returns the row of the screen cell that holds y percent, numbered as model.Alignment.Row: 2 top, 1 middle, 0 bottom.
func Row(y float64) int {
	switch {
	case y < 100.0/3:
		return 2
	case y > 200.0/3:
		return 0
	default:
		return 1
	}
}

// Round rounds a percentage to 3 decimals.
func Round(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// Format returns v rounded to 3 decimals without trailing zeros, such as 90 and 84.667.
func Format(v float64) string {
	return strconv.FormatFloat(Round(v), 'f', -1, 64)
}

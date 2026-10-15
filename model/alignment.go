package model

import "strconv"

// Alignment is the place of a cue on the screen. The zero value is BottomCenter.
type Alignment uint8

// The 9 alignments of the numeric keypad grid.
const (
	BottomCenter Alignment = iota
	BottomLeft
	BottomRight
	MiddleLeft
	MiddleCenter
	MiddleRight
	TopLeft
	TopCenter
	TopRight
)

const gridColumns = 3

// AlignmentFromNumpad returns the alignment of the numeric keypad key n, as in the ASS tag {\an8}.
// It returns false for n outside 1 to 9.
func AlignmentFromNumpad(n int) (Alignment, bool) {
	switch {
	case n == 1:
		return BottomLeft, true
	case n == 2:
		return BottomCenter, true
	case n >= 3 && n <= 9:
		return Alignment(n - 1), true
	default:
		return BottomCenter, false
	}
}

// AlignmentAt returns the alignment in row 0 bottom, 1 middle or 2 top and column 1 left, 2 center or 3 right.
// It returns false for a cell outside the grid.
func AlignmentAt(row, column int) (Alignment, bool) {
	if row < 0 || row > 2 || column < 1 || column > gridColumns {
		return BottomCenter, false
	}
	return AlignmentFromNumpad(gridColumns*row + column)
}

// Numpad returns the numeric keypad key of a, 1 to 9. A value outside the 9 constants counts as BottomCenter.
func (a Alignment) Numpad() int {
	switch {
	case a == BottomLeft:
		return 1
	case a > BottomLeft && a <= TopRight:
		return int(a) + 1
	default:
		return 2
	}
}

// Row returns the grid row of a: 0 bottom, 1 middle, 2 top. A value outside the 9 constants counts as BottomCenter.
func (a Alignment) Row() int {
	return (a.Numpad() - 1) / gridColumns
}

// Column returns the grid column of a: 1 left, 2 center, 3 right. A value outside the 9 constants counts as BottomCenter.
func (a Alignment) Column() int {
	return (a.Numpad()-1)%gridColumns + 1
}

// String returns the name of the constant, for example "TopCenter".
func (a Alignment) String() string {
	names := [...]string{
		"BottomCenter", "BottomLeft", "BottomRight",
		"MiddleLeft", "MiddleCenter", "MiddleRight",
		"TopLeft", "TopCenter", "TopRight",
	}
	if int(a) < len(names) {
		return names[a]
	}
	return "Alignment(" + strconv.Itoa(int(a)) + ")"
}

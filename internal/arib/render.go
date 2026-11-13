package arib

import (
	"cmp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// run is a row of touching ruby characters.
type run struct {
	y, start, end int
	text          string
}

// render returns the lines of the screen from top to bottom and its alignment. Small characters above a row are
// furigana. They become ruby markup on the characters of the row below that they cover.
func (d *Decoder) render() ([]string, model.Alignment) {
	if len(d.cells) == 0 {
		return nil, model.BottomCenter
	}
	rows := map[int][]cell{}
	rubies := map[int][]cell{}
	for _, c := range d.cells {
		if c.ruby {
			rubies[c.y] = append(rubies[c.y], c)
		} else {
			rows[c.y] = append(rows[c.y], c)
		}
	}
	for _, cells := range rows {
		slices.SortStableFunc(cells, func(a, b cell) int { return cmp.Compare(a.x, b.x) })
	}
	runs := buildRuns(rubies)
	readings, lines := placeRuns(rows, runs)
	for _, y := range sortedKeys(rows) {
		if line := renderRow(rows[y], readings[y], runs); line != "" {
			lines[y] = append(lines[y], line)
		}
	}
	var out []string
	for _, y := range sortedKeys(lines) {
		out = append(out, lines[y]...)
	}
	return out, d.alignment()
}

// placeRuns attaches each run to the next row below it. It returns the reading of each character by row and
// position, and the lines of the runs that have no row below.
func placeRuns(rows map[int][]cell, runs []run) (readings map[int]map[int]int, lines map[int][]string) {
	readings = map[int]map[int]int{}
	lines = map[int][]string{}
	rowYs := sortedKeys(rows)
	for i, r := range runs {
		below := slices.IndexFunc(rowYs, func(y int) bool { return y > r.y })
		if below < 0 || !attach(rows[rowYs[below]], readings, rowYs[below], r, i) {
			lines[r.y] = append(lines[r.y], markupcore.EscapeText(strings.Trim(r.text, " \t\n\r\x00\x0b")))
		}
	}
	return readings, lines
}

// buildRuns joins the ruby characters that touch on one y into runs, from top to bottom and from left to right.
func buildRuns(rubies map[int][]cell) []run {
	var runs []run
	for _, y := range sortedKeys(rubies) {
		cells := rubies[y]
		slices.SortStableFunc(cells, func(a, b cell) int { return cmp.Compare(a.x, b.x) })
		for _, c := range cells {
			if n := len(runs); n > 0 && runs[n-1].y == y && runs[n-1].end == c.x {
				runs[n-1].end += c.width
				runs[n-1].text += c.text
			} else {
				runs = append(runs, run{y, c.x, c.x + c.width, c.text})
			}
		}
	}
	return runs
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// attach gives the characters of row that the run covers the reading i. The middle of a character must lie under
// the run, or else the character must overlap it.
func attach(row []cell, readings map[int]map[int]int, y int, r run, i int) bool {
	if readings[y] == nil {
		readings[y] = map[int]int{}
	}
	for _, byMiddle := range []bool{true, false} {
		found := false
		for position, c := range row {
			var covers bool
			if byMiddle {
				covers = 2*r.start <= 2*c.x+c.width && 2*c.x+c.width < 2*r.end
			} else {
				covers = c.x < r.end && c.x+c.width > r.start
			}
			if _, taken := readings[y][position]; covers && !taken {
				readings[y][position] = i
				found = true
			}
		}
		if found {
			return true
		}
	}
	return false
}

func isSpace(text string) bool { return text == " " || text == "\u3000" }

// rowText builds the text of a row with font tags for colors other than white.
type rowText struct {
	text    strings.Builder
	open    string
	end     int
	haveEnd bool
}

// add writes markup for the character c. A gap of at least half a cell before c becomes a space.
func (w *rowText) add(c cell, right int, markup string) {
	gap := ""
	if w.haveEnd && 2*(c.x-w.end) >= c.width {
		gap = " "
	}
	if col := cmp.Or(c.color, white); col != w.open {
		if w.open != white {
			w.text.WriteString("</font>")
		}
		w.text.WriteString(gap)
		if col != white {
			w.text.WriteString(`<font color="` + col + `">`)
		}
		gap = ""
		w.open = col
	}
	w.text.WriteString(gap + markup)
	w.end, w.haveEnd = right, true
}

func (w *rowText) String() string {
	if w.text.Len() == 0 || w.open == white {
		return w.text.String()
	}
	return w.text.String() + "</font>"
}

// renderRow returns the text of a row with font tags for colors other than white and ruby tags for furigana.
// Spaces at the ends of the row drop.
func renderRow(row []cell, readings map[int]int, runs []run) string {
	first, last := trimRow(row, readings)
	w := rowText{open: white}
	for position := first; position <= last; {
		c := row[position]
		reading, hasRun := readings[position]
		base, right, next := baseOfReading(row, readings, position, last)
		markup := markupcore.EscapeText(base)
		if hasRun {
			markup = "<ruby>" + markup + "<rt>" + markupcore.EscapeText(runs[reading].text) + "</rt></ruby>"
		}
		w.add(c, right, markup)
		position = next
	}
	return w.String()
}

// trimRow returns the first and the last position of the row without the spaces at its ends. A space with a
// reading stays.
func trimRow(row []cell, readings map[int]int) (first, last int) {
	first, last = 0, len(row)-1
	for first <= last && !hasReading(readings, first) && isSpace(row[first].text) {
		first++
	}
	for last >= first && !hasReading(readings, last) && isSpace(row[last].text) {
		last--
	}
	return first, last
}

// baseOfReading returns the text and the right edge of the characters from position that share a reading, and the
// position after them. A character without a reading stands alone.
func baseOfReading(row []cell, readings map[int]int, position, last int) (base string, right, next int) {
	reading, hasRun := readings[position]
	var sb strings.Builder
	for next = position; ; {
		sb.WriteString(row[next].text)
		right = row[next].x + row[next].width
		next++
		if r, ok := readings[next]; !hasRun || next > last || !ok || r != reading {
			return sb.String(), right, next
		}
	}
}

func hasReading(readings map[int]int, position int) bool {
	_, ok := readings[position]
	return ok
}

// alignment returns the alignment from the middle of the text on the caption plane: the left, middle or right third
// and the top, middle or bottom third.
func (d *Decoder) alignment() model.Alignment {
	left, right := d.cells[0].x, d.cells[0].x+d.cells[0].width
	top, bottom := d.cells[0].y, d.cells[0].y+d.cells[0].height
	for _, c := range d.cells[1:] {
		left, right = min(left, c.x), max(right, c.x+c.width)
		top, bottom = min(top, c.y), max(bottom, c.y+c.height)
	}
	column, row := 2, 3
	switch middle := (left + right) * 3; {
	case middle < 2*d.planeWidth:
		column = 1
	case middle >= 4*d.planeWidth:
		column = 3
	}
	switch middle := (top + bottom) * 3; {
	case middle < 2*d.planeHeight:
		row = 6
	case middle >= 4*d.planeHeight:
		row = 0
	}
	a, _ := model.AlignmentFromNumpad(row + column)
	return a
}

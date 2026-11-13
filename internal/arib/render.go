package arib

import (
	"cmp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
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
	rowYs := sortedKeys(rows)
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
	for _, cells := range rows {
		slices.SortStableFunc(cells, func(a, b cell) int { return cmp.Compare(a.x, b.x) })
	}

	readings := map[int]map[int]int{}
	lines := map[int][]string{}
	for i, r := range runs {
		var base int
		found := false
		for _, y := range rowYs {
			if y > r.y {
				base, found = y, true
				break
			}
		}
		if !found || !attach(rows[base], readings, base, r, i) {
			lines[r.y] = append(lines[r.y], markupcore.EscapeText(strings.Trim(r.text, " \t\n\r\x00\x0b")))
		}
	}
	for _, y := range rowYs {
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

func isSpace(text string) bool { return text == " " || text == "　" }

// renderRow returns the text of a row with font tags for colors other than white and ruby tags for furigana. A gap
// of at least half a cell between characters becomes a space. Spaces at the ends of the row drop.
func renderRow(row []cell, readings map[int]int, runs []run) string {
	first, last := 0, len(row)-1
	for first <= last && !hasReading(readings, first) && isSpace(row[first].text) {
		first++
	}
	for last >= first && !hasReading(readings, last) && isSpace(row[last].text) {
		last--
	}

	var text strings.Builder
	open := white
	end, haveEnd := 0, false
	for position := first; position <= last; {
		c := row[position]
		reading, hasRun := readings[position]
		var base strings.Builder
		right := 0
		for {
			base.WriteString(row[position].text)
			right = row[position].x + row[position].width
			position++
			next, ok := readings[position]
			if !hasRun || position > last || !ok || next != reading {
				break
			}
		}

		gap := ""
		if haveEnd && 2*(c.x-end) >= c.width {
			gap = " "
		}
		col := cmp.Or(c.color, white)
		if col != open {
			if open != white {
				text.WriteString("</font>")
			}
			text.WriteString(gap)
			if col != white {
				text.WriteString(`<font color="` + col + `">`)
			}
			gap = ""
			open = col
		}
		text.WriteString(gap)
		if hasRun {
			text.WriteString("<ruby>" + markupcore.EscapeText(base.String()) + "<rt>" + markupcore.EscapeText(runs[reading].text) + "</rt></ruby>")
		} else {
			text.WriteString(markupcore.EscapeText(base.String()))
		}
		end, haveEnd = right, true
	}
	if text.Len() == 0 || open == white {
		return text.String()
	}
	return text.String() + "</font>"
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

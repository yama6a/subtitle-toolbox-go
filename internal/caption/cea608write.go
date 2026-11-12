package caption

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const formatDataKey = "scc"

// FrameWord is one word on one frame at 29.97 fps.
type FrameWord struct {
	Frame int64
	Word  uint16
}

// CEA608Words returns the pop-on words of data channel 1 by frame at 29.97 fps, in frame order.
// With fit, it changes what CEA-608 cannot hold and returns each change. A cue that breaks a limit gives a
// *model.UnwritableError of format f.
func CEA608Words(f model.Format, cues []model.Cue, fit bool) ([]FrameWord, []FitChange, error) {
	t := timeline{words: map[int64]uint16{}}
	var changes []FitChange
	for i, c := range cues {
		load, cueChanges, err := loadWords(f, c, i, fit)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, cueChanges...)
		if len(load) == 0 {
			continue
		}
		start, end := frameOf(c.Start), frameOf(c.End)
		eoc, edm, hasEDM := t.schedule(int64(len(load)), start)
		if fit && eoc > start {
			change := lateChange(i, c, eoc, end)
			changes = append(changes, change)
			if change.Action == Dropped {
				continue
			}
		}
		t.place(load, eoc, edm, hasEDM)
		t.nextFree = eoc + 2
		t.previousEnd, t.hasPrevious = end, true
	}
	if t.hasPrevious {
		edm := max(t.previousEnd, t.nextFree)
		t.words[edm], t.words[edm+1] = edmWord, edmWord
	}
	words := make([]FrameWord, 0, len(t.words))
	limit := lastFrame()
	for frame, w := range t.words {
		if frame <= limit {
			words = append(words, FrameWord{frame, w})
		}
	}
	slices.SortFunc(words, func(a, b FrameWord) int { return cmp.Compare(a.Frame, b.Frame) })
	return words, changes, nil
}

//nolint:gochecknoglobals // words of the commands EDM and EOC, built once
var (
	edmWord = cea608.ControlWord(cea608.CommandEDM)
	eocWord = cea608.ControlWord(cea608.CommandEOC)
)

// lateChange describes a caption that shows after its cue start: dropped when its EOC is not before the cue end.
func lateChange(index int, c model.Cue, eoc, end int64) FitChange {
	prefix := "Cue #" + strconv.Itoa(index) + " at " + timecode.Seconds(c.Start) + " s"
	if eoc >= end {
		return FitChange{index, Dropped, prefix + " is left out. Its caption data needs the frames up to " +
			frameSeconds(eoc) + " s, but the cue ends at " + timecode.Seconds(c.End) + " s."}
	}
	return FitChange{index, Delayed, prefix + " shows at " + frameSeconds(eoc) +
		" s. The frames before its start cannot hold its caption data."}
}

// timeline holds the words by frame and the state that the next caption needs.
type timeline struct {
	words       map[int64]uint16
	nextFree    int64
	previousEnd int64
	hasPrevious bool
}

// place puts the EDM of the caption before, the load words in the free frames before the EOC, and the EOC.
func (t *timeline) place(load []uint16, eoc, edm int64, hasEDM bool) {
	if hasEDM {
		t.words[edm], t.words[edm+1] = edmWord, edmWord
	}
	frame := eoc - 1
	for _, w := range slices.Backward(load) {
		for hasWord(t.words, frame) {
			frame--
		}
		t.words[frame] = w
		frame--
	}
	t.words[eoc], t.words[eoc+1] = eocWord, eocWord
}

func hasWord(timeline map[int64]uint16, frame int64) bool {
	_, ok := timeline[frame]
	return ok
}

func frameOf(t time.Duration) int64 {
	return model.FrameRate2997.Frames(max(0, t))
}

// lastFrame returns the last frame whose time is below the time limit. A later frame has no label that a reader takes.
func lastFrame() int64 {
	f := model.FrameRate2997.Frames(timecode.MaxTime)
	if model.FrameRate2997.Duration(f) >= timecode.MaxTime {
		f--
	}
	return f
}

func frameSeconds(frame int64) string {
	return timecode.Number(timecode.FloatSeconds(frameTime(frame)))
}

// schedule returns the frame of the EOC, and the frame of the EDM of the caption before it when it has one.
// The load goes into the free frames before the EOC. When they are too few, the EOC moves after the cue start.
func (t *timeline) schedule(load, start int64) (eoc, edm int64, hasEDM bool) {
	eoc = max(start, t.nextFree+load)
	for {
		hasEDM = t.hasPrevious && t.previousEnd < eoc && max(t.previousEnd, t.nextFree)+1 < eoc
		free := eoc - t.nextFree
		if hasEDM {
			edm = max(t.previousEnd, t.nextFree)
			free -= 2
		}
		if free >= load {
			return eoc, edm, hasEDM
		}
		eoc += load - free
	}
}

// loadWords returns the words that load the caption into hidden memory, without the EOC, and the changes of the fit.
// A cue without text gives no word.
func loadWords(f model.Format, c model.Cue, index int, fit bool) ([]uint16, []FitChange, error) {
	text := make([]string, len(c.Lines))
	for i, line := range c.Lines {
		text[i] = markupcore.RubyAsText(line)
	}
	var changes []FitChange
	if fit {
		text, changes = fitText(text, c.Start, index)
	}
	var lines [][]cea608.Char
	for _, line := range markupcore.CarryOpenTags(text) {
		if chars := cea608.StyledCharacters(line); len(chars) > 0 {
			lines = append(lines, chars)
		}
	}
	if len(lines) == 0 {
		return nil, changes, nil
	}
	if err := checkLimits(f, c, index, lines); err != nil {
		return nil, nil, err
	}

	cells := make([][]cea608.Cell, len(lines))
	for i, chars := range lines {
		cells[i] = cea608.Cells(chars)
	}
	enm, rcl := cea608.ControlWord(cea608.CommandENM), cea608.ControlWord(cea608.CommandRCL)
	words := []uint16{enm, enm, rcl, rcl}
	for i, p := range positions(c, cells) {
		words = append(words, cea608.RowWords(p.row, p.column, cells[i])...)
	}
	return words, changes, nil
}

func checkLimits(f model.Format, c model.Cue, index int, lines [][]cea608.Char) error {
	prefix := "Cue #" + strconv.Itoa(index) + " at " + timecode.Seconds(c.Start) + " s has "
	fail := func(reason string) error {
		e := &model.UnwritableError{Format: f, CueIndex: index, Reason: reason}
		if linewrap.FewestLines(c.Lines, cea608.Columns) <= cea608.MaxLines {
			e.Err = ErrWrapFirst
			e.Reason += " Wrap the lines to 4 lines of 32 characters first."
		} else {
			e.Err = ErrSplitFirst
			e.Reason += " Split the cue into cues of 4 lines of 32 characters, then wrap the lines."
		}
		return e
	}
	if len(lines) > cea608.MaxLines {
		return fail(prefix + strconv.Itoa(len(lines)) + " lines, but SCC allows " + strconv.Itoa(cea608.MaxLines) + ".")
	}
	for _, chars := range lines {
		if len(chars) > cea608.Columns {
			return fail(prefix + "a line with " + strconv.Itoa(len(chars)) + " characters, but SCC allows " +
				strconv.Itoa(cea608.Columns) + ".")
		}
		for _, ch := range chars {
			if !cea608.Encodable(ch.R) {
				return &model.UnwritableError{
					Format: f, CueIndex: index,
					Reason: prefix + `the character "` + string(ch.R) + `", which CEA-608 cannot show.`,
				}
			}
		}
	}
	return nil
}

type position struct{ row, column int }

// positions returns the row and the text column of each line. The rule that holds first wins: the rows and
// columns that a reader stored, the Position of the cue, the alignment.
func positions(c model.Cue, cells [][]cea608.Cell) []position {
	widths := make([]int, len(cells))
	for i, line := range cells {
		widths[i] = textWidth(line)
	}
	out := make([]position, len(cells))
	if rows, columns, ok := storedPositions(c, len(cells)); ok {
		for i := range out {
			out[i] = position{rows[i], clampColumn(columns[i], widths[i])}
		}
		return out
	}
	first, onGrid := c.Position.RowOn(model.Grid{Rows: cea608.Rows, Columns: cea608.Columns}, len(cells))
	if !onGrid {
		first = alignedRow(c.Position.Alignment, len(cells))
	}
	for i := range out {
		column := alignedColumn(c.Position.Alignment, widths[i])
		if onGrid && c.Position.Column > 0 && c.Position.Grid.Columns == cea608.Columns {
			column = c.Position.Column - 1
		}
		out[i] = position{first + i, clampColumn(column, widths[i])}
	}
	return out
}

// textWidth returns the cell count of a line without its leading mid-row codes.
func textWidth(line []cea608.Cell) int {
	width := len(line)
	for _, cell := range line {
		if cell.MidRow == 0 {
			break
		}
		width--
	}
	return width
}

func clampColumn(column, width int) int {
	return max(0, min(column, cea608.Columns-width))
}

// alignedRow returns the first row of a block of lines that the alignment sets.
func alignedRow(a model.Alignment, lines int) int {
	switch a.Row() {
	case 2:
		return 1
	case 1:
		return (cea608.Rows-lines)/2 + 1
	default:
		return cea608.Rows - lines + 1
	}
}

// alignedColumn returns the first column of a line of width cells that the alignment sets.
func alignedColumn(a model.Alignment, width int) int {
	switch a.Column() {
	case 1:
		return 0
	case 3:
		return cea608.Columns - width
	default:
		return (cea608.Columns - width) / 2
	}
}

// storedPositions returns the rows and columns of the scc cue data when they still fit the cue.
func storedPositions(c model.Cue, count int) (rows, columns []int, ok bool) {
	data, _ := c.FormatData(formatDataKey)
	rows, okRows := intList(data, "rows", count)
	columns, okColumns := intList(data, "columns", count)
	if !okRows || !okColumns {
		return nil, nil, false
	}
	for i, row := range rows {
		if row < 1 || row > cea608.Rows || (i > 0 && row <= rows[i-1]) {
			return nil, nil, false
		}
	}
	top := c.Position.Alignment.Row() == 2
	if (rows[0] <= cea608.MaxLines) != top || c.Position.Row != rows[0] {
		return nil, nil, false
	}
	return rows, columns, true
}

// intList returns the list of integers at key when it has count entries. A value of another type counts as missing.
func intList(data *model.Data, key string, count int) ([]int, bool) {
	list, ok := data.List(key)
	if !ok || len(list) != count {
		return nil, false
	}
	out := make([]int, count)
	for i, v := range list {
		n, ok := v.(int64)
		if !ok {
			return nil, false
		}
		out[i] = int(n)
	}
	return out, true
}

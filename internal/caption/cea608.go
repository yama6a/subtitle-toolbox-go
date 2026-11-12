// Package caption turns caption data into cues.
package caption

import (
	"cmp"
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// RollUp says how the reader turns roll-up captions into cues. The empty value acts as RollUpScreen.
type RollUp string

const (
	// RollUpScreen gives one cue per screen of roll-up captions.
	RollUpScreen RollUp = "screen"
	// RollUpLines gives one cue per row, from its first character until it rolls up or the screen clears.
	RollUpLines RollUp = "lines"
)

// CEA608Options holds the read settings of CEA-608 data.
type CEA608Options struct {
	Channel int    // 1 or 2, checked by the format options
	RollUp  RollUp // RollUpScreen or RollUpLines
}

// CEA608Cues decodes the code lines and returns a cue for each change of the displayed captions.
// The scc format data of a cue holds its mode, and the row and column of each line.
// The last cue lasts lastCueDuration, up to the time limit.
func CEA608Cues(lines []cea608.CodeLine, opts CEA608Options, lastCueDuration time.Duration) []model.Cue {
	d := cea608.NewDecoder(opts.Channel)
	states := d.Decode(lines)
	byLine := opts.RollUp == RollUpLines

	var cues []model.Cue
	for i, s := range states {
		if len(s.Lines) == 0 || (byLine && s.Mode == cea608.RollUp) {
			continue
		}
		start := frameTime(s.Frame)
		end := parsing.EndAfter(start, lastCueDuration)
		if i+1 < len(states) {
			end = frameTime(states[i+1].Frame)
			if end <= start {
				continue
			}
		}
		cues = append(cues, newCue(start, end, s.Lines, s.Mode))
	}
	if byLine {
		cues = append(cues, rollUpCues(d.RollUpLines(), lastCueDuration)...)
		slices.SortStableFunc(cues, func(a, b model.Cue) int { return cmp.Compare(a.Start, b.Start) })
	}
	return cues
}

func rollUpCues(rows []cea608.RollUpLine, lastCueDuration time.Duration) []model.Cue {
	var cues []model.Cue
	for _, l := range rows {
		start := frameTime(l.Start)
		end := parsing.EndAfter(start, lastCueDuration)
		if l.HasEnd {
			end = frameTime(l.End)
		}
		if end > start {
			cues = append(cues, newCue(start, end, []cea608.Line{l.Line}, cea608.RollUp))
		}
	}
	return cues
}

// frameTime returns the time of a frame at 29.97 fps in whole milliseconds.
func frameTime(frame int64) time.Duration {
	return model.FrameRate2997.Duration(frame).Round(time.Millisecond)
}

func newCue(start, end time.Duration, lines []cea608.Line, mode cea608.Mode) model.Cue {
	c := model.Cue{Start: start, End: end}
	c.Position = model.Position{
		Alignment: model.BottomCenter,
		Row:       lines[0].Row,
		Column:    lines[0].Column + 1,
		Grid:      model.Grid{Rows: cea608.Rows, Columns: cea608.Columns},
	}
	if lines[0].Row <= cea608.MaxLines {
		c.Position.Alignment = model.TopCenter
	}
	rows := make([]any, len(lines))
	columns := make([]any, len(lines))
	for i, l := range lines {
		c.AddLine(l.Text)
		rows[i] = int64(l.Row)
		columns[i] = int64(l.Column)
	}
	_ = c.SetFormatData("scc", model.NewData(
		model.Field{Key: "mode", Value: string(mode)},
		model.Field{Key: "rows", Value: rows},
		model.Field{Key: "columns", Value: columns},
	))
	return c
}

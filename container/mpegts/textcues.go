package mpegts

import (
	"fmt"
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// textState is the text that a screen shows from start on. No lines clear the screen.
type textState struct {
	start     int64 // PTS, 90 kHz
	lines     []string
	alignment model.Alignment // teletext leaves it zero
}

// textCues turns the states of a text stream into cues. Times count from zero.
// Each state with lines lasts up to the next state. The last one ends at last when that is after its start,
// else after lastCueDuration.
func textCues(number int, states []textState, zero, last int64, lastCueDuration time.Duration) ([]model.Cue, error) {
	merged := mergeStates(states)
	var cues []model.Cue
	for i, st := range merged {
		if len(st.lines) == 0 {
			continue
		}
		start, err := ticksToTime(number, st.start-zero)
		if err != nil {
			return nil, err
		}
		end := parsing.EndAfter(start, lastCueDuration)
		switch {
		case i+1 < len(merged):
			end, err = ticksToTime(number, merged[i+1].start-zero)
		case last > st.start:
			end, err = ticksToTime(number, last-zero)
		}
		if err != nil {
			return nil, err
		}
		cue := model.Cue{Start: start, End: end, Lines: slices.Clone(st.lines)}
		cue.Position.Alignment = st.alignment
		cues = append(cues, cue)
	}
	return cues, nil
}

// mergeStates joins consecutive states with the same lines and alignment.
func mergeStates(states []textState) []textState {
	var merged []textState
	for _, st := range states {
		if n := len(merged); n > 0 && slices.Equal(merged[n-1].lines, st.lines) && merged[n-1].alignment == st.alignment {
			continue
		}
		merged = append(merged, st)
	}
	return merged
}

// ticksToTime returns max(0, ticks) of the 90 kHz clock as whole milliseconds, half up.
func ticksToTime(number int, ticks int64) (time.Duration, error) {
	ms := (max(0, ticks) + 45) / 90
	if ms >= int64(timecode.MaxTime/time.Millisecond) {
		return 0, &model.ParseError{Msg: fmt.Sprintf("The cue times of track %d reach %s s. Times must be below %d hours.",
			number, timecode.Number(float64(ms)/1000), timecode.MaxHours)}
	}
	return time.Duration(ms) * time.Millisecond, nil
}

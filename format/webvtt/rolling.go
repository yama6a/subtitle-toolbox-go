package webvtt

import (
	"regexp"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// maxRepeatDuration is the longest repeat cue. yt-dlp writes it with 10 ms.
const maxRepeatDuration = 10500 * time.Microsecond

var (
	cWordTimestampRegexp = regexp.MustCompile(markupcore.WordTimestampPattern + `<c[.>]`)
	whiteSpaceRunRegexp  = regexp.MustCompile(`[\t\n\v\f\r ]+`)
)

// collapseRollingCues collapses the rolling cues of YouTube automatic captions, as yt-dlp --write-auto-subs saves
// them. Each cue repeats the line of the cue before it above a new line with <c> word timestamps, and a 10 ms cue
// that repeats the last line follows each cue. The collapse drops the 10 ms cues and the repeated lines, and the
// next cue starts where the dropped 10 ms cue started. It starts only once a cue has <c> word timestamps.
//
// It changes cues in place and returns the indexes of the cues to keep, in file order. It decides about a
// 10 ms cue at the cue after it, so a stream needs to hold at most 2 cues.
func collapseRollingCues(cues []model.Cue) []int {
	kept := make([]int, 0, len(cues))
	var held []int
	active := false
	for i := range cues {
		active = active || cWordTimestampRegexp.MatchString(cues[i].Text())
		if len(held) == 2 && active {
			removeRepeatedLine(&cues[i], cues[held[1]])
			kept = append(kept, held[0])
			held = held[:0]
		}
		if len(held) == 1 && repeatsLastLine(cues[i], cues[held[0]]) {
			held = append(held, i)
			continue
		}
		kept = append(kept, held...)
		held = append(held[:0], i)
	}
	if len(held) == 2 && active {
		held = held[:1]
	}
	return append(kept, held...)
}

func repeatsLastLine(c, previous model.Cue) bool {
	d := c.End - c.Start
	return d >= 0 && d <= maxRepeatDuration &&
		c.Start >= previous.Start &&
		len(previous.Lines) > 0 && len(c.Lines) == 1 &&
		plain(c.Lines[0]) == plain(previous.Lines[len(previous.Lines)-1])
}

func removeRepeatedLine(c *model.Cue, repeat model.Cue) {
	if len(c.Lines) < 2 || plain(c.Lines[0]) != plain(repeat.Lines[0]) {
		return
	}
	c.Lines = c.Lines[1:]
	c.Start = min(c.Start, repeat.Start)
}

// plain returns the visible text of a line with each white space run as 1 space.
func plain(line string) string {
	return whiteSpaceRunRegexp.ReplaceAllString(markupcore.VisibleText(line), " ")
}

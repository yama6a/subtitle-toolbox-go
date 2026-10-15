package vtt

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

// Rolling collapses the rolling cues of YouTube automatic captions, as yt-dlp --write-auto-subs saves
// them. Each cue repeats the line of the cue before it above a new line with <c> word timestamps, and a 10 ms cue
// that repeats the last line follows each cue. The collapse drops the 10 ms cues and the repeated lines, and the
// next cue starts where the dropped 10 ms cue started. It starts only once a cue has <c> word timestamps.
//
// It decides about a 10 ms cue at the cue after it, so it holds at most 2 cues.
type Rolling struct {
	held   []Kept
	active bool
	next   int
}

// Kept is a cue that the collapse keeps and its 0-based index among the cues that went in.
type Kept struct {
	Index int
	Cue   model.Cue
}

// Push adds the next cue of the file and appends the cues that are now final to out, in file order.
func (r *Rolling) Push(c model.Cue, out []Kept) []Kept {
	i := r.next
	r.next++
	r.active = r.active || cWordTimestampRegexp.MatchString(c.Text())
	if len(r.held) == 2 && r.active {
		removeRepeatedLine(&c, r.held[1].Cue)
		out = append(out, r.held[0])
		r.held = r.held[:0]
	}
	if len(r.held) == 1 && repeatsLastLine(c, r.held[0].Cue) {
		r.held = append(r.held, Kept{Index: i, Cue: c})
		return out
	}
	out = append(out, r.held...)
	r.held = append(r.held[:0], Kept{Index: i, Cue: c})
	return out
}

// Flush appends the cues that are left at the end of the file to out. The collapse drops a last 10 ms cue.
func (r *Rolling) Flush(out []Kept) []Kept {
	if len(r.held) == 2 && r.active {
		r.held = r.held[:1]
	}
	out = append(out, r.held...)
	r.held = r.held[:0]
	return out
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

// Package readingspeed measures the visible characters, the words and the reading speed of a cue. The info
// statistics and the validation rules both read these numbers, so they agree on every cue.
package readingspeed

import (
	"math"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/markup"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Measures holds the numbers of one cue.
type Measures struct {
	LineLengths []int         // visible length of each line, as markup.VisibleLength counts it
	Characters  int           // sum of LineLengths
	PlainLines  []string      // markup.PlainText of each line
	Words       []string      // markupcore.Words of PlainLines joined with LF
	Duration    time.Duration // End minus Start
	Seconds     float64       // timecode.FloatSeconds of Duration
}

// Measure returns the numbers of c.
func Measure(c model.Cue) Measures {
	m := Measures{
		LineLengths: make([]int, len(c.Lines)),
		PlainLines:  make([]string, len(c.Lines)),
		Duration:    c.End - c.Start,
	}
	m.Seconds = timecode.FloatSeconds(m.Duration)
	for i, line := range c.Lines {
		m.LineLengths[i] = markup.VisibleLength(line)
		m.Characters += m.LineLengths[i]
		m.PlainLines[i] = markup.PlainText(line)
	}
	m.Words = markupcore.Words(strings.Join(m.PlainLines, "\n"))
	return m
}

// CharactersPerSecond returns Characters divided by Seconds, or +Inf for a duration of 0 or less.
func (m Measures) CharactersPerSecond() float64 {
	return perSecond(float64(m.Characters), m.Seconds)
}

// WordsPerMinute returns the word count divided by Seconds, times 60, or +Inf for a duration of 0 or less.
func (m Measures) WordsPerMinute() float64 {
	return perSecond(float64(len(m.Words)), m.Seconds) * 60
}

func perSecond(n, secs float64) float64 {
	if secs <= 0 {
		return math.Inf(1)
	}
	return n / secs
}

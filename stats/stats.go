// Package stats computes the numbers of a subtitle: cue, word and character counts, reading speed, gaps between
// cues and the most used words. docs/statistics.md defines each number.
package stats

import (
	"math"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/markup"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Statistics holds the numbers of a subtitle at the time of the call.
type Statistics struct {
	// CueCount is the number of cues, also those without text.
	CueCount int
	// WordCount is the number of words of the cues with visible text.
	WordCount int
	// CharacterCount is the sum of the visible lengths of all lines.
	CharacterCount int
	// TotalDisplayTime is the sum of End - Start of every cue. A cue that ends before it starts adds a negative value.
	// The sum stops at the largest and the smallest time.Duration.
	TotalDisplayTime time.Duration
	// Span is the latest end minus the start of the first cue. Nil without cues.
	Span *time.Duration
	// CharactersPerSecond covers each cue with characters and a duration above 0. Nil without such cues.
	CharactersPerSecond *Range
	// WordsPerMinute covers the same cues as CharactersPerSecond.
	WordsPerMinute *Range
	// CharactersPerLine covers each line with a visible length above 0. Nil without such lines.
	CharactersPerLine *Range
	// Gaps covers each cue after the first: its start minus the latest end of the earlier cues. A negative gap is
	// an overlap. Nil with fewer than 2 cues.
	Gaps *DurationRange
	// MostUsedWords holds every counted word, the most used first. Words with the same count keep the order of
	// their first use.
	MostUsedWords []WordCount
}

// Range is the minimum, the arithmetic mean and the maximum of a set of numbers.
type Range struct {
	Min, Average, Max float64
}

// DurationRange is the minimum, the arithmetic mean and the maximum of a set of durations.
type DurationRange struct{ Min, Average, Max time.Duration }

// WordCount is one word in lower case and the number of times it occurs.
type WordCount struct {
	Word  string
	Count int
}

// Of computes the statistics of all cues of s. It does not change s.
func Of(s *model.Subtitle) Statistics {
	var (
		st                            Statistics
		first, latestEnd              time.Duration
		gaps                          durationAccumulator
		perSecond, perMinute, perLine accumulator
		counter                       wordCounter
		lower                         = cases.Lower(language.Und)
	)
	for i, c := range s.All() {
		st.CueCount++
		duration := c.End - c.Start
		st.TotalDisplayTime = addSaturating(st.TotalDisplayTime, duration)
		if i == 0 {
			first, latestEnd = c.Start, c.End
		} else {
			gaps.add(c.Start - latestEnd)
			latestEnd = max(latestEnd, c.End)
		}

		characters := 0
		for _, line := range c.Lines {
			if n := markup.VisibleLength(line); n > 0 {
				perLine.add(float64(n))
				characters += n
			}
		}
		if characters == 0 {
			continue
		}
		words := markupcore.Words(markup.PlainText(c.Text()))
		st.CharacterCount += characters
		st.WordCount += len(words)
		for _, w := range words {
			counter.add(normalize(w, lower))
		}
		if duration > 0 {
			perSecond.add(float64(characters) / duration.Seconds())
			perMinute.add(float64(len(words)) / duration.Seconds() * 60)
		}
	}
	if st.CueCount > 0 {
		span := latestEnd - first
		st.Span = &span
	}
	st.CharactersPerSecond = perSecond.result()
	st.WordsPerMinute = perMinute.result()
	st.CharactersPerLine = perLine.result()
	st.Gaps = gaps.result()
	st.MostUsedWords = counter.sorted()
	return st
}

// accumulator keeps the minimum, the sum and the maximum of the numbers it gets.
type accumulator struct {
	min, max, sum float64
	n             int
}

func (a *accumulator) add(v float64) {
	if a.n == 0 {
		a.min, a.max = v, v
	}
	a.min, a.max = min(a.min, v), max(a.max, v)
	a.sum += v
	a.n++
}

func (a *accumulator) result() *Range {
	if a.n == 0 {
		return nil
	}
	return &Range{Min: a.min, Average: a.sum / float64(a.n), Max: a.max}
}

// durationAccumulator is an accumulator for durations. It sums as float64, because a sum of 60,000 gaps can pass
// what an int64 of nanoseconds holds when the cue times are absurd.
type durationAccumulator struct {
	min, max time.Duration
	sum      float64
	n        int
}

func (a *durationAccumulator) add(d time.Duration) {
	if a.n == 0 {
		a.min, a.max = d, d
	}
	a.min, a.max = min(a.min, d), max(a.max, d)
	a.sum += float64(d)
	a.n++
}

func (a *durationAccumulator) result() *DurationRange {
	if a.n == 0 {
		return nil
	}
	return &DurationRange{Min: a.min, Average: time.Duration(math.Round(a.sum / float64(a.n))), Max: a.max}
}

// addSaturating adds b to a and stops at the limits of time.Duration, because 26 cues of 100000 hours pass them.
func addSaturating(a, b time.Duration) time.Duration {
	switch sum := a + b; {
	case b > 0 && sum < a:
		return math.MaxInt64
	case b < 0 && sum > a:
		return math.MinInt64
	default:
		return sum
	}
}

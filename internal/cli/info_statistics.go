package cli

import (
	"cmp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/stats"
)

func statisticsRows(st stats.Statistics) [][]string {
	words := make([]string, 0, stats.MostUsedWordsInJSON)
	for _, w := range st.MostUsedWords[:min(len(st.MostUsedWords), stats.MostUsedWordsInJSON)] {
		words = append(words, w.Word+" ("+strconv.Itoa(w.Count)+")")
	}
	var gaps string
	if g := st.Gaps; g != nil {
		gaps = rangeText(g.Min.Seconds(), g.Average.Seconds(), g.Max.Seconds()) + " s"
	}
	return [][]string{
		{"Words:", strconv.Itoa(st.WordCount)},
		{"Characters:", strconv.Itoa(st.CharacterCount)},
		{"Display time:", formatNumber(st.TotalDisplayTime.Seconds()) + " s"},
		{"Span:", spanText(st.Span)},
		{"Characters per second:", rangeOrDash(st.CharactersPerSecond)},
		{"Words per minute:", rangeOrDash(st.WordsPerMinute)},
		{"Characters per line:", rangeOrDash(st.CharactersPerLine)},
		{"Gaps:", cmp.Or(gaps, "-")},
		{"Most used words:", strings.Join(words, ", ")},
	}
}

func spanText(span *time.Duration) string {
	if span == nil {
		return "-"
	}
	return formatNumber(span.Seconds()) + " s"
}

func rangeOrDash(r *stats.Range) string {
	if r == nil {
		return "-"
	}
	return rangeText(r.Min, r.Average, r.Max)
}

func rangeText(lo, average, hi float64) string {
	return "min " + formatNumber(lo) + ", average " + formatNumber(average) + ", max " + formatNumber(hi)
}

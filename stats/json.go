package stats

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MostUsedWordsInJSON is the number of words that MarshalJSON writes.
const MostUsedWordsInJSON = 10

type jsonRange struct {
	Min     float64 `json:"min"`
	Average float64 `json:"average"`
	Max     float64 `json:"max"`
}

type jsonWord struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

type jsonStatistics struct {
	CueCount            int        `json:"cueCount"`
	WordCount           int        `json:"wordCount"`
	CharacterCount      int        `json:"characterCount"`
	TotalDisplayTime    float64    `json:"totalDisplayTime"`
	Span                *float64   `json:"span"`
	CharactersPerSecond *jsonRange `json:"charactersPerSecond"`
	WordsPerMinute      *jsonRange `json:"wordsPerMinute"`
	CharactersPerLine   *jsonRange `json:"charactersPerLine"`
	Gaps                *jsonRange `json:"gaps"`
	MostUsedWords       []jsonWord `json:"mostUsedWords"`
}

// MarshalJSON writes the shape of PHP SubtitleStatistics::toArray(): durations as seconds, a nil value as null
// and the first MostUsedWordsInJSON words.
func (st Statistics) MarshalJSON() ([]byte, error) {
	out := jsonStatistics{
		CueCount:            st.CueCount,
		WordCount:           st.WordCount,
		CharacterCount:      st.CharacterCount,
		TotalDisplayTime:    st.TotalDisplayTime.Seconds(),
		CharactersPerSecond: newJSONRange(st.CharactersPerSecond),
		WordsPerMinute:      newJSONRange(st.WordsPerMinute),
		CharactersPerLine:   newJSONRange(st.CharactersPerLine),
		MostUsedWords:       make([]jsonWord, min(len(st.MostUsedWords), MostUsedWordsInJSON)),
	}
	for i := range out.MostUsedWords {
		out.MostUsedWords[i] = jsonWord(st.MostUsedWords[i])
	}
	if st.Span != nil {
		out.Span = ptr(st.Span.Seconds())
	}
	if g := st.Gaps; g != nil {
		out.Gaps = &jsonRange{Min: g.Min.Seconds(), Average: g.Average.Seconds(), Max: g.Max.Seconds()}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("encode the statistics: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func ptr[T any](v T) *T { return &v }

func newJSONRange(r *Range) *jsonRange {
	if r == nil {
		return nil
	}
	return &jsonRange{Min: r.Min, Average: r.Average, Max: r.Max}
}
